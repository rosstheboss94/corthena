# Agent-Facing Contract Specification

**Status:** Authoritative
**Owner:** Architecture
**Last updated:** 2026-07-16
**Related:** [Repository guidance](../AGENTS.md), [Specification index](README.md), [Contract examples](examples/agent-facing-contracts.md), [Design pattern](design-pattern.md), [API](api.md), [Quality](quality.md)

This document is the mandatory baseline for every task in the Python/Cython
migration. Read it before planning, editing, reviewing, testing, or selecting
additional task context. It defines how Python contracts are organized and how
to choose the minimum implementation context needed for a task.

Examples and tutorials are intentionally separate so this mandatory document
stays small. They do not add requirements to this specification.

## Production contract rules

- Put behavioral contracts in `<capability>/protocol.py`.
- Put concrete classes in implementation-specific modules.
- Put static conformance checks and usage examples in tests or examples.
- Keep production `protocol.py` and `.pyi` files limited to the imports, types,
  properties, and method signatures required by consumers.
- Do not put tutorial comments, implementation details, or usage examples in a
  production contract.
- Add a concise behavioral docstring only when a signature cannot express an
  important guarantee, constraint, side effect, or error.
- Name contracts by capability with a `Protocol` suffix.
- Name implementations by their concrete behavior or technology.
- Organize packages by capability, not by individual class, and add
  `__init__.py` to every importable package directory.
- Keep implementation imports and implementation details out of contracts.
- Expose only the properties and methods a consumer needs. Expose public
  instance state through properties; shared class metadata remains a
  `ClassVar`.

## Agent context policy

- Every behavior-based object used directly by an agent or passed across a
  package boundary must have a compact agent-facing contract.
- Give an agent only that contract and the request/result models it references
  when the task does not modify the implementation.
- Keep internal helpers hidden behind the public contract. Give a helper its
  own contract only when an agent or another package uses it directly.
- Use `Protocol` when interchangeable implementations share one behavioral
  contract.
- Use a `.pyi` stub when an agent needs a signature-only view of a specific
  concrete class rather than an interchangeable interface.
- Use dataclasses, `TypedDict`, or Pydantic models for data-only contracts.
  Do not create behavioral protocols for records with no behavior.

## Task-to-context routing

| Task | Required implementation context |
|---|---|
| Call an injected capability | Its `protocol.py` and referenced request/result models |
| Implement a new adapter | Its `protocol.py`, referenced models, and only the adapter-specific requirements |
| Construct or directly use a concrete implementation | Its `.pyi` stub and referenced models; read the implementation only if no adequate public interface exists |
| Modify implementation internals | The owning `.py` file and only directly relevant internal dependencies |
| Change a public contract | The contract, referenced models, consumers, implementations, and owning specification |
| Review boundary conformance | The contract, referenced models, changed implementation, and directly affected consumers/tests |

Default to `protocol.py`. Add a `.pyi` file only when concrete public details,
such as a constructor, are required. Do not load sibling implementations,
internal helpers, renderers, persistence adapters, or composition roots merely
because they share a package.

Pyright uses protocols for structural compatibility and `.pyi` files to check
callers against a concrete module's public interface.

## Subsystem context maps

These maps refine the general routing table. Paths are relative to the
migration root. Read the owning route specifications separately as required by
`AGENTS.md`.

### UIClient call

Read:

- `src/corthena/ui/client/protocol.py`
- The request/result models referenced by the methods being called; for Phase
  7, `src/corthena/ui/data_experiments/models.py`

Do not load the simulator, effects runtime, reducer, lifecycle, or renderer
unless the task changes their behavior.

### Phase 7 adapter implementation

Read:

- `src/corthena/ui/client/protocol.py`
- `src/corthena/ui/data_experiments/models.py`
- Only the adapter being implemented, such as
  `src/corthena/ui/simulator.py` or a future coordinator-backed client adapter

Load effects or lifecycle code only when the adapter changes cancellation,
queueing, generation, shutdown, or composition behavior.

### Phase 7 internals

Read:

- The owning implementation file under
  `src/corthena/ui/data_experiments/`
- Only dependencies imported by that file that are relevant to the requested
  change
- The focused tests for the changed behavior

Add `client/protocol.py`, effects, simulator, lifecycle, serialization, shell,
or Raylib files only when the change crosses those boundaries.

When adding another subsystem map, identify one consumer task, one adapter
task, and one internal-modification task. Each entry must state both what to
read and what remains excluded by default.

## Stub rules

- A stub has the same base name and package location as its implementation:
  `text_length.py` is described by `text_length.pyi`.
- A stub contains public imports, types, properties, constructors, and method
  signatures with `...` instead of implementation bodies.
- Python does not execute a stub. When both files exist, Pyright uses the stub
  as the concrete module's public interface.
- Update the stub whenever the implementation's public API changes. A stale
  stub can make invalid callers appear correct.
- Do not add a stub by default. Add one only when an agent or external consumer
  needs the public API of a specific concrete implementation.

See [Agent-facing contract examples](examples/agent-facing-contracts.md) for a
stub layout, package layout, and generic and non-generic templates.
