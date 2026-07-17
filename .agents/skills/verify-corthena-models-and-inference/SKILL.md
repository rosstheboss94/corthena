---
name: verify-corthena-models-and-inference
description: "Verify Phase 9 Models and Inference: registry, artifacts, aliases, compatibility, scoring, determinism, cancellation, races, benchmarks, and golden images."
---

# Verify Corthena Models and Inference

Produce repeatable evidence that the simulated registered-model-to-inference
workflow is typed, deterministic, immutable, compatibility-safe, and ready for
the future real Python client.

## Ground verification

1. Read `python_migration/AGENTS.md`, `python_migration/specs/routing/phase-9.md`,
   `python_migration/specs/ui/workspaces.md`, `python_migration/specs/ui/foundation.md`,
   `python_migration/specs/models.md`, `python_migration/specs/evaluation-and-inference.md`, `python_migration/specs/api.md`,
   `python_migration/specs/ui/visualization.md`, and `python_migration/specs/quality.md`.
2. Inspect Models/Inference state, actions/effects, client types, simulator,
   renderers, layouts, charts, virtual tables, tree buffers, tests, benchmarks,
   and golden harness before selecting checks. Preserve unrelated changes.
3. Stay read-only when asked only to audit. Add tests, scenarios, benchmarks,
   or baselines only when implementation or acceptance-gap closure is in scope.

## Verify typed boundaries and deterministic state

- Exercise every Models and Inference request through `UIClient` and the
  bounded effects runtime; fail if panels import or type-switch on simulator
  details.
- Validate model, run, alias, dataset revision/fingerprint, range, schema,
  engine, feature, target, lookback, correlation, generation, command,
  inference, and export identities at their owning boundaries.
- Assert registry entries, artifact metadata, tree arrays, histories,
  predictions, rankings, and distributions are client-owned immutable copies
  with stable IDs and deterministic ordering.
- Replay identical action sequences under varied completion orders. Require
  identical state, effects, requests, compatibility outcomes, alias events,
  prediction artifacts, ranks, export state, and visible buffers.
- Test cancellation, deduplication, saturation, stale-generation rejection,
  startup and reconnect reconciliation, bounded draining, channel closure, and
  shutdown without process, thread, or task leaks.

## Prove registry, artifact, and alias safety

- Verify only complete final-refit models appear as inference artifacts and
  later registry refreshes never mutate completed entries.
- Check artifact schema and engine versions, model configuration, feature and
  target definitions, training fingerprint/cutoff, seeds, generator version,
  build revision, feature fingerprints, array shapes, and checksums.
- Test tree arrays for length mismatches, invalid child or feature indices,
  cycles, leaf/split conflicts, missing routing, and bounded inspector traversal.
- Exercise explicit alias confirmation, idempotent command replay, concurrent
  refresh, failed assignment, and promotion history. Require transactional
  alias changes and immutable retention of every previous target.
- Preserve stable registry, feature, and node selection across sorting,
  filtering, pagination, refresh, degradation, and recovery.

## Prove compatibility and inference correctness

- Independently reject incompatible model schema, engine, feature descriptors
  or fingerprints, feature schema, target definition, dataset fingerprint,
  lookback, and historical range. Rejection must create no prediction output.
- Cover historical and latest-snapshot scoring, cancellation before and during
  work, incomplete temporary output, failure, recovery, and export transitions.
- Use hand-calculated fixtures to verify descending score order and stable
  symbol-ID tie-breaking. Preserve missing scores and reject ineligible rows
  instead of using later market data.
- Verify each completed prediction carries symbol, timestamp, model, run, data
  fingerprint, feature fingerprints, and score, and that only complete
  checksummed snapshots enter history or export-ready state.
- Check score histograms, ranking pages, model/alias resolution, stable row
  selection, filter/sort/pagination behavior, and display of compatibility
  reasons before submission.

## Verify visible work and rendering

- Replay keyboard and pointer input for model selection, registry filtering,
  alias confirmation, tree navigation, dataset/range selection, inference
  submission, ranking selection, history, and export controls.
- Cover normal, loading, empty, incompatible, failure, retry, canceled,
  degraded, recovered, and busy states for every Models and Inference panel
  without dock-layout replacement.
- Capture Models and Inference Raylib goldens at 1280x720 and 1920x1080 with
  100%, 150%, and 200% scaling. Record seed, clock, asset fingerprint, backend,
  scenario, tolerance, and baseline version; keep image I/O off the UI thread.
- Invoke `$verify-corthena-visualization-performance` for changed chart, table,
  tree, cache, or render-buffer paths. Retain operation-count assertions and
  representative allocation benchmarks.

## Conclude the gate

- Run focused tests, then applicable configured `ruff format --check`, `ruff check`,
  `pyright`, `pytest`, `hypothesis`, `pytest-benchmark`, vulnerability scans, and hidden smoke
  launches.
- Report exact commands, scenario and compatibility coverage, determinism and
  immutability evidence, ranking fixtures, golden differences, skipped checks,
  and residual risks.
- Do not mark Phase 9 complete while any required panel, alias transaction,
  compatibility dimension, inference path, deterministic scenario, golden
  entry, or quality gate is missing.
