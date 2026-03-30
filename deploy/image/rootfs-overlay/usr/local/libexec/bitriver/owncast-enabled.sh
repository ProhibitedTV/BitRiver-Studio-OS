#!/usr/bin/env bash
set -euo pipefail

env_file="/etc/bitriver/bitriver.env"
if [[ -f "${env_file}" ]]; then
  set -a
  # shellcheck source=/dev/null
  . "${env_file}"
  set +a
fi

config_path="${BITRIVER_INITIAL_CONFIG_PATH:-/var/lib/bitriver/config/active.json}"
if [[ ! -f "${config_path}" ]]; then
  exit 1
fi

jq -e '.owncast.enabled == true' "${config_path}" >/dev/null
