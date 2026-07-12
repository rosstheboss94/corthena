# ADR 0006: Curated Python and Cython Dependencies

**Status:** Accepted
**Date:** 2026-07-12

## Context

The migration introduces Python packaging, scientific libraries, native wheels,
Raylib bindings, HTTP services, typed validation, and security tooling. Every
dependency increases compatibility, typing, native-thread, maintenance, and
supply-chain risk.

## Decision

Maintain one approved Python/Cython technology stack. CPython, `uv`, and
`pyproject.toml` own the environment and direct dependency set; `uv.lock`
records resolved versions. Add direct dependencies only for distinct
responsibilities after Windows compatibility and quality gates pass.

- Define approved direct dependencies and their responsibilities only in
  `technology-stack.md`.
- Record resolved versions in `uv.lock`; keep `pyproject.toml` as the direct
  dependency and tool configuration source.
- Isolate native, weakly typed, and library-specific values behind narrow typed
  adapters.
- Prevent overlapping libraries from owning the same responsibility.
- Let scikit-learn and PyTorch own estimator internals, while Corthena owns
  orchestration, validation, manifests, registry, compatibility checks,
  auditability, and UI workflows.
- Use Cython only for measured hot paths or native adapters.

## Alternatives

- Keep a non-Python dependency model.
- Allow each subsystem to choose overlapping Python libraries independently.
- Vendor or reimplement every infrastructure and ML concern internally.

## Consequences

Dependency admission must verify Windows wheels or builds, native library
behavior, typed adapters, cancellation/failure boundaries, license suitability,
security scans, and compatibility with the screenshot and behavioral baseline.

## Affected specifications

- [Technology stack](../technology-stack.md)
- [System architecture](../system-architecture.md)
- [Quality](../quality.md)
- [Frontend foundation](../frontend/foundation.md)
