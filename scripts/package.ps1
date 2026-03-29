param(
  [string]$ImageTag = "ghcr.io/bitriver/studio-os:dev",
  [string]$ContainerRuntime = "podman"
)

. (Join-Path $PSScriptRoot "common.ps1")

$repoRoot = Get-RepoRoot

Invoke-Step "Build repo artifacts" {
  & (Join-Path $PSScriptRoot "build.ps1")
}

Invoke-Step "Build appliance image" {
  & (Join-Path $repoRoot "deploy/image/build-image.ps1") -ImageTag $ImageTag -ContainerRuntime $ContainerRuntime
}
