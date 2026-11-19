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
- [x] Pin Fedora 44, Go 1.22 and Node 22 OCI builder images to immutable
      manifest digests in the committed lock; build wrapper rejects drift.
- [x] Commit the npm v10 workspace lockfile and install via `npm ci` in
      CI and the OS image build, with registry integrity and workspace checks.
- [ ] Publish a **comprehensive** license inventory/SBOM. UEFI CI now
      validates and retains an OCI-image SPDX inventory generated with Syft,
      but separate component/license attribution and manual review are
      required before public distribution.
- [ ] Validate generated SPDX identifiers and package license obligations,
      including exceptions, and publish a release-specific inventory.
- [x] Pin Image Builder CLI to a reviewed OCI digest rather than the
      floating latest tag. Bootc conversion did not emit its requested SPDX
      documents, so UEFI CI uses a commit-pinned Syft scanner instead.
- [x] Build UEFI QCOW2 and verify firstboot + reboot under OVMF/QEMU,
      including operator key rotation (successful run 44166997005, PR #8).
- [ ] Create a **signed, user-installable production release** and document
      flashing, secure updates and rollback. A separate VirtualBox
      experimental preview is intentionally not a production installer.
- [ ] Evaluate VirtualBox preview VMSVGA graphics and kiosk/OBS functionality
      on an actual VirtualBox host; QEMU covers only headless boot validation.
- [ ] Add signed release artifacts, reproducible release notes and checksums.

## Gate 2: First boot and configuration

- [x] Boot a UEFI x86_64 QEMU VM and confirm bitriver.target and required
      systemd services are healthy (run 44166997005; not physical hardware).
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

## Experimental VirtualBox download (not a production release)

The `.github/workflows/virtualbox-preview.yml` test-gates a **separate**
VirtualBox-native VDI with the established OVMF/QEMU smoke suite, packages
it as a ZIP (numbered pieces only if necessary), and publishes a
**one-time explicitly pre-release** asset after successful testing on
`main`. The public disk never includes the QEMU CI SSH key or unrestricted
sudo access. It does include a documented local preview password and a
narrowly scoped operator-key read allowance for local virtual machine
testing; do not use it on bridged/public networks.

See [VirtualBox testing instructions](VIRTUALBOX_PREVIEW.md).
Passing QEMU headless tests does not certify VirtualBox display, OBS or
physical broadcast hardware.

## UEFI VM validation

The experimental `.github/workflows/appliance-uefi.yml` converts the complete
OCI image into a disposable QCOW2 with Image Builder and boots with OVMF/QEMU.
It provisions a transient CI-only SSH key and account in that disk image,
tests required services, first-boot configuration and credential permissions,
then reboots to check identity/key persistence. The test disk is **never
published**; diagnostics, checksums and validated OCI-image SPDX inventory
are stored as workflow artifacts. This covers what Syft can discover from
the assembled container, not a guaranteed complete accounting of all
third-party source code, licenses, or assets.

UEFI validation passed in workflow run 44166997005, including first boot,
operator key rotation, restart, and preserved credential/identity hashes.
A passing VM test is not hardware certification, a signed installer, or a
rollback demonstration.

## Live authentication regression test on UEFI bootc (proposed)

The isolated OVMF/QEMU appliance smoke now checks the **running** local API,
not only Go handler unit tests: anonymous stream/settings/creator-assets
reads and stream start must return 401; a first-boot guest-only operator
key must login successfully; session cookies must enable authenticated
reads while mutations without CSRF or with a foreign Origin return 403;
rotating the key must invalidate both the prior browser session and old
key. These checks run on a disposable CI disk and never print, upload,
or export the one-time key.

This acceptance test is **pending successful image/UEFI CI validation**.
It verifies the already-assembled bootc appliance, **not** an installation
from the interactive ISO, physical-presence claiming, end-user credential
rotation, or signed rollback. Those release gates remain open.

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


## Fedora 44 base digest verification (2026-11-18)

The previous immutable Fedora bootc base digest became unavailable on Quay
(`manifest unknown`), interrupting fresh OS/ISO/UEFI/VirtualBox builds.
[Digest verification workflow 44315376939](https://github.com/ProhibitedTV/BitRiver-Studio-OS/actions/runs/44315376939)
resolved the official Fedora 44 tag to
`quay.io/fedora/fedora-bootc@sha256:5fdfeee1cb12af2a05d1fd2e4e654c7f605003eb06fa4ca0b4e27079f31c7b6f`,
**pulled that exact digest on linux/amd64**, verified architecture and
`VERSION_ID=44`. This is an immutable replacement, not a mutable release
build or a successful install/boot test. The lock, Containerfile, image builder
wrapper, drift guard and image workflow assertions are rotated atomically.
Full-image and UEFI/VirtualBox validation for the replacement digest now
passed in GitHub Actions on PR #21. This **does not** establish installer
disk installation, update origin/rollback, or physical hardware readiness.

## Installer ISO: proof-of-concept workstream

- [ ] Generate an experimental Fedora 44 offline `generic-iso` installer with
      independent Anaconda boot container and embedded BitRiver OCI payload.
      Initial CI intentionally uploads **only evidence**, not the ISO.
- [ ] Boot ISO under UEFI to a **manual** installer, verify there is no
      unattended repartitioning, and install to a disposable blank VM disk.
- [ ] Detach ISO, reboot into the installed BitRiver OS and verify the
      API/systemd services, firstboot identity and persistent recording paths.
- [ ] Replace the development-only `localhost` payload update target with
      a real version-pinned, signed remote origin; verify safe updates and
      failed-update rollback before publishing installable media.
- [ ] Verify license inventory, signatures and physical hardware matrix.

See [ISO installer roadmap](ISO_INSTALLER_ROADMAP.md). The VirtualBox VDI
preview is **not** evidence that the ISO installation flow is safe.

### Headless authentication test does not require OBS readiness

[UEFI run 44561244894](https://github.com/ProhibitedTV/BitRiver-Studio-OS/actions/runs/44561244894) confirmed anonymous access was rejected (401), and initial login/session lookup both succeeded (200). The test then failed because it expected the authenticated `/api/v1/stream` endpoint to return 200 even though OBS was offline; the service correctly returned **503**. The test now uses the local `/api/v1/settings/active` endpoint as its authenticated-read success oracle. It separately checks unauthorized stream control and leaves the stream offline. New CI must still validate CSRF, rotation, session revocation and reboot. This is **not** proof of a working physical installer or broadcast source.

A full UEFI smoke on merged security PR #26 subsequently passed. Its on-disk ISO installation, user-visible claim flow and signed rollback remain separate open acceptance gates.
