$wailsArguments = @($args)
$ErrorActionPreference = 'Stop'

. (Join-Path $PSScriptRoot 'windows-common.ps1')

Assert-WindowsHost
$rootDir = Get-SshManRoot
$wailsVersion = 'v2.13.0'
$previousCgoEnabled = [Environment]::GetEnvironmentVariable('CGO_ENABLED', 'Process')

Push-Location $rootDir
try {
    Assert-SshManCommand -Name 'go' -InstallHint 'Install Go 1.26.5 and restart PowerShell so go.exe is on PATH.' | Out-Null
    Assert-SshManCommand -Name 'node' -InstallHint 'Install a supported Node.js release and restart PowerShell so node.exe is on PATH.' | Out-Null
    Assert-MinGWCompiler | Out-Null

    $env:CGO_ENABLED = '1'

    Write-Host '==> Installing frontend dependencies'
    Invoke-SshManPnpm -ArgumentList @('install', '--frozen-lockfile')

    Write-Host '==> Starting Wails dev on Windows'
    $commandArguments = @(
        'run',
        "github.com/wailsapp/wails/v2/cmd/wails@$wailsVersion",
        'dev'
    ) + $wailsArguments
    Invoke-SshManCommand -FilePath 'go' -ArgumentList $commandArguments
}
finally {
    if ($null -eq $previousCgoEnabled) {
        Remove-Item -LiteralPath 'Env:\CGO_ENABLED' -ErrorAction SilentlyContinue
    }
    else {
        $env:CGO_ENABLED = $previousCgoEnabled
    }

    Pop-Location
}
