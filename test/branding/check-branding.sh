#!/usr/bin/env bash
set -euo pipefail
root="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
cd "$root"

banner=docs/assets/bitriver-studio-logo.svg
icon=docs/assets/bitriver-studio-mark.svg
lockup=docs/assets/bitriver-studio-wordmark.svg

# Explicit copies are necessary for GitHub Markdown, Vite's public directory
# and the bootc root filesystem. A CI drift check keeps the identity unified.
cmp -s "$icon" apps/control-ui/public/favicon.svg
cmp -s "$icon" apps/control-ui/public/branding/bitriver-mark.svg
cmp -s "$icon" deploy/image/rootfs-overlay/usr/local/share/bitriver/branding/bitriver-mark.svg
cmp -s "$lockup" apps/control-ui/public/branding/bitriver-wordmark.svg
cmp -s "$lockup" deploy/image/rootfs-overlay/usr/local/share/bitriver/branding/bitriver-wordmark.svg
test -s "$banner"

grep -Fq 'docs/assets/bitriver-studio-logo.svg' README.md
grep -Fq '/branding/bitriver-wordmark.svg' apps/control-ui/src/App.tsx
grep -Fq '/branding/bitriver-wordmark.svg' apps/control-ui/src/components/OperatorGate.tsx
grep -Fq '/branding/bitriver-mark.svg' apps/control-ui/src/components/OperatorGate.tsx
grep -Fq 'href="/favicon.svg"' apps/control-ui/index.html
grep -Fq 'href="/manifest.webmanifest"' apps/control-ui/index.html
test -s deploy/image/rootfs-overlay/etc/issue.d/bitriver.issue
test -s deploy/image/rootfs-overlay/etc/motd

python3 - <<'PY'
from pathlib import Path
import json
import xml.etree.ElementTree as ET
paths = [
    Path("docs/assets/bitriver-studio-logo.svg"),
    Path("docs/assets/bitriver-studio-mark.svg"),
    Path("docs/assets/bitriver-studio-wordmark.svg"),
]
for path in paths:
    root = ET.fromstring(path.read_text(encoding="utf-8"))
    assert root.tag == "{http://www.w3.org/2000/svg}svg", path
    assert root.attrib.get("viewBox"), path
    assert root.attrib.get("role") == "img", path
    assert root.find("{http://www.w3.org/2000/svg}title") is not None, path
    for elem in root.iter():
        assert elem.tag.rsplit("}", 1)[-1] not in {"script", "foreignObject", "image"}, path
    text = path.read_text(encoding="utf-8")
    assert "http://" not in text.replace('xmlns="http://www.w3.org/2000/svg"', ""), path
    assert "https://" not in text, path

manifest = json.loads(Path("apps/control-ui/public/manifest.webmanifest").read_text())
assert manifest["name"] == "BitRiver Studio OS"
assert manifest["start_url"] == "/"
assert any(i["src"] == "/branding/bitriver-mark.svg" and i["type"] == "image/svg+xml" for i in manifest["icons"])
PY

# After a Vite build, public/ assets must survive into the actual UI bundle
# that the Containerfile installs in the bootc image.
if [[ -d apps/control-ui/dist ]]; then
  cmp -s "$icon" apps/control-ui/dist/favicon.svg
  cmp -s "$icon" apps/control-ui/dist/branding/bitriver-mark.svg
  cmp -s "$lockup" apps/control-ui/dist/branding/bitriver-wordmark.svg
  test -s apps/control-ui/dist/manifest.webmanifest
fi

echo "Branding matches README, control UI, browser identity, and bootc image."
