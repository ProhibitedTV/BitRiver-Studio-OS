param(
  [string]$BaseUrl = "http://127.0.0.1:8080"
)

Set-StrictMode -Version Latest
$ErrorActionPreference = "Stop"

$health = Invoke-WebRequest -Uri "$BaseUrl/healthz" -UseBasicParsing
if ($health.Content.Trim() -ne "ok") {
  throw "Unexpected health response: $($health.Content)"
}

$system = Invoke-RestMethod -Uri "$BaseUrl/api/v1/system"
if ($system.name -ne "studio-a") {
  throw "Unexpected appliance name: $($system.name)"
}

$health = Invoke-RestMethod -Uri "$BaseUrl/api/v1/health"
if (-not $health.overall) {
  throw "Health response did not include an overall state"
}

$stream = Invoke-RestMethod -Uri "$BaseUrl/api/v1/stream"
if (-not $stream.activeSceneId) {
  throw "Stream response did not include an active scene"
}

$outputs = Invoke-RestMethod -Uri "$BaseUrl/api/v1/outputs"
if ($outputs.destinations.Count -lt 1) {
  throw "Expected at least one output destination"
}

$mediaRouter = Invoke-RestMethod -Uri "$BaseUrl/api/v1/media-router"
if (-not $mediaRouter.localIngestPath) {
  throw "Expected media-router endpoint to report a local ingest path"
}
if ($mediaRouter.destinations.Count -lt 1) {
  throw "Expected media-router endpoint to report at least one destination"
}

$sources = Invoke-RestMethod -Uri "$BaseUrl/api/v1/sources"
if ($sources.Count -lt 3) {
  throw "Expected at least three sources in the smoke fixture"
}

$audio = Invoke-RestMethod -Uri "$BaseUrl/api/v1/audio/routes"
if ($audio.channels.Count -lt 1) {
  throw "Expected at least one audio route"
}

$recordings = Invoke-RestMethod -Uri "$BaseUrl/api/v1/recordings"
if (-not $recordings.recordingPath) {
  throw "Expected recordings endpoint to report the current path"
}

$update = Invoke-RestMethod -Uri "$BaseUrl/api/v1/update"
if (-not $update.currentVersion) {
  throw "Expected update endpoint to report a current version"
}
