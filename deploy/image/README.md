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

The Fedora `44` tag itself is mutable. The build wrapper pulls it and resolves
a **specific registry manifest digest** before invoking the Containerfile.
The resolved digest is recorded in the provenance file. To reproduce a build,
pass the previously recorded immutable reference via `-BaseImageDigest`.
A future task must additionally pin the base digest in the repository and
lock npm builder dependencies to eliminate remaining supply-chain drift.

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
