# BitRiver Studio OS Media Plane

## Why MediaMTX

MediaMTX is the right MVP media router for BitRiver Studio OS because it solves the hard appliance problems we actually have:

- stable local ingest endpoints for OBS and remote contributors
- protocol support for RTMP and SRT without inventing custom transport
- a local HTTP API for path and session visibility
- path-based recording at the router layer
- predictable restart behavior under `systemd`

BitRiver does not use MediaMTX as a replacement for OBS. OBS remains the production switcher and the authoritative program encoder. MediaMTX owns ingest, preview fanout, downstream restream coordination, and router-level health.

## MVP Flow

### Ingest

OBS publishes the composed program locally into MediaMTX on the configured ingest path, defaulting to `program-live`.

- OBS local publish target: `rtmp://127.0.0.1:1935/program-live`
- remote contributor slots: named MediaMTX paths such as `guest-1`
- MediaMTX API stays on `127.0.0.1:9997`

That gives BitRiver one stable router boundary for both local program ingress and guest inputs.

### Preview

MediaMTX exposes a dedicated preview path, defaulting to `program-preview`, that points back at the ingest path over appliance-local RTSP.

- local preview path source: `rtsp://127.0.0.1:8554/program-live`
- optional WebRTC preview can be enabled for browser-friendly low-latency readers
- RTSP preview remains the baseline transport for internal appliance wiring and troubleshooting

Keeping preview separate from ingest lets the UI talk about a preview endpoint explicitly without overloading the publish path.

### Restream

MediaMTX remains the router of record, but for MVP it uses a small `ffmpeg` helper process to fan out the live program to downstream RTMP and SRT destinations.

- MediaMTX launches the helper with `runOnReady` when the program ingest path becomes ready
- the helper reads a managed JSON sidecar generated from the BitRiver settings model
- enabled destinations are published with stream copy rather than re-encode
- if the program ingest disappears, MediaMTX terminates the helper with the path lifecycle

This keeps routing ownership in MediaMTX while avoiding a premature custom egress subsystem.

### Recording

The router records the incoming program path locally at the MediaMTX layer.

- default recording root: `/data/recordings/program`
- generated record pattern: `/data/recordings/program/%path/%Y-%m-%d_%H-%M-%S-%f`
- OBS still owns operator-visible program recording state and current file reporting

For MVP, this split is deliberate:

- MediaMTX recording protects the router boundary and gives us a clean appliance-side archive path
- OBS recording remains the operator-facing control because it already integrates with scene switching and the existing UI actions

## Managed Configuration

BitRiver renders `mediamtx.yml` from the versioned control config.

The managed model includes:

- `localIngestPath`
- `localPreviewPath`
- `recordingPath`
- `publishDestinations`
- protocol-specific RTMP and SRT destinations
- `webrtcPreviewEnabled`

On apply, BitRiver:

1. validates the media-router config model
2. renders `mediamtx.yml` plus a destinations sidecar JSON file
3. writes both atomically
4. attempts `systemctl reload mediamtx.service`
5. falls back to `systemctl restart mediamtx.service`
6. restores the previous files if service control fails

That is the contract for safe router updates in MVP.

## What Stays Out Of Scope

These are intentionally not in the MVP media plane:

- free-form dynamic path creation from the browser
- per-destination delivery analytics from remote platforms
- automatic stream-key secret brokering
- router-side transcoding ladders
- cloud relay control
- browser multiview as a production-confidence monitor
- full PipeWire graph authoring in the UI

The goal is a fast, honest appliance demo: stable ingest, visible router state, local recording, and one-to-many downstream fanout without pretending we already built a full broadcast router. 
