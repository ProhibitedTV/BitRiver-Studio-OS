# Offline starter scene provenance

- Upstream repository: https://github.com/ProhibitedTV/OBS-Scene-Assets
- Upstream commit: `e86b82f1df14c2defca0d1e0d69b9d8f1073d176`
- Selected four source HTML files: `assets/scenes/{starting-soon,be-right-back,just-chatting,intermission}.html`
- Selected PNG previews copied byte-for-byte from `assets/previews/scenes/` at the same pinned revision
- HTML modification: removed external `fonts.googleapis.com` @import declarations so the pack does not require network access; system font fallback remains
- Included license: `LICENSE-OBS-Scene-Assets` (MIT, original copyright holder retained)

**No automatic OBS scene collection installation or execution in the control
API origin.** A user must deliberately add the Browser Source in OBS.
Do not imply that the full upstream catalog, external fonts, user credentials,
live radio, games or AI tooling are included in this starter subset.
