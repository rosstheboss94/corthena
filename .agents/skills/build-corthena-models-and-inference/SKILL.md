---
name: build-corthena-models-and-inference
description: "Build Phase 9 simulated Models and Inference workflows: registry, aliases, artifacts, compatibility, scoring, rankings, history, export, client effects, simulator, and panels."
---

# Build Corthena Models and Inference

Build the deterministic simulated registered-model-to-inference workflow
without implementing the real model engine or putting preparation, validation,
or export work on the render thread.

## Ground the change

1. Read `AGENTS.md`, `specs/pages/models/README.md`, `specs/pages/inference/README.md`,
   `specs/general/ui/workspaces.md`, `specs/general/ui/README.md`,
   `specs/pages/models/README.md`, `specs/pages/results/README.md`, and
   `specs/general/quality/README.md`.
2. Read `specs/general/api.md` for client or DTO changes,
   `specs/general/ui/visualization.md` for chart, tree, or virtual-table changes,
   and `specs/general/technology-stack.md` for dependency or tooling changes.
3. Inspect the existing app state, actions/effects, `UIClient`,
   `DemoCoordinator`, Phase 8 reconciliation patterns, layouts, charts, tables,
   and golden harness. Preserve unrelated workspace changes.

## Define typed model and inference workflows

- Add concrete validated request, response, registry, alias-history, artifact,
  compatibility, prediction, ranking, and export values. Keep API DTOs separate
  from ui and domain values.
- Make model, run, dataset revision/fingerprint, range, correlation, generation,
  command, inference, and export identities explicit where applicable.
- Extend `UIClient`, `UIEffect`, the bounded effects runtime, reducers,
  and `DemoCoordinator` together. Panels consume typed state and emit actions;
  they never import or branch on simulator details.
- Clone slices at publication boundaries, preserve stable ordering, reject stale
  generations, and make mutating command retries idempotent by command ID.
- Update `specs/general/api.md` only for a real public or process contract. Internal
  Phase 9 demo methods do not define future coordinator endpoints.

## Build the Models workspace

- Implement the immutable model registry, alias and promotion history, artifact
  metadata, feature importance, and array-based tree inspector.
- Show only final refit models as inference artifacts. Preserve model and tree
  selection across filtering, sorting, pagination, refresh, and reconciliation.
- Represent trees with validated same-length typed arrays and node indices.
  Reject invalid indices, feature bounds, cycles, and leaf/split conflicts
  before publishing inspector state.
- Display schema and engine versions, model configuration, feature and target
  definitions, training fingerprint and cutoff, seed/generator information,
  build revision, feature fingerprints, file checksums, and artifact status.
- Require explicit alias confirmation. Apply alias changes transactionally,
  append immutable history, never mutate a model, and never delete the prior
  alias target or history record.

## Build compatibility and Inference behavior

- Validate model schema, engine version, feature registry descriptors and
  fingerprints, feature schema, target definition, dataset compatibility,
  required lookback, and requested range before enabling submission.
- Support historical-range and latest-snapshot scoring. Failed compatibility
  checks produce field-addressed diagnostics and no inference output.
- Publish only complete immutable prediction snapshots carrying model, run,
  dataset fingerprint, feature fingerprints, symbol, timestamp, and score.
- Rank each timestamp by score with stable symbol-ID tie-breaking. Preserve
  explicit missing or ineligible predictions instead of filling from later
  information.
- Implement the model/alias selector, dataset/range selector, compatibility
  summary, ranked symbols, score distribution, prediction history, and export
  status. Keep simulated export preparation off-thread and publish status only
  after complete checksummed output is represented.

## Preserve deterministic asynchronous behavior

- Generate seeded immutable models, alias events, tree arrays, compatibility
  outcomes, predictions, rankings, distributions, and export transitions from
  the same seed, clock, query, and action sequence.
- Run validation, filtering, sorting, tree preparation, ranking, distribution
  preparation, delays, and export simulation on owned cancellable workers.
- Reuse bounded workflow cancellation, deduplication, busy-state, and startup
  reconciliation behavior. A global snapshot must trigger a newer active
  workspace generation rather than overwrite richer state.
- Cover normal, loading, empty, incompatible, failure, degraded, recovered,
  canceled, and saturated conditions without replacing dock layouts.
- Keep Phase 9 ui-only: do not implement estimator fitting, artifact
  filesystem persistence, coordinator repositories, or real HTTP endpoints.

## Verify before handoff

- Add focused reducer, client-boundary, simulator, effect, panel, idempotency,
  compatibility, ranking, cancellation, stale-generation, immutability,
  replay, benchmark, and golden tests.
- Hand off to `$verify-corthena-models-and-inference`; add visualization
  verification for chart, table, tree, cache, or render-buffer changes.
- Apply the focused quality route in `specs/general/quality/README.md`; use
  `$python-windows-compat-gate` only for dependency, native, toolchain, or shell
  changes. Update living specifications for behavior or contract changes.
