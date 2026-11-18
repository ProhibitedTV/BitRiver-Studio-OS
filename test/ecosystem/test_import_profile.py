#!/usr/bin/env python3
import importlib.util
import io
import json
import os
from pathlib import Path
import stat
import tempfile
import unittest

SCRIPT = Path(__file__).resolve().parents[2] / "scripts/import-bitriver-live-profile.py"
spec = importlib.util.spec_from_file_location("bitriver_import", SCRIPT)
module = importlib.util.module_from_spec(spec)
spec.loader.exec_module(module)

PROFILE = {
    "schemaVersion": "bitriver.studio.ingest/v1",
    "provider": "bitriver-live",
    "channelId": "a" * 32,
    "rtmpServer": "rtmp://ingest.example.test:1935/live",
    "viewerUrl": "https://watch.example.test/viewer/channel",
}


class IngestProfileTests(unittest.TestCase):
    def test_profile_is_key_free(self):
        parsed = module.profile_from_bytes(json.dumps(PROFILE).encode())
        destination = module.destination_for(parsed, "studio-stream")
        self.assertEqual(destination["streamKeyFile"], "/var/lib/bitriver/secrets/stream-keys/studio-stream")
        self.assertEqual(destination["targetUrl"], PROFILE["rtmpServer"])
        self.assertFalse(destination["enabled"])
        self.assertNotIn("streamKey", destination)
        self.assertIn("streamKeyFile", destination)
        self.assertNotIn("streamKey", json.dumps(parsed))

    def test_key_and_unsafe_urls_rejected(self):
        for extra in ["streamKey", "token", "password"]:
            with self.subTest(extra=extra):
                self.assertRaises(ValueError, module.profile_from_bytes, json.dumps({**PROFILE, extra: "secret"}).encode())
        for url in ["rtmp://u:p@example.test/live", "rtmp://example.test/live/PRIVATEKEY",
                    "rtmp://example.test/live?key=secret", "https://example.test/live",
                    "rtmp://example.test/live|malicious"]:
            with self.subTest(url=url):
                self.assertRaises(ValueError, module.profile_from_bytes, json.dumps({**PROFILE, "rtmpServer": url}).encode())
        for name in ["..", "../data", "name.dots", "a" * 65]:
            self.assertRaises(ValueError, module.destination_for, PROFILE, name)

    def test_stored_key_is_private_and_rejects_overwrite(self):
        with tempfile.TemporaryDirectory() as td:
            root = Path(td)
            root.chmod(0o700)
            module.write_secret(root, "bitriver-live", "A" * 48)
            dest = root / "bitriver-live"
            self.assertEqual(stat.S_IMODE(dest.stat().st_mode), 0o600)
            self.assertEqual(dest.read_text().strip(), "A" * 48)
            self.assertRaises(FileExistsError, module.write_secret, root, "bitriver-live", "B" * 48)

    def test_insecure_directory_or_invalid_key_rejected(self):
        with tempfile.TemporaryDirectory() as td:
            root = Path(td)
            root.chmod(0o755)
            self.assertRaises(ValueError, module.write_secret, root, "bitriver-live", "A" * 48)
            root.chmod(0o700)
            self.assertRaises(ValueError, module.write_secret, root, "bitriver-live", "invalid-key")


if __name__ == "__main__":
    unittest.main()
