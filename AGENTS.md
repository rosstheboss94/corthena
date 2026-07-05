# Repository Guidance

## Project

This repository is a specification-first, Windows-focused trading research workstation implemented entirely in Go 1.25.7. It uses first-party Go implementations of tree-based ML, a local Go coordinator, isolated training processes with bounded goroutine pools, and a Raylib desktop UI through `github.com/gen2brain/raylib-go`.

The repository is currently specification-only. Do not invent setup, test, lint, or launch commands before the corresponding project files exist.

## Always-required invariants

- Use Go exclusively for the runtime, build, tooling, extensions, and tests, with the toolchain and version defined by `go.mod`.
- Do not use an external runtime ML implementation.
- Keep every Raylib and Raygui call on the locked UI OS thread.
- Never perform filesystem, database, network, or training work on the render thread.
- Shared slices and mapped arrays used concurrently are read-only; tasks own mutable outputs.
- Keep training deterministic across supported goroutine counts, worker counts, and completion orders.
- Prevent future-data leakage in features, targets, splits, and execution timing.
- Keep completed runs and model artifacts immutable.
- Domain behavior must not live in UI render functions.
- `gofmt`, `go vet`, Staticcheck, tests, and the race detector are required gates once scaffolding exists.
- Approved direct dependencies and their responsibilities are defined only in `specs/technology-stack.md`; exact versions belong only in `go.mod` and `go.sum`.
- Avoid `any`, `map[string]any`, reflection-driven domain models, unchecked type assertions, and untyped boundary payloads.
- Isolate native and weakly typed libraries behind typed adapters and interfaces.

## Before planning or editing

1. Classify the task using the routing table.
2. Read only the listed required specifications.
3. Read `specs/README.md` when the task crosses boundaries or ownership is unclear.
4. Do not load all specifications by default.
5. For dependency, packaging, or tooling changes, read `specs/technology-stack.md`.
6. For public or process-boundary changes, also read `specs/api.md`.
7. For implementation or review, also read `specs/quality.md`.
8. Update the canonical specification when required behavior or a public contract changes.

## Specification routing

| Task | Required specifications |
|---|---|
| Product scope or requirements | `specs/product.md`, `specs/roadmap.md` |
| Dependencies, packaging, or tooling | `specs/technology-stack.md`, `specs/quality.md` |
| Processes, storage, runtime, concurrency | `specs/system-architecture.md`, `specs/decisions/0005-go-hybrid-concurrency.md` |
| Imports, datasets, features, targets | `specs/data-and-features.md` |
| Trees, forests, boosting, model artifacts | `specs/models.md`, `specs/decisions/0002-go-tree-engine-and-artifacts.md` |
| Jobs, scheduling, checkpoints, pause/resume | `specs/training-runtime.md`, `specs/models.md` |
| Evaluation, backtests, registry, inference | `specs/evaluation-and-inference.md` |
| Coordinator, DTOs, health, event streaming | `specs/api.md` plus the owning domain spec |
| Raylib shell, state, docking, styling | `specs/frontend/foundation.md` |
| Workspace or panel behavior | `specs/frontend/workspaces.md`, `specs/frontend/foundation.md` |
| Charts, tables, linked views | `specs/frontend/visualization.md`, `specs/frontend/foundation.md` |
| Tests, typing, performance | `specs/quality.md` plus the owning subsystem spec |
| Delivery status or sequencing | `specs/roadmap.md` |
| Architectural decision changes | Relevant living specs and `specs/decisions/README.md` |

Inspect `screenshots/` only for visual-design tasks.

## Type- and concurrency-safety rules

- Use concrete structs, typed aliases, interfaces, generics only where they improve safety, and exhaustive switches over validated enum values.
- Annotate serialized fields explicitly and validate all DTOs before conversion to domain types.
- Return errors with context; use stable typed or sentinel errors where callers branch on failure.
- Pass `context.Context` through blocking and cancellable boundaries; never store it in domain state.
- Define channel ownership, closure, buffering, and cancellation behavior at every concurrent boundary.
- Do not copy structs containing mutexes, atomics, wait groups, or no-copy guards.
- Keep unavoidable `unsafe` and native conversions inside adapter packages with focused tests and comments explaining the invariant.
- Run race-enabled tests after changing concurrency-sensitive Go code; zero races are required.

## Completion expectations

- Preserve unrelated user changes.
- Run the owning subsystem tests and relevant cross-cutting tests once their project files exist.
- Verify deterministic behavior when changing concurrent or numerical code.
- Update living specifications with behavior changes.
- Add an ADR only for a decision with meaningful alternatives and lasting consequences.
- Report code/spec conflicts instead of silently choosing one.
