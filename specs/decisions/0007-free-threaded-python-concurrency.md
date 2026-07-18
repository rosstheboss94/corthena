# ADR 0007: Free-Threaded Python Concurrency

**Status:** Superseded by [ADR 0008](0008-regular-cpython-concurrency.md)  
**Date:** 2026-07-12  
**Supersedes:** [ADR 0005](0005-python-process-concurrency.md)

## Context

Corthena needs crash isolation and deterministic ownership while permitting
CPU-bound Python work to use multiple cores. CPython provides a free-threaded
build, but native extensions can re-enable the GIL. The initially selected
3.13.9 build was incompatible with the approved CFFI/Raylib dependency path.

## Decision

Use exactly Windows AMD64 CPython `3.14.2+freethreaded` (`cp314t`). Retain the
UI, coordinator, and one-worker-process-per-training-job topology, anonymous
pipe protocols, handshakes, heartbeats, reconciliation, cancellation, and
bounded shutdown. Prefer bounded threads within each process for eligible
Python work and account for Python and library-owned pools under coordinator
CPU leases. Never rely on GIL atomicity; use ownership, locks, queues, or
immutable snapshots. Check GIL state after native imports.

A correct free-threaded build whose GIL is enabled remains functionally
supported in explicit degraded mode with warnings and reduced parallel
performance claims. A regular build or wrong patch version fails startup.

## Alternatives

- Replace the existing topology with one process: rejected because it removes
  UI, coordinator, and per-job crash isolation.
- Use only processes for parallelism: rejected as the default because it adds
  serialization and lifecycle cost where free-threaded code is safe.
- Allow unrestricted threads or native pools: rejected because it breaks CPU
  leases, cancellation, and deterministic resource accounting.

## Consequences

Phase 0 must pass again for `cp314t-win_amd64`. Every native dependency needs a
compatible wheel or approved source build and concurrent-use audit. Cython
extensions define `Py_GIL_DISABLED=1`, declare compatibility only after tests,
and carry the `cp314t` tag. Free-threaded runtime limits and GIL degradation are
visible in health and compatibility evidence.

## Affected specifications

- [Technology stack](../technology-stack.md)
- [System architecture](../system-architecture.md)
- [Python migration](../python-migration.md)
- [Training runtime](../training-runtime.md)
- [API](../api.md)
- [Quality](../quality.md)
