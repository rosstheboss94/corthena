# Frontend Persistence and Responsive Behavior

Optimize for 1920x1080 and remain usable at 1280x720 and Windows scale factors
100–200%. Below roughly 1100 logical pixels, stack secondary panels and move
controls into overflow menus. Default the UI preset to 125%; support 100, 125,
150, 175, and 200%. Effective scale is DPI multiplied by the preset and
clamped to 100–200%, applying to all geometry, typography, controls, hit
targets, clipping, and minimum sizes.

Settings opens from navigation or `Ctrl+,`; `Ctrl+Plus`, `Ctrl+Minus`, and
`Ctrl+0` adjust/reset the preset. At constrained widths abbreviate all seven
tabs and move connection/job detail to the status bar. Preserve the active
analytical panel and store split ratios, not pixels.

Persist global preferences separately from named layouts in versioned,
atomically replaced user-data documents. Coalesce rapid saves on a bounded
worker, reject stale loads, quarantine invalid documents, preserve them for
diagnosis, and fall back to defaults. If the coordinator is unavailable, keep
the shell operational, disable mutations, and show reconnect/restart actions.
