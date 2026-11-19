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
