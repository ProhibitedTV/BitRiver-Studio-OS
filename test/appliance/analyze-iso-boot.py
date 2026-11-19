#!/usr/bin/env python3
"""Classify isolated ISO UEFI boot evidence without claiming installation.

A solid-color framebuffer can be a splash, a graphical failure, or an
uninitialized UI. Never infer Anaconda readiness from an image hash change.
"""
import hashlib
import json
from pathlib import Path
import re
import sys


ANSI = re.compile(r"\x1b\[[0-9;?]*[ -/]*[@-~]")
PPM_HEADER = re.compile(rb"\AP6\s+([0-9]+)\s+([0-9]+)\s+255\s")


def inspect_ppm(path: Path) -> dict:
    data = path.read_bytes()
    match = PPM_HEADER.match(data[:128])
    if not match:
        raise ValueError(f"invalid QEMU P6 framebuffer: {path.name}")
    width, height = (int(match.group(1)), int(match.group(2)))
    if width <= 0 or height <= 0 or width > 8192 or height > 8192:
        raise ValueError(f"invalid PPM dimensions: {path.name}")
    pixels = data[match.end():]
    if len(pixels) != width * height * 3:
        raise ValueError(f"truncated framebuffer: {path.name}")
    rgb = pixels[:3]
    uniform = pixels == rgb * (width * height)
    return {
        "sha256": hashlib.sha256(data).hexdigest(),
        "solidColor": uniform,
        "solidColorRgb": rgb.hex() if uniform else None,
    }


def classify(early: Path, middle: Path, later: Path, serial: Path) -> dict:
    first, second, third = [inspect_ppm(p) for p in (early, middle, later)]
    raw = serial.read_text(errors="replace")
    log = ANSI.sub("", raw)

    # Require positive systemd activity. Substring checks for basic.target or
    # anaconda-pre.service also match "Stopped", "Failed" and diagnostic
    # messages, which are NOT evidence the live ISO initialized correctly.
    basic = bool(re.search(r"\bReached target basic\.target\b", log))
    pre_started = bool(re.search(r"\bStarting anaconda-pre\.service\b", log))
    pre_finished = bool(re.search(r"\bFinished anaconda-pre\.service\b", log))
    anaconda_started = bool(re.search(r"\bStarting anaconda\.service\b", log))
    prerequisite = basic and pre_started

    return {
        "schemaVersion": 4,
        "uefiVMLaunched": True,
        "earlyFramebufferCaptured": True,
        "middleFramebufferCaptured": True,
        "laterFramebufferCaptured": True,
        "earlyToMiddleFramebufferChanged": first["sha256"] != second["sha256"],
        "middleToLaterFramebufferChanged": second["sha256"] != third["sha256"],
        "finalFramebufferUniform": third["solidColor"],
        "finalFramebufferSolidRgb": third["solidColorRgb"],
        "serialCaptured": bool(raw),
        "systemdBasicTargetSeen": basic,
        "anacondaPreServiceSeen": pre_started,
        "anacondaPreServiceStarted": pre_started,
        "anacondaPreServiceFinished": pre_finished,
        "anacondaMainServiceStarted": anaconda_started,
        "bootPrerequisitePassed": prerequisite,
        "networkConnected": False,
        "anacondaUserInterfaceVerified": False,
        "diskInstallationVerified": False,
        "rebootVerified": False,
        "releaseReady": False,
        "observedDracutBuildstampWarning": ".buildstamp missing" in log,
        "observedEddModuleWarning": "could not insert 'edd'" in log,
        "note": (
            "Only a systemd basic.target reached event and anaconda-pre "
            "START event are required. This does NOT establish that pre-"
            "service finished, Anaconda UI appeared, installation worked, "
            "or the installed OS rebooted. A solid-color final frame "
            "requires visual investigation."
        ),
    }


def main() -> int:
    if len(sys.argv) != 6:
        print(
            "usage: analyze-iso-boot.py early.ppm middle.ppm later.ppm serial.log output.json",
            file=sys.stderr,
        )
        return 2
    early, middle, later, serial, output = map(Path, sys.argv[1:])
    evidence = classify(early, middle, later, serial)
    output.write_text(json.dumps(evidence, indent=2) + "\n")
    print("UEFI systemd and Anaconda pre-service START observed:", evidence["bootPrerequisitePassed"])
    print("Anaconda pre-service FINISHED:", evidence["anacondaPreServiceFinished"])
    print("Anaconda main service STARTED:", evidence["anacondaMainServiceStarted"])
    print("Final framebuffer solid color:", evidence["finalFramebufferUniform"])
    print("Interactive Anaconda UI, disk install and reboot: UNVERIFIED")
    if not evidence["bootPrerequisitePassed"]:
        print("FAIL: insufficient UEFI boot prerequisite evidence", file=sys.stderr)
        return 1
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
