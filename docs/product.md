# Product Guide

The canonical product definition for BitRiver Studio OS lives in [PRODUCT.md](../PRODUCT.md).

This document exists to make the rest of the repo easier to navigate during implementation.

## Read This First

- [PRODUCT.md](../PRODUCT.md) defines the product promise, MVP boundaries, non-goals, risks, and roadmap.
- [ARCHITECTURE.md](../ARCHITECTURE.md) turns that product shape into a system design.
- [docs/api.md](api.md) narrows the control plane contract that both `apps/control-api` and `apps/control-ui` need to honor.

## MVP Translation Into Repository Responsibilities

- `apps/control-api` owns orchestration, health, config lifecycle, and hardware-facing adapters.
- `apps/control-ui` owns browser-first operation on the LAN, including scene control, ingest management, and update UX.
- `packages/shared` owns the typed contract between both.
- `deploy/image` owns immutable image assembly and rootfs overlay assets.
- `deploy/systemd` owns appliance boot and runtime unit definitions.

## Product Guardrails For Contributors

- Keep the MVP appliance-shaped. Do not drift into desktop-distro workflows.
- Keep the LAN browser as the primary operator interface.
- Keep OBS as the authoritative production engine and MediaMTX as the ingest plane.
- Favor safe updates, rollback, and crash recovery over flexibility.
- Treat every feature as something a streamer should trust during a real show.
