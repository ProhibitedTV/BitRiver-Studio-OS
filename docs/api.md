# Control API

The canonical machine-readable contract is generated from the Go API package into [packages/shared/openapi/bitriver-control-api.yaml](../packages/shared/openapi/bitriver-control-api.yaml).

## Scope

The control API is the single browser-facing boundary for:

- appliance system and health state
- first-boot setup visibility
- stream, recording, scene, and audio control actions
- MediaMTX-backed source availability
- MediaMTX router configuration, active paths, and session state
- active and draft settings generations
- update status and staged deployment visibility

OBS, MediaMTX, Owncast, and systemd stay behind this local API boundary.

## Endpoint Groups

| Group | Purpose |
| --- | --- |
| `/api/v1/system` | appliance identity, runtime metrics, service status |
| `/api/v1/health` | aggregated appliance health |
| `/api/v1/setup/status` | first-boot, credentials, display mode, hardware report |
| `/api/v1/stream` | stream state plus start/stop actions |
| `/api/v1/outputs` | current destination status, including optional Owncast |
| `/api/v1/media-router` | rendered MediaMTX state, active paths, and router sessions |
| `/api/v1/recordings` | recording state, current file, recording path capacity |
| `/api/v1/scenes` | configured scenes and active scene switching |
| `/api/v1/sources` | local capture availability and network ingest readiness |
| `/api/v1/audio/routes` | audio route state plus mute/unmute actions |
| `/api/v1/settings/*` | active settings, draft, validation, apply |
| `/api/v1/update` | deployment and staged update visibility |
| `/api/v1/ws` | websocket event stream for live snapshots and mutations |

## Event Stream

`/api/v1/ws` is the live state transport for the browser control surface.

- The server sends an initial full snapshot immediately after the websocket upgrade succeeds.
- The controller publishes full-state events on a timer and after mutating actions.
- The OBS adapter also publishes normalized `obs.*` events when its background session is connected.
- UI consumers should treat OBS events as normalized application events, never as raw obs-websocket payloads.

## Settings Model

Settings are file-backed and schema-versioned.

- `active.json` and `draft.json` are persisted on disk
- the stored document carries `schemaVersion`, `generation`, and `updatedAt`
- the API returns settings as `{ schemaVersion, generation, updatedAt, config }`
- legacy plain config files are accepted and normalized on first load
- the normalized config now includes a `mediaRouter` block that renders the managed `mediamtx.yml`

## Dev And CI Mode

The API can run with mocked adapters so CI and local development do not require OBS or MediaMTX.

- `-adapter-mode mock` is the default
- `-adapter-mode appliance` switches to the real obs-websocket, MediaMTX, Owncast, and systemd adapters
