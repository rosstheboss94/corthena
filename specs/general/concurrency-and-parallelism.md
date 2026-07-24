# Concurrency and Parallelism Specification

**Status:** Authoritative

**Owner:** Platform

**Last updated:** 2026-07-12

**Related:** [System architecture](system-architecture.md), [Technology stack](technology-stack.md), [Training runtime](../pages/jobs/runtime.md), [API](api.md), [UI foundation](ui/README.md), [Concurrency verification](quality/concurrency.md), [ADR 0008](../decisions/0008-regular-cpython-concurrency.md)

This document owns workstation-wide concurrency and parallelism behavior.
Subsystem specifications own their concrete queue capacities, worker counts,
timeouts, heartbeat intervals, drain limits, and shutdown deadlines. They must
conform to this document and must not redefine its general policy.

## Terms and runtime roles

Concurrency is the coordination of work whose lifetimes overlap. Parallelism is
the simultaneous execution of compute work. Concurrent work does not imply a
right to consume another CPU, create another worker, or start a native pool.

The policy applies to the Raylib UI, coordinator, job workers, CLI, supported
client, async protocol tasks, owned threads, child processes, native-library
pools, shared-memory transports, and Cython kernels. Process topology remains
owned by [system architecture](system-architecture.md); job scheduling remains
owned by [training runtime](../pages/jobs/runtime.md).

## Execution selection

- The UI process locks its initial OS thread before native initialization. That
  thread alone initializes, polls, renders, calls Raylib/Raygui, captures native
  frames, and shuts the native UI down.
- Owned threads perform blocking I/O and orchestration that cannot be expressed
  as nonblocking protocol work. Database, filesystem, network, decoding,
  training, and blocking library calls stay off the render thread.
- Async tasks orchestrate nonblocking protocol operations. They do not conceal
  CPU work or blocking calls; adapters move such work to an owned execution
  context.
- Lease-bounded child processes are the default for pure-Python CPU
  parallelism. Role processes and per-job workers also provide lifecycle and
  crash isolation.
- Native-library threads and pools run only through validated adapters and
  within a coordinator-issued CPU lease.
- Cython starts GIL-held. A measured typed native kernel may use `nogil` only
  after meeting the evidence requirements below.
- CLI and client operations obey the same ownership, bounds, cancellation, and
  cleanup rules; command completion or explicit close terminates their owned
  work.

Distributed runtimes, external brokers, unbounded executors or queues,
unrestricted pools, detached background work, and hidden oversubscription are
prohibited unless the owning living specifications and technology policy are
revised.

## Declaration and ownership contract

Before implementation, every process, thread, async task, queue, stream, and
pool declares in its owning subsystem specification or typed lifecycle design:

- owner and creation authority;
- sender, receiver, and closer, where messages are involved;
- finite capacity and saturation behavior;
- cancellation signal, propagation path, and cancellation boundary;
- termination condition and bounded shutdown deadline;
- failure reporting and the owner responsible for containment and cleanup.

The owner joins, closes, or terminates the resource and accounts for all work
accepted before closure. A resource must not outlive its owner. Abandoned
sends, daemon lifetime, garbage collection, interpreter exit, or GIL behavior
are not cleanup mechanisms.

Mutable state has exactly one owner or is protected by an explicit lock, queue,
or other documented synchronization primitive. Code must not depend on the GIL,
container implementation details, or incidental atomicity. Locks are not held
across I/O, database, process, native callback, user callback, or other
unbounded boundaries, and lock-bearing objects are not copied between owners.

## CPU leases and nested parallelism

The coordinator is the sole authority for workstation compute leases. A lease
represents the maximum simultaneous CPU-consuming process workers and native or
library threads available to its holder. Role and orchestration threads that do
not consume a lease remain lightweight and may not perform sustained compute.

- A process or adapter obtains a lease before creating or enlarging compute
  capacity.
- Process workers, BLAS/OpenMP threads, estimator pools, tensor-library pools,
  Cython parallel regions, and other native workers all count against the same
  lease.
- Nested parallel work reuses the parent's remaining lease. If no capacity
  remains, the nested operation executes serially or waits at a documented
  scheduler boundary; it never creates hidden capacity.
- Pool-limit environment and library settings are applied before importing or
  initializing libraries that snapshot those settings.
- Lease reductions occur only at deterministic, subsystem-defined safe
  boundaries. Capacity is not preempted by corrupting active work.
- Telemetry reports granted, configured, active, queued, and peak compute use so
  the coordinator can detect oversubscription.

Concrete global budgets, default worker counts, and per-job allocation rules
belong to their subsystem specifications.

## Windows process and native constraints

Windows child processes use an explicit supported multiprocessing context and
an import-safe top-level entry point guarded against recursive launch. Child
arguments are validated serializable values, handles are inherited or
duplicated deliberately, and child startup revalidates the exact regular
CPython runtime, protocol, role, capability token, and resource limits.

Composition, process launch, pool creation, and environment configuration occur
outside import side effects. Each child initializes its own adapters and closes
them before exit. Parent-death, startup failure, broken IPC, and partial child
creation have explicit reconciliation and cleanup paths. Native callbacks and
thread-affine handles never cross process or thread ownership boundaries.

