#!/usr/bin/env bash
set -euo pipefail

if [[ "${BITRIVER_FORCE_KIOSK:-0}" == "1" ]]; then
  exit 0
fi

shopt -s nullglob
for status_file in /sys/class/drm/*/status; do
  if [[ -f "${status_file}" ]] && grep -q '^connected$' "${status_file}"; then
    exit 0
  fi
done

exit 1
