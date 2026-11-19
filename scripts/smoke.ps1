. (Join-Path $PSScriptRoot "common.ps1")

$repoRoot = Get-RepoRoot
$buildDir = Join-Path $repoRoot "build"
$logDir = Join-Path $buildDir "logs"
$goExecutable = (Get-Command go).Source
$currentShell = (Get-Process -Id $PID).Path
Ensure-Directory $buildDir
Ensure-Directory $logDir
Initialize-GoEnv

$apiProcess = $null
$goCommand = @"
`$env:GOCACHE = '$($env:GOCACHE.Replace("'", "''"))'
`$env:GOTMPDIR = '$($env:GOTMPDIR.Replace("'", "''"))'
Set-Location '$((Join-Path $repoRoot "apps/control-api").Replace("'", "''"))'
& '$($goExecutable.Replace("'", "''"))' run ./cmd/bitriverd -http-addr 127.0.0.1:8080
"@

try {
  Invoke-Step "Start control API" {
    $apiProcess = Start-Process `
      -FilePath $currentShell `
      -ArgumentList @("-NoProfile", "-Command", $goCommand) `
      -PassThru

    Wait-ForHttpOk -Url "http://127.0.0.1:8080/healthz" -TimeoutSeconds 45
  }

  Invoke-Step "Smoke checks" {
    & (Join-Path $repoRoot "test/smoke/control-api.ps1") -BaseUrl "http://127.0.0.1:8080"
  }

  Invoke-Step "Integration checks" {
    & (Join-Path $repoRoot "test/integration/config-lifecycle.ps1") -BaseUrl "http://127.0.0.1:8080"
  }
} finally {
  if ($null -ne $apiProcess) {
    Stop-Process -Id $apiProcess.Id -Force -ErrorAction SilentlyContinue
  }
}
