#!/usr/bin/env bash
set -euo pipefail

env_file="/etc/bitriver/bitriver.env"
if [[ -f "${env_file}" ]]; then
  set -a
  # shellcheck source=/dev/null
  . "${env_file}"
  set +a
fi

state_dir="${BITRIVER_STATE_DIR:-/var/lib/bitriver}"
bootstrap_dir="${BITRIVER_BOOTSTRAP_DIR:-${state_dir}/bootstrap}"
config_dir="${BITRIVER_CONFIG_DIR:-${state_dir}/config}"
hardware_dir="${BITRIVER_HARDWARE_DIR:-${state_dir}/hardware}"
setup_status_path="${BITRIVER_SETUP_STATUS_PATH:-${bootstrap_dir}/setup-status.json}"
initial_config_path="${BITRIVER_INITIAL_CONFIG_PATH:-${config_dir}/active.json}"
hardware_report_path="${BITRIVER_HARDWARE_DIR:-${state_dir}/hardware}/capabilities.json"
identity_path="${bootstrap_dir}/device-identity.json"
credentials_path="${bootstrap_dir}/admin-credentials.json"
password_notice_path="${bootstrap_dir}/one-time-admin-password.txt"
marker_path="${bootstrap_dir}/firstboot-complete"
template_path="/usr/local/share/bitriver/templates/initial-config.template.json"
started_at="$(date --iso-8601=seconds)"

install -d -o bitriver -g bitriver -m 0750 "${bootstrap_dir}" "${config_dir}" "${hardware_dir}" "${state_dir}/update" "${state_dir}/runtime"
install -d -o studio -g studio -m 0750 /data/recordings

default_hardware="$(jq -n \
  --arg detectedAt "${started_at}" \
  '{detectedAt: $detectedAt, architecture: "unknown", cpuModel: "unknown", logicalCores: 0, memoryMiB: 0, displayAttached: false, videoDevices: [], audioCaptureDevices: [], networkInterfaces: [], capabilities: {kioskEligible: false, obsEligible: false, recommendedForMvp: false}}')"

printf '%s\n' "$(
  jq -n \
    --arg startedAt "${started_at}" \
    --argjson hardware "${default_hardware}" \
    '{
      state: "in_progress",
      firstBootComplete: false,
      applianceId: "",
      hostname: "bitriver.local",
      adminUsername: "admin",
      credentialsReady: false,
      requiresPasswordRotation: true,
      initialConfigWritten: false,
      hardwareDetected: false,
      displayMode: "headless",
      startedAt: $startedAt,
      issues: [],
      hardware: $hardware
    }'
)" >"${setup_status_path}"

if [[ ! -f "${identity_path}" ]]; then
  short_id="$(systemd-id128 new | tr 'A-Z' 'a-z' | cut -c1-12)"
  appliance_id="bitriver-${short_id}"
  hostname="bitriver-${short_id}"

  printf '%s\n' "$(
    jq -n \
      --arg applianceId "${appliance_id}" \
      --arg hostname "${hostname}" \
      --arg createdAt "${started_at}" \
      '{applianceId: $applianceId, hostname: $hostname, createdAt: $createdAt}'
  )" >"${identity_path}"

  if command -v hostnamectl >/dev/null 2>&1; then
    hostnamectl set-hostname "${hostname}" || true
  fi
fi

appliance_id="$(jq -r '.applianceId' "${identity_path}")"
hostname="$(jq -r '.hostname' "${identity_path}")"

if [[ ! -f "${credentials_path}" ]]; then
  one_time_password="$(openssl rand -hex 16)"
  password_hash="$(openssl passwd -6 -salt "$(openssl rand -hex 8)" "${one_time_password}")"

  printf '%s\n' "$(
    jq -n \
      --arg username "admin" \
      --arg passwordHash "${password_hash}" \
      --arg createdAt "${started_at}" \
      '{username: $username, passwordHash: $passwordHash, createdAt: $createdAt, rotateOnFirstLogin: true}'
  )" >"${credentials_path}"

  cat >"${password_notice_path}" <<EOF
