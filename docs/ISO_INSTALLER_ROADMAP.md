# Installer ISO milestone: technical proof of concept

**State: experimental build path. Not install-tested and NOT a public ISO release.**

The VirtualBox `.vdi` preview runs BitRiver from a pre-installed disk. A
consumer `.iso` must instead boot an **installer**, let a user choose an
explicit destination, install BitRiver, provision an operator identity, and
boot the installed disk without the ISO.

## Why a separate ISO image

The unified osbuild `image-builder` CLI supports a `generic-iso` built from
an installer *bootc* container. The BitRiver OS OCI image is separately
embedded as an offline payload with `--bootc-installer-payload-ref`. This
avoids relying on the deprecated `anaconda-iso` path or requiring the
installer to download BitRiver from a network registry.

Sources:
- https://osbuild.org/docs/developer-guide/projects/image-builder/advanced/bootc/isos/
- https://osbuild.org/docs/bootc/deprecation-notice/

This branch introduces:

1. `deploy/iso/Containerfile`: Fedora 44 Anaconda installer environment,
   aligned with the locked BitRiver bootc base, plus its ISO GRUB layout.
2. `deploy/iso/interactive-defaults.ks`: local bootc image source only.
   **No unattended partitioning, destructive default or embedded password**.
3. `test/appliance/build-iso-candidate.sh`: builds the full BitRiver OCI
   payload and independent installer, embeds the payload, generates a
   `generic-iso` and validates ISO boot descriptors and expected files.
4. `.github/workflows/appliance-iso.yml`: attempts the build for
   relevant PRs and stores checksum, layout and boot-probe evidence. PR
   runs **never upload ISO bytes**. A separately opted-in manual run may
   upload short-lived, explicitly unvalidated QA chunks, but never a
   GitHub Release or public installer.
5. `test/appliance/check-iso-scaffold.sh`: basic static guard against
   network-dependent payloads and accidentally unattended installer defaults.

## Running the developer-only build

On a Linux host with Docker, rootful Podman, PowerShell 7, xorriso, and access
to pinned upstream container images:

```bash
pwsh ./deploy/image/build-image.ps1 -ContainerRuntime docker -ImageTag bitriver/studio-os:ci
bash test/appliance/check-iso-scaffold.sh
bash test/appliance/build-iso-candidate.sh
```

The ISO is stored *only* under `artifacts/iso-candidate/` on that host.
Use `ISO-SHA256SUMS`, `boot-record.txt`, and `iso-provenance.json` for
initial structural inspection. The build refuses to reuse a dirty output
directory. Pull-request workflow runs upload these metadata files only. A manual-only QA download workflow is linked below.

**Do not flash or install this image on a machine with valuable data.**
The installer UI, post-install account provisioning, and update behavior
are **not verified**. The temporary target OCI ref
`localhost/bitriver-studio-os:ci` is not a real remote update origin.
The generated image must never be treated as a production deployment.

## Required proof before publishing an ISO artifact

- [ ] Pinned ISO builder actually supports `generic-iso` on Fedora 44;
      experimental CI must generate a valid ISO and preserve build evidence.
- [ ] QEMU UEFI smoke: boot the ISO, reach interactive Anaconda, confirm
      there is **no automatic disk selection or destructive default**.
- [ ] Install to a disposable blank virtual disk (no other attached disks),
      detach ISO and boot from disk under UEFI.
- [ ] Validate installed bootc identity, `bitriver.target`, all required
      services, `/healthz`, writable recordings and shutdown/reboot.
- [ ] First-run operator claim/provisioning without hardcoded demo passwords
      or the disposable QEMU CI sudo/SSH account; credentials survive reboot.
- [ ] Configure a real, version-pinned signed OCI update origin and test
      safe staging, upgrade, failed update rollback and recovery.
- [ ] Smoke both VirtualBox EFI and QEMU EFI; test a second disk, wrong-disk
      cancellation, disconnected network and interrupted installation.
- [ ] Verify Fedora/Anaconda redistribution notices, SPDX/license inventory,
      boot and ISO artifact checksums/signatures, and clean release provenance.
