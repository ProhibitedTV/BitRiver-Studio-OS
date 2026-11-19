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


def classify(early: Path, middle: Path, later: Path, serial: Path, virtio: Path | None = None) -> dict:
    first, second, third = [inspect_ppm(p) for p in (early, middle, later)]
    raw = serial.read_text(errors="replace")
    log = ANSI.sub("", raw)
    virtio_text = virtio.read_text(errors="replace") if virtio is not None else ""
    # Do not mistake a firmware-only GRUB log for a failed Linux boot:
    # the last console= is tty0, and serial may stop at GRUB despite booting.
    serial_after_grub = bool(re.search(
        r"Linux version|dracut-initqueue|Reached target|Starting anaconda-pre",
        log, re.I
    ))

    # Require positive systemd activity. Substring checks for basic.target or
    # anaconda-pre.service also match "Stopped", "Failed" and diagnostic
    # messages, which are NOT evidence the live ISO initialized correctly.
    basic = bool(re.search(r"\bReached target basic\.target\b", log))
    pre_started = bool(re.search(r"\bStarting anaconda-pre\.service\b", log))
    pre_finished = bool(re.search(r"\bFinished anaconda-pre\.service\b", log))
    anaconda_started = bool(re.search(r"\bStarting anaconda\.service\b", log))
    prerequisite = basic and pre_started
    # A browser crash and guest reboot must never masquerade as a
    # successful graphical installer just because the framebuffer changed.
    webui_launched = bool(re.search(
        r"Running\.\.\. /usr/libexec/anaconda/webui-desktop\b", log
    ))
    # webui-desktop normally prints browser-selection messages to its own
    # stderr (NOT the serial journal). The guest's pkexec audit record,
    # however, proves Firefox was ACTUALLY exec'd under the installer UID.
    firefox_started_unprivileged = bool(re.search(
        r"pkexec\[\d+\]:.*Executing command \[USER=bitriver-installer\]"
        r".*\[COMMAND=.*?/usr/bin/firefox\b", log
    ))
    firefox_selected = bool(re.search(
        r"Using default browser:\s*firefox\b", log, re.I
    )) or firefox_started_unprivileged
    browser_probe_seen = "BITRIVER_ISO_PROBE sample=" in log
    # Regression from real ISO run 44593573847: systemd discarded the probe
    # because it was WantedBy=anaconda.target but After=anaconda.service.
    browser_probe_cycle = bool(re.search(
        r"anaconda\.service: Found ordering cycle:.*bitriver-iso-browser-probe\.service"
        r"|Job bitriver-iso-browser-probe\.service/start deleted", log
    ))
    browser_running_sample_seen = bool(re.search(
        r"BITRIVER_ISO_PROBE sample=\d+ firefox_unprivileged=yes\b", log
    ))
    sandbox_refused = "Running as root without --no-sandbox is not supported" in log
    reboot_seen = bool(re.search(
        r"\bSystem is rebooting\b|program: Running\.\.\. systemctl --no-wall reboot"
        r"|\bReached target reboot\.target\b", log
    ))
    smoke_passed = (
        prerequisite and pre_finished and anaconda_started
        and webui_launched and firefox_selected and firefox_started_unprivileged
        and browser_probe_seen and not browser_probe_cycle
        and not sandbox_refused and not reboot_seen
        and not third["solidColor"]
    )

    return {
        "schemaVersion": 8,
        "uefiVMLaunched": True,
        "earlyFramebufferCaptured": True,
        "middleFramebufferCaptured": True,
        "laterFramebufferCaptured": True,
        "earlyToMiddleFramebufferChanged": first["sha256"] != second["sha256"],
        "middleToLaterFramebufferChanged": second["sha256"] != third["sha256"],
        "finalFramebufferUniform": third["solidColor"],
        # A nonblank screenshot is necessary, NOT sufficient, for a
        # graphical installer. Never claim GUI or installation succeeded.
        "finalFramebufferNonUniform": not third["solidColor"],
        "finalFramebufferSolidRgb": third["solidColorRgb"],
        "serialCaptured": bool(raw),
        "serialGuestBootMessagesSeen": serial_after_grub,
        "anacondaVirtioLogNonempty": bool(virtio_text.strip()),
        "anacondaVirtioLogBytes": len(virtio_text.encode("utf-8")),
        "systemdBasicTargetSeen": basic,
        "anacondaPreServiceSeen": pre_started,
        "anacondaPreServiceStarted": pre_started,
        "anacondaPreServiceFinished": pre_finished,
        "anacondaMainServiceStarted": anaconda_started,
        "bootPrerequisitePassed": prerequisite,
        "anacondaWebuiDesktopLaunched": webui_launched,
        "firefoxBrowserSelected": firefox_selected,
        "firefoxStartedUnprivileged": firefox_started_unprivileged,
        "browserProbeTelemetrySeen": browser_probe_seen,
        "browserProbeOrderingCycleDetected": browser_probe_cycle,
        "firefoxRunningSampleSeen": browser_running_sample_seen,
        "browserSandboxRefusalSeen": sandbox_refused,
        "installerInitiatedReboot": reboot_seen,
        "requiredSmokePassed": smoke_passed,
        "networkConnected": False,
        "anacondaUserInterfaceVerified": False,
        "diskInstallationVerified": False,
        "rebootVerified": False,
        "releaseReady": False,
        "observedDracutBuildstampWarning": ".buildstamp missing" in log,
        "observedEddModuleWarning": "could not insert 'edd'" in log,
        "note": (
            "Only a systemd basic.target reached event and anaconda-pre "
            "START event are required. A GRUB-only serial log means boot " 
            "progress cannot yet be classified, not that Linux failed. "
            "Virtio Anaconda logs are supplemental diagnostics. This does NOT establish that pre-"
            "service finished, Anaconda UI appeared, installation worked, "
            "or the installed OS rebooted. A solid-color final frame "
            "requires visual investigation."
        ),
    }


