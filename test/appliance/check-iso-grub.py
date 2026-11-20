#!/usr/bin/env python3
"""Fail closed on boot-menu regressions in the actual generated ISO.

Anaconda uses the LAST console= argument for installer interaction.
The graphical entry must end in tty0, while the opt-in text fallback uses
the serial console; otherwise a VM can appear stuck on a blank framebuffer.
"""
from pathlib import Path
import re
import sys


MENU = re.compile(r"menuentry\s+'([^']+)'\s+[^{]*\{([^}]*)\}", re.S)
LINUX = re.compile(r"(?m)^\s*linux(?:efi)?\s+(.+)$")
CONSOLE = re.compile(r"(?:^|\s)console=([^\s]+)")
DANGEROUS = re.compile(
    r"(?<!\S)(?:inst\.ks=|inst\.cmdline(?:\s|$)|inst\.noninteractive(?:\s|$)|"
    r"rd\.live\.overlay\.reset(?:\s|$))"
)


def validate(grub: str) -> None:
    entries = MENU.findall(grub)
    if len(entries) < 2:
        raise ValueError("expected an explicit interactive GUI and opt-in text fallback")
    title, body = entries[0]
    if title != "Install BitRiver Studio OS (experimental)":
        raise ValueError("default boot entry is not the graphical installer")
    match = LINUX.search(body)
    if match is None:
        raise ValueError("graphical boot entry missing kernel command")
    graphical = match.group(1)
    if "inst.graphical" not in graphical.split():
        raise ValueError("default installer missing inst.graphical")
    if "inst.text" in graphical.split() or "nomodeset" in graphical.split():
        raise ValueError("default graphical boot redirected to text/basic display")
    if not {"rd.plymouth=0", "plymouth.enable=0"}.issubset(set(graphical.split())):
        raise ValueError("graphical installer missing diagnostic Plymouth splash bypass")
    if CONSOLE.findall(graphical) != ["ttyS0,115200n8", "tty0"]:
        raise ValueError("graphical installer MUST have serial first and tty0 last")
    if DANGEROUS.search(graphical):
        raise ValueError("automatic/unattended installer setting in graphical boot entry")

    fallback_title, fallback_body = entries[1]
    if fallback_title != "Troubleshoot installation (text, basic display)":
        raise ValueError("second boot entry is not the opt-in text fallback")
    match = LINUX.search(fallback_body)
    if match is None:
        raise ValueError("text fallback missing kernel command")
    fallback = match.group(1)
    if "inst.text" not in fallback.split() or "inst.graphical" in fallback.split():
        raise ValueError("text fallback is not explicitly text-only")
    if CONSOLE.findall(fallback) != ["ttyS0,115200n8"]:
        raise ValueError("text fallback must use the serial console")
    if DANGEROUS.search(fallback):
        raise ValueError("automatic/unattended installer setting in text boot entry")
    print("Verified ISO GRUB console order: graphical tty0 LAST, serial text fallback only.")


def main() -> int:
    if len(sys.argv) < 2:
        print("usage: check-iso-grub.py extracted-grub.cfg [other-grub.cfg ...]", file=sys.stderr)
        return 2
    try:
        for filename in sys.argv[1:]:
            path = Path(filename)
            validate(path.read_text())
            print(f"Passed {path.name}")
    except (OSError, ValueError) as exc:
        print(f"ISO GRUB validation failed: {exc}", file=sys.stderr)
        return 1
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
