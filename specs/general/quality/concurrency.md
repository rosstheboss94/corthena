# Concurrency Verification

**Status:** Authoritative

**Owner:** Engineering

**Last updated:** 2026-07-12

**Related:** [Concurrency and parallelism](../concurrency-and-parallelism.md), [Quality](README.md)

This document owns verification of the behavior defined by
[concurrency and parallelism](../concurrency-and-parallelism.md). It does not
redefine runtime policy. Use the concrete capacities, deadlines, worker counts,
cancellation boundaries, and failure scenarios from each owning subsystem.

## Lifecycle and ownership evidence

- Assert that every process, thread, task, queue, stream, shared mapping, and
  pool has the declared owner, participants, capacity, saturation response,
  cancellation path, termination condition, deadline, and failure report.
- Exercise normal completion, rejection, cancellation, timeout, startup
  failure, callback or adapter failure, broken IPC, heartbeat expiry, and forced
  shutdown. Assert dependency-ordered closure and preservation of completed
  immutable evidence.
- Establish pre-run resource baselines and assert return to baseline after each
  lifecycle. Repeat start/stop and injected-failure cycles to detect leaked or
  monotonically growing tasks, threads, processes, handles, mappings, temporary
  files, subscriptions, and CPU leases.
- Test queue and stream saturation for the declared reject, deadline, coalesce,
  or lossy-telemetry behavior. Verify bounded draining and that cancellation
  cannot strand a sender or receiver.

## Synchronization and publication evidence

- Test ownership transfers and explicit synchronization under varied thread and
  process scheduling. Include checks that application correctness does not rely
  on GIL atomicity.
- Assert published arrays, tensors, Arrow buffers, render buffers, manifests,
  and memory maps are read-only. Verify exclusive writer ranges, validation
  before shared-memory publication, reader lifetime accounting, and cleanup of
  abandoned unpublished segments.
- Instrument blocking boundaries to detect locks held across I/O, database,
  process, native callback, or user callback operations.

## Resource accounting and determinism evidence

- Vary worker counts, completion orders, task delays, queue pressure, library
  pool limits, and permitted lease sizes. Require identical logical state,
  artifacts, metrics, and stable-order reductions for the same recorded seed
  and clock.
- Test coordinator lease accounting across child processes, BLAS/OpenMP,
  scikit-learn, PyTorch, adapter-native pools, and Cython parallel regions.
  Exercise nested work and assert configured, active, and peak compute never
  exceed the granted lease.
- Verify seed derivation uses stable logical coordinates and that PID, wall
  clock, dictionary order, scheduling, and arrival order cannot affect results.

## UI and protocol evidence

- Enforce Raylib/Raygui initialization, polling, rendering, capture, and cleanup
  on the locked UI OS thread. Instrument the render loop to reject blocking
  sends, receives, futures, joins, background-held locks, I/O, decoding,
  database, training, and library calls.
- Test async client, coordinator, worker, event-stream, and CLI cancellation,
  reconnect or broken-stream cleanup, bounded submissions, correlation of typed
  failures, and explicit close behavior.

## Native and Cython evidence

- For every native-library pool, test configured limit enforcement, nested-pool
  suppression, cancellation, deterministic supported modes, shutdown, repeated
  initialization, and truthful resource telemetry on Windows.
- For every Cython `nogil` kernel, require Python parity, regular-`cp314` Windows
  build/import coverage, concurrent-call and varied-scheduling determinism,
  immutable or exclusive memory ownership, cancellation-boundary behavior,
  error translation, lease accounting, and a benchmark proving material value.

## Domain integration coverage

Apply these concurrency obligations to coordinator, workers, client, scheduler,
cache, adapters, reducers, imports, jobs, checkpoints, API/event reconciliation,
ui effects, and pause-on-close. Combine them with the owning subsystem's
hand-calculated tables, DTO/message/manifest property tests,
interrupt/resume and corrupt/partial/incompatible-version cases, leakage-safety
tests, crash/stale-heartbeat cases, and immutable registry/inference behavior.
