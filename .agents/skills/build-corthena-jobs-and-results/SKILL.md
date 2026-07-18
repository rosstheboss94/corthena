---
name: build-corthena-jobs-and-results
description: "Build Corthena Phase 8 simulated Jobs and Results workflows, including the virtual job queue, lifecycle commands, checkpoint status, immutable run comparisons, result charts, generation safety, client effects, simulator behavior, and panels. Use for Phase 8 implementation or changes; exclude the real training backend, model engine, registry, and inference."
---

# Build Corthena Jobs and Results

Build the deterministic simulated experiment-to-job-to-results workflow while
keeping training, preparation, filtering, and comparison work off the render
thread.

## Ground the change

1. Read `AGENTS.md`, `specs/contract.md`,
   `specs/design-pattern.md`, and
   `specs/routing/phase-8.md` before planning or editing.
2. Read the route-required Jobs, Results, async-effects, training-runtime,
   evaluation, common-quality, and concurrency-quality specifications. Read
   `api.md` only for public/process contracts and `ui/visualization.md` for
   chart, table, virtualization, cache, or render-buffer changes.
3. Follow the minimum-context rules in `contract.md`. Inspect the existing
   `UIClient` contract, app state/actions/effects, simulator, Jobs/Results
   shell composition, Phase 7 workflow patterns, shared visualizations, tests,
   and golden harness only as the change crosses those boundaries.
4. Preserve unrelated workspace changes. Record behavior or public-contract
   divergence under `specs/missing/`; do not silently rewrite an owning spec.

## Define typed Phase 8 boundaries

- Add concrete immutable values for job summaries/details, ordered stages,
  progress, metrics, leases, worker health, checkpoints, logs, run summaries,
  comparison queries, comparison results, chart series, distributions,
  overlays, and configuration differences.
- Make job, experiment, run, command, correlation, request, comparison, and
  generation identities explicit. Validate enum states, ranges, cursors,
  timestamps, schemas, and cross-field invariants at their owning boundaries.
- Extend `UIClient`, effects, reducers, the bounded effects runtime, and the
  deterministic simulator together. Panels consume typed immutable state and
  emit actions; they never import simulator internals or call backend services.
- Publish client-owned immutable copies in stable logical order. Apply results
  by generation and request identity rather than completion order.
- Keep internal demo operations distinct from future coordinator endpoints.
  Do not change `api.md` merely to describe simulator behavior.

## Build the Jobs workflow

- Implement the virtualized queue plus selected-job stage/progress, live
  metrics, resources, process/checkpoint status, logs, and lifecycle controls.
- Preserve stable job and log selection across refresh, sorting, filtering,
  pagination, reconciliation, degradation, and recovery. Order equal-priority
  jobs by persisted stable identity, never arrival or dictionary order.
- Model exactly the specified lifecycle states. Enable pause, resume, and
  cancel only from valid states; require explicit resume for interruption and
  make accepted command retries idempotent by command ID.
- Simulate successful immutable completion, pause/resume at a durable boundary,
  cooperative cancellation, interruption requiring resume, and fail-closed
  checkpoint incompatibility. Preserve completed immutable outputs and
  auditable state after cancellation or failure.
- Represent checkpoint compatibility, committed progress, and prior valid
  checkpoint retention without implementing real checkpoint filesystem I/O,
  worker processes, estimators, or coordinator persistence.

## Build the Results workflow

- Implement the run browser and filters, up-to-four-run comparison,
  metric/fold table, equity/drawdown charts, fold timeline, IC/prediction
  distributions, prediction/market overlay, and configuration diff.
- Keep validation selection metrics and untouched test metrics as distinct,
  explicitly labeled partitions. Never use test metrics for selection.
- Publish completed run summaries and details as immutable values with stable
  run IDs, chronological folds/windows, metric partitions, configuration,
  provenance, and reference-backtest disclosures.
- Make comparison queries filterable, cancellable, deduplicated, and
  generation ordered. Retain the last immutable comparison while a refresh is
  loading or fails; reject stale completions without blanking valid content.
- Use existing chart/table transforms and render primitives. Prepare sorting,
  filtering, pagination, distributions, overlays, and comparison buffers on
  bounded cancellable workers.

## Preserve deterministic asynchronous behavior

- Derive seeded jobs, lifecycle events, metrics, logs, checkpoints, completed
  runs, comparisons, and visible buffers from stable logical coordinates and a
  recorded clock.
- Coalesce replaceable refreshes, surface typed busy states on saturation, and
  cancel superseded or hidden workflow requests by scope. Drain a bounded
  number of results per frame and close workers in dependency order.
- Cover normal, loading, empty, paused, interrupted, cancelled, failed,
  incompatible, degraded, recovered, stale, and saturated states without
  replacing dock layouts.
- Keep Phase 8 simulator-only. Exclude estimator fitting, real workers and
  checkpoints, coordinator repositories/routes, registry, aliases, model
  artifacts, and inference scoring.

## Verify before handoff

- Add focused model/validation, reducer, client-boundary, simulator, effect,
  panel, lifecycle, checkpoint, comparison, cancellation, saturation,
  stale-generation, replay, immutability, benchmark, and golden tests.
- Hand off to `$verify-corthena-jobs-and-results`. Invoke
  `$verify-corthena-visualization-performance` and
  `$verify-corthena-raylib-visual-system` when shared visualization or visual
  behavior changes.
- Run the route's applicable quality gates. Use
  `$python-windows-compat-gate` only for dependency, native, toolchain, shell,
  or Windows-boundary changes, then finish with `$review-corthena-code`.
- Do not claim Phase 8 acceptance while a required panel, lifecycle scenario,
  checkpoint case, immutable comparison path, stale-generation case, quality
  gate, or manifest-owned entry in the 60-image matrix is missing.
