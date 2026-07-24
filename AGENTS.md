# Repository Guidance

Specification-first, Windows-focused trading workstation. CPython, `uv`,
`pyproject.toml`, and `specs/general/technology-stack.md` define the runtime, tooling,
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

For every task, first read `specs/general/contract.md`; it defines the mandatory
Python authoring and agent-context rules. Then classify the task, read
`specs/general/design-pattern.md`, and read only the route-specific documents
below. Read `specs/README.md` when ownership crosses documents. Read
`specs/general/quality/README.md` for implementation, review, test, or
performance work; `specs/general/technology-stack.md` for dependencies or
tooling; `specs/general/api.md` for public/process boundaries. When behavior or
a public contract changes, do not update the owning specification; add or
update a concise Markdown note under `specs/missing/` that states what changed
and why, using the format in `specs/missing/README.md`.

## Specification routing

| Task | Required specifications |
|---|---|
| All Python/Cython implementation routes | `specs/general/contract.md`, `specs/general/design-pattern.md`, plus one applicable page index |
| Architecture, module boundaries, route handlers, adapters, dependency direction, or design patterns | `specs/general/design-pattern.md` plus the owning page index |
| Product scope or requirements | `specs/general/product.md`, `specs/general/roadmap.md`, plus the owning page index |
| Dependencies, packaging, or tooling | `specs/general/technology-stack.md`, `specs/general/quality/README.md` |
| Python/Cython implementation or migration history | `specs/general/contract.md`, `specs/general/technology-stack.md`, relevant page index, and `specs/history/migration/` evidence when needed |
| Processes, storage, runtime, concurrency | `specs/general/concurrency-and-parallelism.md`, `specs/general/system-architecture.md`; read `specs/decisions/0008-regular-cpython-concurrency.md` for decision rationale |
| Imports, datasets, features, targets | `specs/pages/data/README.md`, `specs/pages/data/ingestion.md`, `specs/pages/data/datasets.md`, plus `specs/pages/research/README.md` for Research consumers |
| Trees, forests, boosting, model artifacts | `specs/pages/models/README.md`, `specs/pages/models/estimators.md`, `specs/pages/models/artifacts-and-registry.md`, and the relevant ADR |
| Jobs, scheduling, checkpoints, pause/resume | `specs/pages/jobs/README.md`, `specs/pages/jobs/runtime.md` |
| Evaluation, backtests, registry, inference | The applicable `specs/pages/results/README.md`, `specs/pages/models/README.md`, or `specs/pages/inference/README.md` |
| Coordinator, DTOs, health, event streaming | `specs/general/api.md` plus the owning page API document |
| Raylib shell, state, docking, styling | `specs/general/ui/README.md`, `shell-and-state.md`, and the owning page index |
| Raylib visual design, styling, geometry, typography, interaction states, or responsive presentation | `specs/general/ui/visual-system.md`, `specs/general/ui/README.md`, plus the owning page or visualization spec |
| Workspace or panel behavior | `specs/general/ui/workspaces.md`, `specs/general/ui/README.md`, plus the owning page index |
| Charts, tables, linked views | `specs/general/ui/visualization.md`, `specs/general/ui/workspaces.md`, and `specs/general/quality/visualization.md` |
| Implementation, review, tests, typing, or performance | `specs/general/quality/README.md`, `specs/general/concurrency-and-parallelism.md` when concurrent or parallel work is involved, plus the owning page spec |
| Delivery status or sequencing | `specs/general/roadmap.md` and the owning page roadmap when present |
| Architectural decision changes | Relevant living specs and `specs/decisions/README.md` |
