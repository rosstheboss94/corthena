# System Architecture

**Status:** Authoritative
**Owner:** Platform
**Last updated:** 2026-07-18
**Related:** [Concurrency and parallelism](concurrency-and-parallelism.md), [Technology stack](technology-stack.md), [Training runtime](../pages/jobs/runtime.md), [API](api.md), [ADR 0001](../decisions/0001-local-process-architecture.md), [ADR 0008](../decisions/0008-regular-cpython-concurrency.md)

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
  scheduler, CPU leases, repositories, credential access, provider adapters,
  bounded import/network workers, and durable metadata ownership

Worker process x active training job
  orchestration owner: state, ordering, checkpoint commits
  bounded compute workers/library pools: deterministic task execution
```

The coordinator owns durable metadata writes and CPU-slot allocation. Worker
processes isolate crashes, cancellation, library state, and job-local memory.
[Concurrency and parallelism](concurrency-and-parallelism.md) owns execution
selection, resource ownership, lease accounting, UI-thread confinement, and
lifecycle rules for this topology.

The UI reaches file preview/import, Massive, credentials, schedules, and the
catalog only through the loopback API and coordinator-backed client adapter.
The coordinator owns provider networking, PyArrow parsing, validation,
deduplication, schedule execution, SQLite transactions, and atomic Parquet
revision promotion. Import workers are bounded coordinator resources, not
training worker processes, and their CPU-consuming capacity obeys coordinator
leases.

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

[Concurrency and parallelism](concurrency-and-parallelism.md) is authoritative
for ownership, synchronization, backpressure, CPU leases, deterministic
ordering, immutable and shared-memory publication, cancellation, and shutdown.
Within this topology, the coordinator owns leases and durable state, each
worker owns its job-local orchestration state, and the UI owns its native OS
thread.

## Storage

- SQLite WAL: datasets, catalog revisions, imports, schedules, experiment
  definitions, jobs, runs, metrics, aliases, and artifact indexes.
- Partitioned Parquet revision directories: canonical bars; Parquet also stores
  row metadata, predictions, and tabular reports.
- Memory-mapped typed files: run-specific feature matrices and targets with versioned manifests.
- JSON manifests plus checksummed library artifacts and Arrow/NumPy files: versioned immutable models and checkpoints.
- User application-data directory: runtime database, data revisions, artifacts,
  caches, logs, UI layouts, and a separate versioned Massive credential
  document.

The Massive credential document is intentionally plaintext and carries an
explicit unencrypted-storage warning in the UI. A platform adapter creates and
replaces it with a protected DACL: inheritance disabled, full control for the
current Windows user and `SYSTEM`, and no access-control entries for other
principals. Creation, replacement, or permission verification fails closed if
that protection cannot be established. The adapter validates document version
and shape before use and quarantines corruption without logging or returning
file contents. It is separate from preferences, layouts, SQLite, and provenance.
The token is never present in snapshots, events, logs, URLs, screenshots,
replay serialization, errors, manifests, or API responses.

The coordinator is the authoritative database writer. Workers write only
job-scoped temporary artifacts and return typed events and validated artifact
references. The coordinator validates and atomically promotes completed
artifacts before indexing them.

For a catalog update, bounded workers write a sibling temporary revision,
flush and close every Parquet partition, validate checksums and the immutable
provenance manifest, then atomically promote the directory. One SQLite
transaction indexes the promoted revision and makes it current. If either
promotion or transaction fails, reconciliation removes or quarantines the
unpublished artifact and retains the prior current revision.

## Reliability Rules

- Files and manifests are written to sibling temporary paths, flushed, checksummed, closed, and atomically replaced.
- Completed runs and models are immutable.
- Mutable catalog updates never mutate active-run materializations.
- Shared inputs are read-only; concurrent tasks own output buffers.
- Startup reconciles stale jobs, temporary artifacts, database state, and worker liveness.
- Startup also reconciles incomplete imports and promoted-but-unindexed catalog
  revisions, quarantines corrupt credential documents without content
  disclosure, and coalesces each schedule's missed executions into at most one
  bounded catch-up range.
- Schedules execute only while the coordinator is open. Shutdown stops new
  schedule/import submissions, propagates cancellation, drains only declared
  bounded work, closes provider and parsing adapters, and preserves the last
  published catalog revision.
- Shutdown applies the canonical dependency ordering; job-specific cooperative
  pause and force-termination consequences are defined by
  [training runtime](../pages/jobs/runtime.md).
- Protocol, schema, engine, dependency, and artifact version mismatches fail closed with stable errors.
