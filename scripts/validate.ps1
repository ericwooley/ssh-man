$ErrorActionPreference = 'Stop'

. (Join-Path $PSScriptRoot 'windows-common.ps1')

Assert-WindowsHost
$rootDir = Get-SshManRoot
$previousCgoEnabled = [Environment]::GetEnvironmentVariable('CGO_ENABLED', 'Process')

Push-Location $rootDir
try {
    Assert-SshManCommand -Name 'go' -InstallHint 'Install Go 1.26.5 and restart PowerShell so go.exe is on PATH.' | Out-Null
    Assert-SshManCommand -Name 'gofmt' -InstallHint 'Install Go 1.26.5 and restart PowerShell so gofmt.exe is on PATH.' | Out-Null
    Assert-SshManCommand -Name 'node' -InstallHint 'Install a supported Node.js release and restart PowerShell so node.exe is on PATH.' | Out-Null
    Assert-MinGWCompiler | Out-Null

    $env:CGO_ENABLED = '1'

    Write-Host '==> Checking Go formatting'
    $goFiles = @(
        Get-Item -LiteralPath (Join-Path $rootDir 'main.go')
        Get-ChildItem -LiteralPath (Join-Path $rootDir 'cmd\app') -Filter '*.go' -File -Recurse
        Get-ChildItem -LiteralPath (Join-Path $rootDir 'internal') -Filter '*.go' -File -Recurse
        Get-ChildItem -LiteralPath (Join-Path $rootDir 'tests') -Filter '*.go' -File -Recurse
    )
    $unformattedGoFiles = @()
    foreach ($goFile in $goFiles) {
        # `gofmt -d` stays read-only and does not report CRLF-only differences,
        # so it is safe for a normal Windows Git checkout.
        $formatDiff = (& gofmt -d $goFile.FullName 2>&1 | Out-String)
        if ($LASTEXITCODE -ne 0) {
            throw "gofmt failed for '$($goFile.FullName)': $formatDiff"
        }
        if (-not [string]::IsNullOrWhiteSpace($formatDiff)) {
            $unformattedGoFiles += $goFile.FullName
        }
    }
    if ($unformattedGoFiles.Count -gt 0) {
        throw "Go files need formatting:`n$($unformattedGoFiles -join "`n")"
    }

    Write-Host '==> Installing frontend dependencies'
    Invoke-SshManPnpm -ArgumentList @('install', '--frozen-lockfile')

    Write-Host '==> Building frontend'
    Invoke-SshManPnpm -ArgumentList @('run', 'build')

    Write-Host '==> Running Go vet'
    Invoke-SshManCommand -FilePath 'go' -ArgumentList @('vet', './...')

    Write-Host '==> Running Go tests'
    Invoke-SshManCommand -FilePath 'go' -ArgumentList @('test', './...')

    Write-Host '==> Running frontend tests'
    Invoke-SshManPnpm -ArgumentList @('run', 'test')

    Write-Host '==> Windows validation complete'
    Write-Host '    Release-script tests remain in scripts/validate.sh because they require Bash and Unix release tools.'
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
