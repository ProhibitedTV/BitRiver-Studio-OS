# Creator Asset Library (OBS Scene Assets)

BitRiver can browse a **local, optional** installation of
[ProhibitedTV/OBS-Scene-Assets](https://github.com/ProhibitedTV/OBS-Scene-Assets).
Its metadata catalog and PNG previews appear in the Asset Studio panel of the
control dashboard. You can search and filter assets, customize common title
and subtitle query parameters, and copy an OBS Browser Source file URL.

This initial integration is deliberately read-only: it does not modify OBS,
install scene collections, execute overlays in the admin browser, or download
untrusted content automatically. One-click OBS source installation is future work.

## Install on the appliance

1. Place a reviewed copy of the repository under the **persistent** directory
   /data/creator-assets/. For example, on a trusted development system:

       git clone https://github.com/ProhibitedTV/OBS-Scene-Assets.git /data/creator-assets

2. Verify that /data/creator-assets/assets/catalog.json and
   /data/creator-assets/assets/previews/ exist.
3. Make the files readable by the bitriver service account and the HTML
   files readable by the studio user running OBS. Do not grant those accounts
   write access to the source checkout unnecessarily.
4. Set BITRIVER_ASSET_LIBRARY_DIR=/data/creator-assets/assets in the
   appliance environment file and restart bitriver-control-api.service.
   This path is already the default in the included image configuration.

For local development, run bitriverd with
-asset-library-dir /absolute/path/to/OBS-Scene-Assets/assets.
The Asset Studio panel appears when the catalog loads; when it is absent the
dashboard shows a clear configuration hint and the broadcast stack continues.

The asset directory must remain mounted at the same path **on the machine
running OBS**, because generated OBS URLs have the form:

    file:///data/creator-assets/assets/scenes/starting-soon.html?title=My+Show

Set the Browser Source width and height to the values shown in the Asset
Studio card. A copied URL is not an automatic OBS scene installation.

## Security boundaries

- Only curated metadata from assets/catalog.json is parsed, with an 8 MiB
  limit. Paths with traversal segments or unexpected extensions are rejected.
- Only files under assets/previews/ with a .png extension are served over
  the control API. Symlink resolution prevents previews escaping that tree.
- The control API **never serves overlay HTML**, which could otherwise run
  arbitrary JavaScript on the same origin as administrator controls.
- OBS itself loads the selected local HTML as a Browser Source. Review packs
  before installation, especially any JavaScript and external network assets.
- Private personal media libraries, AI model secrets, Discord credentials and
  Steam authentication are **not** part of this integration.
- Do not expose the LAN control API or local asset library directly to the
  public internet. Existing authentication and on-air safety controls must be
  reviewed before wider deployment.

## API

GET /api/v1/creator-assets/catalog: returns the local root path, upstream
summary and safe catalog entries. Returns HTTP 503 when the library is
unconfigured or unreadable.

GET /api/v1/creator-assets/preview/previews/<path>.png: reads only PNG
previews from the configured local library. Returns HTTP 404 for invalid
paths or missing files.

Both endpoints are read-only. The catalog does not invoke OBS commands.

## Next milestone

Connect Asset Studio selection to the authenticated OBS adapter using
CreateInput/SetInputSettings from obs-websocket, after validating scene
existence, naming collisions, active-stream safety, and input rollback. Keep
the overlay in OBS's rendering context, never in the control API origin.
