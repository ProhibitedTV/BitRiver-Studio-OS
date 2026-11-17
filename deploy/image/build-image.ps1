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

if ($lock.schemaVersion -ne 1 -or $lock.targetPlatform -ne "linux/amd64") {
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

# Tags can change; resolve to a digest before building. Pass a recorded
# -BaseImageDigest to reproduce an earlier base reference.
if ([string]::IsNullOrWhiteSpace($BaseImageDigest)) {
  & $ContainerRuntime pull $lock.fedoraBootc
  if ($LASTEXITCODE -ne 0) { throw "Failed to pull Fedora bootc base." }
  $baseDigest = (& $ContainerRuntime image inspect $lock.fedoraBootc --format "{{index .RepoDigests 0}}" | Select-Object -First 1)
  if ($LASTEXITCODE -ne 0) { throw "Could not inspect Fedora base digest." }
  $baseDigest = [string]$baseDigest
  $baseDigest = $baseDigest.Trim()
} else {
  $baseDigest = $BaseImageDigest
}
if ($baseDigest -notmatch '^quay\.io/fedora/fedora-bootc@sha256:[0-9a-f]{64}$') {
  throw "An immutable Fedora bootc manifest digest is required."
}
Write-Host "Immutable Fedora base: $baseDigest"

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
