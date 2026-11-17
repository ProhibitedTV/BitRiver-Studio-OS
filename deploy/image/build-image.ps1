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
  throw "Review changes to the Fedora bootc base before building."
}
foreach ($package in @($lock.mediamtx, $lock.owncast)) {
  if ($package.sha256 -notmatch '^[0-9a-f]{64}$') {
    throw "Invalid SHA256 digest in releases.lock.json."
  }
  $uri = [Uri]$package.url
  if ($uri.Scheme -ne "https" -or $uri.Host -ne "github.com" -or
      $uri.AbsolutePath -notlike "/$($package.upstream.Replace('https://github.com/', ''))/releases/download/$($package.version)/*" -or
      -not $package.url.EndsWith("/$($package.archive)", [StringComparison]::Ordinal)) {
    throw "Unexpected or unpinned media archive URL."
  }
}

# A tag alone is mutable. Resolve it to a digest once, or supply the
# previously recorded digest to reproduce an earlier build.
if ([string]::IsNullOrWhiteSpace($BaseImageDigest)) {
  & $ContainerRuntime pull $lock.fedoraBootc
  if ($LASTEXITCODE -ne 0) { throw "Failed to pull Fedora bootc base." }
  $baseDigest = (& $ContainerRuntime image inspect $lock.fedoraBootc --format "{{index .RepoDigests 0}}" | Select-Object -First 1).Trim()
  if ($LASTEXITCODE -ne 0) { throw "Cannot inspect Fedora bootc base digest." }
} else {
  $baseDigest = $BaseImageDigest
}
if ($baseDigest -notmatch '^quay\.io/fedora/fedora-bootc@sha256:[0-9a-f]{64}

& $ContainerRuntime build `
  --file (Join-Path $PSScriptRoot "Containerfile") `
  --platform linux/amd64 `
  --build-arg "FEDORA_BOOTC_IMAGE=$baseDigest" `
  --build-arg "BITRIVER_SOURCE_REV=$sourceRevision" `
  --build-arg "MEDIAMTX_TARBALL_URL=$($lock.mediamtx.url)" `
  --build-arg "MEDIAMTX_SHA256=$($lock.mediamtx.sha256)" `
  --build-arg "OWNCAST_ZIP_URL=$($lock.owncast.url)" `
  --build-arg "OWNCAST_SHA256=$($lock.owncast.sha256)" `
  --tag $ImageTag `
  $repoRoot
if ($LASTEXITCODE -ne 0) { throw "BitRiver bootc image build failed." }

if (-not [string]::IsNullOrWhiteSpace($MetadataPath)) {
  $imageId = (& $ContainerRuntime image inspect $ImageTag --format "{{.Id}}" | Select-Object -First 1).Trim()
  if ($LASTEXITCODE -ne 0 -or -not $imageId) { throw "Could not inspect the built image." }
  $record = [ordered]@{
    imageTag = $ImageTag
    sourceRevision = $sourceRevision
    platform = $lock.targetPlatform
    baseRef = $baseDigest
    localImageId = $imageId
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
) {
  throw "A trusted Fedora bootc manifest digest is required."
}
Write-Host "Immutable Fedora base: $baseDigest"
$sourceRevision = (& git -C $repoRoot rev-parse HEAD | Select-Object -First 1).Trim()
if ($LASTEXITCODE -ne 0 -or $sourceRevision -notmatch '^[a-f0-9]{40}

& $ContainerRuntime build `
  --file (Join-Path $PSScriptRoot "Containerfile") `
  --platform linux/amd64 `
  --build-arg "FEDORA_BOOTC_IMAGE=$baseDigest" `
  --build-arg "MEDIAMTX_TARBALL_URL=$($lock.mediamtx.url)" `
  --build-arg "MEDIAMTX_SHA256=$($lock.mediamtx.sha256)" `
  --build-arg "OWNCAST_ZIP_URL=$($lock.owncast.url)" `
  --build-arg "OWNCAST_SHA256=$($lock.owncast.sha256)" `
  --tag $ImageTag `
  $repoRoot
if ($LASTEXITCODE -ne 0) { throw "BitRiver bootc image build failed." }

if (-not [string]::IsNullOrWhiteSpace($MetadataPath)) {
  $imageId = (& $ContainerRuntime image inspect $ImageTag --format "{{.Id}}" | Select-Object -First 1).Trim()
  if ($LASTEXITCODE -ne 0 -or -not $imageId) { throw "Could not inspect the built image." }
  $record = [ordered]@{
    imageTag = $ImageTag
    platform = $lock.targetPlatform
    baseRef = $baseDigest
    localImageId = $imageId
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
) {
  throw "Could not identify the checked out source revision."
}

& $ContainerRuntime build `
  --file (Join-Path $PSScriptRoot "Containerfile") `
  --platform linux/amd64 `
  --build-arg "FEDORA_BOOTC_IMAGE=$baseDigest" `
  --build-arg "MEDIAMTX_TARBALL_URL=$($lock.mediamtx.url)" `
  --build-arg "MEDIAMTX_SHA256=$($lock.mediamtx.sha256)" `
  --build-arg "OWNCAST_ZIP_URL=$($lock.owncast.url)" `
  --build-arg "OWNCAST_SHA256=$($lock.owncast.sha256)" `
  --tag $ImageTag `
  $repoRoot
if ($LASTEXITCODE -ne 0) { throw "BitRiver bootc image build failed." }

if (-not [string]::IsNullOrWhiteSpace($MetadataPath)) {
  $imageId = (& $ContainerRuntime image inspect $ImageTag --format "{{.Id}}" | Select-Object -First 1).Trim()
  if ($LASTEXITCODE -ne 0 -or -not $imageId) { throw "Could not inspect the built image." }
  $record = [ordered]@{
    imageTag = $ImageTag
    platform = $lock.targetPlatform
    baseRef = $baseDigest
    localImageId = $imageId
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