def main() -> int:
    if len(sys.argv) not in (6, 7):
        print(
            "usage: analyze-iso-boot.py early.ppm middle.ppm later.ppm serial.log output.json [anaconda-virtio.log]",
            file=sys.stderr,
        )
        return 2
    early, middle, later, serial, output = map(Path, sys.argv[1:6])
    virtio = Path(sys.argv[6]) if len(sys.argv) == 7 else None
    evidence = classify(early, middle, later, serial, virtio)
    output.write_text(json.dumps(evidence, indent=2) + "\n")
    print("UEFI systemd and Anaconda pre-service START observed:", evidence["bootPrerequisitePassed"])
    print("Anaconda pre-service FINISHED:", evidence["anacondaPreServiceFinished"])
    print("Anaconda main service STARTED:", evidence["anacondaMainServiceStarted"])
    print("Final framebuffer solid color:", evidence["finalFramebufferUniform"])
    print("Final framebuffer nonuniform:", evidence["finalFramebufferNonUniform"])
    print("Guest messages on serial after GRUB:", evidence["serialGuestBootMessagesSeen"])
    print("Anaconda virtio log bytes:", evidence["anacondaVirtioLogBytes"])
    print("Firefox selected:", evidence["firefoxBrowserSelected"])
    print("Firefox executed as unprivileged installer:", evidence["firefoxStartedUnprivileged"])
    print("Live browser probe telemetry captured:", evidence["browserProbeTelemetrySeen"])
    print("Browser probe systemd ordering cycle detected:", evidence["browserProbeOrderingCycleDetected"])
    print("Firefox alive in at least one probe sample:", evidence["firefoxRunningSampleSeen"])
    print("Anaconda WebUI launcher seen:", evidence["anacondaWebuiDesktopLaunched"])
    print("Browser sandbox refusal:", evidence["browserSandboxRefusalSeen"])
    print("Installer initiated reboot:", evidence["installerInitiatedReboot"])
    print("Required experimental boot smoke:", evidence["requiredSmokePassed"])
    print("Interactive Anaconda UI, disk install and disk boot: UNVERIFIED")
    if not evidence["requiredSmokePassed"]:
        print(
            "FAIL: nonblank screen plus completed Anaconda pre-service, "
            "Anaconda WebUI, unprivileged Firefox launch, working browser "
            "diagnostics without systemd ordering cycles, and "
            "NO sandbox refusal or installer reboot are required. "
            "Installer UI not certified.",
            file=sys.stderr,
        )
        return 1
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
