# BitRiver Studio OS Boot Flow

## Boot Objective

On a supported mini PC, the machine should come up as a BitRiver appliance, not as a general desktop. The product experience is:

- headless on the LAN when no display is attached
- local fullscreen BitRiver control screen on `tty1` when a display is attached

In both modes the system boots straight into the appliance service graph. There is no login manager, desktop shell, or traditional user workflow.

## Startup Sequence

1. UEFI boots the selected immutable deployment.
2. The kernel mounts the immutable root plus persistent state under `/var` and recordings under `/data`.
3. `systemd-sysusers` and `systemd-tmpfiles` create:
   - `bitriver` and `studio` users
   - `/var/lib/bitriver/*`
   - `/run/bitriver/health`
   - `/run/bitriver/studio`
4. `network-online.target` is reached through the wired LAN stack.
5. `bitriver-firstboot.service` runs before all application services.

## `bitriver-firstboot.service`

This unit is idempotent. It performs heavy setup only when files are missing, but it always refreshes current setup state.

It does the following:

1. Creates the persistent appliance state directories.
2. Generates device identity in `/var/lib/bitriver/bootstrap/device-identity.json` if missing.
3. Generates initial admin credentials if missing:
   - hashed credential record in `/var/lib/bitriver/bootstrap/admin-credentials.json`
   - one-time local recovery notice in `/var/lib/bitriver/bootstrap/one-time-admin-password.txt`
4. Runs `bitriver-hw-report` and writes `/var/lib/bitriver/hardware/capabilities.json`.
5. Builds the initial active config in `/var/lib/bitriver/config/active.json` from the detected hardware and the committed template.
6. Writes `/var/lib/bitriver/bootstrap/setup-status.json` with:
   - appliance ID
   - hostname
   - display mode
   - hardware summary
   - first-boot completion state
   - setup issues that should surface as degraded mode

The control API reads this file directly, so setup state is available as soon as the API starts.

## Core Service Bring-Up

Once first-boot completes, `bitriver.target` brings up the main appliance graph:

- `bitriver-control-api.service`
- `mediamtx.service`
- `bitriver-update-agent.service`
- `bitriver-health-supervisor.service`
- `owncast.service` when enabled in the active config
- `bitriver-kiosk.service` when a display is attached
- `obs.service` when a display is attached

The dependency strategy is:

- `bitriver-firstboot.service` must complete before API, ingest, update, or local display services.
- `bitriver-control-api.service` is the browser-facing control plane and is considered core.
- `mediamtx.service` is also core because contributor ingest should survive local display loss.
- `bitriver-update-agent.service` is always present so deployment state is visible even before richer update flows exist.
- `owncast.service` is optional and skips cleanly when the active config has `owncast.enabled = false`.
- `bitriver-kiosk.service` and `obs.service` are display-conditional and do not block headless boot.

## Headless vs Display-Attached Mode

### Headless Mode

If `/sys/class/drm/*/status` reports no connected output:

- `bitriver-kiosk.service` is skipped by `ExecCondition`
- `obs.service` is skipped by `ExecCondition`
- the appliance still comes up on the LAN through `bitriver-control-api.service`
- the health supervisor marks the system `degraded`, not `failed`

This is intentional. For MVP, OBS requires a real graphical session. A headless box without an attached or dummy HDMI sink is still manageable from the LAN, but it is not treated as a fully healthy production node.

### Display-Attached Mode

If a display is attached:

1. `bitriver-kiosk.service` claims `tty1`.
2. It starts `sway` with the committed kiosk config.
3. `sway` launches Chromium in fullscreen kiosk mode pointed at `http://127.0.0.1/`.
4. `obs.service` starts after the kiosk session and connects to the same runtime directory and Wayland display.

The local screen therefore boots directly into the BitRiver control experience rather than a desktop shell.

## Crash and Recovery Behavior

- `bitriver-control-api.service`, `mediamtx.service`, `owncast.service`, and `obs.service` all use `Restart=on-failure`.
- `bitriver-update-agent.service` and `bitriver-health-supervisor.service` use `Restart=always`.
- `bitriver-health-supervisor.service` acts as the appliance watchdog:
  - checks the API liveness endpoint
  - checks MediaMTX readiness through its local API
  - checks Owncast only when enabled
  - checks the local display path only when a display is attached
  - requests targeted `systemctl restart` for failed services
  - writes `/run/bitriver/health/summary.json`

## Product Experience On First Boot

The exact first-boot appliance experience is:

1. The box powers on.
2. First-boot materializes identity, credentials, hardware state, and a valid starter config.
3. The control API starts and exposes `/api/v1/setup/status`.
4. If a display is attached, Chromium opens the local BitRiver UI in fullscreen kiosk mode.
5. If no display is attached, the appliance still becomes reachable on the LAN and reports headless degraded mode until a display or dummy plug is present.

That is the MVP boot model: appliance-first, LAN-operable, no desktop workflow required.
