. (Join-Path $PSScriptRoot "common.ps1")

$repoRoot = Get-RepoRoot
Initialize-GoEnv

Invoke-Step "Generate OpenAPI spec" {
  Set-Location (Join-Path $repoRoot "apps/control-api")
  & go run ./cmd/openapi-gen -out (Join-Path $repoRoot "packages/shared/openapi/bitriver-control-api.yaml")
}
