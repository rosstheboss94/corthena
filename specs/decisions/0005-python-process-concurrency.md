# ADR 0005: Python Process and Thread Ownership

**Status:** Superseded by [ADR 0007](0007-free-threaded-python-concurrency.md); current decision is [ADR 0008](0008-regular-cpython-concurrency.md)
**Date:** 2026-07-12

## Context

The workstation requires a responsive Raylib UI, deterministic job
orchestration, crash isolation, controlled CPU use, cancellation, and typed
process boundaries on Windows. Python adds the GIL, library-owned thread pools,
native extension behavior, and process-spawn constraints.

## Decision

Implement the application, public client, CLI, tests, tools, and extension
model in Python with optional Cython extensions. Use separate processes for the
UI, coordinator, and active training jobs; use explicitly bounded worker
threads, process pools, or library thread pools only behind owned adapters.

- Keep the UI, coordinator, and each active training job in separate processes.
- Lock the UI OS thread before Raylib/Raygui initialization and confine every
  Raylib/Raygui call to that thread.
- Keep blocking I/O, decoding, database, network, training, and library calls
  off the render thread.
- Bound NumPy, scikit-learn, PyTorch, BLAS/OpenMP, and process-pool parallelism
  through coordinator CPU leases and adapter-owned settings.
- Publish shared arrays, Arrow buffers, memory maps, and tensors as immutable;
  tasks own mutable outputs and reducers apply results in stable logical order.
- Use cancellation tokens, process handshakes, queue ownership, heartbeat
  reconciliation, and explicit shutdown deadlines.
- Expose Python version, implementation, build revision, dependency versions,
  component role, PID, thread/process counts, native-library state, and CPU
  lease health.

## Alternatives

- Put all roles in one Python process.
- Use only process parallelism with no library or thread-level parallelism.
- Let each numerical library choose unrestricted thread pools.
- Adopt a distributed task runtime.

## Consequences

Process-level failure containment remains the primary safety boundary.
Adapters must make native-thread ownership, library thread pools, cancellation,
deterministic ordering, and CPU oversubscription explicit. Cython and native
bindings require Windows build validation and narrow typed boundaries.

## Affected specifications

- [System architecture](../general/system-architecture.md)
- [Training runtime](../pages/jobs/runtime.md)
- [Models](../pages/models/README.md)
- [API](../general/api.md)
- [Data and features](../pages/data/ingestion.md)
- [UI foundation](../general/ui/README.md)
- [Quality](../general/quality/README.md)
