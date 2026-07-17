---
name: build-corthena-raylib-visual-system
description: Build or change Corthena Raylib shell, workspace, control, docking, chart, table, modal, notification, interaction-state, or responsive visuals. Use for any implementation that changes Raylib styling, typography, spacing, geometry, clipping, layering, or visual behavior.
---

# Build Corthena Raylib Visual System

Implement screenshot-driven visual changes through the shared visual system
without changing unrelated visuals or moving behavior into rendering code.

## Ground the change

1. Read `python_migration/AGENTS.md`,
   `python_migration/specs/design-pattern.md`, and
   `python_migration/specs/ui/raylib-visual-system.md` completely.
2. Read the owning shell, workspace, visualization, and quality specifications
   selected by `AGENTS.md`. Read the applicable canonical PNG manifest under
   `internal/app/workstation/testdata/`.
3. Inspect relevant canonical PNGs. Use Phase 1–9 JPEG screenshots only as
   design references, never comparison baselines.
4. Inspect the workspace and preserve unrelated changes and visuals.
5. If the request intentionally changes canonical design behavior, require
   explicit user approval before implementation and update the owning spec and
   affected manifest entries with the design change.

## Implement through shared boundaries

- Feed render functions immutable typed, render-neutral view models. Keep
  domain behavior, persistence, network, decoding, training, and blocking work
  outside layout and draw functions.
- Centralize visual tokens and derived state styles at the style/native adapter
  boundary. Use shared primitives for controls, panels, tabs, tables, charts,
  modals, notifications, clipping, text measurement, and pixel snapping.
- Do not add raw color, font, spacing, or interaction-style duplication outside
  that boundary. Read token values from the visual-system spec; do not copy its
  detailed token table into this skill or feature modules.
- Keep Raylib/Raygui values and calls inside narrow adapters and on the locked
  UI OS thread. Emit only typed `UIAction` values from input handling.
- Compute geometry from live window/framebuffer metrics and effective DPI ×
  preset scale. Apply scaling once. Use the same final rectangles for painting,
  clipping, and hit testing.
- Implement every applicable default, hot, focused, active, selected, disabled,
  loading, empty, failure, degraded, and recovered state. Pair semantic color
  with text, icon, pattern, or shape.
- Preserve deterministic stable draw order and balance every clip/scissor scope
  on success and failure paths.

## Drive from screenshots and manifests

Build the smallest coherent primitive or feature change, then capture the
applicable manifest matrix using its recorded viewport, scale, seed, clock,
fixtures/state, assets, backend, and tolerances. Compare decoded RGBA output;
diagnose drift instead of editing images, skipping cases, or loosening
tolerances. Preserve unrelated pixels unless an approved change requires more.

For a modal, reuse the shared surface, typography, spacing, focus, scrim, and
layer primitives. For a responsive panel, prove 1280×720 and 1920×1080 behavior
and every recorded scale. For chart or table work, preserve clipping,
virtualization/LOD meaning, linked state, and stable ordering.

## Verify and hand off

Run focused functional and visual tests plus the repository quality route. Use
`$verify-corthena-raylib-visual-system` for the final visual audit. Report every
manifest case and command run; do not claim skipped checks passed.
