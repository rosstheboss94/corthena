# System Architecture

**Status:** Authoritative  
**Owner:** Platform  
**Last updated:** 2026-07-04  
**Related:** [Technology stack](technology-stack.md), [Training runtime](training-runtime.md), [API](api.md), [ADR 0001](decisions/0001-local-process-architecture.md), [ADR 0005](decisions/0005-go-hybrid-concurrency.md)

## Technology constraints

[Technology stack](technology-stack.md) owns approved direct dependencies and version policy. This subsystem uses Go processes and goroutines, Apache Arrow Go, `database/sql` with SQLite, `net/http`, and `raylib-go` according to their defined roles. Exact versions come only from module files after compatibility validation.

## Process topology

```text
Workstation UI process
├── locked OS thread: input, state reduction, Raylib/Raygui calls, rendering
└── background goroutines: HTTP, Arrow, events, persistence effects

Coordinator process
├── loopback HTTP and WebSocket service
└── scheduler, CPU leases, repositories, and durable metadata ownership

Worker process × active training job
├── orchestration goroutine: state, ordering, checkpoint commits
└── bounded compute goroutines: deterministic task execution
```

The coordinator owns durable metadata writes and CPU-slot allocation. Worker processes isolate crashes, cancellation, and job-local memory. Goroutines inside a worker provide shared-memory parallelism. Raylib and Raygui calls remain on the UI process's locked OS thread.

The UI, coordinator, worker, and CLI are distinct commands that share typed `contract` definitions. The coordinator starts workers with explicit job IDs, artifact paths, lease sizes, protocol versions, and one-time local capability tokens. Workers do not expose a network listener.

## Runtime and component status

Every process records:

- application, schema, engine, and worker-protocol versions;
- Go version, build revision, dirty-build flag, `GOOS`, and `GOARCH`;
- process role, PID, start time, and supported capabilities;
- `GOMAXPROCS`, goroutine count, leased compute slots, and active task count;
- approved native-library and module versions when relevant;
- cgo availability for the UI process and Raylib/Raygui initialization status.

Process status is `starting`, `healthy`, `degraded`, `stopping`, or `failed`. `degraded` is capability-specific and must name the unavailable or reduced capability. It is not a language-runtime mode and never relaxes correctness, race-safety, or determinism requirements.

## Concurrency and ownership

- The UI locks its initial goroutine to the OS thread before initializing Raylib and never dispatches Raylib/Raygui calls elsewhere.
- Each worker has one orchestration goroutine that owns mutable estimator and checkpoint state.
- Compute tasks receive immutable slice views and return task-owned immutable results.
- Channels have one documented closer; cancellation uses `context.Context` and does not rely on abandoned sends.
- Reductions apply results in stable logical-index order, never arrival order.
- The coordinator leases explicit CPU slots before starting workers or enlarging pools.
- Worker pool size cannot exceed its lease; nested parallel sections reuse the same budget or execute serially.
- Shared memory mappings are immutable after publication. Writers use exclusive ranges before publication.

## Storage

- SQLite WAL: datasets, imports, experiment definitions, jobs, runs, metrics, aliases, and artifact indexes.
- Parquet: canonical bars, row metadata, predictions, and tabular reports.
- Memory-mapped typed files: run-specific `float32` feature matrices and `float64` targets with versioned manifests.
- JSON plus Arrow IPC: versioned immutable models and checkpoints.
- User application-data directory: runtime database, data, artifacts, caches, logs, and UI layouts.

The coordinator is the authoritative database writer. Workers write only job-scoped temporary artifacts and return typed events and validated artifact references. The coordinator validates and atomically promotes completed artifacts before indexing them.

## Reliability rules

- Files and manifests are written to sibling temporary paths, flushed, checksummed, closed, and atomically replaced.
- Completed runs and models are immutable.
- Mutable catalog updates never mutate active-run materializations.
- Shared inputs are read-only; concurrent tasks own output buffers.
- No filesystem, database, network, decoding, or training operation blocks the render thread.
- Startup reconciles stale jobs, temporary artifacts, database state, and worker liveness.
- Shutdown requests cooperative pause, waits through an explicit deadline, and reports the consequences of force termination.
- Protocol, schema, engine, and artifact version mismatches fail closed with stable errors.
