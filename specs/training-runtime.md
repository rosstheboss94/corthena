# Training Runtime Specification

**Status:** Authoritative  
**Owner:** Runtime  
**Last updated:** 2026-07-12
**Related:** [Concurrency and parallelism](concurrency-and-parallelism.md), [Technology stack](technology-stack.md), [System architecture](system-architecture.md), [Models](models.md), [ADR 0008](decisions/0008-regular-cpython-concurrency.md)

## Technology Constraints

Use coordinator-launched Python worker processes for job isolation and retain
one process per job. Python process launch APIs, inherited anonymous pipes or
approved local IPC, cancellation tokens, queues, and CPU leases implement the
job runtime. [Concurrency and parallelism](concurrency-and-parallelism.md) owns
their general selection, bounds, nested-parallelism, ownership, cancellation,
and shutdown contracts.

## Job Lifecycle

Job states are:

- `queued`
- `running`
- `pause_requested`
- `paused`
- `completed`
- `failed`
- `cancelled`
- `interrupted`

The coordinator owns all state transitions and persists each accepted
transition before broadcasting it. Workers emit heartbeats, structured
progress, metrics, logs, component status, and checkpoint state. On startup,
jobs persisted as active without a valid live worker become `interrupted` and
require explicit resume.

Closing the UI requests cooperative pause for active jobs configured for
pause-on-close. Workers stop accepting new work, finish the active
checkpointable unit when the estimator supports it, commit it, and acknowledge
pause. The UI waits through a visible deadline and may offer force exit, which
can discard only active uncommitted work.

## CPU Scheduling

- Default to one active training worker process.
- Global compute slots are `max(1, logicalCPUCount - 2)`.
- The coordinator leases slots before starting a worker or changing its pool size.
- Total live leases cannot exceed the global budget.
- With one active job, it receives all available compute slots by default.
- Multiple jobs divide slots according to explicit per-job requests; queued jobs wait when insufficient slots exist.
- Each worker configures Python process and thread pools, BLAS/OpenMP, scikit-learn,
  PyTorch, and adapter pools so their combined live compute does not exceed
  leased slots.
- Lease reductions take effect at deterministic task boundaries after excess workers finish current tasks.
- Coordinator and UI control threads do not consume training leases but must remain lightweight and responsive.

Scheduler decisions use stable queue order and persisted requested resources.
They never depend on dict iteration or worker response race order.

## Worker Ownership and Cancellation

[Concurrency and parallelism](concurrency-and-parallelism.md) supplies the
general declaration, backpressure, cancellation, failure-containment, and
cleanup contract. Training adds these job-specific requirements:

- One orchestration owner owns estimator state, checkpoint state, and task sequencing.
- One writer owner serializes worker-protocol events; one reader owner validates commands.
- Compute workers receive indexed tasks over immutable inputs and return task-owned values.
- Pause and cancel are cooperative and checked at documented fold, stage, epoch, batch, and adapter boundaries.
- Cancel removes no completed immutable artifacts and leaves auditable job metadata.
- Unexpected pipe closure, process exit, heartbeat expiry, or protocol violation triggers worker cleanup and state reconciliation.
- Worker cleanup reconciles job state and preserves completed immutable
  artifacts; verification detects leaked processes, threads, mappings, and
  leases.

## Checkpointing

Checkpoint at documented estimator boundaries. For scikit-learn, use
`partial_fit` only for estimators that support it; otherwise checkpoint
Corthena orchestration state and completed immutable outputs. For PyTorch,
checkpoint model, optimizer, scheduler, epoch, seed, and training-state objects.
A journal record contains committed work state, remaining work identifiers,
deterministic seed coordinates, previous-record checksum, record checksum, and
schema version.

- The orchestration owner is the sole checkpoint producer.
- A dedicated writer may perform I/O only from immutable records and must acknowledge durable flush before the state is reported committed.
- Journal records use length-prefixed canonical JSON with a maximum record size and checksum chain.
- Periodically compact the journal into an atomic JSON manifest plus approved artifact snapshot.
- Resume loads the newest valid snapshot, replays the longest valid journal prefix, and reconstructs state deterministically.
- Recompute only the active incomplete checkpointable unit.
- Keep the latest valid previous snapshot until the new snapshot validates.
- Reject mismatched data, feature, target, model, engine, dependency, artifact, or checkpoint schemas.
- Preserve corrupt or incompatible files for diagnosis; never silently truncate the original artifact.

## Experiments and Sweeps

Support single jobs plus deterministic grid and seeded random sweeps with trial
caps. Pooled experiments train across symbols; per-symbol experiments create
independently tracked child runs. Trial generation uses versioned seed
derivation and persists the complete ordered trial list before execution.

## Public Domain Types

`ModelSpec`, `ExperimentSpec`, and `SweepSpec` are validated Python value
objects or DTO-backed domain types. Commands, worker events, compute results,
and checkpoint records are concrete typed values. Compute workers never mutate
shared command or result objects, and API DTO conversion remains outside
runtime domain packages.
