# Image Build

`deploy/image` contains the immutable appliance image scaffold for BitRiver Studio OS.

## Build Inputs

- Go control API from `apps/control-api`
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

## Result

The final image is designed to:

- boot as an immutable Fedora `bootc` appliance
- ship the BitRiver control plane and control UI
- install the curated runtime stack for OBS, PipeWire, Chromium, and sway
- enable the BitRiver unit set at boot
