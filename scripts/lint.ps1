. (Join-Path $PSScriptRoot "common.ps1")

$repoRoot = Get-RepoRoot
Initialize-GoEnv

Invoke-Step "Go test" {
  Set-Location (Join-Path $repoRoot "apps/control-api")
  & go test ./...
}

Invoke-Step "Go vet" {
  Set-Location (Join-Path $repoRoot "apps/control-api")
  & go vet ./...
}

Invoke-Step "Shared contract lint" {
  Set-Location $repoRoot
  & npm run lint --workspace @bitriver/shared
}

Invoke-Step "Control UI lint" {
  Set-Location $repoRoot
  & npm run lint --workspace @bitriver/control-ui
}
