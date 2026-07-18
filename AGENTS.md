# Repository Guidance

Specification-first, Windows-focused trading workstation. CPython, `uv`,
`pyproject.toml`, and `specs/technology-stack.md` define the runtime, tooling,
tests, packaging, and approved direct dependencies. The Python/Cython
implementation lives at the repository root. Record behavior and
public-contract changes under `specs/missing/` before rewriting runtime code.
Use only commands declared by the existing project files.

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
- Required gates are `ruff format --check`,
  `ruff check`, `pyright`, `pytest`, property and benchmark tests where
  applicable, vulnerability scanning, and Windows Cython extension builds.

## Before planning or editing

For every task, first read `specs/contract.md`; it defines the mandatory Python
authoring and agent-context rules. Then classify the task, read
`specs/design-pattern.md`, and read only the route-specific documents below.
Read `specs/README.md` when ownership crosses documents. Read `specs/quality.md`
for implementation, review, test, or performance work; `technology-stack.md`
for dependencies or tooling; `api.md` for public/process boundaries. When
behavior or a public contract changes, do not update the owning specification;
add or update a concise Markdown note under `specs/missing/` that states what
changed and why, using the format in `specs/missing/README.md`.

## Specification routing

| Task | Required specifications |
|---|---|
| All Python/Cython implementation routes | `specs/contract.md`, `specs/design-pattern.md`, plus the owning subsystem spec |
| Architecture, module boundaries, route handlers, adapters, dependency direction, or design patterns | `specs/design-pattern.md`, plus the owning subsystem spec |
| Product scope or requirements | `specs/product.md`, `specs/roadmap.md` |
| Dependencies, packaging, or tooling | `specs/technology-stack.md`, `specs/quality.md` |
| Python/Cython implementation or migration history | `specs/python-migration.md`, `specs/migration-baseline.md`, `specs/technology-stack.md`, `specs/roadmap.md` |
| Phase 12 Python scaffold and shell | `specs/routing/phase-12.md` |
| Phase 4 docking, reusable controls, preferences, responsive scaling, and layout persistence | `specs/routing/phase-4.md` |
| Phase 5 charts, tables, LOD, visualization caches, virtualization, and performance | `specs/routing/phase-5.md` |
| Phase 5b visualization rendering, interactions, request/pagination parity, and canonical goldens | `specs/routing/phase-5b.md` |
| Processes, storage, runtime, concurrency | `specs/concurrency-and-parallelism.md`, `specs/system-architecture.md`; read `specs/decisions/0008-regular-cpython-concurrency.md` for decision rationale |
| Imports, datasets, features, targets | `specs/data-and-features.md` |
| Trees, forests, boosting, model artifacts | `specs/models.md`, `specs/decisions/0002-python-library-estimators-and-artifacts.md` |
| Jobs, scheduling, checkpoints, pause/resume | `specs/training-runtime.md`, `specs/models.md` |
| Evaluation, backtests, registry, inference | `specs/evaluation-and-inference.md` |
| Coordinator, DTOs, health, event streaming | `specs/api.md` plus the owning domain spec |
| Raylib shell, state, docking, styling | `specs/ui/foundation.md` |
| Raylib visual design, styling, geometry, typography, interaction states, or responsive presentation | `specs/ui/raylib-visual-system.md`, `specs/ui/foundation.md`, plus the owning shell, workspace, or visualization spec |
| Workspace or panel behavior | `specs/ui/workspaces.md`, `specs/ui/foundation.md` |
| Charts, tables, linked views | `specs/ui/visualization.md`, `specs/ui/foundation.md` |
| Implementation, review, tests, typing, or performance | `specs/quality.md`, `specs/concurrency-and-parallelism.md` when concurrent or parallel work is involved, plus the owning subsystem spec |
| Delivery status or sequencing | `specs/roadmap.md` |
| Architectural decision changes | Relevant living specs and `specs/decisions/README.md` |
