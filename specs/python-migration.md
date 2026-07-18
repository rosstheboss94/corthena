# Python/Cython Migration

**Status:** Authoritative  
**Owner:** Architecture  
**Last updated:** 2026-07-18
**Related:** [Concurrency and parallelism](concurrency-and-parallelism.md), [Technology stack](technology-stack.md), [Migration baseline](migration-baseline.md), [Roadmap](roadmap.md), [System architecture](system-architecture.md), [Quality](quality.md), [ADR 0002](decisions/0002-python-library-estimators-and-artifacts.md), [ADR 0008](decisions/0008-regular-cpython-concurrency.md), [ADR 0006](decisions/0006-curated-python-dependencies.md)

This document records the completed repository-root and simulator-backed UI
cutover from the retired Go implementation to Python/Cython. It also preserves
the compatibility rules that apply to subsequent Python development. The
cutover does not imply that the planned coordinator, worker, supported client,
CLI, persistence, or real research backend has been implemented.

## Completed Cutover

The accepted cutover completed these steps:

1. Python/Cython became the approved stack through specifications, ADRs,
   repository guidance, and configured quality gates.
2. `pyproject.toml`, `uv.lock`, the `src/corthena` package, named workstation
   entry point, tool configuration, and Windows Cython smoke coverage were
   established at the repository root.
3. The Raylib shell, immutable UI state, effects, docking, persistence, charts,
   tables, linked views, and simulator-backed Phase 6--9 workflows were ported
   behind `UIClientProtocol`.
4. Required assets, screenshots, golden manifests, and retained cutover PNGs
   moved into Python-owned package and test paths.
5. The superseded Go `cmd/` and `internal/` runtime trees and the temporary
   `python_migration/` staging root were retired.

The real coordinator, worker protocol runtime, supported Python client, CLI,
repositories, domain engine, estimator integration, and inference backend are
post-cutover product work owned by the roadmap and domain specifications.

## Command and Package Mapping

| Surface | Python/Cython target and status |
|---|---|
| workstation entry point | Implemented as `corthena.workstation.__main__` and `corthena-workstation` |
| coordinator entry point | Planned as `corthena.coordinator.__main__` or `corthena-coordinator` |
| worker entry point | Planned as `corthena.worker.__main__` or `corthena-worker` |
| research CLI entry point | Planned as `corthena.cli.__main__` or `corthena` |
| simulator-backed UI | Implemented under `corthena.ui` |
| UI client boundary | Implemented as `corthena.ui.client.UIClientProtocol` |
| supported backend client | Planned as `corthena.client` |
| public process contracts | Planned as `corthena.contracts` |
| build and test tooling | Implemented with `uv`, Ruff, Pyright, pytest, Hypothesis, pytest-benchmark, vulnerability scanning, and the Cython build |

Planned surfaces must preserve the accepted UI workflows and contract semantics
when their real adapters replace the simulator.

## Compatibility Rules

- Runtime, tools, tests, packaging, and extensions use exactly regular Windows
  AMD64 CPython `3.14.2`; startup rejects free-threaded and other versions.
  Phase 0 acceptance established the `cp314-win_amd64` ABI; repeat the gate
  when native dependencies or toolchains change.
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

## Retained Cutover and Golden Policy

The reviewed lossless PNGs and manifests retained under `tests/goldens/` are
the frozen cutover evidence and regression references. The retired Go source
tree is no longer an active authority. JPEGs under `screenshots/` remain
inspection aids and are never normative. See
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
Screenshots and retained PNG baselines should be regenerated only for an
intentional accepted visual change with updated manifest metadata.

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
