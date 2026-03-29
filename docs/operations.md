# Operations

## Local Development Loop

1. Install dependencies with `make install`.
2. Run the API with `make dev-api`.
3. Run the UI with `make dev-ui`.
4. Use `make smoke` before opening a PR.

## Release Validation

The minimum release checklist for this repo is:

- `make lint`
- `make build`
- `make smoke`
- review `PRODUCT.md` and `ARCHITECTURE.md` deltas for scope drift
- review `packages/shared/openapi/bitriver-control-api.yaml` for contract changes
- review `deploy/systemd` and `deploy/image` together so image and unit assumptions stay aligned

## Image Build Flow

`deploy/image` is set up for a bootc-style image assembly flow:

- compile `bitriverd`
- build the control UI
- layer both into the immutable image
- install the curated runtime packages
- apply the rootfs overlay
- enable the appliance unit set

The image build script expects MediaMTX and Owncast release asset URLs at build time because those binaries are not treated as ad hoc runtime downloads on the appliance.

## Day-2 Appliance Operations Model

- safe updates are staged, then applied on reboot
- rollback is deployment-based, not package-based
- factory reset clears config and secrets, with an explicit choice to preserve or wipe recordings
- support bundles are the default artifact for field debugging

## Logging And Diagnostics

Development and deployment should assume journald as the primary sink for:

- `bitriverd`
- OBS session wrappers
- MediaMTX
- Owncast
- health checks and boot orchestration units
