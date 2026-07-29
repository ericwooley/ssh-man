Set-StrictMode -Version Latest

function Get-SshManRoot {
    return (Resolve-Path -LiteralPath (Join-Path $PSScriptRoot '..')).Path
}

function Assert-WindowsHost {
    if ([Environment]::OSVersion.Platform -ne [PlatformID]::Win32NT) {
        throw 'This PowerShell helper is intended for Windows. Use the matching .sh script on macOS or Linux.'
    }
}

function Assert-SshManCommand {
    param(
        [Parameter(Mandatory = $true)]
        [string] $Name,

        [Parameter(Mandatory = $true)]
        [string] $InstallHint
    )

    $command = Get-Command -Name $Name -ErrorAction SilentlyContinue
    if ($null -eq $command) {
        throw "Missing required command '$Name'. $InstallHint"
    }

    return $command
}

function Assert-MinGWCompiler {
    $gcc = Assert-SshManCommand -Name 'gcc' -InstallHint 'Install the MSYS2 UCRT64 MinGW-w64 GCC package and add C:\msys64\ucrt64\bin to PATH.'
    $target = (& $gcc.Source -dumpmachine 2>&1 | Out-String).Trim()

    if ($LASTEXITCODE -ne 0) {
        throw "Unable to inspect the GCC target using '$($gcc.Source) -dumpmachine'."
    }

    if ($target -notmatch 'mingw') {
        throw "GCC must target Windows for CGO, but '$($gcc.Source)' targets '$target'. Put the MinGW-w64 bin directory (for example C:\msys64\ucrt64\bin) before other GCC installations on PATH."
    }

    $goArchitecture = (& go env GOARCH 2>&1 | Out-String).Trim()
    if ($LASTEXITCODE -ne 0) {
        throw 'Unable to determine the Go target architecture using go env GOARCH.'
    }
    $expectedTarget = switch ($goArchitecture) {
        'amd64' { '^x86_64-.*mingw'; break }
        '386' { '^i[3-6]86-.*mingw'; break }
        'arm64' { '^(aarch64|arm64)-.*mingw'; break }
        default { throw "Unsupported Windows Go architecture '$goArchitecture' for the CGO build helper." }
    }
    if ($target -notmatch $expectedTarget) {
        throw "GCC target '$target' does not match Go architecture '$goArchitecture'. Put the matching MinGW-w64 bin directory before other compilers on PATH."
    }

    return $gcc
}

function Invoke-SshManCommand {
    param(
        [Parameter(Mandatory = $true)]
        [string] $FilePath,

        [string[]] $ArgumentList = @()
    )

    & $FilePath @ArgumentList
    if ($LASTEXITCODE -ne 0) {
        $renderedArguments = $ArgumentList -join ' '
        throw "Command failed with exit code ${LASTEXITCODE}: $FilePath $renderedArguments"
    }
}

function Invoke-SshManPnpm {
    param(
        [string[]] $ArgumentList = @()
    )

    $rootDir = Get-SshManRoot
    $launcher = Join-Path $rootDir 'scripts\pnpm.cjs'
    Invoke-SshManCommand -FilePath 'node' -ArgumentList (@($launcher) + $ArgumentList)
}
