# Specification Index

**Status:** Authoritative  
**Owner:** Project  
**Last updated:** 2026-07-04

This directory defines the required behavior of the Python/Cython trading
research workstation. Read only the documents relevant to the task. `AGENTS.md`
contains the default routing table.

## Document map

| Document | Owns |
|---|---|
| [design-pattern.md](design-pattern.md) | Modular monolith boundaries, route design pattern, dependency direction, SOLID and pattern usage |
| [product.md](product.md) | Goals, scope, users, terminology, and assumptions |
| [technology-stack.md](technology-stack.md) | Approved Python/Cython dependencies, tooling, dependency policy, and admission gates |
| [python-migration.md](python-migration.md) | Python/Cython implementation order, package mapping, compatibility rules, and screenshot baseline policy |
| [migration-baseline.md](migration-baseline.md) | Authoritative legacy ownership map and functional, replay, and visual parity evidence |
| [system-architecture.md](system-architecture.md) | Processes, storage, dependencies, runtime modes, and system data flow |
| [data-and-features.md](data-and-features.md) | Imports, catalog, features, targets, and materialization |
| [models.md](models.md) | Model specification index; estimator and artifact rules are split |
| [training-runtime.md](training-runtime.md) | Jobs, scheduling, concurrency, checkpoints, pause, and recovery |
| [evaluation-and-inference.md](evaluation-and-inference.md) | Evaluation and inference specification index |
| [api.md](api.md) | Coordinator contracts, Python client, DTOs, Arrow transfer, and event streaming |
| [frontend/foundation.md](frontend/foundation.md) | Frontend foundation index; shell, effects, and persistence are split |
| [frontend/workspaces.md](frontend/workspaces.md) | Workspace index and shared rules; panels are split by workspace |
| [frontend/visualization.md](frontend/visualization.md) | Charts, linked views, tables, and rendering performance |
| [quality.md](quality.md) | Quality index; common, concurrency, and visualization gates are split |
| [roadmap.md](roadmap.md) | Delivery order and current implementation status |
| [decisions/README.md](decisions/README.md) | Architectural decision records |
| [routing/](routing/) | Non-authoritative phase-specific reading maps |

## Reading policy

1. Start from the route in `AGENTS.md`.
2. Read `design-pattern.md` for every Python/Cython migration route.
3. Read this index when a task crosses boundaries or its owner is unclear.
4. Read `technology-stack.md` for dependency, packaging, or tooling changes.
5. Read `api.md` only when a public or process boundary changes.
6. Read `quality.md` with the subsystem being implemented or reviewed.
7. Inspect `screenshots/` only for visual-design or migration golden-baseline work.
8. Do not bulk-load every specification.

## Authority and maintenance

- The user's current request takes precedence over these documents.
- Living specifications define current required behavior.
- ADRs explain important decisions but do not replace living specifications.
- Define each normative rule in one owning document and link to it elsewhere.
- Update the owning specification in the same change as a behavior or contract change.
- Report code/spec conflicts; do not silently choose one.
- Each document should remain focused enough to load independently.
