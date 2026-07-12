---
name: build-corthena-charts-and-tables
description: Build Phase 5 first-party charts, LOD, bounded visualization caches, generation-safe requests, and virtualized tables.
---

# Build Corthena Charts and Tables

Implement visualization components without moving numerical work onto the UI
thread, leaking Raylib values, or making work proportional to source row count.

## Ground the change

1. Read `python_migration/AGENTS.md`, `python_migration/specs/roadmap.md`,
   `python_migration/specs/frontend/visualization.md`,
   `python_migration/specs/frontend/foundation.md`, and
   `python_migration/specs/quality.md`.
2. Read `python_migration/specs/technology-stack.md` for dependencies or native adapter work.
3. Read `python_migration/specs/frontend/workspaces.md` only when panel workflow behavior is in
   scope. Read `python_migration/specs/api.md` only for client or Arrow request changes.
4. Inspect existing docking, controls, app state, effects, and native UI package
   boundaries before creating packages. Preserve unrelated workspace changes.

## Keep visualization kernels pure

- Keep transforms, clipping, tick selection, LOD aggregation, virtualization,
  sorting, and selection in pure Python modules with no Raylib imports.
- Retain `float64` through transforms and aggregation. Reject non-finite or
  out-of-range coordinates before checked conversion to final `float32` draw
  values.
- Define typed immutable inputs and render-ready outputs. Do not expose Arrow
  builders, native vectors, pointers, `any`, or weak maps.
- Preserve stable source-index tie-breaking and deterministic output order.
- Keep panel/domain behavior out of draw functions; render functions consume
  prepared buffers and emit typed actions only.

## Build charts

- Support the layers owned by the visualization spec: candlestick and volume,
  line and area, histogram and scatter, equity and drawdown, heatmap, feature
  importance, predictions, trades, and train/validation/test regions.
- Implement pointer pan, wheel zoom, box selection, crosshair, typed tooltip,
  series visibility, reset-to-fit, and linked symbol/range propagation through
  existing control and link-group state.
- Bucket dense data by horizontal pixel range. Preserve OHLC semantics for
  candles and first, last, minimum, and maximum samples for continuous series.
- Bound render work by viewport width after LOD, not source rows.

## Build asynchronous data and caches

- Perform Arrow decode, LOD, sorting, filtering, and request preparation on
  owned background workers with explicit cancellation and bounded queues.
- Tag requests and results with monotonically ordered generation tokens. Drop
  stale results before they enter visible state.
- Deduplicate equivalent requests and use a byte-bounded LRU whose accounting
  includes owned buffers. Never evict or mutate data still published to a frame.
- Make backpressure explicit and keep UI-thread sends nonblocking.

## Build virtualized tables

- Compute visible row and column windows from scroll offsets and viewport size.
  Measure and render only those cells plus a bounded overscan.
- Preserve stable row IDs across sorting, filtering, pagination, and updates;
  selection follows IDs rather than visible indexes.
- Implement typed deterministic sort/null behavior, resizable headers, pinned
  identifier columns, keyboard and pointer selection, and bounded copy output.
- Keep server-side filter and pagination requests cancellable and generation
  safe.

## Verify before handoff

- Add hand-calculated transform, clipping, LOD, interaction, stable-ID,
  virtualization, cache, cancellation, and stale-generation tests.
- Run the owning tests and `$verify-corthena-visualization-performance`.
- Hand off to `$verify-corthena-visualization-performance` and apply the
  focused quality route; use `$python-windows-compat-gate` for native adapter,
  dependency, toolchain, or application-shell changes.
- Update living specifications for behavior changes. Mark Phase 5 complete only
  when every done condition and required gate passes.
