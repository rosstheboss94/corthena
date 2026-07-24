# Data API Resources

**Status:** Authoritative  
**Owner:** Data  
**Last updated:** 2026-07-23  
**Related:** [General API](../../general/api.md), [Ingestion](ingestion.md), [Workspace](workspace.md)

The [general API](../../general/api.md) owns transport versioning, command and
correlation IDs, common events, pagination, DTO validation, client behavior,
worker protocol, health, and error envelopes. This document owns Data resource
semantics and Data-specific error mapping.

## Resources

- Catalog and dataset/revision queries are cursor-paginated with stable sort
  keys and immutable revision fingerprints.
- `POST /api/v1/data/files/preview` accepts a coordinator-readable path and
  explicit row/byte bounds; it returns bounded typed detection, mapping, and
  diagnostics only.
- `POST /api/v1/data/imports` accepts a validated `ImportPlan`; import status
  and cancellation use `/api/v1/data/imports/{import_id}` and `/cancel`.
- Massive discovery and pulls use
  `/api/v1/data/providers/massive/symbols` and `/pulls`.
- Schedules use `GET/POST /api/v1/data/schedules`, revisioned
  `GET/PATCH/DELETE /api/v1/data/schedules/{schedule_id}`, and manual `/run`.
- Credential status, save/replace, test, and delete use
  `/api/v1/settings/api-tokens/massive`.

## DTO and secret rules

Typed DTOs cover provider identity, credential status, preview/mapping,
`ImportPlan`, pull, schedule, progress, provenance, catalog revision, source
snapshot, dataset version, and reconciliation. Mutations carry command and
correlation IDs plus the expected catalog or schedule revision where relevant.
Credential status exposes presence and safe test metadata only. Token values
are accepted only by save/replace and test request bodies, never returned,
serialized, logged, captured, or included in events, errors, URLs, provider
fixtures, or replay state. A test may use transient supplied text or the saved
credential but cannot echo or distinguish token contents.

## Errors and reconciliation

Data maps validation, stale revision, cancellation, authentication,
entitlement, rate-limit, provider, filesystem, and atomic-publication failures
to stable error codes. Rate-limit responses may include a bounded retry time and
provider request ID, but never credentials, Authorization values, secret query
parameters, or raw provider bodies. Provider payloads are validated inside the
connector and converted to Corthena values; raw pages are not proxied or
archived. Events are progress/invalidation hints and clients reconcile through
REST after reconnect, sequence gaps, unknown events, or startup.

