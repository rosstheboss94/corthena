# Coordinator API Specification

**Status:** Authoritative  
**Owner:** Platform  
**Last updated:** 2026-07-04  
**Related:** [Technology stack](technology-stack.md), [System architecture](system-architecture.md), [Frontend foundation](frontend/foundation.md)

## Technology constraints

Use `net/http` and `http.ServeMux` for the loopback service and clients, typed Go structs plus explicit validation for JSON DTOs, the approved WebSocket package for events, Apache Arrow Go for Arrow IPC, and typed `database/sql` repositories. Do not add a web framework, ORM, external message broker, or dependency-injection framework.

## Boundary and versioning

Expose a versioned service bound only to IPv4 and IPv6 loopback. It coordinates the UI, CLI, supported Go client package, scheduler, and durable metadata. There is no user authentication in the single-user local release.

- JSON endpoints live below `/api/v1/`.
- The event stream is `/api/v1/events`.
- Dense responses use Arrow IPC with an explicit schema-version header.
- Every request accepts or receives a correlation ID.
- Unsupported API, schema, engine, and worker-protocol versions fail closed.
- Mutating requests use command IDs so safe retries cannot duplicate accepted work.

## Resources and commands

Provide typed endpoints for dataset imports and catalog queries; experiment and sweep submission; job controls; runs, metrics, reports, and comparisons; model artifacts and aliases; historical and latest-snapshot inference; and runtime health.

Dense numerical and chart responses use Arrow IPC streams. Commands and metadata use validated JSON DTOs. Collections are cursor-paginated and filterable with documented stable sort keys.

## Events and reconciliation

Provide a WebSocket event feed for job state and progress, worker heartbeats and resources, component status, checkpoints, logs and failures, and catalog/cache invalidation.

Every event contains an event ID, event type, schema version, timestamp, correlation ID when applicable, and a typed payload. Clients treat events as invalidation and progress hints, then reconcile through REST after startup, reconnect, sequence gaps, or unknown event types. A bounded backoff with jitter governs reconnect attempts.

## Supported Go client

The public `contract` package owns API DTOs, enums, validation errors, and Arrow schema identifiers. The public `client` package owns a context-aware HTTP client, typed resource and command methods, Arrow stream decoding into client-owned typed values, WebSocket subscription and reconnect, and explicit `Close` behavior.

Domain packages do not import `client`. Public packages do not expose database rows, Arrow builders, Raylib values, native pointers, or internal repository types.

## Worker protocol

Coordinator-launched workers communicate over inherited anonymous pipes using length-prefixed JSON envelopes. Each envelope contains protocol version, job ID, sequence number, message kind, and a typed payload. Large data moves through checksummed job-scoped files referenced by manifest rather than through the pipe.

- The coordinator sends start, pause, resume, cancel, lease-update, and shutdown commands.
- The worker sends ready, heartbeat, progress, metric, checkpoint, artifact, log, failure, and stopped events.
- Exactly one writer goroutine owns each pipe.
- Sequence gaps, malformed frames, wrong job IDs, oversized frames, or protocol mismatches terminate the session and mark the job interrupted or failed according to persisted state.
- Workers receive a one-time capability token at launch and echo it in the initial handshake.

## Runtime health

Health responses include service and protocol versions; Go version and build data; `GOOS`, `GOARCH`, role, PID, and uptime; cgo and Raylib/Raygui status where relevant; module/native versions; goroutine count, `GOMAXPROCS`, process/task counts, and CPU leases; storage availability; and named degradation reasons.

## Type and error rules

- DTO structs are distinct from domain models and have explicit JSON field names.
- Every inbound DTO has an explicit validator; conversion to domain values occurs only after validation.
- Errors contain stable machine code, human message, optional field path, correlation ID, and retryability.
- Unknown JSON fields and incompatible schema versions are rejected at durable and command boundaries.
- Timestamps use UTC RFC 3339 with nanosecond precision in JSON.
- Identifiers are opaque strings with domain-specific Go types.
