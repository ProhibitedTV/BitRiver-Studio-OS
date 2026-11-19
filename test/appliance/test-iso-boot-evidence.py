#!/usr/bin/env python3
"""No QEMU needed: regressions for ISO boot evidence and false positives."""
import importlib.util
from pathlib import Path
import tempfile
import unittest


SCRIPT = Path(__file__).with_name("analyze-iso-boot.py")
spec = importlib.util.spec_from_file_location("iso_boot_evidence", SCRIPT)
probe = importlib.util.module_from_spec(spec)
spec.loader.exec_module(probe)


class BootEvidenceTests(unittest.TestCase):
    def setUp(self):
        self._temp = tempfile.TemporaryDirectory()
        self.addCleanup(self._temp.cleanup)
        root = Path(self._temp.name)
        self.early = root / "early.ppm"
        self.mid = root / "middle.ppm"
        self.later = root / "later.ppm"
        self.serial = root / "serial.log"
        self.virtio = root / "anaconda-virtio.log"
        self.virtio.write_text("")
        self.early.write_bytes(b"P6\n2 1\n255\n" + bytes([0, 0, 0]) * 2)
        self.mid.write_bytes(b"P6\n2 1\n255\n" + bytes([255, 255, 255]) * 2)
        self.later.write_bytes(b"P6\n2 1\n255\n" + bytes([57, 169, 226]) * 2)

    def classify(self, log):
        self.serial.write_text(log)
        return probe.classify(self.early, self.mid, self.later, self.serial, self.virtio)

    def test_valid_basic_target_and_pre_service_start_do_not_claim_installer(self):
        result = self.classify(
            "\x1b[0;32m[ OK ] Reached target basic.target - Basic System.\x1b[0m\n"
            "Starting anaconda-pre.service - pre-anaconda logging service...\n"
        )
        self.assertTrue(result["bootPrerequisitePassed"])
        self.assertTrue(result["anacondaPreServiceStarted"])
        self.assertFalse(result["anacondaPreServiceFinished"])
        self.assertFalse(result["anacondaMainServiceStarted"])
        self.assertFalse(result["anacondaUserInterfaceVerified"])
        self.assertFalse(result["diskInstallationVerified"])
        self.assertFalse(result["releaseReady"])
        self.assertTrue(result["finalFramebufferUniform"])
        self.assertEqual(result["finalFramebufferSolidRgb"], "39a9e2")
        self.assertTrue(result["earlyToMiddleFramebufferChanged"])
        self.assertTrue(result["middleToLaterFramebufferChanged"])

    def test_failed_or_stopped_events_are_not_success(self):
        result = self.classify(
            "[ OK ] Stopped target basic.target - Basic System.\n"
            "[FAILED] Failed to start anaconda-pre.service - logging.\n"
            "Failed to reach target basic.target.\n"
        )
        self.assertFalse(result["systemdBasicTargetSeen"])
        self.assertFalse(result["anacondaPreServiceStarted"])
        self.assertFalse(result["bootPrerequisitePassed"])

    def test_pre_service_can_finish_without_proving_installer_ui(self):
        result = self.classify(
            "[ OK ] Reached target basic.target - Basic System.\n"
            "Starting anaconda-pre.service - pre-anaconda logging service...\n"
            "[ OK ] Finished anaconda-pre.service - pre-anaconda logging service.\n"
            "Starting anaconda.service - Anaconda Installer...\n"
        )
        self.assertTrue(result["anacondaPreServiceFinished"])
        self.assertTrue(result["anacondaMainServiceStarted"])
        self.assertFalse(result["anacondaUserInterfaceVerified"])
        self.assertFalse(result["releaseReady"])

    def test_framebuffer_content_cannot_be_inferred_from_changed_hash(self):
        self.later.write_bytes(b"P6\n2 1\n255\n" + bytes([0, 0, 0, 255, 255, 255]))
        result = self.classify(
            "[ OK ] Reached target basic.target - Basic System.\n"
            "Starting anaconda-pre.service...\n"
        )
        self.assertFalse(result["finalFramebufferUniform"])
        self.assertIsNone(result["finalFramebufferSolidRgb"])
        self.assertFalse(result["anacondaUserInterfaceVerified"])

    def test_grub_only_serial_is_inconclusive_not_installer_proof(self):
        result = self.classify(
            "GRUB version 2.12\nBooting `Install BitRiver Studio OS`\n"
        )
        self.assertFalse(result["serialGuestBootMessagesSeen"])
        self.assertFalse(result["bootPrerequisitePassed"])
        self.assertFalse(result["anacondaUserInterfaceVerified"])

    def test_virtio_log_is_separate_evidence_and_never_claims_ui(self):
        self.virtio.write_text("Anaconda: initializing storage UI\n")
        result = self.classify("GRUB version 2.12\n")
        self.assertTrue(result["anacondaVirtioLogNonempty"])
        self.assertGreater(result["anacondaVirtioLogBytes"], 0)
        self.assertFalse(result["bootPrerequisitePassed"])
        self.assertFalse(result["anacondaUserInterfaceVerified"])
        self.assertFalse(result["releaseReady"])

    def test_truncated_framebuffer_is_rejected(self):
        self.later.write_bytes(b"P6\n2 1\n255\n" + bytes([0, 0, 0]))
        with self.assertRaises(ValueError):
            self.classify("Starting anaconda-pre.service")

    def test_malformed_framebuffer_is_rejected(self):
        self.early.write_bytes(b"P3\n2 1\n255\n0 0 0")
        with self.assertRaises(ValueError):
            self.classify("Starting anaconda-pre.service")


if __name__ == "__main__":
    unittest.main()
