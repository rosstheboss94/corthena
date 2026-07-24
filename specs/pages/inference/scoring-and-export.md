# Inference Scoring and Export

**Status:** Authoritative
**Owner:** Inference
**Last updated:** 2026-07-23
**Related:** [Models artifacts and registry](../models/artifacts-and-registry.md), [Data datasets](../data/datasets.md), [Inference workspace](workspace.md)

Models owns final refit artifacts, aliases, and promotion. Inference consumes
the selected immutable artifact and applies the compatibility gates below.

Score a selected historical range or latest imported snapshot. Validate model
schema, engine, feature descriptors/fingerprints, feature schema, target,
dataset compatibility, and required lookback before scoring. Persist each
prediction with symbol, timestamp, model ID, run ID, data fingerprint, feature
fingerprints, and score. Immutable loaded models predict concurrently into
caller-owned ranges; cancellation discards incomplete temporary output and
publishes no incomplete predictions.

Ranked outputs use stable symbol identifiers for ties. Prediction history is
immutable and export preparation is generation-bound, cancellable, and typed;
stale or failed generations publish neither predictions nor export-ready state.

`SplitSpec` and `PortfolioSpec` are validated Python value objects or
DTO-backed domain types. Results use typed domain values and explicit API DTO
conversion, never unstructured maps.
