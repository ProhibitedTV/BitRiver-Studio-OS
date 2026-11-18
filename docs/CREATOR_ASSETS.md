# Creator Asset Library (OBS Scene Assets)

BitRiver ships a **small, offline, MIT-licensed starter subset** of
[ProhibitedTV/OBS-Scene-Assets](https://github.com/ProhibitedTV/OBS-Scene-Assets):
Starting Soon, Be Right Back, Just Chatting and Intermission. The pinned
HTML + PNG previews are embedded in the immutable OS at
`/usr/local/share/bitriver/creator-assets/assets/` and appear immediately
in Asset Studio without a network connection. Only remote font-import rules
were removed from the upstream HTML for offline use; provenance and license
are preserved alongside the pack.

Its metadata catalog and PNG previews appear in the Asset Studio panel of
the control dashboard. You can search and filter assets, customize common
title/subtitle query parameters, and copy an OBS Browser Source file URL.
The full upstream library remains optional. **Assets are never installed
silently**. An operator can explicitly install an asset as a new, inactive
OBS scene or copy its file URL for manual setup. Installation does not
switch the current scene, start an output, or overwrite existing OBS inputs.

Only locally installed, catalog-listed HTML can be selected. The control API
never renders overlay HTML in its own browser origin or downloads remote assets.

## Use the included offline scenes

Boot the OS, sign in to its local control interface and open Asset Studio.
Four starter scenes should be visible. Select one and copy the OBS Browser
Source URL, for example:

    file:///usr/local/share/bitriver/creator-assets/assets/scenes/starting-soon.html

Choose **Install as new OBS scene** to create a separate inactive scene
(named `BitRiver - Starting Soon`) with a local 1920 × 1080 Browser Source.
The newly installed scene also appears in the BitRiver **Scene Rail** with a
stable ID; you can switch to it manually from there. The operator must
**stop streaming and recording first**; OBS refuses duplicate
scene/input names, and a failed input creation attempts to remove only the
new scene. Never assume a failed request is safe to retry without inspecting
OBS, because a websocket disconnect may leave creation status unknown.

Alternatively, copy the URL and add it manually with a 1920 × 1080 Browser
Source. Review the result in OBS and switch scenes manually before going live. The files are read-only and shared across all users;
this prevents a creator's edits from silently modifying the base image.
To customize permanently, copy an asset into a persistent directory and
change the OBS source URL. No streaming keys are bundled.

## Install the full library on the appliance

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
   The default is the **read-only offline starter subset**. Changing this
   variable switches the catalog to your larger, persistent installation;
   it does not merge asset catalogs or mutate existing OBS scenes.

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
- Installation requires an operator session and CSRF token in appliance mode.
  A missing or invalid asset, out-of-library symlink, invalid dimensions, or
  duplicate OBS names is rejected. Active streams and recordings block installs.
- OBS commands are not retried after uncertain mutation outcomes. If OBS
  disconnects during creation, inspect the scene collection before retrying.
  No destructive cleanup touches pre-existing scenes or global sources.
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

POST /api/v1/creator-assets/install: accepts `{ "path": "scenes/starting-soon.html", "title": "My Show", "subtitle": "Episode 1" }`.
The file path must match a local catalog asset. Creates a new OBS scene and a
Browser Source with the catalog dimensions and encoded title/subtitle.
Returns **201** with scene details on success; **400/404/422** for invalid
input or assets, **409** for on-air/recording state or name collisions, and
**503** when the library or OBS is unavailable. Does not activate the scene.

The catalog and PNG preview endpoints remain read-only. All API endpoints are
protected by the appliance operator-auth boundary when enabled.

## Next milestone

Test asset installation and rendering with a real Fedora bootc QEMU guest and
OBS browser plugin. Validate packaged HTML behavior, persistent OBS scene
collections, and visual results under VirtualBox/physical GPU drivers. Provide
a reviewed uninstall/reinstall action without disturbing user-created scenes.
