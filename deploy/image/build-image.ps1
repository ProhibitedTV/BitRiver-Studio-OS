param(
  [string]$ImageTag = "ghcr.io/bitriver/studio-os:dev",
  [string]$ContainerRuntime = "podman",
  [string]$MediaMtxTarballUrl = $env:MEDIAMTX_TARBALL_URL,
  [string]$OwncastTarballUrl = $env:OWNCAST_TARBALL_URL
)

Set-StrictMode -Version Latest
$ErrorActionPreference = "Stop"

if ([string]::IsNullOrWhiteSpace($MediaMtxTarballUrl)) {
  throw "MEDIAMTX_TARBALL_URL is required for image packaging."
}

if ([string]::IsNullOrWhiteSpace($OwncastTarballUrl)) {
  throw "OWNCAST_TARBALL_URL is required for image packaging."
}

$repoRoot = Split-Path -Parent (Split-Path -Parent $PSScriptRoot)

& $ContainerRuntime build `
  --file (Join-Path $PSScriptRoot "Containerfile") `
  --build-arg "MEDIAMTX_TARBALL_URL=$MediaMtxTarballUrl" `
  --build-arg "OWNCAST_TARBALL_URL=$OwncastTarballUrl" `
  --tag $ImageTag `
  $repoRoot
