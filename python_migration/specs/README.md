# Specification Index

**Status:** Authoritative  
**Owner:** Project  
**Last updated:** 2026-07-16

This directory defines the required behavior of the Python/Cython trading
research workstation. Read only the documents relevant to the task. `AGENTS.md`
contains the default routing table.

## Document map

| Document | Owns |
|---|---|
| [design-pattern.md](design-pattern.md) | Modular monolith boundaries, route design pattern, dependency direction, SOLID and pattern usage |
| [contract.md](contract.md) | Mandatory agent-facing protocol, stub, and minimum-context rules, including subsystem context maps |
| [examples/agent-facing-contracts.md](examples/agent-facing-contracts.md) | Optional non-authoritative protocol, stub, and package-layout examples |
| [product.md](product.md) | Goals, scope, users, terminology, and assumptions |
| [technology-stack.md](technology-stack.md) | Approved Python/Cython dependencies, tooling, dependency policy, and admission gates |
| [python-migration.md](python-migration.md) | Python/Cython implementation order, package mapping, compatibility rules, and screenshot baseline policy |
| [migration-baseline.md](migration-baseline.md) | Authoritative legacy ownership map and functional, replay, and visual parity evidence |
| [system-architecture.md](system-architecture.md) | Processes, storage, dependencies, runtime modes, and system data flow |
| [concurrency-and-parallelism.md](concurrency-and-parallelism.md) | Workstation-wide execution selection, ownership, CPU leases, cancellation, shutdown, and deterministic parallelism |
| [data-and-features.md](data-and-features.md) | Imports, catalog, features, targets, and materialization |
| [models.md](models.md) | Model specification index; estimator and artifact rules are split |
| [training-runtime.md](training-runtime.md) | Jobs, scheduling, job-specific resource policy, checkpoints, pause, and recovery |
| [evaluation-and-inference.md](evaluation-and-inference.md) | Evaluation and inference specification index |
| [api.md](api.md) | Coordinator contracts, Python client, DTOs, Arrow transfer, and event streaming |
| [ui/foundation.md](ui/foundation.md) | UI foundation index; shell, effects, and persistence are split |
| [ui/raylib-visual-system.md](ui/raylib-visual-system.md) | Raylib tokens, typography, geometry, states, responsive presentation, and visual governance |
| [ui/workspaces.md](ui/workspaces.md) | Workspace index and shared rules; panels are split by workspace |
| [ui/visualization.md](ui/visualization.md) | Charts, linked views, tables, and rendering performance |
| [quality.md](quality.md) | Quality index; common, concurrency, and visualization gates are split |
| [roadmap.md](roadmap.md) | Delivery order and current implementation status |
| [decisions/README.md](decisions/README.md) | Architectural decision records |
| [missing/](missing/) | Behavior and public-contract changes not yet incorporated into authoritative specifications |
| [routing/](routing/) | Non-authoritative phase-specific reading maps |

## Reading policy

1. Start from the route in `AGENTS.md`.
2. Read `contract.md` before every task; it owns Python authoring and
   agent-context rules.
3. Read `design-pattern.md` for every Python/Cython migration route.
4. Read this index when a task crosses boundaries or its owner is unclear.
5. Read `technology-stack.md` for dependency, packaging, or tooling changes.
6. Read `api.md` only when a public or process boundary changes.
7. Read `quality.md` with the subsystem being implemented or reviewed.
8. Inspect `screenshots/` only for visual-design or migration golden-baseline work.
9. Do not bulk-load every specification.

## Authority and maintenance

- The user's current request takes precedence over these documents.
- Living specifications define current required behavior.
- ADRs explain important decisions but do not replace living specifications.
- Define each normative rule in one owning document and link to it elsewhere.
- Do not rewrite an owning specification to match a behavior or public-contract
  change automatically. Record what changed and why under `missing/` until the
  user explicitly requests an authoritative specification update.
- Report code/spec conflicts; do not silently choose one.
- Each document should remain focused enough to load independently.
