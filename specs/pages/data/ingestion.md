# Data and Feature Specification

**Status:** Authoritative
**Owner:** Data
**Last updated:** 2026-07-18
**Related:** [Technology stack](../../general/technology-stack.md), [System architecture](../../general/system-architecture.md), [Evaluation](../results/README.md), [Data API](api.md), [Data workspace](workspace.md)

## Technology constraints

Use typed Python and approved NumPy/Cython kernels for computation, PyArrow for
CSV/Parquet/Arrow data, typed SQLite repositories for catalog/import/schedule
metadata, `httpx` for Massive, and standard-library paths, hashing, and atomic
writes. Pandas may be used behind adapters for tabular convenience but is not a
public contract type. Do not add an ORM, Massive SDK, or dynamically loaded
unvalidated feature language.

`exchange_calendars` may be used only through a typed first-party calendar
adapter after it passes the exact regular CPython 3.14.2 Windows dependency
gate. Milestone completion is blocked unless the admitted adapter correctly
provides US-equity sessions, holidays, daylight-saving behavior, and early
closes.

## Canonical bars and policies

Canonical long-form bars contain:

- UTC `timestamp`;
- normalized `symbol`;
- `open`, `high`, `low`, and `close` as canonical finite price values; and
- nonnegative canonical `volume`.

Supported intervals are 1m, 5m, 15m, 1h, and 1d. Every dataset declares one
session policy (`regular` or `all`) and one adjustment policy (`raw` or
`provider_split_adjusted`). These policies are part of dataset identity and
provenance; imports with incompatible policies cannot silently append.
Corthena does not calculate corporate actions. Massive data is never described
as dividend-adjusted.

Normalize input timestamps to UTC using the declared source timezone. Validate
unique `(symbol, timestamp)` keys, chronological ordering, finite prices,
nonnegative volume, `low <= open/close <= high`, interval consistency, session
membership where required, and declared adjustment metadata before publication.

## Import plans and file ingestion

Typed file-preview requests name a CSV or Parquet path and a strict row/byte
preview bound. PyArrow parsing and preview run on bounded coordinator-owned
workers. Preview results contain typed detected columns, a bounded sample,
parse diagnostics, and mapping suggestions; they never expose Arrow builders or
unbounded source content.

An `ImportPlan` declares source identity, timestamp/symbol/OHLCV mapping,
timezone, symbols, interval, session and adjustment policy, destination dataset,
mode, optional replacement bounds, expected catalog revision, command ID, and
correlation ID. Modes are:

- create a new logical dataset;
- append keys not present in the current revision; or
- replace an explicit symbol and UTC range for corrections.

Validation completes before mutation. Rejected duplicate, malformed,
out-of-range, incompatible-policy, cancelled, or stale-revision input records a
typed import outcome without changing the current revision.

## Market-data connector

`MarketDataConnectorProtocol` is the capability contract for provider
connection tests, symbol discovery, and historical pulls. Request/result models
use Corthena provider, symbol, interval, adjustment, pagination, progress,
cancellation, and error types. Native provider payloads and HTTP objects remain
inside concrete adapters.

Massive is the only shipped implementation. It uses `httpx` directly with the
saved token in an Authorization header, never in a URL. It supports:

- authenticated connection tests and typed authentication/entitlement errors;
- paginated, searchable US-stock discovery with explicit user selection;
- paginated custom aggregate pulls and the provider's
  50,000-base-aggregate maximum;
- interval mapping for 1m, 5m, 15m, 1h, and 1d;
- raw and provider split-adjusted requests as distinct policies;
- provider request IDs, empty intervals, and partial pages;
- cooperative cancellation between bounded network/page/parse operations; and
- `Retry-After` plus a bounded retry/backoff policy for retryable failures.

Do not archive raw provider pages. Persist the request identity and validated
canonical result provenance only.

## Session calendar and completed bars

The typed calendar adapter returns session opens, closes, holidays, and early
closes in UTC for supported US-equity sessions. Regular-session validation and
bar completion use this adapter. All-session imports preserve provider bars
outside the regular session but still use declared interval and completion
rules.

An import cutoff excludes every bar whose interval has not completed at the
injected clock time. Scheduled corrections overlap the last completed bar,
deduplicate deterministically by `(symbol, timestamp)`, and replace the affected
range atomically so late provider corrections are repeatable.

## Catalog revisions and provenance

The catalog is mutable, but each published revision is immutable. Canonical
bars are partitioned in a temporary sibling revision directory, validated,
flushed, closed, fingerprinted, and atomically promoted before one SQLite
transaction makes that revision current. Partial directories are never
published. Failed, cancelled, stale, unauthorized, rate-limited, or malformed
work leaves the prior revision current and is reconciled at startup.

Every revision has an immutable, versioned provenance manifest containing:

- source kind and source checksum, or Massive request/provider identity;
- column mapping and source timezone when applicable;
- selected symbols, interval, session policy, and adjustment policy;
- requested UTC range and actual canonical coverage;
- command, correlation, import, dataset, and parent revision identities;
- import start/completion timestamps from the injected clock;
- application, schema, connector, and calendar-adapter versions; and
- partition checksums plus the canonical content fingerprint.

Do not duplicate source files beside canonical data. Every run records the
catalog revision and content fingerprint. Active and paused runs retain exact
materializations. Completed runs remain auditable but may not be reproducible
after catalog corrections and cache eviction.

