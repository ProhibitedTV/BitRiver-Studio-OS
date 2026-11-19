#!/usr/bin/env python3
"""Verify the offline asset pack is small, closed and usable without network."""
import json
from pathlib import Path
import struct

root = Path(__file__).resolve().parents[2]
base = root / "deploy/image/rootfs-overlay/usr/local/share/bitriver/creator-assets"
assets = base / "assets"
expected = {"starting-soon", "be-right-back", "just-chatting", "intermission"}

catalog = json.loads((assets / "catalog.json").read_text(encoding="utf-8"))
assert len(catalog["assets"]) == len(expected)
assert catalog["summary"]["sourceRevision"] == "e86b82f1df14c2defca0d1e0d69b9d8f1073d176"
assert {asset["path"].removeprefix("scenes/").removesuffix(".html") for asset in catalog["assets"]} == expected
assert "MIT License" in (base / "LICENSE-OBS-Scene-Assets").read_text()
assert (base / "PROVENANCE.md").is_file()
assert "BITRIVER_ASSET_LIBRARY_DIR=/usr/local/share/bitriver/creator-assets/assets" in (
    root / "deploy/image/rootfs-overlay/etc/bitriver/bitriver.env"
).read_text()

for asset in catalog["assets"]:
    assert asset["category"] == "scenes"
    assert (asset["width"], asset["height"]) == (1920, 1080)
    assert asset["path"].startswith("scenes/") and asset["path"].endswith(".html")
    assert asset["preview"].startswith("previews/scenes/") and asset["preview"].endswith(".png")
    html = (assets / asset["path"]).read_text(encoding="utf-8")
    assert "<!DOCTYPE html>" in html
    assert "https://" not in html and "http://" not in html
    assert "@import" not in html and "<iframe" not in html
    assert "fetch(" not in html and "XMLHttpRequest" not in html
    png = (assets / asset["preview"]).read_bytes()
    assert len(png) > 1024 and png[:8] == b"\x89PNG\r\n\x1a\n"
    assert struct.unpack(">II", png[16:24]) == (1920, 1080)

print("Pinned offline starter scenes, thumbnails, licensing and default path verified.")