BitRiver Studio OS one-time admin credentials
Username: admin
Password: ${one_time_password}

Rotate this password during setup. This file is intended for local recovery only.
EOF

  chmod 0600 "${password_notice_path}" "${credentials_path}"
fi

/usr/local/bin/bitriver-hw-report --output "${hardware_report_path}" >/dev/null

primary_video="$(jq -r '.videoDevices[0] // empty' "${hardware_report_path}")"
primary_audio="$(jq -r '.audioCaptureDevices[0] // empty' "${hardware_report_path}")"
display_mode="headless"
issues=()

if jq -e '.displayAttached == true' "${hardware_report_path}" >/dev/null; then
  display_mode="kiosk"
else
  issues+=("no local display detected; LAN control is available but kiosk and OBS remain skipped until HDMI or a dummy plug is attached")
fi

if [[ -z "${primary_video}" ]]; then
  issues+=("no local video capture device detected during first boot")
fi

if [[ ! -f "${initial_config_path}" ]]; then
  timezone="$(timedatectl show --property=Timezone --value 2>/dev/null || echo UTC)"

  jq \
    --arg systemName "${hostname}" \
    --arg timezone "${timezone}" \
    --arg primaryVideo "${primary_video}" \
    --arg primaryAudio "${primary_audio}" \
    --arg hostname "${hostname}" \
    '
      .system.name = $systemName
      | .system.timezone = $timezone
      | .sources += (if $primaryVideo != "" then [{
          id: "cam-host",
          name: "Host Camera",
          kind: "local_video",
          deviceRef: $primaryVideo
        }] else [] end)
      | .audio.channels = (if $primaryAudio != "" then [{
          id: "mic-host",
          name: "Host Mic",
          deviceRef: $primaryAudio
        }] else [] end)
      | .scenes += (
          if $primaryVideo != "" and $primaryAudio != "" then [{
            id: "host-cam",
            name: "Host Camera",
            sources: ["cam-host", "mic-host", "lower-third"]
          }]
          elif $primaryVideo != "" then [{
            id: "host-cam",
            name: "Host Camera",
            sources: ["cam-host", "lower-third"]
          }]
          else [{
            id: "guest-ready",
            name: "Guest Ready",
            sources: ["guest-1", "lower-third"]
          }]
          end
        )
      | .sources |= map(
          if .id == "guest-1" then .publishUrl = ("srt://" + $hostname + ":8890?streamid=publish:guest-1") else . end
        )
    ' "${template_path}" >"${initial_config_path}"
fi

completed_at="$(date --iso-8601=seconds)"
state="ready"
if [[ ${#issues[@]} -gt 0 ]]; then
  state="degraded"
fi

issues_json='[]'
if [[ ${#issues[@]} -gt 0 ]]; then
  issues_json="$(printf '%s\n' "${issues[@]}" | jq -R . | jq -s .)"
fi

printf '%s\n' "$(
  jq -n \
    --arg state "${state}" \
    --arg applianceId "${appliance_id}" \
    --arg hostname "${hostname}" \
    --arg adminUsername "admin" \
    --arg displayMode "${display_mode}" \
    --arg startedAt "${started_at}" \
    --arg completedAt "${completed_at}" \
    --argjson issues "${issues_json}" \
    --argjson hardware "$(cat "${hardware_report_path}")" \
    '{
      state: $state,
      firstBootComplete: true,
      applianceId: $applianceId,
      hostname: $hostname,
      adminUsername: $adminUsername,
      credentialsReady: true,
      requiresPasswordRotation: true,
      initialConfigWritten: true,
      hardwareDetected: true,
      displayMode: $displayMode,
      startedAt: $startedAt,
      completedAt: $completedAt,
      issues: $issues,
      hardware: $hardware
    }'
)" >"${setup_status_path}"

touch "${marker_path}"
chown bitriver:bitriver "${identity_path}" "${credentials_path}" "${setup_status_path}" "${initial_config_path}" "${hardware_report_path}" "${marker_path}"
chown bitriver:bitriver "${password_notice_path}"
