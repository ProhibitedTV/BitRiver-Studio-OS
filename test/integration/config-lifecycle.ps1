param(
  [string]$BaseUrl = "http://127.0.0.1:8080"
)

Set-StrictMode -Version Latest
$ErrorActionPreference = "Stop"

$active = Invoke-RestMethod -Uri "$BaseUrl/api/v1/config/active"
$draftConfig = $active.config
$draftConfig.system.name = "studio-integration"

$draftBody = $draftConfig | ConvertTo-Json -Depth 10
Invoke-RestMethod `
  -Uri "$BaseUrl/api/v1/config/draft" `
  -Method Put `
  -ContentType "application/json" `
  -Body $draftBody | Out-Null

$validation = Invoke-RestMethod -Uri "$BaseUrl/api/v1/config/validate" -Method Post
if (-not $validation.valid) {
  throw "Draft validation unexpectedly failed"
}

$apply = Invoke-RestMethod -Uri "$BaseUrl/api/v1/config/apply" -Method Post
if (-not $apply.appliedGeneration) {
  throw "Apply response did not include an applied generation"
}

$updated = Invoke-RestMethod -Uri "$BaseUrl/api/v1/config/active"
if ($updated.config.system.name -ne "studio-integration") {
  throw "Active config did not reflect the applied draft"
}
