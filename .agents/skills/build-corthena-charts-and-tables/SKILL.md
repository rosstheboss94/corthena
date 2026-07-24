---
name: build-corthena-charts-and-tables
description: Build Corthena Phase 5 first-party chart and table infrastructure, including typed transforms, clipping, layers, LOD, interactions, bounded visualization caches and workers, generation safety, virtualization, benchmarks, and generic Raylib golden scenarios. Use for Phase 5 visualization primitives; do not use for Phase 6 Research workflow behavior or later domain workflows.
---

# Build Corthena Charts and Tables

## Ground the implementation

1. Read `AGENTS.md` and every required document in
   `specs/general/ui/visualization.md` and `specs/general/quality/visualization.md` before editing.
2. Treat the living specifications, legacy Go chart/table implementation, and
   manifest-owned `phase5-golden` matrix as canonical. Update an owning
   specification first when behavior or a public contract changes.
3. Inspect the completed Phase 1-4 lifecycle, state/effects, docking, controls,
   persistence, visual-system, and native-adapter boundaries. Preserve them.

## Build pure visualization kernels

- Keep transforms, inverse transforms, clipping, ticks, LOD, virtualization,
  sorting, filtering, pagination, and selection in typed pure Python without
  Raylib imports. Retain `float64` until checked final draw conversion.
- Reject non-finite, degenerate, or out-of-range inputs before publication.
  Publish immutable render-ready values with stable source-index tie-breaking
  and deterministic output independent of completion order.
- Implement every generic layer owned by `ui/visualization.md`. Keep
  Research queries, fixtures, features, targets, and workflow state out.

## Bound charts, workers, and caches

- Preserve OHLCV semantics in candle buckets and stable first, last, minimum,
  and maximum samples in continuous LOD. Instrument work counts and keep final
  preparation proportional to viewport width after source-range selection.
- Route pan, zoom, box selection, crosshair, typed tooltip, visibility,
  reset-to-fit, keyboard, and generic linked-axis actions through typed state
  and Phase 4 controls.
- Keep decoding and preparation on owned bounded workers. Define queue
  capacities, saturation, deduplication, cancellation, generation ordering,
  stale-result rejection, failure reporting, and bounded shutdown.
- Use a byte-bounded deterministic LRU with complete owned-buffer accounting.
  Never mutate or evict data still published to a frame.

## Virtualize tables

- Compute row and column windows from the final viewport and scroll state.
  Measure and render only visible cells plus bounded overscan.
- Preserve stable row-ID selection across sorting, filtering, pagination,
  insertion, removal, and update. Define typed deterministic null ordering.
- Support resizable headers, pinned identifier columns, keyboard and pointer
  selection, and bounded copy output without work proportional to total rows.

## Preserve visual and phase boundaries

- Use `$build-corthena-raylib-visual-system` for tokens, chart/table primitives,
  interaction states, clipping, scaling, and draw order.
- Keep Raylib/Raygui on the locked UI thread and all I/O, decoding, sorting,
  filtering, LOD, and blocking work off it. Emit only typed actions.
- Exclude Phase 6+ workflows, real coordinator/network/repository behavior,
  new dependencies without approval, and Cython without measured evidence.

## Test and hand off

- Add hand-calculated, property, replay, cache, race, cancellation, stale-result,
  virtualization, work-count, benchmark, and repeated-lifecycle tests.
- Capture the exact six-case `phase5-golden` matrix and compare decoded RGBA
  using manifest-owned inputs and unchanged tolerances.
- Hand off in the route's required order to
  `$verify-corthena-visualization-performance`,
  `$verify-corthena-raylib-visual-system`, `$python-windows-compat-gate`, and
  `$review-corthena-code`. Keep Phase 5 Pending until all evidence passes.
