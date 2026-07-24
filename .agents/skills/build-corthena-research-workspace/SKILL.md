---
name: build-corthena-research-workspace
description: "Build Phase 6 Research workspace behavior: OHLCV, features, targets, linked queries, deterministic demo data, effects, and panel states."
---

# Build Corthena Research Workspace

Build the complete Research workflow on the Phase 5 visualization foundation
without leaking simulator details into panels or moving data work onto the UI
thread.

## Ground the change

1. Read `AGENTS.md`, `specs/general/roadmap.md`, `specs/pages/research/README.md`,
   `specs/general/ui/workspaces.md`, `specs/general/ui/README.md`,
   `specs/general/ui/visualization.md`, `specs/pages/data/ingestion.md`, and
   `specs/general/quality/README.md`.
2. Read `specs/general/api.md` for client or Arrow boundary changes. Read
   `specs/general/technology-stack.md` for dependency or native-adapter changes.
3. Inspect the existing app state, effects runtime, simulator, link groups,
   Phase 5 chart/table packages, and native renderer before adding types or
   packages. Preserve unrelated workspace changes.

## Define the Research client boundary

- Add concrete validated request and immutable response types for the data
  needed by OHLCV, features, targets, distributions, and paginated rows.
- Keep request identity explicit: dataset, symbols, interval, time range,
  selected series, target configuration, resolution, cursor, and generation
  as applicable. Normalize UTC ranges and reject invalid combinations.
- Keep Arrow objects inside adapters. Publish client-owned typed slices or
  Phase 5 render-ready buffers; clone mutable slices at boundaries.
- Extend `UIClient`, effects, and `DemoCoordinator` together. Panels must
  depend only on typed state/actions/effects and never import the simulator.
- Update `specs/general/api.md` when a public or process contract changes; do not define
  real coordinator endpoints solely to satisfy the demo implementation.

## Build deterministic asynchronous behavior

- Generate seeded, chronologically ordered OHLCV, feature, target, and row data
  with stable IDs and explicit missing values. Repeating seed, clock, request,
  and action sequence must reproduce identical messages and buffers.
- Run request preparation, Arrow decode, aggregation, sorting, filtering, and
  simulator delays off the render thread through owned cancellable workers.
- Reuse Phase 5 request deduplication, generation filtering, byte-bounded
  caching, and table pagination. Do not create a parallel cache or worker model.
- Make UI sends nonblocking. Cancel superseded or hidden-panel work and reject
  stale completions before visible state changes.
- Provide deterministic normal, loading, empty, failure, degraded/reconnecting,
  recovered, canceled, and queue-saturated scenarios.

## Assemble the linked Research panels

- Build the primary candlestick/volume chart with feature and target overlays,
  crosshair tooltip, pan/zoom/box selection, visibility controls, and reset.
- Build the feature browser, series inspector, target preview, distributions,
  and virtualized row table from typed panel state and reusable controls.
- Synchronize only supported dataset, symbol, interval, and visible-range
  scopes through the panel's assigned link group. Independent groups must not
  receive changes.
- Preserve stable feature and row selection across refreshed, sorted, filtered,
  and paginated data. Keep keyboard and pointer behavior deterministic.
- Render loading, empty, error, retry, degraded, and recovered states inside
  each panel without replacing the dock layout or global context.
- Keep draw functions limited to prepared buffers and typed action emission;
  keep feature, target, timing, and scenario behavior in pure reducers or
  background preparation.

## Preserve leakage safety

- Show feature values only at timestamps where their declared lookback is
  available. Preserve missing values rather than backfilling from the future.
- Derive target previews from the configured forward horizon and exclude rows
  lacking a valid future target. Keep feature and target timestamps explicit.
- Keep source ordering stable by timestamp, symbol, and source row ID. Never
  use target or future-bar information to prepare a feature overlay.
- Make train/validation/test regions descriptive only; linked viewport changes
  must not alter split or target membership.

## Verify before handoff

- Add reducer, simulator, effect, panel, link-group, cancellation, and typed
  client-boundary tests while implementing each slice.
- Run `$verify-corthena-research-vertical-slice` and, for visualization kernel
  changes, `$verify-corthena-visualization-performance`.
- Hand off to `$verify-corthena-research-vertical-slice` and apply the focused
  quality route; use `$python-windows-compat-gate` for dependency, native,
  toolchain, or shell changes.
- Update living specifications for behavior or contract changes. Do not mark
  Phase 6 complete while any panel, scenario, leakage test, client-boundary
  path, golden matrix, or required quality gate is missing.
