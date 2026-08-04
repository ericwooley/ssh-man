[CmdletBinding()]
param(
    [Parameter(Mandatory = $true)]
    [ValidatePattern('^(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)$')]
    [string]$Version,

    [Parameter(Mandatory = $true)]
    [string]$ExecutablePath,

    [ValidateRange(1, 120)]
    [int]$TimeoutSeconds = 30,

    [ValidateRange(1, 60)]
    [int]$DesktopStartupSeconds = 10
)

$ErrorActionPreference = 'Stop'
Set-StrictMode -Version Latest

if (-not $IsWindows) {
    throw 'Windows release artifact validation must run on Windows.'
}

$resolvedExecutable = (Resolve-Path -LiteralPath $ExecutablePath).Path
$stream = [System.IO.File]::OpenRead($resolvedExecutable)
try {
    $firstByte = $stream.ReadByte()
    $secondByte = $stream.ReadByte()
}
finally {
    $stream.Dispose()
}

if ($firstByte -ne 0x4d -or $secondByte -ne 0x5a) {
    throw "Release artifact is not a Windows PE executable: $resolvedExecutable"
}

$versionInfo = [System.Diagnostics.FileVersionInfo]::GetVersionInfo($resolvedExecutable)
$acceptedVersions = @($Version, "$Version.0")
$fixedFileVersion = @(
    $versionInfo.FileMajorPart
    $versionInfo.FileMinorPart
    $versionInfo.FileBuildPart
    $versionInfo.FilePrivatePart
) -join '.'
if ($fixedFileVersion -ne "$Version.0") {
    throw "Fixed file version is '$fixedFileVersion', expected '$Version.0'."
}

$fixedProductVersion = @(
    $versionInfo.ProductMajorPart
    $versionInfo.ProductMinorPart
    $versionInfo.ProductBuildPart
    $versionInfo.ProductPrivatePart
) -join '.'
if ($fixedProductVersion -ne "$Version.0") {
    throw "Fixed product version is '$fixedProductVersion', expected '$Version.0'."
}

if ($versionInfo.ProductVersion -notin $acceptedVersions) {
    throw "ProductVersion is '$($versionInfo.ProductVersion)', expected '$Version'."
}

$process = Start-Process `
    -FilePath $resolvedExecutable `
    -ArgumentList @('--help') `
    -PassThru

try {
    if (-not $process.WaitForExit($TimeoutSeconds * 1000)) {
        throw "Windows executable did not finish its --help smoke check within $TimeoutSeconds seconds."
    }
    if ($process.ExitCode -ne 0) {
        throw "Windows executable --help smoke check exited with code $($process.ExitCode)."
    }
}
finally {
    if (-not $process.HasExited) {
        Stop-Process -Id $process.Id -Force -ErrorAction SilentlyContinue
    }
    $process.Dispose()
}

$desktopProcess = Start-Process `
    -FilePath $resolvedExecutable `
    -PassThru

try {
    if ($desktopProcess.WaitForExit($DesktopStartupSeconds * 1000)) {
        throw "Windows desktop application exited during its startup smoke check with code $($desktopProcess.ExitCode)."
    }
}
finally {
    if (-not $desktopProcess.HasExited) {
        Stop-Process -Id $desktopProcess.Id -Force -ErrorAction SilentlyContinue
        $null = $desktopProcess.WaitForExit(5000)
    }
    $desktopProcess.Dispose()
}

Write-Host 'Windows release artifact validation passed.'
