# Jobs Workspace

Default panels are the virtualized job queue, selected-job stage/progress,
live metrics, worker/CPU resources, process/checkpoint status, and logs.

Pause, resume, and cancel are enabled only from typed coordinator lifecycle
states; interrupted jobs require explicit resume. Stable job identities
synchronize ordered stages, metrics, leases, health, durable checkpoints, and
logs without exposing mutable simulator state. Commands carry correlation,
command, and generation identities. The demo covers successful immutable
completion, pause/resume at durable boundaries, cooperative cancellation,
interruption requiring resume, and fail-closed checkpoint incompatibility.

Snapshot loads and lifecycle commands use typed `UIClientProtocol` operations.
Commands are idempotent by command identity, generation-bound, cancellable, and
published only after the simulator or future coordinator adapter returns a
validated result.

**Status:** Authoritative
**Owner:** Runtime
**Last updated:** 2026-07-23
**Related:** [jobs page index](README.md)
