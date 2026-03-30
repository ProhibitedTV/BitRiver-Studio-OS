# BitRiver Studio OS Health Checks

## Health Model

BitRiver uses two layers of health handling:

- process-level restart policy in systemd
- appliance-level watchdog logic in `bitriver-health-supervisor.service`

The health supervisor writes `/run/bitriver/health/summary.json`, and the control API reads that file into `/api/v1/system`.

## Service Liveness and Readiness

### `bitriver-firstboot.service`

- Liveness: oneshot unit completed successfully.
- Readiness:
  - `/var/lib/bitriver/bootstrap/setup-status.json` exists
  - `/var/lib/bitriver/bootstrap/device-identity.json` exists
  - `/var/lib/bitriver/config/active.json` exists
  - `/var/lib/bitriver/hardware/capabilities.json` exists

If these files are missing, the appliance is not ready to start the control plane.

### `bitriver-control-api.service`

- Liveness:
  - systemd unit is `active`
  - `http://127.0.0.1:8080/healthz` returns `200 ok`
- Readiness:
  - `GET /api/v1/system` returns `200`
  - `GET /api/v1/setup/status` returns `200`
  - setup state is not `pending`

Restart policy:

- `Restart=on-failure`
- `RestartSec=2`
- `StartLimitIntervalSec=120`
- `StartLimitBurst=6`

### `mediamtx.service`

- Liveness: systemd unit is `active`.
- Readiness:
  - MediaMTX process is running
  - `http://127.0.0.1:9997/v3/paths/list` returns `200`

Restart policy:

- `Restart=on-failure`
- `RestartSec=2`

### `owncast.service`

- Optional service.
- Liveness:
  - unit is started only when `/usr/local/libexec/bitriver/owncast-enabled.sh` succeeds
- Readiness:
  - `http://127.0.0.1:8081/` returns `200`

If Owncast is disabled in the active config, the supervisor records that as healthy optional state rather than as a fault.

### `bitriver-update-agent.service`

- Liveness: systemd unit is `active`.
- Readiness:
  - `/var/lib/bitriver/update/status.json` exists
  - the file is refreshed at least once every two polling intervals

The update agent currently reports deployment state and staged-image visibility. It does not apply updates by itself.

### `bitriver-kiosk.service`

- Conditional service.
- Starts only when a display is attached.
- Liveness: unit is `active`.
- Readiness:
  - `sway` is running on `tty1`
  - Chromium is launched in kiosk mode by the sway config

If no display is attached, skipping this service is expected and is reported as degraded headless mode, not as failure.

### `obs.service`

- Conditional service.
- Starts only when a display is attached.
- Liveness: unit is `active`.
- Readiness:
  - compositor session already exists
  - OBS process is connected to the shared Wayland runtime directory

If the display path is missing, the supervisor marks production as degraded and requests operator correction instead of looping the entire appliance.

## Degraded Mode Rules

The appliance is `degraded` when:

- first-boot completed with actionable issues
- no display is attached, so kiosk and OBS are intentionally skipped
- Owncast is enabled but not ready
- update agent state is stale

The appliance is `error` when:

- first-boot state is missing or incomplete
- control API liveness fails
- MediaMTX readiness fails

## Supervisor Actions

`bitriver-health-supervisor.service` loops every 15 seconds and:

1. reads setup status
2. probes API and MediaMTX readiness
3. probes Owncast only when enabled
4. checks whether the local display path should be present
5. requests targeted `systemctl restart` for failed services
6. writes a fresh health summary JSON file

This service is the practical watchdog for the appliance MVP. It is intentionally narrow and restart-oriented rather than trying to implement cluster-style remediation.
