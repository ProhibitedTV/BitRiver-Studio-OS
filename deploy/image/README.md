# Image Build

`deploy/image` contains the immutable appliance image scaffold for BitRiver Studio OS.

## Build Inputs

- Go control API, `bitriver-hw-report` hardware probe and local `bitriver-operator-key` recovery CLI from `apps/control-api`
- Browser UI from `apps/control-ui`
- systemd units from `deploy/systemd`
- rootfs overlay assets from `deploy/image/rootfs-overlay`
- release archives for MediaMTX and Owncast

## Locked Image Inputs

The build reads [`releases.lock.json`](releases.lock.json). This file pins
exact upstream URLs and **SHA-256 digests** for the x86_64 media binaries:

- MediaMTX v1.21.4, official Linux amd64 TAR.GZ (MIT license).
- Owncast v0.3.0, official Linux 64-bit ZIP (MIT license).
- Fedora bootc 44, a supported Fedora base release line.

The Containerfile checks each archive's SHA256 **before extracting it** and
refuses missing values. Owncast is deliberately unzipped, not extracted as a
tarball. The local preflight downloads both release assets and checks their
hashes and archive member names:

```bash
bash test/appliance/check-release-assets.sh
```

The checked-in release lock now pins **all three OCI image stages** by manifest
digest: Fedora 44, Go 1.22/Bookworm and Node 22/Bookworm. These exact
references were captured from the successful Fedora bootc image workflow run
44166997083. The build wrapper rejects any unreviewed base-image override and
records the immutable references in its provenance file.

The digests prevent a tag being silently redirected to different image bytes,
but the build is **not bit-for-bit reproducible**: Fedora RPM repositories,
the CI-only image-builder CLI and Fedora RPM repositories still need
independent pinning and verification. The UI now uses a committed npm v10
lockfile with SHA-512 registry integrity hashes and clean `npm ci` installs.
Upgrade these images deliberately by editing the lock and its validation tests.

## Locked JavaScript Workspaces

`package-lock.json` is checked in for the root, the control UI and shared
contracts. Both normal GitHub Actions CI and the OCI Node build stage run:

```bash
npm ci --ignore-scripts --no-audit --no-fund
```

`npm ci` rejects mismatches between workspace manifests and the lock instead
of silently rewriting dependency versions. CI also verifies all registry
entries have pinned package tarball URLs and SHA-512 integrity digests.
`--ignore-scripts` avoids executing arbitrary npm install lifecycle hooks;
actual UI/TypeScript builds run separately afterward. To intentionally update
dependencies, regenerate and review the **root** lockfile and rerun all CI
and UEFI appliance validations. Do not commit nested workspace lockfiles.

## Build Example (on Linux)

```powershell
./deploy/image/build-image.ps1 -ContainerRuntime docker -ImageTag bitriver/studio-os:dev -MetadataPath artifacts/image-build-provenance.json
```

Or with Podman:

```powershell
./deploy/image/build-image.ps1 -ContainerRuntime podman -ImageTag bitriver/studio-os:dev
```

Requires an OCI container engine, PowerShell 7, git, and internet access to the
pinned registries. This only builds a **bootc OCI container image**, not a
flashable disk image. Neither command installs or boots an appliance.

## Automated Image Validation

[appliance-image workflow](../../.github/workflows/appliance-image.yml)
attempts the full image build on image-related PRs or manual invocation. It
checks the release archives first and publishes a short, non-release build
provenance JSON with resolved base digest, local image ID, and exact inputs.
It **does not push**, sign, release, produce a UEFI image, or certify a boot.

The normal CI workflow checks app builds and pinned release archive inputs;
`bash test/appliance/check-scaffold.sh` remains a faster offline-friendly
check of hardware-report wiring and shell syntax.

## UEFI VM smoke-test pipeline (non-release)

The separate [appliance-uefi workflow](../../.github/workflows/appliance-uefi.yml)
builds the OCI image, converts a **local copy** into an x86_64 QCOW2 disk
using Image Builder, then boots the disk with OVMF firmware under QEMU. It
checks the required headless services, first-boot setup JSON, hardware report,
local `/healthz`, operator credentials file permissions, persistent recording
directory, and identity/key preservation after a complete VM reboot.

The VM-only image contains an **ephemeral authorized SSH public key** and a
temporary passwordless-sudo test account for CI inspection. This customization
is injected into the QCOW2 at conversion time and **does not exist in the
shipping OCI image**. **Do not redistribute the CI QCOW2**: it is deliberately
not uploaded as a release artifact. Only a disk SHA-256 and test diagnostics
are retained for 14 days.

The conversion is performed with:

```bash
bash test/appliance/build-qcow2.sh
bash test/appliance/smoke-uefi-qcow2.sh
```

These scripts require a container image already built as
`bitriver/studio-os:ci`, rootful Podman, Docker, QEMU, OVMF, SSH, jq,
`qemu-img`, and a disposable SSH keypair in `artifacts/vm/`. They intentionally
fail rather than inject a static password or quietly skip firstboot tests.

For the appliance itself, `/data` points at persistent
`/var/lib/bitriver/data` because bootc treats the base image as immutable.
`openssh-server` starts with root login and password authentication disabled;
a real device still needs user-owned SSH public key provisioning before
remote login is possible.

**Validation boundary:** a passing UEFI VM check does not validate physical
USB capture devices, kiosk/Wayland display, long-duration streaming, secure
updates or rollback. Neither a CI QCOW2 nor an OCI container is a signed,
installable **consumer release**.

## Licenses

Upstream application sources are available under the MIT license:
[MediaMTX](https://github.com/bluenviron/mediamtx/blob/v1.21.4/LICENSE)
and [Owncast](https://github.com/owncast/owncast/blob/v0.3.0/LICENSE).
Fedora base packages and dependencies have their own licenses. Before public
distribution, publish the full generated package/SBOM license inventory
(including bundled Owncast assets and MediaMTX dependencies). MIT labels here
do **not** imply the entire Fedora-based image is MIT-licensed.

## Result

The final image is designed to:

- boot as an immutable Fedora `bootc` appliance
- ship the BitRiver control plane and control UI
- install the curated runtime stack for OBS, PipeWire, Chromium, and sway
- enable the BitRiver unit set at boot
