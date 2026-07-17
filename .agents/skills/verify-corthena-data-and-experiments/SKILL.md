---
name: verify-corthena-data-and-experiments
description: "Verify Phase 7 Data and Experiments: imports, revisions, validation, drafts, estimates, autosave, submission, determinism, races, and goldens."
---

# Verify Corthena Data and Experiments

Produce repeatable evidence that the simulated dataset-to-experiment workflow
is typed, deterministic, atomic, immutable after submission, and safe for
future backend replacement.

## Ground verification

1. Read `python_migration/AGENTS.md`, `python_migration/specs/routing/phase-7.md`,
   `python_migration/specs/ui/workspaces.md`, `python_migration/specs/ui/foundation.md`,
   `python_migration/specs/data-and-features.md`, `python_migration/specs/quality.md`, and `python_migration/specs/api.md`.
2. Inspect Data/Experiments state, actions/effects, client types, simulator,
   renderers, layout/link groups, persistence workers, virtual tables, and
   existing tests before selecting checks. Preserve unrelated changes.
3. Stay read-only when asked only to audit. Add tests, scenarios, benchmarks,
   or baselines only when implementation or acceptance-gap closure is in
   scope.

## Verify typed boundaries and deterministic state

- Exercise every Data and Experiments request through `UIClient` and
  the effects runtime; fail if panels import or branch on simulator details.
- Validate dataset/revision/fingerprint, import mode/range, symbols,
  interval, correlation, generation, draft revision, configuration values,
  command ID, and submission identity at their owning boundaries.
- Assert published slices/pages/logs/diagnostics are client-owned immutable
  copies with stable IDs and deterministic ordering.
- Replay identical action sequences under varied completion orders. Require
  identical state, emitted effects, requests, validation results, estimates,
  catalog revisions, submissions, and visible buffers.
- Test cancellation before and during import, validation, estimation,
  autosave, and submission; test queue saturation, stale-generation rejection,
  bounded draining, queue closure, and shutdown with explicit lifecycle checks.

## Prove catalog and import correctness

- Cover normal, empty, malformed, duplicate, append, replacement, canceled,
  failed, degraded, recovered, and saturated import scenarios.
- Check UTC normalization, timestamp/symbol/source-row ordering, unique
  `(symbol, timestamp)` keys, finite OHLC values, OHLC relationships,
  nonnegative volume, declared adjustments, and selected replacement bounds.
- Prove atomicity: failed or canceled validation/import leaves the prior
  catalog revision and content fingerprint untouched; accepted append or
  replacement creates exactly one new revision.
- Verify Data link groups propagate only supported dataset/symbol/interval/
  range scopes and independent comparison groups remain unchanged.

## Prove experiment draft and submission behavior

- Exercise configuration-tree search, typed property edits, feature metadata,
  target/split/model/portfolio/sweep validation, field errors, and stable
  selection across refreshes.
- Assert estimates are deterministic, prepared off the render thread, and
  keyed by the validated draft and catalog fingerprint.
- Test autosave coalescing, revision ordering, startup recovery, corrupt or
  stale draft fallback, cancellation, retry, and late-load protection.
- Verify a valid submission freezes an immutable experiment definition with a
  catalog revision/fingerprint and command ID. Repeated safe retries must
  return the same accepted definition; later catalog/draft changes must not
  mutate it.
- Use hand-calculated fixtures to prove feature lookbacks, forward targets,
  split membership, and resource estimates do not use future observations.

## Verify visible work and rendering

- Replay keyboard and pointer input for catalog selection, import controls,
  timeline range selection, table sorting/filtering/pagination, config-tree
  navigation, property edits, autosave indicators, validation focus, and
  submission confirmation.
- Cover loading, empty, error, retry, canceled, degraded, recovered, and busy
  states for every Data and Experiments panel without dock-layout replacement.
- Capture Data and Experiments Raylib goldens at 1280x720 and 1920x1080 with
  100%, 150%, and 200% scaling. Record seed, clock, asset fingerprint,
  backend, scenario, tolerance, and baseline version; keep image I/O and
  comparison off the render thread.
- Invoke `$verify-corthena-visualization-performance` if virtual tables,
  coverage timelines, cache behavior, or chart/table kernels change. Retain
  operation-count assertions and representative allocation benchmarks.

## Conclude the gate

- Run focused tests, then applicable configured `ruff format --check`, `ruff check`,
  `pyright`, `pytest`, `hypothesis`, `pytest-benchmark`, vulnerability scans, and hidden smoke
  launches.
- Report exact commands, scenario coverage, determinism evidence, atomicity
  and immutability cases, golden differences, skipped checks, and residual
  risks. Do not mark Phase 7 complete while any required workflow, scenario,
  atomicity assertion, autosave/submission path, golden entry, or quality gate
  is missing.
