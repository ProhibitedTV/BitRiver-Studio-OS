. (Join-Path $PSScriptRoot "common.ps1")

$repoRoot = Get-RepoRoot
$buildDir = Join-Path $repoRoot "build"
Ensure-Directory $buildDir
Initialize-GoEnv

Invoke-Step "Generate OpenAPI" {
  & (Join-Path $PSScriptRoot "generate-openapi.ps1")
}

Invoke-Step "Build shared contracts" {
  Set-Location $repoRoot
  & npm run build --workspace @bitriver/shared
}

Invoke-Step "Build control UI" {
  Set-Location $repoRoot
  & npm run build --workspace @bitriver/control-ui
}

Invoke-Step "Build control API" {
  Set-Location (Join-Path $repoRoot "apps/control-api")
  & go build -trimpath -o (Join-Path $buildDir "bitriverd") ./cmd/bitriverd
}
