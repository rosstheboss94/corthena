---
name: verify-corthena-docking-and-persistence
description: Verify Corthena Phase 4 docking, reusable controls, preferences, responsive scaling, and layout persistence. Use for acceptance or regression audits of dock mutations and geometry, widget interaction routing, deterministic replay, persistence revisioning and recovery, bounded background I/O, or manifest-owned Phase 4 visual parity; reject Phase 5+ scope.
---

# Verify Corthena Docking and Persistence

## Establish the contract

1. Read `AGENTS.md` and every required document in
   `specs/routing/phase-4.md`.
2. Inspect the complete target diff and the matching legacy Go docking,
   controls, layouts, preferences, effects, and native dock-shell references.
3. Record every input and tolerance owned by the Phase 1-4 manifest entry for
   `phase4_dockable_data.png`. Treat JPEGs only as design references.

## Audit docking, geometry, and interaction

- Exercise activate, reorder, move, split/dock, close/reopen, maximize/restore,
  and resize operations. Verify immutable publication, stable IDs, valid trees,
  deterministic ratios and geometry, empty-split collapse, hidden-panel state,
  and minimum extents after every mutation.
- Verify hierarchical widget IDs and deterministic hot, active, focus, capture,
  clipping, keyboard, pointer, release, and cancellation routing. Painting,
  clipping, and hit testing must use the same final rectangles.
- Replay identical action streams under varied completion orders and queue
  pressure. Reject arrival-order state, mutable published snapshots, GIL-based
  synchronization, unbounded work, blocking frames, and native-value leakage.

## Audit persistence and lifecycle

- Verify separate versioned preference and named-layout documents, canonical
  serialization round trips, strict validation, revision conflicts, migrations,
  stale-load and stale-save rejection, and fallback defaults.
- Inject corrupt, duplicate-field, oversized, partial, incompatible, and missing
  documents; quarantine invalid evidence without destroying diagnostics. Inject
  atomic create, write, flush, close, and replace failures.
- Saturate bounded queues and verify save coalescing, typed busy/failure states,
  cancellation, dependency-ordered shutdown, idempotent cleanup, and no task,
  thread, handle, queue, or temporary-file growth. Filesystem I/O must remain off
  the UI thread.

## Verify responsive and visual parity

- Exercise 1280x720 and 1920x1080 across every applicable scale preset. Verify
  DPI multiplied by preset is applied once, constrained layouts preserve the
  primary workflow, and panels do not overlap, escape clipping, or fall below
  documented minimums.
- Use `$verify-corthena-raylib-visual-system` to audit tokens, shared primitives,
  interaction states, accessibility, clipping, native containment, and stable
  draw order.
- Capture the Phase 4 scenario with the manifest's exact viewport, scale, seed,
  clock, fixture/state, layout revision, assets, backend, build identity, hidden
  frames, channel tolerance, and differing-pixel ratio. Compare decoded RGBA.
  Reject skipped cases, JPEG baselines, manual waivers, edited captures, or
  tolerance inflation.

## Report acceptance

Lead with actionable findings and cite the smallest useful file locations.
List every functional, lifecycle, responsive, and manifest case exercised and
every command run or unable to run. Phase 4 remains Pending while any required
case is missing or failing. Reject Phase 5 charts/tables and Phase 6+ domain
workflow changes from this route.
