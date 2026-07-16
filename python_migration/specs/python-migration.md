# Python/Cython Migration

**Status:** Authoritative  
**Owner:** Architecture  
**Last updated:** 2026-07-12  
**Related:** [Concurrency and parallelism](concurrency-and-parallelism.md), [Technology stack](technology-stack.md), [Migration baseline](migration-baseline.md), [Roadmap](roadmap.md), [System architecture](system-architecture.md), [Quality](quality.md), [ADR 0002](decisions/0002-python-library-estimators-and-artifacts.md), [ADR 0008](decisions/0008-regular-cpython-concurrency.md), [ADR 0006](decisions/0006-curated-python-dependencies.md)

This document owns the Python/Cython implementation layer before runtime code is rewritten. It
does not relax product behavior, UI workflows, typed boundaries, immutability,
future-data leakage prevention, or auditability.

## Migration Order

1. Revise authoritative specs, ADRs, agent guidance, and quality gates to make
   Python/Cython the approved stack.
2. Create Python project scaffolding: `pyproject.toml`, `uv.lock`,
   `corthena/...` package layout, CLI entry points, tool configuration, and
   Windows Cython build smoke tests.
3. Port the Raylib shell with the same workspace tabs, dock state, preferences,
   layouts, input handling, simulator client boundary, and screenshot
   scenarios.
4. Port ui state, effects, charts, tables, linked views, and workspace
   workflows behind the same `UIClient` concept.
5. Port coordinator contracts, DTO validators, event stream, health, process
   launch, cancellation, and reconciliation.
6. Port data import, catalog revisions, feature/target materialization,
   leakage-safe split validation, and typed repository boundaries.
7. Port experiment submission, estimates, jobs, checkpoints where supported,
   immutable runs, results, model registry, aliases, and inference workflows.
8. Integrate scikit-learn and PyTorch estimators behind Corthena-owned
   orchestration, manifests, compatibility checks, and audit metadata.
9. Retire or archive superseded legacy runtime code only after equivalent
   Python workflows pass behavioral, quality, and screenshot acceptance.

## Command and Package Mapping

| Legacy surface | Python/Cython target |
|---|---|
| workstation entry point | `corthena.workstation.__main__` or `corthena-workstation` script |
| coordinator entry point | `corthena.coordinator.__main__` or `corthena-coordinator` script |
| worker entry point | `corthena.worker.__main__` or `corthena-worker` script |
| research CLI entry point | `corthena.cli.__main__` or `corthena` script |
| ui modules | `corthena.ui...` |
| workstation application modules | `corthena.app.workstation...` |
| simulator modules | `corthena.simulator...` |
| `client` package concept | `corthena.client` |
| `contract` package concept | `corthena.contracts` |
| legacy build/test tooling | `uv`, `ruff`, `pyright`, `pytest`, `hypothesis`, `pytest-benchmark`, vulnerability scanner, Cython build |

The mapping is conceptual until Python scaffolding exists. Do not create
parallel behavior with different user workflows or public contract semantics.

## Compatibility Rules

- Runtime, tools, tests, packaging, and extensions use exactly regular Windows
  AMD64 CPython `3.14.2`; startup rejects free-threaded and other versions.
  Repeat Phase 0 for the `cp314-win_amd64` ABI.
- Preserve UI, coordinator, and per-training-job topology. Apply
  [concurrency and parallelism](concurrency-and-parallelism.md) to execution
  selection, synchronization, resource bounds, determinism, cancellation, and
  shutdown throughout the port.

- Preserve existing UI workflows, workspace names, state transitions, layout
  behavior, link-group semantics, component health states, and user-visible
  validation results unless an owning spec changes them.
- Preserve API concepts: versioned loopback service, validated DTOs, rejected
  unknown fields, correlation IDs, stable error codes, event invalidation, and
  REST reconciliation.
- Preserve immutable completed runs, artifacts, aliases transactions, registry
  entries, audit metadata, data fingerprints, feature schema fingerprints, and
  compatibility checks.
- Preserve future-data leakage prevention in imports, features, targets,
  splits, embargo/purge rules, evaluation, refits, and inference.
- Preserve deterministic outputs for repeated seed/action sequences. Where a
  third-party library cannot guarantee byte-identical numerical output across
  thread counts, record the limit in the manifest and constrain execution to a
  deterministic supported mode.
- Keep blocking I/O, decoding, database, network, training, and library calls
  off the render thread.
- Treat native/library values as adapter-local. Public and domain boundaries use
  typed Python values, dataclasses, Pydantic DTOs, NumPy arrays behind typed
  adapters, or project manifest types.

## Ownership Boundary

Corthena owns:

- orchestration, scheduling, cancellation, pause/resume policy, and job state;
- DTO validation, domain conversion, manifests, schema versions, compatibility
  checks, registry promotion, immutable aliases, and audit metadata;
- data import validation, feature/target leakage rules, split rules, and
  materialization fingerprints;
- UI state, workspace workflows, simulator contracts, screenshot scenarios, and
  visual acceptance;
- artifact wrappers, hashes, dependency metadata, and failure semantics.

scikit-learn and PyTorch own approved estimator internals, fitting algorithms,
prediction behavior, metrics where admitted, and library-native artifact
payloads. Corthena never exposes raw library artifacts as public contracts.

## Cython Rule

Start every module in typed Python. Add Cython only after a benchmark or native
adapter requirement identifies a hot path or boundary that Python cannot meet.
Cython modules must have typed adapters, Windows build coverage, focused tests,
and fallback or failure behavior documented in the owning spec. Ordinary Cython
code holds the GIL. The canonical
[native parallelism and `nogil` admission](concurrency-and-parallelism.md#native-parallelism-and-nogil-admission)
rules govern any release. Windows builds verify the regular `cp314` extension
tag.

## Screenshot and Golden Migration Policy

The approved legacy implementation and its manifest-backed tests are the
canonical parity source. Lossless legacy PNG manifests, not the migration
JPEGs, are acceptance baselines for visual parity during the implementation.
See
[Migration parity baseline](migration-baseline.md) for ownership, capture, and
pixel-comparison rules.

Each screenshot baseline records metadata beside the image:

- phase and scenario name;
- viewport size;
- Windows scale factor;
- seed and scenario clock;
- dataset fixture and fixture version;
- layout name and serialized app state revision;
- font and icon asset versions;
- rendering backend and relevant native dependency versions;
- build revision and dirty-build flag.

The required viewport matrix is 1280x720 and 1920x1080 at 100%, 150%, and 200%
scaling where feasible. A skipped scale or viewport requires a metadata reason.
Screenshots should be regenerated only for intentional visual changes or
approved migration parity updates.

## Acceptance Gates

- `ruff format --check`
- `ruff check`
- `pyright`
- `pytest`
- `hypothesis` coverage for parsers, DTOs, manifests, leakage rules, and model
  compatibility
- `pytest-benchmark` for hot paths and migration parity checks
- `pip-audit` or `osv-scanner`
- Windows Cython extension build and import smoke tests
- Deterministic replay of phase workflows with equivalent state transitions and
  screenshot baselines
