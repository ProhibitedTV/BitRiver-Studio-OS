param(
  [ValidateSet("api", "ui")]
  [string]$Target = "api"
)

. (Join-Path $PSScriptRoot "common.ps1")

$repoRoot = Get-RepoRoot
Initialize-GoEnv

switch ($Target) {
  "api" {
    Set-Location (Join-Path $repoRoot "apps/control-api")
    & go run ./cmd/bitriverd -http-addr 127.0.0.1:8080
  }
  "ui" {
    Set-Location $repoRoot
    & npm run dev --workspace @bitriver/control-ui
  }
}
