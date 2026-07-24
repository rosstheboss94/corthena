# ADR 0003: Mutable Catalog with Run Pinning

**Status:** Accepted
**Date:** 2026-07-04

## Context

Users need to append and correct local market data without retaining a complete immutable copy of every catalog state.

## Decision

Keep the logical catalog mutable. Record revision/content fingerprints for every run and retain exact materialized arrays while a run is active or paused.

## Alternatives

- Immutable dataset snapshots for every change.
- Unversioned mutable data.
- Optional freezing only for selected runs.

## Consequences

Paused jobs resume exactly. Completed results remain auditable, but exact reproduction may become impossible after corrections and cache eviction. The UI and reports must disclose this limitation.

## Affected specifications

- [Data and features](../pages/data/ingestion.md)
- [System architecture](../general/system-architecture.md)
