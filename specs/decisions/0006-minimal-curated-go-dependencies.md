# ADR 0006: Minimal Curated Go Dependencies

**Status:** Accepted  
**Date:** 2026-07-04

## Context

The Windows desktop UI, columnar storage, embedded database, memory mapping, and event streaming require a small set of direct dependencies. Every dependency increases compatibility, typing, concurrency, maintenance, and supply-chain risk.

## Decision

Maintain one approved Go technology stack, prefer the standard library, and admit a direct dependency only for a distinct responsibility after the compatibility and quality gates pass.

- Define approved direct dependencies and their responsibilities only in `technology-stack.md`.
- Record exact versions only in `go.mod` and `go.sum`.
- Isolate native and weakly typed packages behind narrow typed adapters.
- Keep estimator algorithms, orchestration, repositories, CLI behavior, charts, tables, docking, retries, and lifecycle assertions as first-party Go code.
- Prevent overlapping libraries from owning the same responsibility.

## Alternatives

- Adopt framework-rich persistence, logging, scheduling, and UI stacks.
- Implement every infrastructure concern internally.
- Allow subsystems to select overlapping libraries independently.

## Consequences

The project carries focused first-party infrastructure but has fewer native and transitive compatibility risks. Dependency additions require explicit ownership, Windows validation, typed boundaries, failure and cancellation tests, and specification updates.

## Affected specifications

- [Technology stack](../technology-stack.md)
- [System architecture](../system-architecture.md)
- [Quality](../quality.md)
- [Frontend foundation](../frontend/foundation.md)
