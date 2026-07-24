# Coordinator API Specification

**Status:** Authoritative
**Owner:** Platform
**Last updated:** 2026-07-18
**Related:** [Technology stack](technology-stack.md), [System architecture](system-architecture.md), [UI foundation](ui/README.md)

## Technology Constraints

Use FastAPI for the loopback service, `httpx` for clients, typed Pydantic or
dataclass DTOs plus explicit validation, the approved WebSocket adapter for
events, PyArrow for Arrow IPC, and typed SQLite repositories. Do not add an
ORM, external message broker, distributed task queue, or dependency-injection
framework.

The Phase 0 loopback compatibility package is a narrow probe adapter, not the
coordinator API. It exposes the typed probe contract and the
`run_loopback_probe()` compatibility facade. It does not expose the internal
FastAPI application factory or allow framework objects across the adapter
boundary.

## Boundary and Versioning

Expose a versioned service bound only to IPv4 and IPv6 loopback. It coordinates
the UI, CLI, supported Python client package, scheduler, and durable metadata.
There is no user authentication in the single-user local release.

- JSON endpoints live below `/api/v1/`.
- The event stream is `/api/v1/events`.
- Dense responses use Arrow IPC with an explicit schema-version header.
- Every request accepts or receives a correlation ID.
- Unsupported API, schema, engine, dependency, and worker-protocol versions fail closed.
- Mutating requests use command IDs so safe retries cannot duplicate accepted work.

## Resources and Commands

Provide typed endpoints for dataset imports and catalog queries; experiment and
sweep submission; job controls; runs, metrics, reports, and comparisons; model
artifacts and aliases; historical and latest-snapshot inference; and runtime
health.

Dense numerical and chart responses use Arrow IPC streams. Commands and
metadata use validated JSON DTOs. Collections are cursor-paginated and
filterable with documented stable sort keys.

Page-specific resources live with their owning page. For example, Data routes,
DTO semantics, secret handling, provider errors, and catalog reconciliation are
specified in [Data API](../pages/data/api.md). The shared contract remains the
transport envelope, pagination, versioning, event, client, worker, health, and
common error rules defined below.

## Events and Reconciliation

Provide a WebSocket event feed for job and import state/progress, worker
heartbeats and resources, component status, checkpoints, logs and failures,
schedule execution, and catalog/cache invalidation.

Every event contains an event ID, event type, schema version, timestamp,
correlation ID when applicable, and a typed payload. Clients treat events as
invalidation and progress hints, then reconcile through REST after startup,
reconnect, sequence gaps, or unknown event types. A bounded backoff with jitter
governs reconnect attempts.

Data events may contain operation, dataset, schedule, generation, progress,
safe provider request, and catalog-revision identities. They never contain API
tokens, source preview rows, Authorization values, provider URLs carrying
credentials, or raw provider pages.

## Supported Python Client

The public `corthena.contracts` package owns API DTOs, enums, validation
errors, and Arrow schema identifiers. The public `corthena.client` package owns
a cancellable HTTP client, typed resource and command methods, Arrow stream
decoding into client-owned typed values, WebSocket subscription and reconnect,
and explicit close behavior.

Domain packages do not import `corthena.client`. Public packages do not expose
database rows, Arrow builders, Raylib values, native pointers, tensors, raw
library estimator objects, or internal repository types.

The UI-facing coordinator adapter implements the Data operations in
`UIClientProtocol`: credential management, file preview/import, symbol
discovery, Massive pull submission, schedule CRUD/manual run, cancellation,
progress, and reconciliation. It preserves command/correlation/generation
identities and converts public API DTOs into immutable UI-owned values.

## Worker Protocol

Coordinator-launched workers communicate over inherited anonymous pipes or an
approved local IPC channel using length-prefixed JSON envelopes. Each envelope
contains protocol version, job ID, sequence number, message kind, and a typed
payload. Large data moves through checksummed job-scoped files referenced by
manifest rather than through the pipe.

- The coordinator sends start, pause, resume, cancel, lease-update, and shutdown commands.
- The worker sends ready, heartbeat, progress, metric, checkpoint, artifact, log, failure, and stopped events.
- Exactly one writer owner serializes each pipe or stream.
- Sequence gaps, malformed frames, wrong job IDs, oversized frames, or protocol mismatches terminate the session and mark the job interrupted or failed according to persisted state.
- Workers receive a one-time capability token at launch and echo it in the initial handshake.

## Runtime Health

Health responses include service and protocol versions; Python version,
implementation, build data, OS, architecture, role, PID, and uptime; native
extension and Raylib/Raygui status where relevant; dependency/native versions;
thread/process/task counts, library pool limits, and CPU leases; storage
availability; `python_abi` and platform identity. Exact regular CPython 3.14.2
is healthy; a free-threaded or wrong-patch interpreter cannot start the service.

## Type and Error Rules

- DTOs are distinct from domain models and have explicit JSON field names.
- Every inbound DTO has an explicit validator; conversion to domain values occurs only after validation.
- Errors contain stable machine code, human message, optional field path, correlation ID, and retryability.
- Unknown JSON fields and incompatible schema versions are rejected at durable and command boundaries.
- Timestamps use UTC RFC 3339 with nanosecond precision in JSON.
- Identifiers are opaque strings with domain-specific typed wrappers or value objects.
- Secret-bearing inbound DTOs use dedicated types with redacted representations
  and cannot be serialized into events, snapshots, logs, replay state, error
  details, or response models.
