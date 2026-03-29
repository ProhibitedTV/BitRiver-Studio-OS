# Architecture Guide

The canonical runtime design for BitRiver Studio OS lives in [ARCHITECTURE.md](../ARCHITECTURE.md).

This companion guide maps that system design into the repo structure.

## Runtime To Repository Map

| Runtime concern | Repository location |
| --- | --- |
| Control API and orchestration | `apps/control-api/` |
| Browser control surface | `apps/control-ui/` |
| Shared API contract | `packages/shared/` |
| bootc image assembly | `deploy/image/` |
| Unit files and boot targets | `deploy/systemd/` |
| Sample operator configs | `config/examples/` |
| Runbooks and support docs | `docs/` |

## Immediate Architectural Priorities

1. Replace in-memory appliance state in `apps/control-api` with persisted config generations backed by SQLite.
2. Add adapter packages for OBS, MediaMTX, PipeWire, Owncast, and update orchestration behind explicit interfaces.
3. Move the UI from snapshot polling to the event transport described in the architecture.
4. Turn `deploy/image` into a reproducible image pipeline that installs the curated runtime stack and enables the units in `deploy/systemd`.

## Non-Negotiable Boundaries

- The control plane must not become the media plane.
- The appliance image must remain immutable-first.
- Systemd, not ad hoc shell orchestration, owns service ordering.
- The local API contract must be treated as a versioned product surface.
