# Phase 5 Foundation Task Route

Non-authoritative navigation aid; canonical behavior remains in linked specs.

## Read first

- Required: `AGENTS.md`, `design-pattern.md`, Phase 5 in `roadmap.md`,
  `ui/foundation.md`, `ui/foundation-shell-state.md`,
  `ui/foundation-async-effects.md`, `ui/raylib-visual-system.md`,
  `ui/visualization.md`, `migration-baseline.md`, `quality-common.md`,
  `quality-concurrency.md`, and `quality-visualization.md`.
- Conditional: `technology-stack.md` for dependency, native, Cython, or tooling
  changes; `api.md` for deliberate client, Arrow, or process-boundary changes;
  and `ui/workspaces.md` only to enforce the Phase 6 boundary.
- Current implementation and evidence: `src/corthena/ui/visualization.py`,
  `src/corthena/ui/visualization_runtime.py`, `src/corthena/ui/table.py`,
  `tests/test_ui_phase5.py`, and the Phase 5 benchmark tests.

## Scope

- Implement pure typed `float64` transforms, inverse transforms, clipping,
  ticks, checked final draw conversion, and immutable render-ready foundation
  buffers.
- Implement deterministic pixel-bucket LOD that preserves OHLCV semantics and
  stable first, last, minimum, and maximum continuous samples. Instrument work
  so preparation is bounded by viewport width after source-range selection.
- Implement bounded cancellable preparation workers, generation and
  stale-result checks, nonblocking backpressure, immutable publication, and a
  byte-bounded deterministic LRU with complete accounting.
- Implement row and column virtualization, bounded overscan and copy output,
  resizable and pinned headers, stable row-ID selection, and deterministic typed
  sort, filter, pagination, and null behavior.
- Leave complete generic layer rendering, interaction wiring, cross-scope
  request deduplication, pagination-worker parity, and canonical visual
  acceptance to `phase-5b.md`.

## Exclusions

Exclude Phase 6 Research queries, deterministic market fixtures, feature and
target workflows, workspace-specific linked behavior, and every Phase 7+
domain workflow. Exclude real coordinator/network/repository behavior and
unmeasured Cython optimization. Do not add a dependency, public API, serialized
schema, or baseline outside its owning specification.

## Required skill order

1. `$build-corthena-charts-and-tables`
2. `$build-corthena-raylib-visual-system`
3. `$python-best-practices`
4. `$verify-corthena-visualization-performance`
5. `$verify-corthena-raylib-visual-system`
6. `$python-windows-compat-gate`
7. `$review-corthena-code`

## Completion evidence

- Hand-calculated and property tests cover transforms, clipping, degeneracy,
  non-finite and range rejection, ticks, stable tie-breaking, OHLCV buckets,
  and continuous LOD.
- Concurrency and cache tests cover exact byte boundaries, replacement and
  eviction, oversized entries, saturation, cancellation before and during
  work, stale generations, channel closure, bounded shutdown, immutable
  publication, varied completion order, and repeated leak checks.
- Table tests cover empty, first, middle, last, resized, pinned, and overscrolled
  windows; stable row selection across mutations; deterministic null ordering;
  and cell work bounded by visible rows, columns, and overscan.
- Instrumented counts prove chart work is bounded by viewport width and table
  work by the visible window. Configured benchmarks record input and viewport
  sizes, work counts, time, allocations, and bytes; the documented out-of-CI
  near-ten-million-row run records throughput and peak Python/native memory.
- Every applicable configured common, concurrency, visualization, Windows,
  formatting, linting, typing, test, property, benchmark, vulnerability, and
  native build/import gate for this foundation passes, and its review has no
  unresolved findings. Phase 5 is complete with this accepted evidence;
  `phase-5b.md` owns every remaining parity and final-acceptance obligation.
