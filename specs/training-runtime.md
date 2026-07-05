# Training Runtime Specification

**Status:** Authoritative  
**Owner:** Runtime  
**Last updated:** 2026-07-04  
**Related:** [Technology stack](technology-stack.md), [System architecture](system-architecture.md), [Models](models.md), [ADR 0005](decisions/0005-go-hybrid-concurrency.md)

## Technology constraints

Use coordinator-launched Go worker processes for job isolation and bounded goroutine pools inside each worker. Use `os/exec`, inherited anonymous pipes, `context`, channels, `sync`, `sync/atomic`, and explicit CPU leases. Do not add an external scheduler, broker, distributed runtime, or nested numerical thread pool.

## Job lifecycle

Job states are:

- `queued`
- `running`
- `pause_requested`
- `paused`
- `completed`
- `failed`
- `cancelled`
- `interrupted`

The coordinator owns all state transitions and persists each accepted transition before broadcasting it. Workers emit heartbeats, structured progress, metrics, logs, component status, and checkpoint state. On startup, jobs persisted as active without a valid live worker become `interrupted` and require explicit resume.

Closing the UI requests cooperative pause for active jobs configured for pause-on-close. Workers stop accepting new node work, finish the active tree node, commit it, and acknowledge pause. The UI waits through a visible deadline and may offer force exit, which can discard only active uncommitted node work.

## CPU scheduling

- Default to one active training worker process.
- Global compute slots are `max(1, logicalCPUCount - 2)`.
- The coordinator leases slots before starting a worker or changing its pool size.
- Total live leases cannot exceed the global budget.
- With one active job, it receives all available compute slots by default.
- Multiple jobs divide slots according to explicit per-job requests; queued jobs wait when insufficient slots exist.
- Each worker sets `GOMAXPROCS` to its current lease and owns one bounded pool with no more compute goroutines than leased slots.
- Lease reductions take effect at deterministic task boundaries after excess workers finish current tasks.
- Coordinator and UI goroutines do not consume training leases but must remain lightweight and responsive.

Scheduler decisions use stable queue order and persisted requested resources. They never depend on map iteration or worker response race order.

## Worker ownership and cancellation

- One orchestration goroutine owns estimator state, checkpoint state, and task sequencing.
- One writer goroutine serializes worker-protocol events; one reader goroutine validates commands.
- Compute goroutines receive indexed tasks over immutable inputs and return task-owned values.
- Pause and cancel are cooperative and checked at documented node, tree, fold, and stage boundaries.
- Cancel removes no completed immutable artifacts and leaves auditable job metadata.
- Unexpected pipe closure, process exit, heartbeat expiry, or protocol violation triggers worker cleanup and state reconciliation.
- Every worker process and goroutine has a bounded shutdown path; tests detect leaked processes and goroutines.

## Checkpointing

Build trees iteratively and journal every completed node. A journal record contains committed split/leaf state, tree position, remaining work identifiers, deterministic seed coordinates, previous-record checksum, record checksum, and schema version.

- The orchestration goroutine is the sole checkpoint producer.
- A dedicated writer may perform I/O only from immutable records and must acknowledge durable flush before the state is reported committed.
- Journal records use length-prefixed canonical JSON with a maximum record size and checksum chain.
- Periodically compact the journal into an atomic JSON manifest plus Arrow IPC snapshot.
- Resume loads the newest valid snapshot, replays the longest valid journal prefix, and reconstructs row membership deterministically.
- Recompute only the active incomplete node.
- Keep the latest valid previous snapshot until the new snapshot validates.
- Reject mismatched data, feature, target, model, engine, artifact, or checkpoint schemas.
- Preserve corrupt or incompatible files for diagnosis; never silently truncate the original artifact.

## Experiments and sweeps

Support single jobs plus deterministic grid and seeded random sweeps with trial caps. Pooled experiments train across symbols; per-symbol experiments create independently tracked child runs. Trial generation uses the versioned deterministic random generator and persists the complete ordered trial list before execution.

## Public domain types

`ModelSpec`, `ExperimentSpec`, and `SweepSpec` are validated Go value structs. Commands, worker events, compute results, and checkpoint records are concrete typed values. Compute goroutines never mutate shared command or result objects, and API DTO conversion remains outside runtime domain packages.
