# Control API

The canonical machine-readable API contract lives at [packages/shared/openapi/bitriver-control-api.yaml](../packages/shared/openapi/bitriver-control-api.yaml).

## Contract Scope

The control API is the single browser-facing interface for:

- system health and appliance metadata
- show state and operator actions
- source and ingest status
- audio mixer state
- storage visibility
- config draft, validation, and apply lifecycle

Direct access to `obs-websocket`, MediaMTX, Owncast, and future update channels stays behind the control API boundary.

## Endpoint Groups

| Group | Purpose |
| --- | --- |
| `/api/v1/system` | appliance identity, build version, health summary |
| `/api/v1/show` | stream state, record state, active scene, operator controls |
| `/api/v1/sources` | local devices and network ingest slots |
| `/api/v1/audio` | exposed audio channels and mixer actions |
| `/api/v1/storage` | recording path capacity and warnings |
| `/api/v1/config/*` | active config, draft config, validation, apply |

## Authentication

The scaffold keeps auth out of the local development loop so contributors can iterate quickly, but the production target remains the LAN-first session model defined in [ARCHITECTURE.md](../ARCHITECTURE.md):

- first-boot claim flow
- admin password
- same-origin session cookie
- CSRF protection on mutating requests

## Current Scaffold Behavior

- The Go API currently exposes a coherent in-memory appliance model for fast UI and contract iteration.
- Mutating control actions update that in-memory state immediately so the UI can exercise the intended operator flows.
- The event stream transport described in the architecture is reserved but not wired in this first scaffold; the UI polls snapshot endpoints for now.
