[CmdletBinding()]
param(
    [Parameter(Mandatory = $true)]
    [ValidatePattern('^(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)$')]
    [string]$Version,

    [Parameter(Mandatory = $true)]
    [string]$ArtifactsDirectory
)

$ErrorActionPreference = 'Stop'
Set-StrictMode -Version Latest

if (-not $IsWindows) {
    throw 'Windows installer lifecycle tests must run on Windows.'
}

$resolvedArtifactsDirectory = (Resolve-Path -LiteralPath $ArtifactsDirectory).Path

function Invoke-ProcessWithTimeout {
    param(
        [Parameter(Mandatory = $true)]
        [string]$FilePath,

        [string[]]$ArgumentList = @(),

        [int]$TimeoutSeconds = 120
    )

    $process = Start-Process `
        -FilePath $FilePath `
        -ArgumentList $ArgumentList `
        -PassThru

    if (-not $process.WaitForExit($TimeoutSeconds * 1000)) {
        Stop-Process -Id $process.Id -Force -ErrorAction SilentlyContinue
        throw "Process timed out after $TimeoutSeconds seconds: $FilePath"
    }

    return $process.ExitCode
}

function Assert-InstalledState {
    param(
        [Parameter(Mandatory = $true)]
        [string]$ExecutablePath,

        [Parameter(Mandatory = $true)]
        [string]$RegistryPath,

        [Parameter(Mandatory = $true)]
        [string]$ExpectedDisplayName
    )

    if (-not (Test-Path -LiteralPath $ExecutablePath -PathType Leaf)) {
        throw "Installed executable is missing: $ExecutablePath"
    }

    if (-not (Test-Path -LiteralPath $RegistryPath)) {
        throw "Uninstall registry entry is missing: $RegistryPath"
    }

    $registryEntry = Get-ItemProperty -LiteralPath $RegistryPath
    if ($registryEntry.DisplayName -ne $ExpectedDisplayName) {
        throw "Unexpected installed display name: $($registryEntry.DisplayName)"
    }
    if ($registryEntry.DisplayVersion -ne $Version) {
        throw "Unexpected installed version: $($registryEntry.DisplayVersion)"
    }
}

function Test-InstallerChannel {
    param(
        [Parameter(Mandatory = $true)]
        [string]$InstallerName,

        [Parameter(Mandatory = $true)]
        [string]$ProductCode,

        [Parameter(Mandatory = $true)]
        [string]$ProductName
    )

    $installerPath = Join-Path $resolvedArtifactsDirectory $InstallerName
    $installDirectory = Join-Path `
        ([Environment]::GetFolderPath([Environment+SpecialFolder]::ProgramFiles)) `
        "Eric Wooley\$ProductName"
    $executablePath = Join-Path $installDirectory 'ssh-man.exe'
    $stagedExecutablePath = "$executablePath.new"
    $registryPath = "HKLM:\SOFTWARE\Microsoft\Windows\CurrentVersion\Uninstall\$ProductCode"

    if (-not (Test-Path -LiteralPath $installerPath -PathType Leaf)) {
        throw "Installer artifact is missing: $installerPath"
    }

    Write-Host "Installing $ProductName"
    $installExitCode = Invoke-ProcessWithTimeout `
        -FilePath $installerPath `
        -ArgumentList @('/S')
    if ($installExitCode -ne 0) {
        throw "Initial installation exited with code $installExitCode."
    }
    Assert-InstalledState `
        -ExecutablePath $executablePath `
        -RegistryPath $registryPath `
        -ExpectedDisplayName $ProductName

    $beforeUpgradeHash = (Get-FileHash -Algorithm SHA256 -LiteralPath $executablePath).Hash
    $applicationProcess = Start-Process -FilePath $executablePath -PassThru
    $binaryLock = $null

    try {
        Start-Sleep -Seconds 5
        if ($applicationProcess.HasExited) {
            throw "$ProductName exited before the running-application upgrade test."
        }

        Set-ItemProperty -LiteralPath $registryPath -Name 'DisplayVersion' -Value '0.0.0-test-locked'
        $binaryLock = [System.IO.File]::Open(
            $executablePath,
            [System.IO.FileMode]::Open,
            [System.IO.FileAccess]::Read,
            [System.IO.FileShare]::Read
        )

        Write-Host "Requiring a safe silent-upgrade failure while $ProductName is locked"
        $upgradeExitCode = Invoke-ProcessWithTimeout `
            -FilePath $installerPath `
            -ArgumentList @('/S')

        if ($upgradeExitCode -ne 1) {
            throw "Locked-binary upgrade exited with code $upgradeExitCode instead of 1."
        }

        $afterUpgradeHash = (Get-FileHash -Algorithm SHA256 -LiteralPath $executablePath).Hash
        if ($afterUpgradeHash -ne $beforeUpgradeHash) {
            throw 'Silent upgrade changed the installed executable after reporting failure.'
        }

        $lockedRegistryEntry = Get-ItemProperty -LiteralPath $registryPath
        if ($lockedRegistryEntry.DisplayVersion -ne '0.0.0-test-locked') {
            throw 'Silent upgrade changed uninstall metadata after reporting failure.'
        }

        if (Test-Path -LiteralPath $stagedExecutablePath) {
            throw "Failed silent upgrade left a staged executable: $stagedExecutablePath"
        }
    }
    finally {
        if ($null -ne $binaryLock) {
            $binaryLock.Dispose()
        }
        if (-not $applicationProcess.HasExited) {
            Stop-Process -Id $applicationProcess.Id -Force
            $applicationProcess.WaitForExit()
        }
    }

    Write-Host "Retrying silent upgrade after closing $ProductName"
    $retryExitCode = Invoke-ProcessWithTimeout `
        -FilePath $installerPath `
        -ArgumentList @('/S')
    if ($retryExitCode -ne 0) {
        throw "Silent upgrade retry exited with code $retryExitCode."
    }

    Assert-InstalledState `
        -ExecutablePath $executablePath `
        -RegistryPath $registryPath `
        -ExpectedDisplayName $ProductName

    $uninstallerPath = Join-Path $installDirectory 'uninstall.exe'
    $temporaryUninstallerName = "ssh-man-uninstall-$([Guid]::NewGuid().ToString('N')).exe"
    $temporaryUninstallerPath = Join-Path `
        ([System.IO.Path]::GetTempPath()) `
        $temporaryUninstallerName
    Write-Host "Uninstalling $ProductName"
    Copy-Item -LiteralPath $uninstallerPath -Destination $temporaryUninstallerPath
    try {
        $uninstallExitCode = Invoke-ProcessWithTimeout `
            -FilePath $temporaryUninstallerPath `
            -ArgumentList @('/S', "_?=$installDirectory")
    }
    finally {
        Remove-Item -LiteralPath $temporaryUninstallerPath -Force -ErrorAction SilentlyContinue
    }
    if ($uninstallExitCode -ne 0) {
        throw "Uninstallation exited with code $uninstallExitCode."
    }
    if (Test-Path -LiteralPath $executablePath) {
        throw "Uninstallation left the application executable: $executablePath"
    }
    if (Test-Path -LiteralPath $registryPath) {
        throw "Uninstallation left the registry entry: $registryPath"
    }
}

Test-InstallerChannel `
    -InstallerName 'ssh-man-windows-amd64-installer.exe' `
    -ProductCode 'tech.moonpixels.ssh-man' `
    -ProductName 'SSH Man'

Test-InstallerChannel `
    -InstallerName 'ssh-man-experimental-windows-amd64-installer.exe' `
    -ProductCode 'tech.moonpixels.ssh-man.experimental' `
    -ProductName 'SSH Man Experimental'

Write-Host 'Installer lifecycle tests passed.'
