#!/usr/bin/env python3
"""Validate actual xorriso generic-iso output, not old live-installer paths."""
from pathlib import Path
import re
import sys

def validate(boot: str, listing: str) -> None:
    # A generated hybrid installation ISO should boot in UEFI; BIOS is
    # deliberate too. El Torito boot entries are not an installed OS test.
    assert re.search(r"El Torito boot img\s*:\s*\d+\s+UEFI\s+y", boot), "UEFI boot entry missing"
    assert re.search(r"El Torito boot img\s*:\s*\d+\s+BIOS\s+y", boot), "BIOS boot entry missing"
    paths = set()
    for line in listing.splitlines():
        line = line.strip()
        if len(line) > 2 and line.startswith("'/'") and line.endswith("'"):
            paths.add(line[1:-1])
        elif len(line) > 2 and line.startswith("'/") and line.endswith("'"):
            paths.add(line[1:-1])
    for required in (
        "/LiveOS/squashfs.img",
        "/images/pxeboot/vmlinuz",
        "/images/pxeboot/initrd.img",
        "/EFI/BOOT/BOOTX64.EFI",
        "/boot/grub2/grub.cfg",
    ):
        if required not in paths:
            raise ValueError(f"Missing boot-critical ISO file: {required}")
    # Do not silently accept a thin installer which lacks the offline OS.
    # Squashfs existence alone cannot prove an embedded bootc payload.
    print("Hybrid BIOS/UEFI and Fedora generic-iso root/kernel/initramfs layout verified.")
    print("This is structural validation only, NOT proof of installer UI, disk installation or reboot.")

def main() -> int:
    if len(sys.argv) != 3:
        print("Usage: check-iso-layout.py boot-record.txt iso-file-list.txt", file=sys.stderr)
        return 2
    try:
        validate(Path(sys.argv[1]).read_text(), Path(sys.argv[2]).read_text())
    except (ValueError, AssertionError, OSError) as e:
        print(f"ISO structural validation failed: {e}", file=sys.stderr)
        return 1
    return 0

if __name__ == "__main__":
    raise SystemExit(main())
