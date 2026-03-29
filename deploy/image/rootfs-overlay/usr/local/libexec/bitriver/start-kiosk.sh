#!/usr/bin/env bash
set -euo pipefail

export XDG_RUNTIME_DIR="/run/user/$(id -u)"
mkdir -p "${XDG_RUNTIME_DIR}"

exec /usr/bin/sway --config /etc/sway/config.d/bitriver-kiosk.conf
