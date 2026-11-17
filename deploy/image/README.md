# Image Build

`deploy/image` contains the immutable appliance image scaffold for BitRiver Studio OS.

## Build Inputs

- Go control API, `bitriver-hw-report` hardware probe and local `bitriver-operator-key` recovery CLI from `apps/control-api`
- Browser UI from `apps/control-ui`
- systemd units from `deploy/systemd`
- rootfs overlay assets from `deploy/image/rootfs-overlay`
- release archives for MediaMTX and Owncast

## Required Environment

The image build does not fetch random artifacts at appliance boot time. It expects explicit release archive URLs at build time:

- `MEDIAMTX_TARBALL_URL`
- `OWNCAST_TARBALL_URL`

## Example

```bash
powershell -ExecutionPolicy Bypass -File deploy/image/build-image.ps1 `
  -ImageTag ghcr.io/bitriver/studio-os:dev `
  -ContainerRuntime podman
```

## Early Validation

Run `bash test/appliance/check-scaffold.sh` from the repository root. CI builds and runs the hardware report binary, validates its JSON contract, and syntax-checks appliance shell scripts. This is **not** a full image build or UEFI boot test. See [release readiness](../../docs/RELEASE_READINESS.md) for the required release gates.

## Result

The final image is designed to:

- boot as an immutable Fedora `bootc` appliance
- ship the BitRiver control plane and control UI
- install the curated runtime stack for OBS, PipeWire, Chromium, and sway
- enable the BitRiver unit set at boot
