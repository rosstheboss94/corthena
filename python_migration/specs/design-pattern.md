# Design Pattern and Modular Monolith Specification

**Status:** Authoritative  
**Owner:** Architecture  
**Last updated:** 2026-07-12  
**Related:** [Specification index](specs/README.md), [Technology stack](specs/technology-stack.md), [System architecture](specs/system-architecture.md), [API](specs/api.md), [Python migration](specs/python-migration.md)

This is the shared architecture pattern specification for Python/Cython routes.
Future agents should update this document when module boundaries, public
interfaces, dependency direction, or recurring design patterns change.

## Architectural style

Corthena uses a Modular Monolith. The application remains one local product and
one coordinated codebase, but the code is split into cohesive modules with
explicit public interfaces and enforced dependency direction.

This fits the project because the V1 target is a Windows-first, single-user
research workstation, not a distributed service. The system still has separate
runtime processes for safety and responsiveness:

```text
Raylib UI process
  -> loopback HTTP/WebSocket client
  -> typed contracts

Coordinator process
  -> API routes, scheduler, CPU leases, repositories, artifact promotion
  -> launches one worker process per active training job

Worker process
  -> job orchestration, estimator adapters, checkpoint state, typed events

CLI / supported Python client
  -> typed contracts and client APIs
```

The module architecture should not be confused with microservices. Do not add
external brokers, distributed task queues, distributed runtimes, ORMs, extra UI
frameworks, or network services unless an owning spec is revised.

## Boundary rules

- Modules communicate through public APIs, typed DTOs, commands, events,
  repositories, adapters, and value objects.
- A module may import another module's documented public package. It must not
  import another module's internal implementation.
- DTOs are distinct from domain values. Inbound DTOs validate unknown fields,
  schema versions, identifiers, timestamps, and business rules before domain
  conversion.
- Public and domain boundaries must not expose Raylib values, Arrow builders,
  SQLite rows, pandas objects, raw estimator objects, tensors, native handles,
  Cython pointers, or untyped dictionaries.
- The coordinator owns durable metadata writes, scheduling, CPU leases, job
  state transitions, artifact promotion, registry indexing, and alias
  transactions.
- Workers own job-local mutable estimator and checkpoint state. They receive
  immutable inputs, emit typed protocol events, and do not expose a network
  listener.
- UI panels consume typed state and emit `UIAction` values. They do not call
  repositories, filesystem, network, workers, simulator internals, or
  coordinator internals directly.
- Raylib/Raygui calls remain on the UI process's locked OS thread. Blocking I/O,
  database, network, Arrow decoding, training, and library calls remain off the
  render thread.
- Shared NumPy arrays, tensors, Arrow buffers, memory maps, and render buffers
  are immutable after publication. Tasks own output ranges.
- Reducers apply results in stable logical order, never arrival order.
- Completed runs, artifacts, model outputs, prediction outputs, aliases history,
  and manifests are immutable and versioned.

## Dependency direction

Allowed direction is inward toward stable contracts and domain concepts, then
outward through adapters selected at process composition roots.

```text
frontend ─┐
cli ──────┼──> client ──> contracts
external ─┘

coordinator/api ──> application/orchestration ──> domain
                                      │            │
                                      │            ├──> public module APIs
                                      │            └──> value objects
                                      │
                                      ├──> repositories interfaces
                                      ├──> worker protocol contracts
                                      └──> artifact/manifest services

infrastructure adapters ──> public internal interfaces
  SQLite, Arrow, Parquet, Raylib, PyTorch, scikit-learn, Cython, filesystem
```

Concrete adapters may depend on third-party libraries. Domain and application
logic depend on typed internal interfaces, not concrete infrastructure.

## Recommended project layout

Organize primarily by business capability. Avoid deeply nested
`domain/application/infrastructure` stacks inside every module unless a route
has clear complexity that justifies it.

