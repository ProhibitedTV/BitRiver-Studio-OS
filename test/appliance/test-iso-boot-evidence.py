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
        self.assertFalse(result["finalFramebufferNonUniform"])
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
        self.assertTrue(result["finalFramebufferNonUniform"])
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

    def test_firefox_prerequisites_never_certify_a_disk_install(self):
        self.later.write_bytes(b"P6\n2 1\n255\n" + bytes([0, 0, 0, 255, 255, 255]))
        result = self.classify(
            "Reached target basic.target\n"
            "Starting anaconda-pre.service\n"
            "Finished anaconda-pre.service\n"
            "Starting anaconda.service\n"
            "program: Running... /usr/libexec/anaconda/webui-desktop -t default\n"
            "anaconda: ui.webui: Using default browser: firefox\n"
            "pkexec[2798]: root: Executing command [USER=bitriver-installer] "
            "[TTY=unknown] [CWD=/] [COMMAND=/usr/bin/env DISPLAY=:1 "
            "/usr/bin/firefox --new-instance]\n"
            "BITRIVER_ISO_PROBE sample=3 firefox_unprivileged=yes "
            "xwayland=yes cockpit_http=200 browser_error_count=0\n"
        )
        self.assertTrue(result["firefoxStartedUnprivileged"])
        self.assertTrue(result["browserProbeTelemetrySeen"])
        self.assertTrue(result["firefoxRunningSampleSeen"])
        self.assertTrue(result["requiredSmokePassed"])
        self.assertFalse(result["anacondaUserInterfaceVerified"])
        self.assertFalse(result["diskInstallationVerified"])
        self.assertFalse(result["releaseReady"])

    def test_browser_selection_message_without_unprivileged_launch_fails(self):
        self.later.write_bytes(b"P6\n2 1\n255\n" + bytes([0, 0, 0, 255, 255, 255]))
        result = self.classify(
            "Reached target basic.target\n"
            "Starting anaconda-pre.service\n"
            "Finished anaconda-pre.service\n"
            "Starting anaconda.service\n"
            "program: Running... /usr/libexec/anaconda/webui-desktop\n"
            "Using default browser: firefox\n"
        )
        self.assertTrue(result["firefoxBrowserSelected"])
        self.assertFalse(result["firefoxStartedUnprivileged"])
        self.assertFalse(result["requiredSmokePassed"])

    def test_actual_serial_pkexec_firefox_detected_without_selection_message(self):
        self.later.write_bytes(b"P6\n2 1\n255\n" + bytes([0, 0, 0, 255, 255, 255]))
        result = self.classify(
            "Reached target basic.target\n"
            "Starting anaconda-pre.service\n"
            "Finished anaconda-pre.service\n"
            "Starting anaconda.service\n"
            "program: Running... /usr/libexec/anaconda/webui-desktop\n"
            "pkexec[2798]: root: Executing command [USER=bitriver-installer] "
            "[TTY=unknown] [CWD=/] [COMMAND=/usr/bin/env DISPLAY=:1 "
            "/usr/bin/env MOZ_APP_TITLE= /usr/bin/firefox --new-instance]\n"
            "BITRIVER_ISO_PROBE sample=10 firefox_unprivileged=yes "
            "xwayland=yes cockpit_http=200 browser_error_count=0\n"
        )
        self.assertTrue(result["browserProbeTelemetrySeen"])
        self.assertFalse(result["browserProbeOrderingCycleDetected"])
        self.assertTrue(result["firefoxBrowserSelected"])
        self.assertTrue(result["firefoxStartedUnprivileged"])
        self.assertTrue(result["requiredSmokePassed"])
        self.assertFalse(result["anacondaUserInterfaceVerified"])

    def test_probe_deleted_by_systemd_cycle_fails_closed(self):
        self.later.write_bytes(b"P6\n2 1\n255\n" + bytes([0, 0, 0, 255, 255, 255]))
        result = self.classify(
            "Reached target basic.target\n"
            "Starting anaconda-pre.service\n"
            "Finished anaconda-pre.service\n"
            "Starting anaconda.service\n"
            "program: Running... /usr/libexec/anaconda/webui-desktop\n"
            "pkexec[2781]: root: Executing command [USER=bitriver-installer] "
            "[COMMAND=/usr/bin/env DISPLAY=:1 /usr/bin/firefox]\n"
            "BITRIVER_ISO_PROBE sample=7 firefox_unprivileged=yes\n"
            "anaconda.service: Found ordering cycle: anaconda.target/start "
            "after bitriver-iso-browser-probe.service/start after "
            "anaconda.service/start - after anaconda.target\n"
            "anaconda.service: Job bitriver-iso-browser-probe.service/start "
            "deleted to break ordering cycle\n"
        )
        self.assertTrue(result["browserProbeOrderingCycleDetected"])
        self.assertTrue(result["browserProbeTelemetrySeen"])
        self.assertTrue(result["firefoxStartedUnprivileged"])
        self.assertFalse(result["requiredSmokePassed"])
        self.assertFalse(result["releaseReady"])

    def test_missing_browser_probe_does_not_fake_success(self):
        self.later.write_bytes(b"P6\n2 1\n255\n" + bytes([0, 0, 0, 255, 255, 255]))
        result = self.classify(
            "Reached target basic.target\n"
            "Starting anaconda-pre.service\n"
            "Finished anaconda-pre.service\n"
            "Starting anaconda.service\n"
            "program: Running... /usr/libexec/anaconda/webui-desktop\n"
            "pkexec[2781]: root: Executing command [USER=bitriver-installer] "
            "[COMMAND=/usr/bin/env DISPLAY=:1 /usr/bin/firefox]\n"
        )
        self.assertTrue(result["firefoxStartedUnprivileged"])
        self.assertFalse(result["browserProbeTelemetrySeen"])
        self.assertFalse(result["requiredSmokePassed"])

    def test_display_and_profile_access_detected_without_certifying_install(self):
        self.later.write_bytes(b"P6\n2 1\n255\n" + bytes([0, 0, 0, 255, 255, 255]))
        result = self.classify(
            "Reached target basic.target\n"
            "Starting anaconda-pre.service\n"
            "Finished anaconda-pre.service\n"
            "Starting anaconda.service\n"
            "program: Running... /usr/libexec/anaconda/webui-desktop\n"
            "pkexec[2811]: root: Executing command [USER=bitriver-installer] "
            "[COMMAND=/usr/bin/env DISPLAY=:1 /usr/bin/firefox]\n"
            "BITRIVER_ISO_PROBE sample=8 firefox_unprivileged=yes "
            "xwayland=yes cockpit_http=200 browser_error_count=unavailable "
            "x11_connectable=no x11_windows=unavailable "
            "root_runtime_mode=700 firefox_profile_traversable=no\n"
        )
        self.assertTrue(result["x11AccessProbeSeen"])
        self.assertFalse(result["x11DisplayAccessible"])
        self.assertTrue(result["firefoxProfileAccessProbeSeen"])
        self.assertFalse(result["firefoxProfileTraversable"])
        self.assertTrue(result["requiredSmokePassed"])
        self.assertFalse(result["anacondaUserInterfaceVerified"])
        self.assertFalse(result["releaseReady"])

    def test_display_access_positive_does_not_certify_installer_ui(self):
        result = self.classify(
            "BITRIVER_ISO_PROBE sample=10 firefox_unprivileged=yes "
            "xwayland=yes cockpit_http=200 browser_error_count=0 "
            "x11_connectable=yes x11_windows=3 "
            "root_runtime_mode=711 firefox_profile_traversable=yes\n"
        )
        self.assertTrue(result["x11AccessProbeSeen"])
        self.assertTrue(result["x11DisplayAccessible"])
        self.assertTrue(result["firefoxProfileAccessProbeSeen"])
        self.assertTrue(result["firefoxProfileTraversable"])
        self.assertFalse(result["anacondaUserInterfaceVerified"])
        self.assertFalse(result["releaseReady"])

    def test_firefox_root_process_is_not_accepted(self):
        self.later.write_bytes(b"P6\n2 1\n255\n" + bytes([0, 0, 0, 255, 255, 255]))
        result = self.classify(
            "Reached target basic.target\n"
            "Starting anaconda-pre.service\n"
            "Finished anaconda-pre.service\n"
            "Starting anaconda.service\n"
            "program: Running... /usr/libexec/anaconda/webui-desktop\n"
            "pkexec[2798]: root: Executing command [USER=root] "
            "[COMMAND=/usr/bin/firefox]\n"
        )
        self.assertFalse(result["firefoxStartedUnprivileged"])
        self.assertFalse(result["requiredSmokePassed"])

    def test_slitherer_root_sandbox_crash_and_reboot_are_rejected(self):
        self.later.write_bytes(b"P6\n2 1\n255\n" + bytes([0, 0, 0, 255, 255, 255]))
        result = self.classify(
            "Reached target basic.target\n"
            "Starting anaconda-pre.service\n"
            "Finished anaconda-pre.service\n"
            "Starting anaconda.service\n"
            "program: Running... /usr/libexec/anaconda/webui-desktop -t default\n"
            "anaconda: ui.webui: Using default browser: slitherer\n"
            "Running as root without --no-sandbox is not supported\n"
            "program: Running... systemctl --no-wall reboot\n"
            "System is rebooting.\n"
        )
        self.assertTrue(result["bootPrerequisitePassed"])
        self.assertTrue(result["installerInitiatedReboot"])
        self.assertTrue(result["browserSandboxRefusalSeen"])
        self.assertFalse(result["firefoxBrowserSelected"])
        self.assertFalse(result["requiredSmokePassed"])

    def test_firefox_selection_does_not_excuse_a_reboot(self):
        self.later.write_bytes(b"P6\n2 1\n255\n" + bytes([0, 0, 0, 255, 255, 255]))
        result = self.classify(
            "Reached target basic.target\n"
            "Starting anaconda-pre.service\n"
            "Finished anaconda-pre.service\n"
            "Starting anaconda.service\n"
            "program: Running... /usr/libexec/anaconda/webui-desktop\n"
            "Using default browser: firefox\n"
            "System is rebooting.\n"
        )
        self.assertTrue(result["firefoxBrowserSelected"])
        self.assertTrue(result["installerInitiatedReboot"])
        self.assertFalse(result["requiredSmokePassed"])

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
