[CmdletBinding()]
param(
    [Parameter(Mandatory = $true)]
    [ValidatePattern('^(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)$')]
    [string]$Version,

    [Parameter(Mandatory = $true)]
    [string]$InstallerPath
)

$ErrorActionPreference = 'Stop'
Set-StrictMode -Version Latest

if (-not $IsWindows) {
    throw 'Windows installer lifecycle tests must run on Windows.'
}

$resolvedInstaller = (Resolve-Path -LiteralPath $InstallerPath).Path
$installDirectory = Join-Path `
    ([Environment]::GetFolderPath([Environment+SpecialFolder]::ProgramFiles)) `
    'Eric Wooley\SSH Man'
$executablePath = Join-Path $installDirectory 'ssh-man.exe'
$stagedExecutablePath = "$executablePath.new"
$registryPath = 'HKLM:\SOFTWARE\Microsoft\Windows\CurrentVersion\Uninstall\tech.moonpixels.ssh-man'

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

    try {
        if (-not $process.WaitForExit($TimeoutSeconds * 1000)) {
            Stop-Process -Id $process.Id -Force -ErrorAction SilentlyContinue
            throw "Process timed out after $TimeoutSeconds seconds: $FilePath"
        }

        return $process.ExitCode
    }
    finally {
        $process.Dispose()
    }
}

function Assert-InstalledState {
    if (-not (Test-Path -LiteralPath $executablePath -PathType Leaf)) {
        throw "Installed executable is missing: $executablePath"
    }

    if (-not (Test-Path -LiteralPath $registryPath)) {
        throw "Uninstall registry entry is missing: $registryPath"
    }

    $registryEntry = Get-ItemProperty -LiteralPath $registryPath
    if ($registryEntry.DisplayName -ne 'SSH Man') {
        throw "Unexpected installed display name: $($registryEntry.DisplayName)"
    }
    if ($registryEntry.DisplayVersion -ne $Version) {
        throw "Unexpected installed version: $($registryEntry.DisplayVersion)"
    }
}

Write-Host 'Installing SSH Man'
$installExitCode = Invoke-ProcessWithTimeout `
    -FilePath $resolvedInstaller `
    -ArgumentList @('/S')
if ($installExitCode -ne 0) {
    throw "Initial installation exited with code $installExitCode."
}
Assert-InstalledState

$beforeUpgradeHash = (Get-FileHash -Algorithm SHA256 -LiteralPath $executablePath).Hash
$applicationProcess = Start-Process -FilePath $executablePath -PassThru
$binaryLock = $null

try {
    Start-Sleep -Seconds 5
    if ($applicationProcess.HasExited) {
        throw 'SSH Man exited before the running application upgrade test.'
    }

    Set-ItemProperty -LiteralPath $registryPath -Name 'DisplayVersion' -Value '0.0.0-test-locked'
    $binaryLock = [System.IO.File]::Open(
        $executablePath,
        [System.IO.FileMode]::Open,
        [System.IO.FileAccess]::Read,
        [System.IO.FileShare]::Read
    )

    Write-Host 'Requiring a safe silent upgrade failure while SSH Man is locked'
    $upgradeExitCode = Invoke-ProcessWithTimeout `
        -FilePath $resolvedInstaller `
        -ArgumentList @('/S')

    if ($upgradeExitCode -ne 1) {
        throw "Locked binary upgrade exited with code $upgradeExitCode instead of 1."
    }

    $afterUpgradeHash = (Get-FileHash -Algorithm SHA256 -LiteralPath $executablePath).Hash
    if ($afterUpgradeHash -ne $beforeUpgradeHash) {
        throw 'Silent upgrade changed the installed executable after a failure.'
    }

    $lockedRegistryEntry = Get-ItemProperty -LiteralPath $registryPath
    if ($lockedRegistryEntry.DisplayVersion -ne '0.0.0-test-locked') {
        throw 'Silent upgrade changed uninstall data after a failure.'
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
    $applicationProcess.Dispose()
}

Write-Host 'Retrying silent upgrade after closing SSH Man'
$retryExitCode = Invoke-ProcessWithTimeout `
    -FilePath $resolvedInstaller `
    -ArgumentList @('/S')
if ($retryExitCode -ne 0) {
    throw "Silent upgrade retry exited with code $retryExitCode."
}
Assert-InstalledState

$uninstallerPath = Join-Path $installDirectory 'uninstall.exe'
$temporaryUninstallerPath = Join-Path `
    ([System.IO.Path]::GetTempPath()) `
    "ssh-man-uninstall-$([Guid]::NewGuid().ToString('N')).exe"

Write-Host 'Uninstalling SSH Man'
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

Write-Host 'Windows installer lifecycle tests passed.'