```text
project-root/
├── pyproject.toml
├── uv.lock
├── README.md
├── src/
│   └── corthena/
│       ├── app/              # process composition roots and shared startup
│       ├── contracts/        # public DTOs, enums, errors, Arrow schemas
│       ├── client/           # supported Python client
│       ├── coordinator/      # loopback API, scheduler, workers, health
│       ├── worker/           # worker entry point and job protocol runtime
│       ├── cli/              # command-line surface
│       ├── frontend/         # Raylib shell, state, effects, widgets
│       ├── domain/           # shared value objects and core invariants
│       ├── data/             # imports, catalog, canonical bars
│       ├── features/         # feature registry and materialization
│       ├── statistics/       # metrics and stable numerical reductions
│       ├── datasets/         # dataset queries and materialized matrices
│       ├── experiments/      # experiment definitions, sweeps, estimates
│       ├── training/         # job orchestration and checkpoint policy
│       ├── models/           # estimator adapters and model specs
│       ├── evaluation/       # folds, metrics, comparisons
│       ├── registry/         # model artifacts, aliases, compatibility
│       ├── inference/        # historical/latest scoring workflows
│       ├── backtesting/      # reference analytical portfolios
│       ├── risk/             # analytical risk calculations
│       ├── portfolio/        # portfolio reporting abstractions
│       ├── execution/        # V1 reference execution rules, no broker live orders
│       ├── persistence/      # repository implementations and migrations
│       ├── orchestration/    # cross-module application services
│       ├── monitoring/       # health, status, logs, diagnostics
│       ├── events/           # event envelopes, IDs, reconciliation helpers
│       ├── config/           # typed settings and application paths
│       ├── platform/         # OS/native adapters behind typed interfaces
│       └── cython_ext/       # measured Cython kernels behind Python APIs
├── tests/
├── benchmarks/
├── scripts/
├── migrations/
├── configs/
└── docs/
```

## Module responsibilities

`contracts` owns public API DTOs, event envelopes, stable error codes, command
IDs, correlation IDs, and Arrow schema identifiers. It must not import domain
implementations, repositories, UI, workers, or estimator libraries.

`client` owns the supported Python API client, HTTP/WebSocket behavior,
reconnect, reconciliation helpers, Arrow decoding into client-owned values, and
explicit close behavior.

`coordinator` owns FastAPI routes under `/api/v1/`, `/api/v1/events`, health,
scheduling, CPU leases, durable state transitions, repository coordination,
artifact promotion, and worker launch.

`worker` owns the length-prefixed worker protocol runtime, heartbeat/progress
events, job-local mutable state, checkpoint coordination, cancellation, and
bounded library pools.

`frontend` owns Raylib/Raygui UI state, docking, widgets, effects, linked
contexts, chart/table rendering, generation tokens, and immutable render-ready
buffers.

Business modules such as `data`, `features`, `training`, `models`,
`evaluation`, `registry`, and `inference` own validated domain behavior. They
must keep persistence, UI, and third-party library details behind interfaces or
adapters.

`persistence`, `platform`, and `cython_ext` are adapter-heavy modules. They are
allowed to import infrastructure libraries and native bindings, but their
public surface remains typed Python.

## Route design pattern

Every coordinator API route follows this shape:

```text
HTTP request
  -> request DTO validation
  -> command/query object
  -> application service
  -> domain module public API
  -> repository/adapter through interface
  -> typed DTO or Arrow response
  -> event invalidation/progress when applicable
```

Route handlers should be thin. They own protocol concerns: path, method,
headers, correlation ID, command ID, DTO validation, error mapping, status code,
and response encoding. They should not contain domain algorithms, SQL, file
promotion logic, estimator calls, or UI assumptions.

Mutating routes require command IDs so retries cannot duplicate accepted work.
Requests and events carry correlation IDs. Errors include a stable machine code,
human message, optional field path, correlation ID, and retryability.

Events are hints, not state authority. Clients reconcile through REST after
startup, reconnect, sequence gaps, or unknown event types.

## SOLID guidance

Use SOLID as coupling control, not as a reason to create excessive files,
interfaces, inheritance, or abstractions.

- Single responsibility: separate concerns that change for different reasons,
  such as import validation, feature calculation, training orchestration,
  persistence, route handling, and UI rendering. Do not make every function tiny
  just to satisfy the label.
- Open/closed: create extension points for real variation, such as estimator
  adapters, feature implementations, repositories, and artifact codecs. Do not
  create speculative extension points.
