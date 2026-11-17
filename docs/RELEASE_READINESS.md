# Official Release Readiness

**Status: pre-release scaffold.** Passing application CI is not proof that a bootc
appliance image can boot or operate a real live stream. A public release should
not be tagged until the gates below have evidence (run logs, tested hardware,
and reproducible images). The hardware probe work is one concrete step.

## Gate 1: Reproducible appliance image

- [x] Implement and package the hardware-report binary required by first boot.
- [x] Exercise hardware-report JSON on CI and fixture-based discovery tests.
- [x] Syntax-check all appliance shell scripts in CI.
- [x] Build the **complete bootc OCI image** in CI with upstream media archives
      (successful image run 44156876970 on Fedora 44, PR #7).
- [x] Pin verified MediaMTX/Owncast archive releases by SHA256 and validate
      bytes/format in CI (MediaMTX tar.gz; Owncast ZIP).
- [ ] Pin the Fedora base **digest in the repository**, not just the
      release line; the build wrapper resolves and records an immutable digest.
- [ ] Commit npm dependency locks and account for all builder image digests.
- [ ] Publish a full license inventory/SBOM; Fedora and bundled dependencies
      have licenses beyond the two upstream MIT application licenses.
- [ ] Generate UEFI QCOW2 and exercise firstboot + reboot under QEMU via
      the new appliance-uefi workflow; keep unchecked until a run passes.
- [ ] Create a user-installable release artifact, document flashing, and
      verify secure update and rollback behavior.
- [ ] Add signed release artifacts, reproducible release notes and checksums.

## Gate 2: First boot and configuration

- [ ] Boot an actual UEFI x86_64 machine/VM from the produced image and
      confirm systemd reaches bitriver.target without failed required units.
- [ ] Verify first-boot identity, hardware inventory, initial config, ownership
      and expected permissions in /var/lib/bitriver and /data/recordings.
- [ ] Verify reboot retains identities and operator configuration.
- [ ] Verify headless mode degrades gracefully rather than claiming OBS ready.
- [ ] Verify display-attached Chromium kiosk and OBS can share Wayland and
      PipeWire runtime reliably after a reboot and application crash.

## Gate 3: Safety and security (release blockers)

- [x] Pre-release local operator key, session cookie, CSRF, origin checking and
      fail-closed appliance binding implemented (not a complete claim flow).
- [x] Local operator key rotation and per-request session revocation implemented;
      must still validate file ownership and recovery on the installed image.
- [ ] Implement and test real admin claim/authentication, session protection
      and authorization on all mutating control endpoints; generated initial
      passwords alone do not secure the current API.
- [ ] Keep obs-websocket and media-management ports bound to appropriate
      interfaces and protect stream keys and configuration secrets.
- [ ] Prevent untrusted Browser Source content from executing in the admin
      origin and review bundled third-party dependencies.
- [ ] Disallow OS updates/reboot during live recording or streaming.
- [ ] Test rollback from failed config and image upgrades.

## Gate 4: End-to-end live production

- [ ] Test certified UVC camera/capture device + USB microphone.
- [ ] Demonstrate a 30+ minute stable 1080p stream with local MKV recording.
- [ ] Verify source recovery, operator-visible health and recording durability
      after simulated crashes and network disconnection.
- [ ] Validate remote RTMP/SRT contributor paths and Owncast as a destination.
- [ ] Test low-storage behavior and recording retention safeguards.

## Gate 5: Creator-facing workflows

- [x] Optional OBS asset catalog and preview browser (read-only).
- [ ] Safe one-click asset installation and rollback through obs-websocket.
- [ ] Test complete brand/show preset setup without manually editing OBS files.
- [ ] Add optional media library, AI and Discord integrations only behind
      explicit configuration and permission boundaries.
- [ ] Keep desktop gaming/Steam isolated from dependable Broadcast Mode.

## UEFI VM validation

The experimental `.github/workflows/appliance-uefi.yml` converts the complete
OCI image into a disposable QCOW2 with Image Builder and boots with OVMF/QEMU.
It provisions a transient CI-only SSH key and account in that disk image,
tests required services, first-boot configuration and credential permissions,
then reboots to check identity/key persistence. The test disk is **never
published**; diagnostics and checksums are stored as workflow artifacts.

This validation still needs a **successful workflow run** before any UEFI gate
can be checked. A passing VM test is not a hardware certification, signed
installer, or rollback demonstration.

## First-boot hardware probe

The image now builds /usr/local/bin/bitriver-hw-report from the Go source.
The first-boot service runs it with --output to write capabilities.json.
It discovers Linux CPU/memory, connected DRM displays, video and ALSA
capture device nodes, and network interfaces.

Capability flags are **heuristics, not hardware certification**: GPU encoding,
video capture compatibility, audio routing and runtime stability still need
testing on supported physical machines.

Developers can validate without the image build:

    bash test/appliance/check-scaffold.sh

This checks shell syntax, image wiring and the generated JSON report. It does
not boot the image, download media archives, or test any broadcast hardware.
