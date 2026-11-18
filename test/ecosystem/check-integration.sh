#!/usr/bin/env bash
set -euo pipefail
root="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
cd "$root"
python3 -m unittest discover -s test/ecosystem -p 'test_*.py' -v
node --experimental-strip-types --test apps/control-ui/src/lib/bitriver-live-profile.test.mjs
python3 -m py_compile scripts/import-bitriver-live-profile.py
bash -n deploy/image/rootfs-overlay/usr/local/libexec/bitriver/mediamtx-restream.sh
grep -Fq 'COPY scripts/import-bitriver-live-profile.py' deploy/image/Containerfile
grep -Fq 'streamKeyFile' apps/control-api/internal/model/types.go
grep -Fq 'streamKeyFile' packages/shared/src/contracts.ts
test -f apps/control-ui/src/components/LiveDestinationSetup.tsx
grep -Fq 'Connect BitRiver-Live' apps/control-ui/src/components/LiveDestinationSetup.tsx
grep -Fq 'destinationForLiveProfile' apps/control-ui/src/lib/bitriver-live-profile.ts
grep -Fq 'StreamKeyFile' apps/control-api/internal/adapters/mediamtx/render.go
grep -Fq '/var/lib/bitriver/secrets/stream-keys' deploy/image/rootfs-overlay/usr/local/libexec/bitriver/mediamtx-restream.sh
grep -Fq 'd /var/lib/bitriver/secrets/stream-keys 0700 bitriver bitriver -' deploy/image/rootfs-overlay/usr/lib/tmpfiles.d/bitriver.conf

# Only a key-free schema example may be committed.
python3 - <<'PY'
import json
from pathlib import Path
profile = json.loads(Path("config/examples/bitriver-live-ingest-profile.json").read_text())
assert set(profile) == {"schemaVersion", "provider", "channelId", "rtmpServer", "viewerUrl"}
assert profile["provider"] == "bitriver-live"
assert "streamKey" not in json.dumps(profile)
PY
echo 'BitRiver-Live ingest profile checks passed.'
