. (Join-Path $PSScriptRoot "common.ps1")

$repoRoot = Get-RepoRoot
$buildDir = Join-Path $repoRoot "build"
$logDir = Join-Path $buildDir "logs"
Ensure-Directory $buildDir
Ensure-Directory $logDir
Initialize-GoEnv

$apiProcess = $null

try {
  Invoke-Step "Start control API" {
    $apiProcess = Start-Job -ScriptBlock {
      param($ApiWorkdir, $GoCache, $GoTmpDir)

      $env:GOCACHE = $GoCache
      $env:GOTMPDIR = $GoTmpDir
      Set-Location $ApiWorkdir
      go run ./cmd/bitriverd -http-addr 127.0.0.1:8080
    } -ArgumentList (Join-Path $repoRoot "apps/control-api"), $env:GOCACHE, $env:GOTMPDIR

    Wait-ForHttpOk -Url "http://127.0.0.1:8080/healthz"
  }

  Invoke-Step "Smoke checks" {
    & (Join-Path $repoRoot "test/smoke/control-api.ps1") -BaseUrl "http://127.0.0.1:8080"
  }

  Invoke-Step "Integration checks" {
    & (Join-Path $repoRoot "test/integration/config-lifecycle.ps1") -BaseUrl "http://127.0.0.1:8080"
  }
} finally {
  if ($null -ne $apiProcess) {
    Stop-Job -Job $apiProcess -ErrorAction SilentlyContinue
    Remove-Job -Job $apiProcess -Force -ErrorAction SilentlyContinue
  }
}
