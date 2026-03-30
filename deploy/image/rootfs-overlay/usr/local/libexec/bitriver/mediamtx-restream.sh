#!/usr/bin/env bash
set -euo pipefail

DESTINATIONS_PATH="${1:?destinations sidecar path is required}"
SOURCE_URL="${2:?source url is required}"

if ! command -v jq >/dev/null 2>&1; then
  echo "jq is required for mediamtx-restream.sh" >&2
  exit 1
fi

if ! command -v ffmpeg >/dev/null 2>&1; then
  echo "ffmpeg is required for mediamtx-restream.sh" >&2
  exit 1
fi

if [[ ! -f "${DESTINATIONS_PATH}" ]]; then
  echo "destinations sidecar not found: ${DESTINATIONS_PATH}" >&2
  exit 1
fi

mapfile -t OUTPUTS < <(
  jq -r '
    .publishDestinations[]
    | select(.enabled == true)
    | if .protocol == "rtmp" then
        "[f=flv:onfail=ignore]" + .targetUrl
      elif .protocol == "srt" then
        "[f=mpegts:onfail=ignore]" + .targetUrl
      else
        empty
      end
  ' "${DESTINATIONS_PATH}"
)

if [[ "${#OUTPUTS[@]}" -eq 0 ]]; then
  exec sleep infinity
fi

TEE_TARGETS="$(printf '%s|' "${OUTPUTS[@]}")"
TEE_TARGETS="${TEE_TARGETS%|}"

exec ffmpeg \
  -nostdin \
  -hide_banner \
  -loglevel warning \
  -fflags +genpts \
  -i "${SOURCE_URL}" \
  -map 0:v? \
  -map 0:a? \
  -c copy \
  -f tee "${TEE_TARGETS}"
