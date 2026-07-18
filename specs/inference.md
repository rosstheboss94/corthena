# Inference and Registry

After walk-forward evaluation, refit the selected configuration through a
recorded cutoff; the final model is the inference artifact. Runs and artifacts
are immutable. Human aliases such as `candidate` and `champion` point to final
models; promotion is manual, coordinator-owned, transactional, and recorded in
immutable history.

Score a selected historical range or latest imported snapshot. Validate model
schema, engine, feature descriptors/fingerprints, feature schema, target,
dataset compatibility, and required lookback before scoring. Persist each
prediction with symbol, timestamp, model ID, run ID, data fingerprint, feature
fingerprints, and score. Immutable loaded models predict concurrently into
caller-owned ranges; cancellation discards incomplete temporary output and
only complete checksummed artifacts are indexed.

`SplitSpec` and `PortfolioSpec` are validated Python value objects or
DTO-backed domain types. Results use typed domain values and explicit API DTO
conversion, never unstructured maps.
