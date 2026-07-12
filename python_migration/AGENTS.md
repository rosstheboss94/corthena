# Repository Guidance

Specification-first, Windows-focused trading workstation. CPython, `uv`,
`pyproject.toml`, and `specs/technology-stack.md` define the runtime, tooling,
tests, packaging, and approved direct dependencies. The repository is in a
Python/Cython migration layer: revise authoritative specs before rewriting
runtime code, and do not invent commands before project files exist.

All Python rewrite code belongs under this migration root:
`C:\Users\torra\Desktop\Projects\corthena\python_migration`. Keep the existing
Go implementation in the repository root, `cmd/`, and `internal/` available as
the reference implementation until the Python rewrite reaches accepted parity.

## Invariants

- Use exactly regular CPython `3.14.2` (`cp314-win_amd64`) for runtime, tooling,
  tests, packaging, and extension loading; reject the free-threaded build and
  every other patch version at startup;
  use Cython only for measured hot paths or native adapters.
- Never rely on the GIL for application-level synchronization. Mutable state has one owner or is
  protected by an explicit lock or queue; published snapshots are immutable.
- Use bounded processes for pure-Python CPU parallelism, threads for I/O and
  orchestration, and library/native threads only within coordinator CPU leases.
- Keep Raylib/Raygui on the locked UI OS thread; keep I/O, decoding, database,
  network, training, library calls, and blocking work off the render thread.
- Published NumPy arrays, memory maps, Arrow buffers, and tensors are read-only;
  tasks own mutable outputs.
- Preserve deterministic results across worker counts and completion orders.
- Prevent future-data leakage; completed runs and artifacts are immutable.
- Keep domain behavior out of render functions and native/weakly typed values in adapters.
- Use typed validated boundaries with type hints, dataclasses or Pydantic DTOs,
  Pyright, and explicit validators; avoid `Any`, unchecked casts, untyped dict
  payloads, reflection-style models, and native/library values past adapters.
- Once scaffolding exists, required gates are `ruff format --check`,
  `ruff check`, `pyright`, `pytest`, property and benchmark tests where
  applicable, vulnerability scanning, and Windows Cython extension builds.

## Before planning or editing

Classify the task, read `specs/design-pattern.md`, then read only the route-specific
documents below. Read `specs/README.md` when ownership crosses documents. Read
`specs/quality.md` for implementation, review, test, or performance work;
`technology-stack.md` for dependencies or tooling; `api.md` for public/process
boundaries. Update the owning spec when behavior or a public contract changes.

## Specification routing

| Task | Required specifications |
|---|---|
| All Python/Cython migration routes | `specs/design-pattern.md`, plus the owning subsystem spec |
| Architecture, module boundaries, route handlers, adapters, dependency direction, or design patterns | `specs/design-pattern.md`, plus the owning subsystem spec |
| Product scope or requirements | `specs/product.md`, `specs/roadmap.md` |
| Dependencies, packaging, or tooling | `specs/technology-stack.md`, `specs/quality.md` |
| Python/Cython migration | `specs/python-migration.md`, `specs/migration-baseline.md`, `specs/technology-stack.md`, `specs/roadmap.md` |
| Phase 12 Python scaffold and shell | `specs/routing/phase-12.md` |
| Processes, storage, runtime, concurrency | `specs/system-architecture.md`, `specs/decisions/0008-regular-cpython-concurrency.md` |
| Imports, datasets, features, targets | `specs/data-and-features.md` |
| Trees, forests, boosting, model artifacts | `specs/models.md`, `specs/decisions/0002-python-library-estimators-and-artifacts.md` |
| Jobs, scheduling, checkpoints, pause/resume | `specs/training-runtime.md`, `specs/models.md` |
| Evaluation, backtests, registry, inference | `specs/evaluation-and-inference.md` |
| Coordinator, DTOs, health, event streaming | `specs/api.md` plus the owning domain spec |
| Raylib shell, state, docking, styling | `specs/frontend/foundation.md` |
| Workspace or panel behavior | `specs/frontend/workspaces.md`, `specs/frontend/foundation.md` |
| Charts, tables, linked views | `specs/frontend/visualization.md`, `specs/frontend/foundation.md` |
| Tests, typing, performance | `specs/quality.md` plus the owning subsystem spec |
| Delivery status or sequencing | `specs/roadmap.md` |
| Architectural decision changes | Relevant living specs and `specs/decisions/README.md` |
