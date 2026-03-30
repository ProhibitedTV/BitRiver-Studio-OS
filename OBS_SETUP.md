# OBS Adapter Setup

BitRiver Studio OS talks to OBS through `obs-websocket` 5.x. The control API expects OBS to be running locally on the appliance, with websocket control enabled and protected by a password.

## Local Development

1. Install OBS 30+.
2. Open `Tools` -> `WebSocket Server Settings`.
3. Enable the websocket server.
4. Set the server port to `4455`.
5. Enable authentication and set a password.
6. Start OBS with a scene collection that includes the scenes and inputs referenced by the active BitRiver settings.
7. Run the control API in appliance adapter mode:

```bash
go run ./cmd/bitriverd \
  -adapter-mode appliance \
  -obs-websocket-url ws://127.0.0.1:4455 \
  -obs-websocket-password your-password
```

For local UI work, keep the control API and OBS on the same machine unless you intentionally want to debug LAN routing.

## Required OBS Settings

- OBS 30+ with built-in `obs-websocket` 5.x.
- Websocket server enabled.
- Port `4455` unless the control API is configured otherwise.
- Authentication enabled with a non-empty password.
- A writable recording path configured inside OBS.
- Streaming and recording outputs configured so `StartStream` and `StartRecord` have valid destinations.

Recommended for BitRiver:

- Run OBS with a dedicated BitRiver profile and scene collection.
- Keep scene names stable; BitRiver maps config scene IDs to OBS scene names.
- Keep input names stable; audio route control targets OBS input names.

## Troubleshooting

### Auth fails immediately

Symptoms:

- control API logs show `authentication failed`
- OBS never appears healthy even though it is running

Checks:

- confirm the password passed to `-obs-websocket-password`
- confirm authentication is enabled in OBS
- re-save the websocket password in OBS and restart OBS
- make sure you are talking to OBS 5.x, not an older websocket plugin

### Connection is refused or times out

Symptoms:

- control API logs show dial or handshake failures

Checks:

- confirm OBS is running
- confirm the websocket server is enabled
- confirm the port matches `-obs-websocket-url`
- test locally against `ws://127.0.0.1:4455`
- if using a remote OBS instance, verify firewall rules and routing

### Scene or input actions fail

Symptoms:

- `SetCurrentProgramScene` or audio mute actions return not found errors

Checks:

- confirm the BitRiver settings use the exact OBS scene names
- confirm the audio route names match OBS input names
- avoid renaming OBS scenes or inputs without updating BitRiver settings

### Stream or recording actions fail

Symptoms:

- `StartStream` or `StartRecord` returns an OBS-side error

Checks:

- confirm OBS output settings are fully configured
- confirm the target streaming service is valid
- confirm the recording directory exists and is writable
- check the OBS log for encoder or device failures

### Event-driven state looks stale

Symptoms:

- the UI updates after a delay or only after polling

Checks:

- confirm the control API is running in `appliance` adapter mode
- confirm OBS remains connected long enough for the background session to subscribe
- check control API logs for reconnect loops or repeated websocket disconnects
