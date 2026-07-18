---
name: build-corthena-docking-and-persistence
description: Build Corthena Phase 4 docking, reusable controls, preferences, responsive scaling, and layout persistence over the completed Phase 1-3 ui. Use for implementing or changing dock-tree mutations and geometry, widget interaction routing, UI scale preferences, named layouts, background persistence, corruption recovery, migration, or the Phase 4 hidden capture; do not use for Phase 5 charts and tables or domain workflows.
---

# Build Corthena Docking and Persistence

## Ground the implementation

1. Read `AGENTS.md` and every required document in
   `specs/routing/phase-4.md` before editing.
2. Treat the living specifications, legacy Go implementation, and manifest-owned
   `phase4_dockable_data.png` as canonical. Update the owning specification first
   when behavior or a public contract changes.
3. Preserve the Phase 1 UI-thread lifecycle, Phase 2 immutable state/effects
   boundaries, and Phase 3 shell composition. Keep I/O and native values outside
   render-neutral layout, control, and mutation code.

## Build deterministic docking and controls

- Represent layouts as immutable typed dock trees with stable panel and node IDs.
  Implement pure activate, reorder, move, split/dock, close/reopen,
  maximize/restore, and resize mutations with exhaustive validation.
- Collapse empty splits, retain hidden-panel instance state, enforce minimum
  extents, persist ratios rather than pixels, and derive geometry deterministically
  from the live viewport and effective DPI multiplied by the selected preset.
- Support full and constrained-width arrangements without overlap or silent loss
  of essential actions. Apply scale once and use the same snapped rectangles for
  painting, clipping, and hit testing.
- Build controls from shared visual primitives and hierarchical widget IDs.
  Define deterministic hot, active, focused, captured, clipped, keyboard,
  pointer, release, and cancellation behavior. Emit only typed `UIAction` values.

## Persist preferences and layouts safely

- Keep global preferences and named layouts in separate, versioned, strictly
  validated documents. Reject unknown, duplicate, corrupt, oversized, stale, or
  incompatible data before constructing domain values.
- Use atomic replacement and bounded background workers. Coalesce replaceable
  saves, keep render-thread submission nonblocking, reject stale completions,
  and define cancellation, saturation, failure reporting, and shutdown.
- Quarantine invalid documents for diagnosis, fall back to validated defaults,
  and provide deterministic recovery and explicit version migrations. Never
  perform filesystem work on the UI thread.

## Preserve visual and phase boundaries

- Use `$build-corthena-raylib-visual-system` for shared tokens, primitives,
  responsive geometry, interaction states, clipping, and stable draw order.
- Keep docking and persistence behavior outside rendering and native adapters.
  Publish immutable snapshots and never rely on the GIL for synchronization.
- Exclude Phase 5 charts, tables, LOD, and virtualization and every Phase 6+
  domain workflow or real coordinator behavior.

## Test and hand off

- Add focused invariant, property, replay, interaction, persistence, recovery,
  migration, saturation, cancellation, shutdown, and repeated-lifecycle tests.
- Exercise 1280x720 and 1920x1080 at applicable scale presets, then capture the
  exact manifest-owned Phase 4 scenario and compare decoded RGBA output without
  changing inputs or tolerances.
- Run applicable configured checks honestly, then hand off in the route's
  required skill order.
