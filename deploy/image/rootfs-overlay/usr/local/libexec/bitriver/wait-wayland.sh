#!/usr/bin/env bash
set -euo pipefail

runtime_dir="${BITRIVER_RUNTIME_DIR:-/run/bitriver/studio}"
wayland_display="${BITRIVER_WAYLAND_DISPLAY:-wayland-1}"
socket_path="${runtime_dir}/${wayland_display}"

for _ in $(seq 1 20); do
  if [[ -S "${socket_path}" ]]; then
    exit 0
  fi
  sleep 1
done

echo "Wayland socket not ready at ${socket_path}" >&2
exit 1
