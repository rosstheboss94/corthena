---
name: verify-corthena-jobs-and-results
description: "Verify Corthena Phase 8 simulated Jobs and Results workflows for typed boundaries, lifecycle controls, checkpoint safety, immutable comparisons, metric separation, determinism, cancellation, stale generations, concurrency, performance, and the 60-image golden matrix. Use for Phase 8 acceptance, regression testing, or audits; exclude real backend and Phase 9 behavior."
---

# Verify Corthena Jobs and Results

Produce repeatable evidence that Phase 8 is typed, deterministic,
generation-safe, immutable after completion, and ready for a future backend
adapter.

## Ground verification

1. Read `python_migration/AGENTS.md`, `python_migration/specs/contract.md`,
   `python_migration/specs/design-pattern.md`, and
   `python_migration/specs/routing/phase-8.md` before selecting checks.
2. Read the route-required Jobs, Results, async-effects, training-runtime,
   evaluation, common-quality, and concurrency-quality specifications. Add
   `api.md` for public/process boundary checks and `ui/visualization.md` plus
   visualization quality requirements for changed visualizations.
3. Inspect only the relevant `UIClient` contract and values, state/actions,
   reducer/effects, simulator, Jobs/Results panels, shared chart/table paths,
   focused tests, benchmarks, and golden manifest.
4. Stay read-only for review-only requests. Add or update tests, benchmarks,
   captures, or baselines only when implementation or acceptance-gap closure
   is explicitly in scope.

## Verify typed boundaries and replay

- Exercise every Jobs and Results request through `UIClient` and the bounded
  effects runtime. Fail if panels import simulator details, perform blocking
  work, or publish native/library values.
- Validate job, experiment, run, checkpoint, command, correlation, request,
  comparison, generation, cursor, range, schema, and metric identities at
  their owning boundaries.
- Assert pages, stages, metrics, logs, checkpoints, run details, comparison
  values, and render-ready buffers are immutable client-owned publications
  with stable IDs and deterministic order.
- Replay identical seeds, clocks, and action sequences under varied completion
  orders, delays, worker counts, and queue pressure. Require identical state,
  effects, requests, lifecycle histories, completed runs, comparisons, and
  visible buffers.
- Test cancellation, supersession, deduplication, saturation, stale-generation
  rejection, bounded draining, channel closure, repeated startup/shutdown, and
  injected failures without leaked threads, tasks, queues, or subscriptions.

## Prove Jobs lifecycle and checkpoint safety

- Exercise the complete specified lifecycle and reject every illegal control
  transition. Verify control enablement matches typed state and repeated
  command IDs cannot duplicate an accepted transition.
- Cover stable virtual-queue ordering and selection through sorting, filtering,
  pagination, refresh, reconciliation, degradation, and recovery.
- Prove successful immutable completion, pause/resume only at a durable
  boundary, cooperative cancellation, interruption requiring explicit resume,
  and preservation of completed immutable outputs and audit history.
- Inject corrupt, partial, checksum-invalid, version-incompatible, and
  identity-mismatched checkpoint metadata. Require fail-closed behavior,
  retention of the prior valid checkpoint, and no silent repair or truncation.
- Verify stages, progress, metrics, leases, worker health, checkpoint status,
  and logs remain synchronized by stable job identity and logical event order.

## Prove Results correctness and immutability

- Exercise run filtering, stable selection, virtual paging, and comparison of
  zero through four completed runs. Reject duplicate, unknown, incomplete, or
  over-limit comparison selections at typed boundaries.
- Assert validation metrics used for selection remain separate from explicitly
  labeled test metrics. Test results must not affect model selection or earlier
  fold state.
- Use hand-calculated fixtures for MSE, MAE, per-timestamp Pearson IC, stable
  average-rank IC, fold stability, equity, drawdown, turnover, and reference
  backtest disclosures. Preserve missing values instead of substituting zero.
- Check chronological train/validation/test windows, purge/embargo boundaries,
  next-open execution, stable symbol-ID tie-breaking, transaction costs, and
  rejection of future or later-fill information.
- Verify completed summaries, details, chart series, distributions, overlays,
  and configuration differences never mutate after publication.
- Supersede and cancel comparison queries at each boundary. Require stale
  results to be discarded while the last valid immutable comparison remains
  visible through loading, failure, and recovery.

## Verify visible behavior and bounded work

- Replay pointer and keyboard paths for queue selection, sorting/filtering,
  pagination, lifecycle controls, checkpoint diagnostics, log navigation, run
  filters, comparison selection, chart interaction, and configuration diff.
- Cover normal, loading, empty, paused, interrupted, cancelled, failed,
  incompatible, degraded, recovered, busy, and stale states without dock-layout
  replacement or paint/hit-test/clip divergence.
- Require proportional-work evidence for virtual queues, logs, run pages,
  comparisons, charts, and distributions. Invoke
  `$verify-corthena-visualization-performance` for changed chart, table,
  virtualization, cache, LOD, worker, or render-buffer paths.
- Capture every manifest-owned Phase 8 scenario at 1280x720 and 1920x1080 with
  100%, 150%, and 200% scaling, totaling the required 60-image matrix. Record
  seed, clock, asset fingerprint, backend, scenario, baseline version, and the
  owning comparison thresholds. Treat missing manifest scenarios, baselines,
  or specified thresholds as acceptance blockers rather than inventing them.
- Use `$verify-corthena-raylib-visual-system` for visual-system compliance and
  canonical golden evidence when rendering or interaction changes.

## Conclude the gate

- Run focused tests first, then every applicable configured format, lint,
  typing, unit/property, benchmark, vulnerability, hidden-launch, concurrency,
  and Windows/native gate.
- Report exact commands, lifecycle transition coverage, checkpoint fault
  matrix, comparison and leakage fixtures, replay/concurrency evidence,
  performance evidence, golden differences, skipped checks, and residual risks.
- Finish with `$review-corthena-code`. Do not mark Phase 8 complete while any
  required panel, lifecycle/control case, checkpoint failure, immutable result
  path, stale-generation race, golden entry, or applicable quality gate is
  missing or failing.
