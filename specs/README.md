# Specification Index

**Status:** Authoritative  
**Owner:** Project  
**Last updated:** 2026-07-04

This directory defines the required behavior of the Go trading research workstation. Read only the documents relevant to the task. `AGENTS.md` contains the default routing table.

## Document map

| Document | Owns |
|---|---|
| [product.md](product.md) | Goals, scope, users, terminology, and assumptions |
| [technology-stack.md](technology-stack.md) | Approved Go modules, tooling, dependency policy, and admission gates |
| [system-architecture.md](system-architecture.md) | Processes, storage, dependencies, runtime modes, and system data flow |
| [data-and-features.md](data-and-features.md) | Imports, catalog, features, targets, and materialization |
| [models.md](models.md) | Go estimator behavior, tree algorithms, and model artifacts |
| [training-runtime.md](training-runtime.md) | Jobs, scheduling, concurrency, checkpoints, pause, and recovery |
| [evaluation-and-inference.md](evaluation-and-inference.md) | Walk-forward evaluation, backtesting, registry, refitting, and scoring |
| [api.md](api.md) | Coordinator contracts, Go client, DTOs, Arrow transfer, and event streaming |
| [frontend/foundation.md](frontend/foundation.md) | Raylib shell, state, docking, styling, input, and asynchronous client |
| [frontend/workspaces.md](frontend/workspaces.md) | Workspace panels and user workflows |
| [frontend/visualization.md](frontend/visualization.md) | Charts, linked views, tables, and rendering performance |
| [quality.md](quality.md) | Tests, static analysis, race safety, determinism, performance, and compatibility gates |
| [roadmap.md](roadmap.md) | Delivery order and current implementation status |
| [decisions/README.md](decisions/README.md) | Architectural decision records |

## Reading policy

1. Start from the route in `AGENTS.md`.
2. Read this index when a task crosses boundaries or its owner is unclear.
3. Read `technology-stack.md` for dependency, packaging, or tooling changes.
4. Read `api.md` only when a public or process boundary changes.
5. Read `quality.md` with the subsystem being implemented or reviewed.
6. Inspect `screenshots/` only for visual-design work.
7. Do not bulk-load every specification.

## Authority and maintenance

- The user's current request takes precedence over these documents.
- Living specifications define current required behavior.
- ADRs explain important decisions but do not replace living specifications.
- Define each normative rule in one owning document and link to it elsewhere.
- Update the owning specification in the same change as a behavior or contract change.
- Report code/spec conflicts; do not silently choose one.
- Each document should remain focused enough to load independently.
