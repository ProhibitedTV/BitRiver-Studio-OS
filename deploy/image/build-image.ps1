param(
  [string]$ImageTag = "bitriver/studio-os:dev",
  [string]$ContainerRuntime = "podman",
  [string]$BaseImageDigest = "",
  [string]$MetadataPath = ""
)

Set-StrictMode -Version Latest
$ErrorActionPreference = "Stop"

$repoRoot = Split-Path -Parent (Split-Path -Parent $PSScriptRoot)
$lockPath = Join-Path $PSScriptRoot "releases.lock.json"
$lock = Get-Content -Raw -LiteralPath $lockPath | ConvertFrom-Json

if ($lock.schemaVersion -ne 2 -or $lock.targetPlatform -ne "linux/amd64") {
  throw "Unsupported image release lock schema or target platform."
}
if ($lock.fedoraBootc -ne "quay.io/fedora/fedora-bootc:44") {
  throw "Review the Fedora bootc base before building."
}
foreach ($package in @($lock.mediamtx, $lock.owncast)) {
  if ($package.sha256 -notmatch '^[a-f0-9]{64}$') {
    throw "Invalid SHA256 digest in releases.lock.json."
  }
  $expectedUrl = "$($package.upstream)/releases/download/$($package.version)/$($package.archive)"
  if ($package.url -cne $expectedUrl -or -not $package.url.StartsWith("https://github.com/", [StringComparison]::Ordinal)) {
    throw "Unexpected or unpinned media archive URL."
  }
}

# Release builds must use the exact three digests recorded in the lock.
# A caller-supplied base is accepted only when it matches the lock. Pin
# updates must be reviewed in source control, not injected via an override.
$baseDigest = [string]$lock.builderImages.fedoraBootc
$goBuilderRef = [string]$lock.builderImages.go
$nodeBuilderRef = [string]$lock.builderImages.node
if ($baseDigest -cne "quay.io/fedora/fedora-bootc@sha256:5f2947d685c24029b2eb4e1f0634dc4ac06116a88d9dd93da4b5571b2f0e28e1" -or
    $goBuilderRef -cne "docker.io/library/golang:1.22-bookworm@sha256:3d699e4d15d0f8f13c9195c0632a16702b8cbdece2955af1c23b37ae5d55a253" -or
    $nodeBuilderRef -cne "docker.io/library/node:22-bookworm@sha256:e13726ca604184d7a91fcacab1fc09eefd642dc28e79ac34e85540dd93b78cfb") {
  throw "Build image digest lock has drifted: review and update the pinned references."
}
if (-not [string]::IsNullOrWhiteSpace($BaseImageDigest) -and $BaseImageDigest -cne $baseDigest) {
  throw "BaseImageDigest must match releases.lock.json. Edit the lock to upgrade."
}
& $ContainerRuntime pull $baseDigest
if ($LASTEXITCODE -ne 0) { throw "Failed to pull pinned Fedora bootc base." }
Write-Host "Pinned Fedora base: $baseDigest"
Write-Host "Pinned Go builder: $goBuilderRef"
Write-Host "Pinned Node builder: $nodeBuilderRef"

$sourceRevision = (& git -C $repoRoot rev-parse HEAD | Select-Object -First 1)
if ($LASTEXITCODE -ne 0 -or [string]::IsNullOrWhiteSpace($sourceRevision)) {
  throw "Unable to identify the source revision."
}
$sourceRevision = ([string]$sourceRevision).Trim()
if ($sourceRevision -notmatch '^[a-f0-9]{40}$') {
  throw "Invalid source revision."
}

$buildArgs = @(
  "build",
  "--file", (Join-Path $PSScriptRoot "Containerfile"),
  "--platform", "linux/amd64",
  "--build-arg", "FEDORA_BOOTC_IMAGE=$baseDigest",
  "--build-arg", "GO_BUILDER_IMAGE=$goBuilderRef",
  "--build-arg", "NODE_BUILDER_IMAGE=$nodeBuilderRef",
  "--build-arg", "BITRIVER_SOURCE_REV=$sourceRevision",
  "--build-arg", "MEDIAMTX_TARBALL_URL=$($lock.mediamtx.url)",
  "--build-arg", "MEDIAMTX_SHA256=$($lock.mediamtx.sha256)",
  "--build-arg", "OWNCAST_ZIP_URL=$($lock.owncast.url)",
  "--build-arg", "OWNCAST_SHA256=$($lock.owncast.sha256)",
  "--tag", $ImageTag,
  $repoRoot
)
& $ContainerRuntime @buildArgs
if ($LASTEXITCODE -ne 0) { throw "BitRiver bootc image build failed." }

if (-not [string]::IsNullOrWhiteSpace($MetadataPath)) {
  $imageId = (& $ContainerRuntime image inspect $ImageTag --format "{{.Id}}" | Select-Object -First 1)
  if ($LASTEXITCODE -ne 0 -or [string]::IsNullOrWhiteSpace($imageId)) { throw "Cannot inspect the image." }
  $record = [ordered]@{
    imageTag = $ImageTag
    sourceRevision = $sourceRevision
    platform = $lock.targetPlatform
    baseRef = $baseDigest
    goBuilderRef = $goBuilderRef
    nodeBuilderRef = $nodeBuilderRef
    localImageId = ([string]$imageId).Trim()
    mediamtxVersion = $lock.mediamtx.version
    mediamtxArchiveSha256 = $lock.mediamtx.sha256
    owncastVersion = $lock.owncast.version
    owncastArchiveSha256 = $lock.owncast.sha256
    releaseReady = $false
    note = "Build provenance only; no UEFI boot, rollback or hardware verification."
  }
  $metadataDir = Split-Path -Parent $MetadataPath
  if ($metadataDir) { New-Item -ItemType Directory -Force -Path $metadataDir | Out-Null }
  $record | ConvertTo-Json -Depth 5 | Set-Content -Path $MetadataPath -Encoding utf8
}
