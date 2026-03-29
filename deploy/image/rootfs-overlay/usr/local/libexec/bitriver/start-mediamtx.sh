#!/usr/bin/env bash
set -euo pipefail

CONFIG_PATH="${BITRIVER_MEDIAMTX_CONFIG:-/etc/mediamtx/mediamtx.yml}"
exec /usr/local/bin/mediamtx "${CONFIG_PATH}"
