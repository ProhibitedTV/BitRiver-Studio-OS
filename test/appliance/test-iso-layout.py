#!/usr/bin/env python3
"""Fixture based on actual generated Fedora 44 generic-iso evidence (run 44238158073)."""
import importlib.util
from pathlib import Path
import unittest

MODULE = Path(__file__).with_name("check-iso-layout.py")
spec = importlib.util.spec_from_file_location("iso_layout_check", MODULE)
module = importlib.util.module_from_spec(spec)
spec.loader.exec_module(module)

BOOT = """Boot record : El Torito , MBR protective-msdos-label grub2-mbr cyl-align-off GPT
El Torito images : 2
El Torito boot img : 1 BIOS y none 0x0000 0x00 4 141
El Torito boot img : 2 UEFI y none 0x0000 0x00 40960 1312007
"""
FILES = """xorriso 1.5.6 : RockRidge filesystem manipulator, libburnia project.
'/EFI/BOOT/BOOTX64.EFI'
'/EFI/BOOT/grub.cfg'
'/LiveOS/squashfs.img'
'/boot/grub2/grub.cfg'
'/images/pxeboot/initrd.img'
'/images/pxeboot/vmlinuz'
"""

class ISOLayoutTests(unittest.TestCase):
    def test_actual_generic_iso_layout(self):
        module.validate(BOOT, FILES)

    def test_reject_no_installer_root_image(self):
        with self.assertRaisesRegex(ValueError, "squashfs"):
            module.validate(BOOT, FILES.replace("'/LiveOS/squashfs.img'", ""))

    def test_reject_no_kernel(self):
        with self.assertRaisesRegex(ValueError, "vmlinuz"):
            module.validate(BOOT, FILES.replace("'/images/pxeboot/vmlinuz'", ""))

    def test_reject_no_initrd(self):
        with self.assertRaisesRegex(ValueError, "initrd"):
            module.validate(BOOT, FILES.replace("'/images/pxeboot/initrd.img'", ""))

    def test_reject_no_uefi(self):
        with self.assertRaisesRegex(AssertionError, "UEFI"):
            module.validate(BOOT.replace("2 UEFI y", "2 NONE y"), FILES)

    def test_reject_no_bios(self):
        with self.assertRaisesRegex(AssertionError, "BIOS"):
            module.validate(BOOT.replace("1 BIOS y", "1 NONE y"), FILES)

if __name__ == "__main__":
    unittest.main()
