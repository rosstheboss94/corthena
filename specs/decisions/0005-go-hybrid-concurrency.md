# ADR 0005: Go Process and Goroutine Hybrid Concurrency

**Status:** Accepted  
**Date:** 2026-07-04

## Context

The workstation requires deterministic multi-core training, crash isolation, resumable jobs, a responsive Raylib UI, strict typed boundaries, and controlled CPU use on a Windows workstation.

## Decision

Implement the application, public client, CLI, tests, tools, and feature-extension model in Go. Use processes for role and training-job isolation, with bounded goroutine pools for parallel work inside each process.

- Keep the UI, coordinator, and each active training job in separate processes.
- Use bounded goroutine pools inside workers under coordinator-owned CPU leases.
- Lock the UI goroutine to its OS thread and confine all Raylib and Raygui calls to it.
- Use the approved Raylib and Raygui Go bindings; permit cgo only inside approved adapter packages.
- Use immutable shared inputs, task-owned outputs, stable task indices, and ordered reductions.
- Use explicit context cancellation, channel ownership, process handshakes, and heartbeat reconciliation.
- Register feature implementations explicitly at compile time.
- Expose Go version, build, component, goroutine, and CPU-lease health.

## Alternatives

- Put all roles in one process and rely only on goroutines.
- Use process parallelism without goroutine pools.
- Load feature implementations dynamically at runtime.
- Adopt a distributed task runtime.

## Consequences

Process-level failure containment and shared-memory parallelism coexist. Race safety, goroutine lifecycle, deterministic reduction order, CPU oversubscription, cgo thread affinity, and process protocol validation are explicit engineering obligations. Custom features require a rebuild, which favors Windows portability, typed contracts, auditability, and deterministic deployment.

## Affected specifications

- [System architecture](../system-architecture.md)
- [Training runtime](../training-runtime.md)
- [Models](../models.md)
- [API](../api.md)
- [Data and features](../data-and-features.md)
- [Frontend foundation](../frontend/foundation.md)
- [Quality](../quality.md)
