---
name: build-corthena-data-and-experiments
description: "Build Phase 7 simulated Data and Experiments workflows: catalog/import, validation, configuration, estimates, autosave, and immutable submission."
---

# Build Corthena Data and Experiments

Build the deterministic simulated dataset-to-experiment workflow without
putting simulator, persistence, validation, or import work on the render
thread.

## Ground the change

1. Read `AGENTS.md`, `specs/routing/phase-7.md`,
   `specs/ui/workspaces.md`, `specs/ui/foundation.md`,
   `specs/data-and-features.md`, and `specs/quality.md`.
2. Read `specs/api.md` for client or DTO changes, `specs/technology-stack.md`
   for dependency/tooling changes, and `specs/README.md` when ownership spans
   Data, Experiments, and ui state.
3. Inspect the existing app state/actions/effects, `UIClient`,
   `DemoCoordinator`, layouts, controls, virtual tables, and Phase 6 patterns.
   Preserve unrelated workspace changes.

## Define typed workflows

- Add concrete validated request, response, draft, catalog, import, and
  experiment value types. Keep JSON/process DTOs separate from domain values.
- Make request identity explicit: dataset/revision, source kind, symbols,
  interval, replacement range or append mode, correlation ID, generation,
  draft revision, and immutable submission ID as applicable.
- Extend `UIClient`, `UIEffect`, the effects runtime, and
  `DemoCoordinator` together. Panels depend only on typed state/actions and
  must not import simulator packages or type-switch on simulator values.
- Reject invalid source/range combinations, duplicate IDs, invalid intervals,
  invalid split/model/feature configurations, stale draft revisions, and
  mutable resubmission attempts at their owning boundaries.
- Update `specs/api.md` only when a public/process contract changes; internal
  demo contracts do not define future coordinator endpoints.

## Build deterministic Data behavior

- Provide seeded catalog datasets with stable dataset IDs, content
  fingerprints, revisions, symbol/time coverage, declared adjustments, and
  deterministic validation diagnostics.
- Model import states in place: idle, queued, validating, importing, ready,
  rejected, failed, canceled, and saturated. Support deterministic normal,
  empty, duplicate, malformed, append, and selected-range replacement cases.
- Keep append and replacement atomic in the simulator: validate before
  publishing a new catalog revision; preserve the prior immutable snapshot on
  failure or cancellation.
- Keep timestamp normalization, `(symbol, timestamp)` ordering, OHLC checks,
  finite prices, nonnegative volume, and correction-range rules consistent
  with `specs/data-and-features.md`. Do not simulate future data to satisfy a
  visible range.
- Run preparation, filtering, sorting, validation, delays, and autosave I/O
  on owned cancellable workers. Reuse the existing bounded effect runtime,
  generation handling, and typed busy behavior.

## Build the Data workspace

- Implement the catalog table, coverage timeline, import/validation queue,
  dataset inspector, and import logs from typed state and reusable controls.
- Preserve stable dataset/import selection through refreshes, filters,
  pagination, retries, and catalog revisions.
- Render loading, empty, validation-error, failure, canceled, degraded, and
  recovered states inside panels without replacing the dock layout.
- Propagate only supported dataset, symbol, interval, and range context
  through the assigned link group; comparison groups remain independent.

## Build the Experiments workspace

- Implement a virtualized experiment list, searchable configuration section
  tree, typed property editor, validation summary, resource estimate, and
  contextual inspector.
- Represent dataset, selected compiled features, target, split, model,
  portfolio, and optional sweep with concrete draft values. Show feature name,
  semantic version, lookback, output schema, and fingerprint; never accept
  source paths or runtime scripts.
- Make validation and resource estimation deterministic and generation-safe.
  Present field-specific errors without mutating the last valid draft.
- Autosave drafts through coalesced background effects with revision checks,
  cancellation, recovery, and retryable failure UI. A late load must not
  overwrite a local edit.
- Freeze a validated submission into a new immutable experiment definition.
  Safe retries use a command ID and must not duplicate or mutate an accepted
  definition.

## Preserve data and execution safety

- Retain catalog revision and content fingerprint in every draft/submission.
  Changing a catalog after submission must not rewrite the submitted
  experiment definition.
- Keep target/split configuration explicit and validate it before submission.
  Do not derive features, targets, or resource estimates from future bars.
- Keep source ordering stable by timestamp, symbol, and source row ID. Treat
  completed imports and submitted definitions as immutable client-owned data.

## Verify before handoff

- Add reducer, client-boundary, simulator, effect, panel, link-group,
  autosave, cancellation, stale-generation, and immutable-submission tests
  with each slice.
- Run `$verify-corthena-data-and-experiments`; use
  `$verify-corthena-visualization-performance` if chart/table kernels,
  virtualization, or caches change.
- Hand off to `$verify-corthena-data-and-experiments` and apply the focused
  quality route; use `$python-windows-compat-gate` for dependency or native changes.
- Update living specs for behavior or contract changes. Do not mark Phase 7
  complete while a Data/Experiments panel, deterministic scenario, autosave
  path, immutable submission path, validation case, or required quality gate
  is missing.
