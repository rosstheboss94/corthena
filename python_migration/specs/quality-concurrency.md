# Concurrency Verification

Every process, thread, queue, async task, and library pool has an owner,
termination condition, and cancellation path; every queue or stream documents
sender, receiver, closer, and buffering. Never hold
locks across I/O, process, database, or callback boundaries; never copy
lock-bearing objects. Publish shared arrays, tensors, buffers, and mapped files
only after immutability.

Run lifecycle and concurrency tests for coordinator, workers, client,
scheduler, cache, adapters, and reducers. Vary worker counts, library pool
limits, and completion orders and require deterministic artifacts and metrics.
Test CPU leases, nested pools, cancellation, queue closure, bounded draining,
shutdown, and process/thread leaks. Enforce Raylib main-thread ownership.
Do not treat the GIL as an application synchronization primitive. Test process-
and thread-pool cancellation, stable-order reductions, lease accounting across
process and native pools, and deterministic completion under varied scheduling.
Every Cython `nogil` kernel additionally requires Python parity, concurrent-call,
cancellation-boundary where applicable, and benchmark evidence.

Required behavior coverage includes hand-calculated model/metric/serialization
tables, DTO/message/manifest/checkpoint/migration/layout property tests,
interrupt/resume boundaries, corrupt/partial/incompatible versions, leakage
in features/targets/purge/embargo/execution, imports during paused runs, API
and client integration, aliases/inference rejection, crashes/stale heartbeats,
and pause-on-close.
