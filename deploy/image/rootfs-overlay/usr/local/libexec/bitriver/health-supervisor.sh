#!/usr/bin/env bash
set -euo pipefail

env_file="/etc/bitriver/bitriver.env"
if [[ -f "${env_file}" ]]; then
  set -a
  # shellcheck source=/dev/null
  . "${env_file}"
  set +a
fi

setup_status_path="${BITRIVER_SETUP_STATUS_PATH:-/var/lib/bitriver/bootstrap/setup-status.json}"
health_status_path="${BITRIVER_HEALTH_STATUS_PATH:-/run/bitriver/health/summary.json}"
initial_config_path="${BITRIVER_INITIAL_CONFIG_PATH:-/var/lib/bitriver/config/active.json}"
interval_seconds="${BITRIVER_HEALTH_INTERVAL_SECONDS:-15}"

install -d -m 0755 "$(dirname "${health_status_path}")"

append_check() {
  local key="$1"
  local state="$2"
  local message="$3"
  local checks_json="$4"
  jq \
    --arg key "${key}" \
    --arg state "${state}" \
    --arg message "${message}" \
    '. + [{key: $key, state: $state, message: $message}]' <<<"${checks_json}"
}

merge_state() {
  local current="$1"
  local candidate="$2"

  if [[ "${current}" == "error" || "${candidate}" == "healthy" ]]; then
    printf '%s' "${current}"
    return
  fi

  if [[ "${candidate}" == "error" ]]; then
    printf 'error'
    return
  fi

  printf 'degraded'
}

while true; do
  overall="healthy"
  checks='[]'

  if [[ -f "${setup_status_path}" ]]; then
    setup_state="$(jq -r '.state // "pending"' "${setup_status_path}")"
    setup_check_state="error"
    case "${setup_state}" in
      ready) setup_check_state="healthy" ;;
      degraded) setup_check_state="degraded" ;;
      *) setup_check_state="error" ;;
    esac
    checks="$(append_check "first_boot" "${setup_check_state}" "first-boot setup state: ${setup_state}" "${checks}")"
    case "${setup_state}" in
      ready) ;;
      degraded) overall="$(merge_state "${overall}" "degraded")" ;;
      *) overall="$(merge_state "${overall}" "error")" ;;
    esac
  else
    checks="$(append_check "first_boot" "error" "setup status file missing" "${checks}")"
    overall="$(merge_state "${overall}" "error")"
  fi

  if curl --fail --silent http://127.0.0.1:8080/healthz >/dev/null 2>&1; then
    checks="$(append_check "control_api" "healthy" "control API liveness passed" "${checks}")"
  else
    systemctl restart bitriver-control-api.service || true
    checks="$(append_check "control_api" "error" "control API liveness failed; restart requested" "${checks}")"
    overall="$(merge_state "${overall}" "error")"
  fi

  if curl --fail --silent http://127.0.0.1:9997/v3/paths/list >/dev/null 2>&1; then
    checks="$(append_check "mediamtx" "healthy" "MediaMTX API responded" "${checks}")"
  else
    systemctl restart mediamtx.service || true
    checks="$(append_check "mediamtx" "error" "MediaMTX readiness failed; restart requested" "${checks}")"
    overall="$(merge_state "${overall}" "error")"
  fi

  if /usr/local/libexec/bitriver/owncast-enabled.sh >/dev/null 2>&1; then
    if curl --fail --silent http://127.0.0.1:8081/ >/dev/null 2>&1; then
      checks="$(append_check "owncast" "healthy" "Owncast enabled and reachable" "${checks}")"
    else
      systemctl restart owncast.service || true
      checks="$(append_check "owncast" "degraded" "Owncast enabled but not reachable; restart requested" "${checks}")"
      overall="$(merge_state "${overall}" "degraded")"
    fi
  else
    checks="$(append_check "owncast" "healthy" "Owncast disabled by active config" "${checks}")"
  fi

  if /usr/local/libexec/bitriver/display-attached.sh; then
    if systemctl is-active --quiet bitriver-kiosk.service && systemctl is-active --quiet obs.service; then
      checks="$(append_check "local_display" "healthy" "display-attached kiosk and OBS session active" "${checks}")"
    else
      systemctl restart bitriver-kiosk.service || true
      systemctl restart obs.service || true
      checks="$(append_check "local_display" "degraded" "display attached but kiosk or OBS inactive; restart requested" "${checks}")"
      overall="$(merge_state "${overall}" "degraded")"
    fi
  else
    checks="$(append_check "local_display" "degraded" "headless mode active; kiosk and OBS intentionally skipped" "${checks}")"
    overall="$(merge_state "${overall}" "degraded")"
  fi

  if systemctl is-active --quiet bitriver-update-agent.service; then
    checks="$(append_check "update_agent" "healthy" "update agent loop active" "${checks}")"
  else
    systemctl restart bitriver-update-agent.service || true
    checks="$(append_check "update_agent" "degraded" "update agent inactive; restart requested" "${checks}")"
    overall="$(merge_state "${overall}" "degraded")"
  fi

  jq -n \
    --arg overall "${overall}" \
    --arg checkedAt "$(date --iso-8601=seconds)" \
    --argjson checks "${checks}" \
    '{
      overall: $overall,
      checkedAt: $checkedAt,
      degradedMode: ($overall != "healthy"),
      checks: $checks
    }' >"${health_status_path}"

  sleep "${interval_seconds}"
done