- Liskov substitution: implementations of a shared protocol must preserve input,
  output, error, cancellation, determinism, ownership, and side-effect
  contracts.
- Interface segregation: consumers should depend on focused operations they
  actually use. Do not split interfaces until the design becomes harder to use
  than the dependency it removes.
- Dependency inversion: application and domain logic depend on stable internal
  interfaces. Concrete SQLite, Arrow, Raylib, scikit-learn, PyTorch, filesystem,
  IPC, and Cython implementations are assembled at composition roots.

## Pattern guidance

### Strategy

Use Strategy when multiple interchangeable algorithms implement the same
behavior: feature kernels, split policies, estimator adapters, metric
calculators, artifact codecs, or scoring modes.

Prefer a direct function when there is one implementation and no real source of
variation. The risk is creating protocols and registries before the second
implementation exists.

### Factory

Use Factory when object creation depends on configuration, dependency versions,
process role, runtime capability, registered implementation, or environment.
Examples include estimator adapter creation, repository selection, artifact
codec selection, and Cython fallback selection.

Prefer constructors for simple objects. The risk is hiding straightforward
dependencies behind opaque creation logic.

### Builder

Use Builder when constructing a complex object requires staged configuration,
many optional parts, cross-field validation, or immutable finalization. Examples
include experiment definitions, model specs, materialization manifests, and UI
layout revisions.

Prefer dataclasses or Pydantic/dataclass DTO validation for simple records. The
risk is replacing clear initialization with ceremony.

### Adapter

Use Adapter to translate third-party or native systems into stable Corthena
interfaces. Required adapter boundaries include Raylib/Raygui, SQLite, Arrow,
Parquet, pandas, scikit-learn, PyTorch, Cython compiled modules, filesystem
atomics, and worker IPC.

Prefer direct first-party code when no external boundary exists. The risk is
wrapping every internal object and obscuring the real data flow.

### Repository

Use Repository for durable persistence and retrieval: dataset catalog metadata,
jobs, runs, metrics, artifacts, aliases, layouts, and migrations.

Do not create repositories for temporary in-memory values, pure calculations,
render buffers, or simple DTO transformations. The risk is turning normal data
flow into unnecessary persistence-style indirection.

### Dependency Injection

Use explicit constructor or function injection. Composition roots assemble
process-specific dependencies for the UI, coordinator, worker, CLI, and tests.

Do not introduce a dependency-injection framework unless the technology stack is
revised. The risk is making dependency flow harder to inspect and test.

## Cython boundary

Most code stays typed Python. Cython is allowed only for measured CPU-bound hot
paths or approved native adapters.

```text
cython_ext/
├── __init__.py
├── rolling/
│   ├── __init__.py
│   ├── api.py
│   ├── _rolling.pyx
│   ├── _rolling.pxd
│   └── _rolling.pyi
├── indicators/
├── aggregation/
└── backtest/
```

Rules:

- Keep orchestration and business logic in Python.
- Hide compiled implementations behind stable Python-facing APIs such as
  `api.py`.
- Use underscore-prefixed names for internal compiled modules.
- Keep related `.pyx`, `.pxd`, and `.pyi` files together.
- Business modules do not import internal compiled modules directly.
- Keep Python-to-Cython crossings coarse-grained where practical.
- Provide pure-Python fallback or documented failure behavior where appropriate.
- Add parity tests and benchmarks before accepting Cython as the implementation.

## Determinism, ownership, and quality

Every process, thread, queue, async task, stream, and library pool has an owner,
cancellation path, bounded shutdown path, and documented sender/receiver/closer
where applicable.

Do not let results depend on dict iteration, wall clock, PID, scheduling,
process order, or library completion order. Use stable seed derivation and apply
reductions in logical order.

Prevent future-data leakage in imports, feature and target construction, split
generation, purge/embargo, evaluation, refit, inference, and reference
backtests.

Once scaffolding exists, relevant changes must pass the quality gates owned by
the specs: `ruff format --check`, `ruff check`, `pyright`, `pytest`,
`hypothesis` where applicable, `pytest-benchmark` for hot paths, vulnerability
scanning, Windows Cython build/import checks, lifecycle/concurrency tests, and
golden-image checks for visual changes.
