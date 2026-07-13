# Phase 5b Acceptance and Visual-Parity Task Route

Non-authoritative navigation aid; canonical behavior remains in linked specs.

## Read first

- Required: `AGENTS.md`, `design-pattern.md`, Phase 5b in `roadmap.md`,
  `frontend/foundation.md`, `frontend/foundation-shell-state.md`,
  `frontend/foundation-async-effects.md`, `frontend/raylib-visual-system.md`,
  `frontend/visualization.md`, `migration-baseline.md`, `quality-common.md`,
  `quality-concurrency.md`, and `quality-visualization.md`.
- Conditional: `technology-stack.md` for dependency, native, Cython, or tooling
  changes; `api.md` for deliberate client, Arrow, or process-boundary changes;
  and `frontend/workspaces.md` only to enforce the Phase 6 boundary.
- Legacy parity references: `internal/frontend/chart/`,
  `internal/frontend/table/`, `internal/frontend/golden/`, visualization-related
  state and effects, `internal/frontend/nativeui/visualization_windows.go`, and
  their tests, fuzz targets, services, benchmarks, and capture behavior.

## Scope

- Complete render-ready layer preparation and Raylib rendering for OHLCV,
  line, area, histogram, scatter, equity, drawdown, heatmap, feature importance,
  predictions, trades, and train/validation/test partition regions.
- Route pan, zoom, box selection, crosshair, typed tooltips, series visibility,
  reset-to-fit, keyboard navigation, and generic linked-axis actions through
  immutable typed state and Phase 4 controls.
- Add cross-scope visualization-request deduplication. A shared request has
  independent watchers and preserves generation ordering, per-watcher
  cancellation, declared saturation behavior, stale-result rejection,
  immutable publication, and bounded shutdown; one watcher must not cancel or
  consume another watcher's result.
- Port pagination-worker parity with typed page requests and results, stable
  cursor/sort/filter keys, request deduplication, independent cancellation,
  bounded queues and saturation behavior, generation ordering, and stale-result
  rejection.
- Render through shared visual tokens and native primitives on the locked UI OS
  thread. Layout must use the same final rectangles for painting, clipping, hit
  testing, and capture, with deterministic draw order and balanced clip scopes.

## Exclusions

Exclude Research-specific queries, deterministic market fixtures, features,
targets, linked-workspace behavior, and every Phase 7+ domain workflow. Exclude
real coordinator/network/repository behavior, new dependencies without owning
spec approval, and Cython optimization without measured evidence.

## Canonical golden contract

- Legacy Go is the visual authority. Add a deterministic Go capture helper, a
  reviewed manifest, and exactly six lossless PNGs under
  `internal/app/workstation/testdata/phase5-golden/`.
- Capture 1280x720 and 1920x1080 at 100%, 150%, and 200% scale with a fixed
  seed, fixed UTC clock, generic visualization fixture, layout/app revision,
  fingerprinted assets, rendering backend/dependency identity, build revision,
  dirty-build state, and deterministic hidden-frame setup.
- Record channel tolerance `3` and maximum differing-pixel ratio `0.002` in
  every manifest entry. Compare Python Raylib captures to the reviewed Go
  baselines using decoded RGBA.
- JPEG baselines, hand-edited captures, skipped cases, perceptual waivers, and
  tolerance inflation are prohibited. Diagnose drift or obtain explicit
  approval for an intentional canonical visual change.

## Required skill order

1. `$build-corthena-charts-and-tables`
2. `$build-corthena-raylib-visual-system`
3. `$python-best-practices`
4. `$verify-corthena-visualization-performance`
5. `$verify-corthena-raylib-visual-system`
6. `$python-windows-compat-gate`
7. `$review-corthena-code`

## Completion evidence

- Hand-calculated and property tests cover every layer, clipping, stable draw
  ordering, typed tooltip values, all pointer and keyboard interactions, reset,
  visibility, linked-axis actions, and deterministic replay.
- Race and concurrency tests cover cross-scope deduplication with independent
  watchers, generation and completion ordering, per-watcher and whole-request
  cancellation, queue saturation, stale results, immutable publication,
  channel closure, bounded shutdown, and varied worker counts.
- Pagination tests cover typed request/result validation, stable
  cursor/sort/filter identities, deduplication, independent cancellation,
  first/middle/last/empty pages, mutation races, stale generations, saturation,
  bounded queues, and deterministic replay.
- Instrumentation proves chart preparation remains proportional to viewport
  width and table work to the visible window. Repeated normal, cancellation,
  saturation, failure, and shutdown lifecycles return resources to baseline.
- All six manifest cases pass decoded-RGBA comparison against reviewed Go PNGs
  at the exact recorded inputs and unchanged tolerance `3` / ratio `0.002`.
- `ruff format --check`, `ruff check`, strict Pyright, full pytest/property and
  configured benchmark suites, vulnerability audit, and the exact regular
  CPython `3.14.2` Windows/Cython build and import gate pass. The final ordered
  visualization-performance, visual-system, Windows-compatibility, and code
  reviews have no unresolved findings.

Keep Phase 5b Pending and Phase 6 blocked while any required implementation,
legacy baseline, Python comparison, configured gate, or audit evidence is
missing.
