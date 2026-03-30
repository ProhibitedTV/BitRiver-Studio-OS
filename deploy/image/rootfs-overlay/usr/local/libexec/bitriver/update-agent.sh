#!/usr/bin/env bash
set -euo pipefail

env_file="/etc/bitriver/bitriver.env"
if [[ -f "${env_file}" ]]; then
  set -a
  # shellcheck source=/dev/null
  . "${env_file}"
  set +a
fi

update_dir="${BITRIVER_UPDATE_DIR:-/var/lib/bitriver/update}"
status_path="${BITRIVER_UPDATE_STATUS_PATH:-${update_dir}/status.json}"
poll_seconds="${BITRIVER_UPDATE_POLL_SECONDS:-60}"

install -d -o bitriver -g bitriver -m 0750 "${update_dir}"

while true; do
  checked_at="$(date --iso-8601=seconds)"
  booted_image=""
  staged_image=""
  deployment_source="fallback"

  if command -v bootc >/dev/null 2>&1; then
    deployment_source="bootc"
    bootc_status="$(bootc status --json 2>/dev/null || true)"
    if [[ -n "${bootc_status}" ]]; then
      booted_image="$(jq -r '.status.booted.image.image.reference // empty' <<<"${bootc_status}")"
      staged_image="$(jq -r '.status.staged.image.image.reference // empty' <<<"${bootc_status}")"
    fi
  fi

  jq -n \
    --arg checkedAt "${checked_at}" \
    --arg deploymentSource "${deployment_source}" \
    --arg currentImage "${booted_image}" \
    --arg stagedImage "${staged_image}" \
    '{
      checkedAt: $checkedAt,
      deploymentSource: $deploymentSource,
      currentImage: $currentImage,
      stagedImage: $stagedImage,
      staged: ($stagedImage != "")
    }' >"${status_path}"

  chown bitriver:bitriver "${status_path}" || true
  sleep "${poll_seconds}"
done
