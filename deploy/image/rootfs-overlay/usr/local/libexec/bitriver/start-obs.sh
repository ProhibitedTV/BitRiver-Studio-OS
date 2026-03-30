#!/usr/bin/env bash
set -euo pipefail

export XDG_RUNTIME_DIR="${BITRIVER_RUNTIME_DIR:-/run/bitriver/studio}"
export WAYLAND_DISPLAY="${BITRIVER_WAYLAND_DISPLAY:-wayland-1}"
mkdir -p "${XDG_RUNTIME_DIR}"
chmod 0700 "${XDG_RUNTIME_DIR}"

exec /usr/bin/obs \
  --disable-updater \
  --multi \
  --profile "BitRiver" \
  --collection "BitRiver"
