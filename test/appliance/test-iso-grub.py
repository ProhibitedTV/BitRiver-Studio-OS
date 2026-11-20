#!/usr/bin/env python3
"""Regression tests for GUI/serial installer boot-menu ordering."""
import importlib.util
from pathlib import Path
import unittest


source = Path(__file__).with_name("check-iso-grub.py")
spec = importlib.util.spec_from_file_location("check_iso_grub", source)
grub = importlib.util.module_from_spec(spec)
spec.loader.exec_module(grub)


def example(graphic_consoles="console=ttyS0,115200n8 console=tty0", text_extra=""):
    return (
        "set timeout=8\n"
        "menuentry 'Install BitRiver Studio OS (experimental)' --class fedora {\n"
        f"  linux /images/pxeboot/vmlinuz root=live:CDLABEL=BITRIVER-ISO rd.live.image inst.graphical rd.plymouth=0 plymouth.enable=0 {graphic_consoles}\n"
        "  initrd /images/pxeboot/initrd.img\n"
        "}\n"
        "menuentry 'Troubleshoot installation (text, basic display)' --class fedora {\n"
        f"  linux /images/pxeboot/vmlinuz root=live:CDLABEL=BITRIVER-ISO inst.text nomodeset {text_extra}console=ttyS0,115200n8\n"
        "  initrd /images/pxeboot/initrd.img\n"
        "}\n"
    )


class ISOGrubTests(unittest.TestCase):
    def test_expected_graphical_and_text_modes(self):
        grub.validate(example())

    def test_missing_graphical_plymouth_bypass_rejected(self):
        with self.assertRaisesRegex(ValueError, "Plymouth splash bypass"):
            grub.validate(example().replace("plymouth.enable=0 ", "", 1))

    def test_reverse_console_order_rejected(self):
        with self.assertRaisesRegex(ValueError, "tty0 last"):
            grub.validate(example("console=tty0 console=ttyS0,115200n8"))

    def test_serial_only_gui_rejected(self):
        with self.assertRaisesRegex(ValueError, "tty0 last"):
            grub.validate(example("console=ttyS0,115200n8"))

    def test_missing_graphical_mode_rejected(self):
        with self.assertRaisesRegex(ValueError, "inst.graphical"):
            grub.validate(example().replace("inst.graphical", "inst.text", 1))

    def test_default_text_first_rejected(self):
        base = example()
        first, second = base.split("menuentry 'Troubleshoot installation", 1)
        header, rest = first.split("menuentry 'Install", 1)
        with self.assertRaisesRegex(ValueError, "default boot entry"):
            grub.validate(header + "menuentry 'Troubleshoot installation" + second +
                          "menuentry 'Install" + rest)

    def test_kickstart_autoinstall_rejected(self):
        with self.assertRaisesRegex(ValueError, "unattended"):
            grub.validate(example().replace("inst.graphical ", "inst.graphical inst.ks=http://example.invalid/qa.ks ", 1))

    def test_text_fallback_requires_serial_console(self):
        with self.assertRaisesRegex(ValueError, "text fallback must"):
            grub.validate(example().replace(
                "nomodeset console=ttyS0,115200n8",
                "nomodeset console=tty0",
            ))


if __name__ == "__main__":
    unittest.main()