## Backpressure and UI responsiveness

All queues, streams, subscriptions, executor submissions, and result buffers
are bounded. Their owners define one saturation response appropriate to the
message class: reject with a stable error, block only on a non-UI owner through
a deadline, coalesce replaceable state, drop explicitly lossy telemetry with a
counter, or cancel upstream work. Silent loss and unlimited buffering are
prohibited.

The render thread never waits for a send, receive, future, join, lock held by
background work, network response, file operation, or worker shutdown. It
publishes bounded commands nonblockingly, consumes a bounded amount of ready
work per frame, and renders the latest immutable state. Visible progress and
shutdown decisions derive from state rather than blocking the frame loop.

## Determinism and publication

Scheduling, worker count, process identity, arrival order, dictionary order,
wall-clock timing, and native completion order must not affect logical results.
Tasks and events carry stable logical identifiers. Owners reorder results by
those identifiers and perform reductions in a specified stable order.

Random seeds derive from versioned logical coordinates, not execution
placement. Wall-clock reads enter through an injected clock and persisted
timestamps are not algorithmic ordering keys. Replay inputs record the seed,
clock, resource configuration, and relevant dependency identity.

Tasks own mutable outputs. Publication is a one-way transition to an immutable
snapshot. Published NumPy arrays, tensors, Arrow buffers, memory maps, render
buffers, messages, and manifests are read-only and are never mutated in place.
Consumers publish a new version rather than changing an observed object.

Shared-memory writers have exclusive, nonoverlapping ranges and publish only
after completion, validation, and a documented synchronization handoff. A
versioned manifest records dtype/schema, shape, byte order, size, checksum,
producer, and lifetime. Readers map read-only, validate before use, hold an
explicit lifetime reference, and unmap before the owner deletes or replaces the
backing object. Partially written or abandoned segments are never published and
are reclaimed during reconciliation.

## Cancellation, shutdown, and failure containment

Cancellation is cooperative first, idempotent, and propagated from the
requesting owner through every queue, adapter, task, and child it started.
Blocking adapters either support bounded cancellation or declare the safe
boundary and deadline at which control returns. Cancellation never publishes a
partial result as complete and never removes completed immutable evidence.

Shutdown proceeds in dependency order:

1. stop accepting new external and upstream work;
2. signal cancellation or subsystem-specific pause;
3. close producer sides so consumers can drain or terminate;
4. drain only the bounded work required by the owning subsystem;
5. flush and close durable writers and protocol streams;
6. await tasks, join threads, and join child processes through declared
   deadlines;
7. terminate remaining child processes or fail closed according to the owning
   subsystem, recording lost uncommitted work;
8. release native resources, shared memory, leases, and UI resources in reverse
   ownership order.

Failure remains within the smallest owning boundary. Task failure cancels or
reports to its task group; adapter failure invalidates that adapter; worker
failure is reconciled by the coordinator; UI background failure degrades a
capability without moving work onto the render thread. Unknown exceptions,
broken IPC, heartbeat expiry, deadline expiry, and forced termination become
typed failures with correlation and resource identity. Cleanup failures are
reported and do not replace the initiating failure.

## Health, resources, and leak prevention

Health and diagnostics expose process role and identity; owner/resource type;
task, thread, process, queue, and pool counts; queue capacity and occupancy;
lease grant and use; cancellation and shutdown state; last progress or
heartbeat; saturation, drop, retry, and forced-termination counters; and a
stable failure code. User-facing status omits secrets and one-time capability
tokens.

Normal completion, rejection, cancellation, timeout, startup failure, adapter
failure, broken IPC, and forced shutdown must release tasks, threads, children,
handles, mappings, temporary files, and leases. Tests establish a bounded
baseline before each lifecycle and assert return to it after cleanup. Repeated
start/stop and fault-injection runs must not show monotonic resource growth.

## Native parallelism and `nogil` admission

An adapter may enable library-native parallelism only with Windows evidence
that it honors configured limits, counts every native worker against the lease,
does not create nested hidden pools, has explicit cancellation and shutdown
behavior, contains native mutable state, and produces deterministic supported
results across repeated runs and permitted worker counts. Unsupported modes are
disabled explicitly.

A Cython kernel may release the GIL only when measurement shows the GIL-held
typed implementation is a material bottleneck and all of the following exist:

- a stable typed Python-facing adapter and Python parity tests;
- no Python objects, Python callbacks, Python allocation, or shared mutable
  state in the `nogil` region;
- exclusive or read-only memory ownership with bounds and lifetime validation;
- deterministic concurrent-call, varied-scheduling, and varied-worker-count
  tests;
- documented cancellation boundaries and bounded return behavior;
- exception/error translation after the GIL is reacquired;
- Windows regular-`cp314` build/import evidence and benchmarks demonstrating
  the benefit without oversubscription.

OpenMP or native threads used by a kernel are subject to the same adapter and
lease evidence. Releasing the GIL is never evidence of thread safety by itself.

## Verification ownership

[Concurrency verification](quality/concurrency.md) owns acceptance tests for
this policy. Subsystem specifications supply the numeric limits and scenarios
used by those tests. ADR 0008 records why regular CPython and this execution
model were selected; this living specification owns current behavior.