## Schedules

Schedules support manual, hourly, and daily execution and persist in SQLite.
Each schedule declares dataset, symbols, interval, session and adjustment
policy, range anchor, enabled state, cadence, and optimistic revision. The
coordinator is the only scheduler owner; schedules run only while it is open.

At startup, missed executions coalesce into one bounded catch-up range per
schedule. Do not enqueue one task per missed occurrence. Schedule execution
excludes incomplete bars, overlaps the last completed bar, deduplicates, and
publishes at most one replacement revision. Queue capacity, catch-up bounds,
retry limits, cancellation boundaries, and shutdown deadlines are explicit in
the implementation contract and verified under saturation and restart.

## Built-in features

Built-in features include lagged returns, rolling price and volume statistics,
price/volume ratios, rolling volatility and range measures, and per-timestamp
cross-sectional ranks and z-scores.

Learned transforms and histogram bins fit only on training observations.
Cross-sectional transforms operate only on symbols present at the timestamp.
Missing feature values remain represented as IEEE-754 NaN values for learned
tree routing.

## Compiled feature registry

Custom features are typed Python implementations registered explicitly through
the approved extension model, with Cython only for measured hot paths. A feature
implementation satisfies a typed interface equivalent to:

```python
class Feature(Protocol):
    def descriptor(self) -> FeatureDescriptor: ...

    def compute(
        self,
        context: FeatureContext,
        input: FeatureInput,
        output: FeatureOutput,
    ) -> None: ...
```

`FeatureDescriptor` declares a stable name, semantic version, lookback, output
columns and dtypes, configuration schema version, and implementation
fingerprint. `FeatureInput` exposes immutable typed views. `FeatureOutput`
exposes only the task's exclusive output range.

- Registration occurs explicitly during command construction; package `init`
  side effects are prohibited.
- Duplicate `(name, version)` registrations fail startup.
- Experiment definitions reference registry name, version, and validated typed
  configuration.
- Runs record feature descriptors, configuration hashes, application build
  revision, and implementation fingerprints.
- Changing feature behavior requires a new feature version or engine version.
- V1 feature implementations are explicitly registered Python code; runtime
  source compilation, unvalidated plugins, and subprocess feature scripts are
  not supported.

## Targets and timing

- Configure an N-bar forward simple or log return.
- Features use information through bar `t` close.
- Reference execution occurs at bar `t+1` open.
- The default target measures the configured forward open-to-open horizon.
- Rows without a valid future target are excluded.

## Materialization

- Produce disk-backed little-endian `float32` feature matrices and `float64`
  target vectors.
- Store symbol, timestamp, split, and row identifiers in Parquet.
- Use stable row ordering defined by timestamp, symbol, and stable source row ID.
- Describe each matrix with a versioned JSON manifest containing dtype,
  dimensions, byte order, row ordering, file size, checksum, data fingerprint,
  feature/target configuration, and engine version.
- Partition computation into deterministic chunks with exclusive output ranges.
- Flush and close files before checksum and atomic promotion.
- Reopen promoted input mappings read-only before concurrent work.
- Key caches by data fingerprint, feature configuration, target configuration,
  implementation fingerprints, and engine version.

Materialization readers reject overflowed dimensions, incorrect file sizes,
unsupported byte order or dtype, checksum failures, and incompatible schema or
engine versions before mapping data.

## Public domain types

Provider identity, `CredentialStatus`, file preview and mapping, `ImportPlan`,
pull request/result, `Schedule`, progress, provenance, `CatalogRevision`,
`DatasetQuery`, `FeatureSpec`, `FeatureDescriptor`, and `TargetSpec` are
validated value objects or DTO-backed domain types. Execution receives
immutable copies. API DTOs are separate contract types and convert explicitly
to domain values. Secrets are not domain values and appear only in transient
inbound credential commands.

## Real-ingestion verification

- File coverage includes bounded CSV/Parquet preview, automatic and arbitrary
  mappings, timezone conversion, malformed schemas, duplicate keys, invalid
  OHLCV, interval/policy mismatch, create/append/range replacement,
  cancellation, stale revisions, and failure-injected atomic rollback.
- Massive adapter tests use recorded typed fixtures, never paid network calls.
  Cover Bearer headers, ticker and aggregate pagination, the 50,000-base-bar
  boundary, empty intervals, partial pages, `Retry-After`, exhausted retries,
  entitlements, cancellation, raw/split-adjusted policies, and request IDs.
- Calendar coverage includes normal sessions, weekends, holidays,
  daylight-saving transitions, and early closes for every supported interval.
- Scheduler coverage includes hourly/daily/manual execution, restart catch-up,
  missed-run coalescing, incomplete-bar exclusion, overlap correction,
  idempotency, queue saturation, cancellation, and bounded shutdown.
- Security coverage uses sentinel tokens and proves they never occur in logs,
  state snapshots, serialization, errors, URLs, manifests, captures, event or
  API bodies, or provider fixtures. Corrupt credential documents are
  quarantined without echoing content, and invalid Windows permissions fail
  closed.
- Integration coverage runs UI client -> loopback coordinator -> SQLite and
  canonical Parquet through restart, REST reconciliation, event sequence gaps,
  varied completion order, cancellation, and injected persistence/network
  failures while preserving the last published revision.
