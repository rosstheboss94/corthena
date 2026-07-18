# ADR 0008: Regular CPython with Measured Cython Acceleration

**Status:** Accepted  
**Date:** 2026-07-12  
**Supersedes:** [ADR 0007](0007-free-threaded-python-concurrency.md)

## Context

Corthena requires a supported Windows runtime, deterministic ownership, crash
isolation, and controlled CPU parallelism. The experimental free-threaded ABI
adds native-wheel and binding risk and is not required to preserve the existing
UI, coordinator, and per-training-job process boundaries.

## Decision

Use exactly regular Windows AMD64 CPython `3.14.2` with the
`cp314-win_amd64` ABI. Keep separate UI and coordinator processes and one worker
process per active training job. Use lease-bounded processes for pure-Python CPU
parallelism, threads for I/O and orchestration, and bounded library-native pools
when their adapters prove safe GIL release and deterministic behavior.

Keep ordinary Python and Cython code GIL-held. Release the GIL only inside a
typed Cython native kernel that accesses no Python objects or shared mutable
state and has parity, deterministic concurrent-call, cancellation-boundary where
applicable, and benchmark evidence. Continue explicit ownership, immutable
publication, stable-order reductions, cancellation, heartbeats, bounded
shutdown, CPU leases, and Raylib UI-thread confinement. Do not use the GIL as
an application synchronization contract.

## Alternatives

- Continue with free-threaded CPython: rejected for the supported runtime due
  to experimental ABI and native-dependency compatibility risk.
- Collapse process boundaries: rejected because it weakens UI responsiveness
  and coordinator/job crash isolation.
- Convert orchestration or business logic to Cython: rejected because Cython is
  reserved for measured hot kernels and native adapters.
- Permit unrestricted process or native-library pools: rejected because it
  violates CPU leases and deterministic resource accounting.

## Consequences

Phase 0 must pass again in a clean regular CPython 3.14.2 environment. Runtime
health reports exact version, implementation, ABI/platform, role, and bounded
resource counts without free-threaded or GIL-degradation fields. Extensions
carry the regular `cp314` tag and no longer define `Py_GIL_DISABLED` or declare
free-threading compatibility. Historical 3.13t and 3.14t reports remain as
superseded evidence.

## Affected specifications

- [Concurrency and parallelism](../concurrency-and-parallelism.md)
- [Technology stack](../technology-stack.md)
- [System architecture](../system-architecture.md)
- [Python migration](../python-migration.md)
- [Training runtime](../training-runtime.md)
- [API](../api.md)
- [Quality](../quality.md)
