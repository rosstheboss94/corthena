# Data and Feature Specification

**Status:** Authoritative  
**Owner:** Data  
**Last updated:** 2026-07-04  
**Related:** [Technology stack](technology-stack.md), [Evaluation](evaluation-and-inference.md), [API](api.md)

## Technology constraints

Use first-party Go kernels over typed slices for computation, Apache Arrow Go for CSV/Parquet/Arrow data, typed `database/sql` repositories for catalog metadata, and standard-library paths, hashing, and atomic writes. Do not introduce a dataframe layer, external numerical runtime, ORM, or dynamically loaded feature language.

## Canonical bars

Import long-form CSV or Parquet with:

- `timestamp`
- `symbol`
- adjusted `open`, `high`, `low`, `close`
- adjusted `volume`

Normalize timestamps to UTC and require declared adjustment metadata. Validate unique `(symbol, timestamp)` keys, chronological order, finite prices, nonnegative volume, and OHLC relationships.

Imports support atomic append, rejecting keys already present, and explicit atomic replacement of a selected symbol/date range for corrections.

The catalog is mutable. Every run records the catalog revision and content fingerprint. Active and paused runs retain their exact materialized files. Completed runs remain auditable but may not be reproducible after catalog corrections and cache eviction.

## Built-in features

Built-in features include lagged returns, rolling price and volume statistics, price/volume ratios, rolling volatility and range measures, and per-timestamp cross-sectional ranks and z-scores.

Learned transforms and histogram bins fit only on training observations. Cross-sectional transforms operate only on symbols present at the timestamp. Missing feature values remain represented as IEEE-754 NaN values for learned tree routing.

## Compiled feature registry

Custom features are compiled Go implementations, not runtime plugins. A feature implementation satisfies a typed interface equivalent to:

```go
type Feature interface {
	Descriptor() FeatureDescriptor
	Compute(context.Context, FeatureInput, FeatureOutput) error
}
```

`FeatureDescriptor` declares a stable name, semantic version, lookback, output columns and dtypes, configuration schema version, and implementation fingerprint. `FeatureInput` exposes immutable typed views. `FeatureOutput` exposes only the task's exclusive output range.

- Registration occurs explicitly during command construction; package `init` side effects are prohibited.
- Duplicate `(name, version)` registrations fail startup.
- Experiment definitions reference registry name, version, and validated typed configuration.
- Runs record feature descriptors, configuration hashes, application build revision, and implementation fingerprints.
- Changing feature behavior requires a new feature version or engine version.
- V1 feature implementations are compiled, explicitly registered Go code; runtime-loaded plugins, subprocess feature scripts, and runtime source compilation are not supported.

## Targets and timing

- Configure an N-bar forward simple or log return.
- Features use information through bar `t` close.
- Reference execution occurs at bar `t+1` open.
- The default target measures the configured forward open-to-open horizon.
- Rows without a valid future target are excluded.

## Materialization

- Produce disk-backed little-endian `float32` feature matrices and `float64` target vectors.
- Store symbol, timestamp, split, and row identifiers in Parquet.
- Use stable row ordering defined by timestamp, symbol, and stable source row ID.
- Describe each matrix with a versioned JSON manifest containing dtype, dimensions, byte order, row ordering, file size, checksum, data fingerprint, feature/target configuration, and engine version.
- Partition computation into deterministic chunks with exclusive output ranges.
- Flush and close files before checksum and atomic promotion.
- Reopen promoted input mappings read-only before concurrent work.
- Key caches by data fingerprint, feature configuration, target configuration, implementation fingerprints, and engine version.

Materialization readers reject overflowed dimensions, incorrect file sizes, unsupported byte order or dtype, checksum failures, and incompatible schema or engine versions before mapping data.

## Public domain types

`DatasetQuery`, `FeatureSpec`, `FeatureDescriptor`, and `TargetSpec` are validated Go value structs. Execution receives immutable copies. API DTOs are separate `contract` structs and convert explicitly to domain values.
