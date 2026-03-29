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

$show = Invoke-RestMethod -Uri "$BaseUrl/api/v1/show"
if (-not $show.activeSceneId) {
  throw "Show response did not include an active scene"
}

$sources = Invoke-RestMethod -Uri "$BaseUrl/api/v1/sources"
if ($sources.Count -lt 3) {
  throw "Expected at least three sources in the smoke fixture"
}