- [ ] Validate actual graphics, audio, USB capture and stream/recovery on
      supported hardware before calling it an official BitRiver release.

Related: [Release Readiness](RELEASE_READINESS.md) and
[Release blocker #4](https://github.com/ProhibitedTV/BitRiver-Studio-OS/issues/4).


### ISO structure evidence update (2026-11-18)

[Workflow 44238158073](https://github.com/ProhibitedTV/BitRiver-Studio-OS/actions/runs/44238158073)
successfully assembled a 2,583 MiB hybrid BIOS/UEFI ISO but failed the
**post-build** root-image check. Inspection of its retained xorriso file list
shows Fedora's actual `/LiveOS/squashfs.img` and
`/images/pxeboot/{vmlinuz,initrd.img}`; earlier grep tests expected
unquoted paths, but xorriso outputs single-quoted file names. A new
fixture-tested parser recognizes the real output and still requires
BIOS+UEFI entries, root image, bootloader, kernel and initrd.

This proves **structure**, not boot, graphical Anaconda readiness, disk
installation, bootc payload completeness or installed-OS reboot. Public
installer publication stays blocked until those checks pass.


### First-stage disposable UEFI launch proof (non-release)

The experimental ISO workflow now attempts to start the candidate under
QEMU/OVMF with **no network**, a new throwaway 32 GiB sparse disk in /tmp
and no host storage passed through. It captures a framebuffer PPM screenshot
and serial log as CI diagnostics for manual review. This first version of
the boot probe is **non-gating**: a candidate may still pass structural CI
without proving Anaconda is visible. The screenshot and serial log must be
inspected before treating it as a boot test.

Do not equate a QEMU process being alive, a screenshot being captured or an
El Torito entry with a successful installation. A second gate must install
to a disposable disk, remove ISO, reboot the installed OS, verify systemd,
API, firstboot and persistent storage, then run rollback testing before
publishing a download as a supported release.

## Evidence-based boot diagnostic (2026-11-18)

The [experimental ISO workflow run 44317503522](https://github.com/ProhibitedTV/BitRiver-Studio-OS/actions/runs/44317503522)
successfully built the ISO and booted it using QEMU/OVMF. Its saved serial
output reached the GRUB menu and `Booting Install BitRiver Studio OS`.
**The framebuffer after 70 seconds did not show Anaconda.** It instead
showed early dracut messages including a missing `.buildstamp` warning
and a failure to insert the `edd` module. These warnings are
**observations, not yet a proven fatal root cause**.

The boot harness now captures an early and a later frame (after 75 and
215 seconds), sends graphical-entry kernel messages to serial, and records
whether the frames differ. It still marks Anaconda UI verification as
**false** until the installer is visibly inspected, and keeps disk
installation / reboot verification false regardless. This prevents
confusing an active QEMU process with a ready Anaconda installation.

## Required QEMU boot prerequisite gate (2026-11-18)

The installer branch now requires the experimental CI boot probe to detect
both `basic.target` and `anaconda-pre.service` on the *real* QEMU/UEFI
guest's serial console, not only a running QEMU process or captured screenshot.
It uses a 64 GiB disposable sparse virtual disk, no external network or
host-storage pass-through, and allows additional boot time under TCG.
The boot probe outputs a machine-readable JSON report and three screenshots.

**This is still not graphical Anaconda UI, disk installation, installed-OS
reboot or tested update rollback.** Those remain hard release blockers.
The manually opted-in QA ISO export is denied if the prerequisite boot
evidence fails; CI still never publishes a production release.

## UEFI evidence audit: boot-start signal versus installer UI (2026-11-18)

[Run 44443213727](https://github.com/ProhibitedTV/BitRiver-Studio-OS/actions/runs/44443213727)
passed the required kernel/systemd/Anaconda-pre **start** gate on the ISO branch.
Inspection of the downloadable diagnostics showed:

- The serial log reached `basic.target` and recorded **Starting**
  `anaconda-pre.service`; it did **not** record that this service finished.
- The screenshot at ~395 seconds was a **single solid-blue frame** rather
  than a visually demonstrated installer UI.
- No complete installation or disk reboot was attempted.

The ISO's second operator-selectable GRUB entry now offers
`inst.text nomodeset rd.plymouth=0 plymouth.enable=0` as an alternative
to the default graphical installer on problematic virtual displays.
This is a **fallback**, not a verified functioning text installer; it does
not change default boot or installed-OS graphics configuration.

The new `analyze-iso-boot.py` classifier and fixture tests distinguish the
pre-service **start** event from **finish**, record blank solid-color frames,
and continue to assert `anacondaUserInterfaceVerified: false`. A green
boot-prerequisite gate is **not** a green installer or release gate.

## Graphical boot console-selection fix (2026-11-18)

The previous generated UEFI GRUB command contained
`inst.graphical console=tty0 console=ttyS0,115200n8`.
This was a **configuration bug**: [Anaconda's documented console selection](https://anaconda-installer.readthedocs.io/en/latest/user-guide/boot-options.html)
uses the last `console=` argument for the installer interface, so the
last serial console could redirect the graphical UI away from QEMU's display.

The default boot entry now deliberately uses
`inst.graphical console=ttyS0,115200n8 console=tty0`, retaining serial boot
messages while selecting the VM display. The opt-in text recovery entry
continues to use serial last. A new test validates **both extracted BIOS
and UEFI GRUB configurations** and rejects reversed console order, a text
default, or unattended install flags. The QEMU boot probe also matches the
8 GiB/4-vCPU VM QA memory and CPU target.

**Expected behavior is not proof:** a new ISO must be built and its framebuffer
reviewed, then Anaconda's actual interactive disk selection, offline install,
installed-disk boot and persistent firstboot verified. A passing boot-prerequisite
check still does not certify those steps.

## Manual QA ISO workflow on default branch

The `main` branch provides a deliberately separate GitHub Actions
[experimental ISO VM QA launcher](https://github.com/ProhibitedTV/BitRiver-Studio-OS/actions/workflows/iso-qa-download.yml).
It checks out this experimental installer branch rather than merging
unvalidated installer tooling to `main`. The operator must explicitly
select `export_qa_iso`; no automatic PR, push, scheduled, or default
dispatch publishes the ISO. It packages chunks and verified hashes
for **isolated disposable-VM testing only**, not a release.

See [VM install acceptance instructions](ISO_VM_INSTALL_QA.md) before
testing. The test ISO still has no verified complete installation,
installed-disk boot or secure update origin.

## Splash-independent ISO launch diagnosis (2026-11-18)

Real ISO CI [run 44451971959](https://github.com/ProhibitedTV/BitRiver-Studio-OS/actions/runs/44451971959)
passed the boot *prerequisite* but its final screenshot was uniformly blue
(`4ab4ed`) even with corrected graphical console ordering. Serial logs
showed `anaconda-pre.service`, `plymouth-quit.service` and
`plymouth-quit-wait.service` starting without a completed Anaconda startup.
This is not enough to conclude that Plymouth caused the delay; software TCG
emulation might also explain it.

The next diagnostic iteration disables the *experimental ISO graphical boot*
splash via `rd.plymouth=0 plymouth.enable=0` but retains `inst.graphical`.
The last QEMU screenshot moves from ~395 to ~615 seconds; installer UI,
offline disk installation and installed-disk reboot remain unverified.

## Fedora bootc locked-image recovery (2026-11-19)

The previously reviewed Fedora 44 digest `sha256:5fdfeee1cb12af2a05d1fd2e4e654c7f605003eb06fa4ca0b4e27079f31c7b6f` returned
`manifest unknown` on fresh GitHub-hosted runners, blocking the VirtualBox,
QEMU and ISO builds before the installer boot probe could run. This is an
**upstream manifest-availability failure**, not installer UI proof.

A read-only [Fedora base probe run 44556792647](https://github.com/ProhibitedTV/BitRiver-Studio-OS/actions/runs/44556792647)
resolved Fedora 44 to `sha256:21be546e96dd831f5c92ecd0c84ea54205b2659a1724d1efa0b24ba465bc1556`, pulled the exact
immutable image on `linux/amd64`, and verified `VERSION_ID=44`.
The reviewed digest is now used consistently by the lockfile, both Fedora
Containerfiles, PowerShell guard and the archive verifier. No mutable-tag
fallback was introduced. Installation, reboot and stable release remain
blocked pending fresh CI and actual disposable-VM acceptance evidence.

## Explicit graphical Web UI dependencies (2026-11-19)

In [failed ISO boot run 44562080450](https://github.com/ProhibitedTV/BitRiver-Studio-OS/actions/runs/44562080450), Fedora 44 successfully started `anaconda.service`, launched GNOME Kiosk under Wayland, found the 64 GiB disposable disk and finished the storage initialization thread. The final 615-second framebuffer nevertheless remained **uniform blue** (`4ab4ed`), and the Anaconda virtio log was empty. The serial log did **not** demonstrate a working installer page. It showed software GL fallback on the Bochs virtual GPU and repeated brltty mount warnings; neither is conclusively responsible for the blank screen.

The installer Containerfile previously requested `anaconda` and `anaconda-install-img-deps` but never expressly requested Fedora's **`anaconda-webui` graphical frontend or a browser**. Fedora documents this frontend as a separate package, with a browser required in custom ISO environments. The next build now installs `anaconda-webui` and `firefox` explicitly and fails its image build if `/usr/libexec/anaconda/webui-desktop` or the browser is absent. This is a testable dependency correction, **not a verified fix for the blue screen**. The nonblank-screen release gate remains in force. See [Fedora WebUI packaging](https://packages.fedoraproject.org/pkgs/anaconda-webui/anaconda-webui/fedora-44.html) and [Anaconda customization guidance](https://anaconda-installer.readthedocs.io/en/latest/anaconda-webui/docs/customization.html).

## Fedora 44 browser crash isolated (2026-11-19)

Although [experimental ISO workflow run 44581097335](https://github.com/ProhibitedTV/BitRiver-Studio-OS/actions/runs/44581097335) was green, its **actual last framebuffer showed a systemd reboot**, not the installer. The serial log confirms that `webui-desktop` selected **Slitherer**, a Chromium-based browser, launched it as root, and received `Running as root without --no-sandbox is not supported`. Anaconda then invoked `systemctl --no-wall reboot`. No installation took place.

The next live ISO selects **Firefox** through Anaconda's documented `[User Interface] webui_web_engine` configuration and provides a dedicated **non-root browser account**. The installer does not disable Chromium sandboxing. The required isolated UEFI smoke now rejects guest-initiated reboot or sandbox refusal, and requires a completed Anaconda pre-service, main Anaconda launch, Web UI launcher event, explicit Firefox selection and nonuniform final framebuffer. These are **diagnostics only**, not verification of an interactive disk-selection UI.

Reference: https://anaconda-installer.readthedocs.io/en/latest/anaconda-webui/docs/customization.html

## Firefox process verification and browser/display probe (2026-11-19)

Actual guest serial evidence from failed candidate 44588274146 shows pkexec launched
/usr/bin/firefox as dedicated bitriver-installer UID 970; Cockpit's Web UI
service also started. But webui-desktop writes its browser-selection text
to output not captured on serial. The previous classifier incorrectly
marked Firefox as absent. The final framebuffer was still solid blue.

The boot classifier now recognizes the unprivileged pkexec Firefox event
and refuses to treat a selection message or root-owned browser as proof.
A read-only diagnostic unit in the live ISO samples Firefox process status,
Xwayland presence, loopback-only Cockpit HTTP response status and the count
of errors in a local browser log. It emits fixed fields on serial without
printing user input, raw logs, browser arguments or credentials. The unit
cannot repartition or install and does not create a network connection.

Working visual installation, disk selection, offline disk install and
booting the installed system are STILL UNVERIFIED. This diagnostic change
does not alter the release safety gates.

Evidence: https://github.com/ProhibitedTV/BitRiver-Studio-OS/actions/runs/44588274146
