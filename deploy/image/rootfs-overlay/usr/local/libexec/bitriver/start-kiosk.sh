#!/usr/bin/env bash
set -euo pipefail

export XDG_RUNTIME_DIR="${BITRIVER_RUNTIME_DIR:-/run/bitriver/studio}"
mkdir -p "${XDG_RUNTIME_DIR}"
chmod 0700 "${XDG_RUNTIME_DIR}"

exec /usr/bin/sway --config /etc/sway/config.d/bitriver-kiosk.conf
