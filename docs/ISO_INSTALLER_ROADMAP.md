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
4. `.github/workflows/appliance-iso.yml`: attempts the image build on
   relevant PRs and manually. It preserves checksum, layout and provenance
   **only**. It does **not** upload ISO bytes or publish a GitHub Release.
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
directory. The GitHub workflow uploads these metadata files only.

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
