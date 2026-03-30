# Troubleshooting

## Local Development

### `npm install` fails at the workspace root

- Verify Node.js 22+ and npm 10+.
- Run the install from the repo root so workspace links resolve.

### `make dev-ui` cannot reach the API

- Start the API first with `make dev-api`.
- Confirm `http://127.0.0.1:8080/healthz` returns `ok`.
- Confirm the Vite dev server proxy is forwarding `/api` to port `8080`.

### `make smoke` times out waiting for the API

- Check that Go is installed and on `PATH`.
- Run `make dev-api` manually and verify startup logs.
- Confirm nothing else is bound to port `8080`.

## Control Plane

### Config validation fails

- Check duplicate source IDs.
- Check duplicate scene IDs.
- Ensure at least one scene exists.
- Ensure recording path and destination mode are set.
- Ensure `mediaRouter.localIngestPath` and `mediaRouter.localPreviewPath` are different.
- Ensure RTMP destinations use `rtmp://` or `rtmps://` and SRT destinations use `srt://`.

### Stream or recording state looks stale in the UI

- The scaffold currently uses polling, not the final event transport.
- Refresh the page or wait for the next poll interval before assuming state drift.

### MediaMTX applies fail

- Check that `bitriver-control-api.service` can write `/etc/mediamtx` and `/etc/bitriver`.
- Inspect `journalctl -u mediamtx.service` for config parse or startup failures.
- Inspect `journalctl -u bitriver-control-api.service` for reload-to-restart fallback logs.
- Confirm `ffmpeg` and `jq` are present in the appliance image if downstream restreams are enabled.

## Appliance Packaging

### Image build fails while fetching MediaMTX or Owncast

- Confirm `MEDIAMTX_TARBALL_URL` and `OWNCAST_TARBALL_URL` are set for the packaging command.
- Confirm the referenced archives match the target Linux architecture.

### Unit files and overlay assets diverge

- Review `deploy/systemd/` and `deploy/image/rootfs-overlay/` together.
- Pay special attention to environment files, expected binary paths, and enabled targets.

### The appliance boots headless and never starts the local screen

- Run `bitriver-hw-report` and check `displayAttached`.
- Confirm a physical HDMI display or dummy plug is present.
- Inspect `bitriver-kiosk.service` and `obs.service`; both are skipped intentionally when no display is detected.
