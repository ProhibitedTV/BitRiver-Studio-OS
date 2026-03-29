#!/usr/bin/env bash
set -euo pipefail

install -d -o bitriver -g bitriver -m 0750 /var/lib/bitriver
install -d -o bitriver -g bitriver -m 0750 /var/lib/bitriver/generations
install -d -o bitriver -g bitriver -m 0750 /var/lib/bitriver/runtime
install -d -o bitriver -g bitriver -m 0750 /var/lib/owncast
install -d -o studio -g studio -m 0750 /data/recordings

if [[ ! -f /var/lib/bitriver/.bootstrapped ]]; then
  date --iso-8601=seconds >/var/lib/bitriver/.bootstrapped
fi
