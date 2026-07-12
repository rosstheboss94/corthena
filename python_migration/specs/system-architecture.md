# System Architecture

**Status:** Authoritative  
**Owner:** Platform  
**Last updated:** 2026-07-12
**Related:** [Technology stack](technology-stack.md), [Training runtime](training-runtime.md), [API](api.md), [ADR 0001](decisions/0001-local-process-architecture.md), [ADR 0008](decisions/0008-regular-cpython-concurrency.md)

## Technology Constraints

[Technology stack](technology-stack.md) owns approved direct dependencies and
version policy. This subsystem uses Python processes, owned background workers,
PyArrow, SQLite repositories, FastAPI/httpx, and Python Raylib/Raygui bindings
according to their defined roles. Exact versions come only from
`pyproject.toml` and `uv.lock` after compatibility validation.

## Process Topology

```text
Workstation UI process
  locked OS thread: input, state reduction, Raylib/Raygui calls, rendering
  background workers: HTTP, Arrow, events, persistence effects

Coordinator process
  loopback HTTP and WebSocket service
  scheduler, CPU leases, repositories, and durable metadata ownership

Worker process x active training job
  orchestration owner: state, ordering, checkpoint commits
  bounded compute workers/library pools: deterministic task execution
```

The coordinator owns durable metadata writes and CPU-slot allocation. Worker
processes isolate crashes, cancellation, library state, and job-local memory.
Bounded processes are the default pure-Python CPU parallelism mechanism;
threads serve I/O and orchestration, while process and library pools remain explicitly owned and lease-bound. Raylib and
Raygui calls remain on the UI process's locked OS
thread.

The UI, coordinator, worker, and CLI are distinct entry points that share typed
contract definitions. The coordinator starts workers with explicit job IDs,
artifact paths, lease sizes, protocol versions, and one-time local capability
tokens. Workers do not expose a network listener.

## Runtime and Component Status

Every process records:

- application, schema, engine, and worker-protocol versions;
- Python version and implementation, build revision, dirty-build flag, OS, and architecture;
- Python ABI/platform tag and exact regular-build validation;
- process role, PID, start time, and supported capabilities;
- process count, thread count, library pool limits, leased compute slots, and active task count;
- approved native-library and dependency versions when relevant;
- native extension availability for the UI process and Raylib/Raygui initialization status.

Process status is `starting`, `healthy`, `degraded`, `stopping`, or `failed`.
`degraded` is capability-specific and must name the unavailable or reduced
capability. Regular CPython 3.14.2 is healthy; a free-threaded or wrong-patch
interpreter fails startup. Degradation
never relaxes correctness, explicit synchronization, or determinism.

## Concurrency and Ownership

- The UI locks its initial OS thread before initializing Raylib and never dispatches Raylib/Raygui calls elsewhere.
- Code never uses GIL-provided atomicity as an application synchronization
  contract; mutable state has one owner or an explicit lock/queue.
- Each worker has one orchestration owner that owns mutable estimator and checkpoint state.
- Compute tasks receive immutable array, tensor, or memory-map views and return task-owned immutable results.
- Queues and streams have documented senders, receivers, and closers; cancellation uses explicit tokens/events and does not rely on abandoned sends.
- Reductions apply results in stable logical-index order, never arrival order.
- The coordinator leases explicit CPU slots before starting workers or enlarging pools.
- Worker and library pool size cannot exceed its lease; nested parallel sections reuse the same budget or execute serially.
- Shared memory mappings are immutable after publication. Writers use exclusive ranges before publication.

## Storage

- SQLite WAL: datasets, imports, experiment definitions, jobs, runs, metrics, aliases, and artifact indexes.
- Parquet: canonical bars, row metadata, predictions, and tabular reports.
- Memory-mapped typed files: run-specific feature matrices and targets with versioned manifests.
- JSON manifests plus checksummed library artifacts and Arrow/NumPy files: versioned immutable models and checkpoints.
- User application-data directory: runtime database, data, artifacts, caches, logs, and UI layouts.

The coordinator is the authoritative database writer. Workers write only
job-scoped temporary artifacts and return typed events and validated artifact
references. The coordinator validates and atomically promotes completed
artifacts before indexing them.

## Reliability Rules

- Files and manifests are written to sibling temporary paths, flushed, checksummed, closed, and atomically replaced.
- Completed runs and models are immutable.
- Mutable catalog updates never mutate active-run materializations.
- Shared inputs are read-only; concurrent tasks own output buffers.
- No filesystem, database, network, decoding, training, or blocking library operation runs on the render thread.
- Startup reconciles stale jobs, temporary artifacts, database state, and worker liveness.
- Shutdown requests cooperative pause, waits through an explicit deadline, and reports the consequences of force termination.
- Protocol, schema, engine, dependency, and artifact version mismatches fail closed with stable errors.
