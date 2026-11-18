# BitRiver Studio OS — unified visual identity

The mark is a **flowing B**: a simple two-channel monogram that suggests a river,
data transport and live creation. It replaces the earlier radio-wave illustration.

![BitRiver Studio OS brand banner](assets/bitriver-studio-logo.svg)

| Asset | Preview | Role |
| --- | --- | --- |
| Full banner | ![BitRiver banner](assets/bitriver-studio-logo.svg) | GitHub README and documentation |
| Compact mark | ![BitRiver flowing B mark](assets/bitriver-studio-mark.svg) | Favicon, app/desktop icon, small placements |
| Wordmark | ![BitRiver Studio OS wordmark](assets/bitriver-studio-wordmark.svg) | Control dashboard, operator login, application chrome |

## Design

- Primary background: `#071923` (night navy)
- Brand light: `#F2FAFE`
- Signal cyan: `#61D6F4`
- River teal: `#39C7A9`
- Supporting linework: `#285461`
- Fonts: system-safe Segoe UI / DejaVu Sans / Arial fallbacks; no embedded
  proprietary font assets

All source assets are self-contained, accessible, lightweight SVGs without
external links, raster resources or scripts. The compact mark is square with
rounded backing so it works on both GitHub light and dark themes.

## Where it lands

The same assets are installed at multiple required locations:

1. **README and docs:** `docs/assets/bitriver-studio-{logo,mark,wordmark}.svg`.
2. **Control UI and operator sign-in:** Vite public assets at
   `apps/control-ui/public/branding/`, used in the dashboard and authentication UI.
3. **Browser tab and app metadata:** `apps/control-ui/public/favicon.svg`,
   `manifest.webmanifest`, and `apps/control-ui/index.html`.
4. **The bootc OS itself:** prepackaged under
   `/usr/local/share/bitriver/branding/` in the immutable image. The
   local kiosk serves the branded built UI.
5. **Text consoles:** `/etc/issue.d/bitriver.issue` and `/etc/motd`
   identify the OS to a local operator.

The files are **deliberately identical copies** rather than symlinks between
Vite, docs, and Linux image build contexts. CI runs
`bash test/branding/check-branding.sh` and rejects inconsistent copies.

## Future landing surfaces

The **Anaconda installer, GRUB, and Plymouth graphical boot splash are not yet
branded**. Their rendering and fallback behavior must be tested against the
experimental ISO before a branded installer is distributed; merely copying
graphics into the image is not enough. The canonical SVG assets are ready for
those integrations when the install/reboot gate passes.

A passing CI build or a pretty login page does not validate OBS graphics,
a working Wayland session, NVIDIA drivers, or a public installer.
