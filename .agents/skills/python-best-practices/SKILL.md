---
name: python-best-practices
description: Implement, refactor, test, or review Python or Cython code in Corthena using typed, deterministic, Windows-safe practices. Use for Python/Cython migration work, including Raylib/native adapters, concurrency, DTOs, and code reviews.
---

# Python Best Practices

## Ground the work

1. Inspect `python_migration/AGENTS.md`, the current workspace, and the owning
   specification before planning or editing. Follow its routing table; read
   `python_migration/specs/README.md` when ownership crosses documents.
2. Preserve unrelated changes. Treat living specifications as canonical and
   report conflicts rather than resolving them silently.
3. Update the owning specification for behavioral or public-contract changes.
   Read [Corthena migration rules](references/corthena-migration.md) when
   selecting a route or applying project-only constraints.

## Keep boundaries typed

- Give public APIs, DTOs, repositories, manifests, and adapters explicit types.
  Use immutable boundary values and validated dataclass or Pydantic DTOs;
  reject unknown fields before domain conversion.
- Keep interfaces narrow and inject dependencies explicitly. Raise typed,
  meaningful exceptions. Use context managers for resources and explicit
  cleanup for lifetimes they cannot manage.
- Do not introduce `Any`, unchecked casts, reflection-style models, or untyped
  public dictionaries. Convert framework, library, and native values in narrow
  typed adapters; do not let them into domain code.
- Keep domain logic independent of UI, persistence, native libraries, and
  framework-specific values. Depend inward through typed ports and adapters.

## Preserve ownership and determinism

- Define ownership, cancellation, shutdown, and backpressure for every
  process, thread, task, queue, and pool. Bound work and avoid sharing mutable
  library objects across owners.
- Make results stable across worker counts and completion orders. Seed and
  record replay inputs where required; use stable ordering.
- Publish arrays, memory maps, Arrow buffers, and tensors read-only. Let tasks
  own mutable outputs and publish immutable snapshots only.

## Use Cython deliberately

- Start in typed Python. Add Cython only for a measured hot path or a justified
  native boundary.
- Preserve a stable Python-facing API, provide `.pyi` coverage, test parity
  with the Python behavior, and document fallback and failure behavior.
- Benchmark the claimed benefit and keep native values contained in adapters.

## Keep Windows UI safe

- Lock one UI OS thread before native initialization. Initialize, poll, render,
  call Raylib/Raygui, and shut down only on that thread.
- Keep I/O, networking, persistence, decoding, training, and blocking library
  calls off the render thread. Exchange bounded, immutable typed messages and
  never block the render thread on a send.

## Verify honestly

- Run only applicable configured quality gates from the owning specifications.
  Do not invent Python commands before `pyproject.toml` exists.
- Add focused tests for validation, ownership, cancellation, determinism,
  adapter containment, and cleanup when the change affects them.
- Report executed checks, failures, and skipped checks accurately.
