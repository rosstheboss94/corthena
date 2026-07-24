# Post-Migration Roadmap

**Status:** Authoritative
**Owner:** Project
**Last updated:** 2026-07-18

The Python/Cython repository-root migration and simulator-backed Raylib
workstation are complete. Retained phase routes and migration records describe
that accepted work but no longer define future delivery order.

Real Data Ingestion is split into two sequential milestones. Milestone 1a makes
the complete workflow visible and reviewable through the deterministic
simulator. Milestone 1b implements the real coordinator-backed behavior behind
the accepted UI contract. Research, Training, Results, Models, and Inference
backend milestones remain intentionally unspecified until Milestone 1b is
accepted and the project is replanned.

The combined scope is historical US-equity OHLCV from manual CSV/Parquet
imports and explicit-symbol Massive pulls. Massive is the only shipped provider,
but a typed connector contract must permit later adapters without leaking
provider values into domain or public contracts.

The combined scope excludes live streaming, trades, quotes, fundamentals,
options, crypto, corporate-action calculation, unrestricted full-market
downloads, and automatic selection of every ticker.

## Milestone 1a: Data Ingestion UI

Build a fully interactive, simulator-backed preview of the complete Data
ingestion experience. Milestone 1a establishes the UI behavior and typed
`UIClientProtocol` boundary that Milestone 1b must preserve.

### Workflows

- Add New Dataset, Import File, Pull from Massive, and Schedules flows while
  retaining Catalog, Coverage, Import Queue, Dataset Inspector, and Import Logs.
- Add Settings -> API Tokens with simulated save, replace, test, and delete
  actions for Massive. Show a fixed saved/not-saved status, safe test metadata,
  and the explicit plaintext-storage warning planned for Milestone 1b.
- Provide a simulated asynchronous file browser and import wizard for CSV and
  Parquet. Show automatic column detection and editable mapping for timestamp,
  symbol, OHLCV, source timezone, interval, session policy, and adjustment
  policy.
- Let users preview dataset creation, append, and explicit symbol/range
  replacement; review validation summaries; confirm submissions; observe
  progress; and cancel work.
- Provide simulated searchable US-stock discovery, explicit symbol selection,
  requested range, 1m/5m/15m/1h/1d interval, regular/all-session selection,
  raw/provider-split-adjusted selection, and create/append/range-replace modes.
- Provide simulated schedule creation, editing, enable/disable, manual run, and
  deletion for hourly and daily schedules. Explain that real schedules will run
  only while the coordinator is open and will coalesce missed executions at
  startup.

### Simulator and UI contract

- Extend typed UI state, actions, effects, render-neutral models, and
  `UIClientProtocol` operations for credential status/actions, file preview and
  import, symbol discovery, Massive pulls, schedule CRUD/manual run,
  cancellation, progress, and reconciliation.
- Use deterministic fixtures, injected clocks, stable identities, bounded
  effects, and immutable results. Identical actions, seeds, clocks, and resource
  configurations must replay to identical state and screenshots.
- Simulate loading, preview, ready, confirming, queued, running, cancelling,
  cancelled, validating, validation failure, authentication failure,
  entitlement failure, rate limiting, general failure, reconnecting,
  reconciling, recovery, empty results, and completion.
- Preserve generation safety, stable selection, nonblocking render-thread
  behavior, bounded per-frame draining, cancellation, saturation handling, and
  stale-result rejection.
- Keep transient token entry outside published `AppState`, snapshots, replay
  serialization, logs, URLs, manifests, captures, tooltips, errors, and
  clipboard history. The simulator consumes and discards submitted token text;
  it never persists or returns it.

### Milestone 1a exclusions

Milestone 1a performs no real source-file reads, PyArrow parsing, Massive
requests, credential persistence or permission changes, coordinator/API calls,
SQLite or Parquet writes, catalog publication, calendar evaluation, background
schedule execution, provider retries, or startup catch-up. Simulated catalog
revisions, provider request IDs, validation results, progress, and recovery are
fixtures only and do not claim durable ingestion.

### Milestone 1a acceptance

Milestone 1a is complete only when:

- every workflow above is navigable and interactive through the simulator,
  including success, validation, authentication, entitlement, rate-limit,
  cancellation, failure, reconciliation, and recovery paths;
- interaction tests cover forms, mappings, confirmations, schedule mutations,
  token-buffer clearing, stable identities, stale generations, saturation, and
  cancellation;
- deterministic replay and varied completion-order tests produce identical
  accepted state;
- lifecycle tests prove bounded effects, nonblocking Raylib ownership, clean
  cancellation, and leak-free repeated startup/shutdown;
- canonical visual scenarios cover API-token settings, file mapping, Massive
  pull, schedule editing, active progress, validation error, authentication
  error, and recovered/reconciled state at 1280x720 and 1920x1080 at 100%, 150%,
  and 200% scale with token-free fixtures; and
- `ruff format --check`, `ruff check`, `pyright`, the applicable pytest/property,
  replay, lifecycle, Windows, visual, and finding-free review gates pass.

Passing Milestone 1a proves the ingestion experience and backend-swappable UI
contract, not real data ingestion.

## Milestone 1b: Real Data Ingestion Logic

Implement real coordinator-backed behavior behind the accepted Milestone 1a
workflows without changing their interaction model or visual semantics.

### Coordinator and persistence

- Build the minimum coordinator, versioned loopback Data API, application-data
  paths, typed SQLite repositories, and coordinator-backed `UIClientProtocol`
  adapter required for durable imports. The UI never reads source files, calls
  Massive, or writes credentials, the catalog, SQLite, Parquet, or manifests
  directly.
- Persist catalog, import, progress, and schedule metadata in SQLite WAL. Store
  canonical bars in partitioned Parquet revision directories promoted
  atomically only after complete validation.
- Publish one catalog revision and content fingerprint per successful import or
  scheduled correction. Failed, cancelled, stale, unauthorized, rate-limited,
  or malformed work leaves the prior revision unchanged.
- Store an immutable provenance manifest with source checksum or Massive
  request identity, mapping, symbols, interval, session and adjustment policy,
  requested range, timestamps, application/schema versions, partition
  checksums, and content fingerprint. Do not archive provider response pages or
  duplicate source files.

### Massive credentials

- Replace simulated credential actions with versioned loopback save/replace,
  test, status, and delete commands. Secrets are accepted only by save/test
  commands and are absent from returned DTOs.
- Store the token in a separate versioned plaintext application-data document
  with the restrictive Windows permissions defined by the owning architecture
  and persistence specifications. Fail closed if permissions cannot be created
  or verified.
- Never include the token in snapshots, events, logs, URLs, screenshots,
  replay serialization, errors, manifests, provider fixtures, or API responses.
  Quarantine corrupt credential documents without logging or returning their
  contents.

### File ingestion

- Connect the first-party asynchronous browser and wizard to bounded
  coordinator preview/import operations. The coordinator, not the UI, opens the
  selected CSV or Parquet file.
- Parse CSV and Parquet through PyArrow on bounded workers. Normalize UTC
  timestamps and canonical OHLCV types; validate key uniqueness, ordering,
  finite prices, nonnegative volume, OHLC relationships, interval consistency,
  session membership where required, and declared adjustment policy.
- Support dataset creation, append, and explicit symbol/range replacement
  against an expected catalog revision. Complete validation before mutation and
  publish at most one new revision.

### Massive connector

- Implement `MarketDataConnectorProtocol` with Massive as the only shipped
  adapter. Use existing `httpx` directly; do not add a Massive SDK.
- Support Bearer-authenticated connection tests, paginated searchable US-stock
  discovery, paginated custom aggregate pulls, request IDs, cancellation,
  authentication and entitlement errors, `Retry-After`, bounded retries, and
  empty or partial pages.
- Map 1m, 5m, 15m, 1h, and 1d to Massive custom bars and respect the provider's
  50,000-base-aggregate maximum.
- Represent raw and provider split-adjusted data as distinct policies. Never
  describe Massive data as dividend-adjusted.
- Follow the official [Custom Bars API](https://massive.com/docs/rest/stocks/aggregates/custom-bars),
  [REST authentication quickstart](https://massive.com/docs/rest/quickstart),
  and [ticker discovery endpoint](https://massive.com/docs/rest/stocks/tickers/all-tickers).

### Sessions and schedules

- Support regular-session and all-session imports. Put `exchange_calendars`
  behind a typed adapter and admit it only after the exact CPython 3.14.2
  Windows compatibility gate. Milestone completion is blocked if it cannot
  correctly provide US-equity holidays and early closes.
- Replace simulated schedule operations with durable manual, hourly, and daily
  execution while the coordinator is open. Startup coalesces missed executions
  into one bounded catch-up range per schedule.
- Exclude incomplete bars, overlap the last completed bar for deterministic
  correction, deduplicate by `(symbol, timestamp)`, and atomically publish a
  replacement revision. Queue capacity, catch-up ranges, retry limits,
  cancellation boundaries, and shutdown are bounded.

### API and reconciliation

- Add typed public provider, credential-status, file-preview/mapping,
  import-plan, pull, schedule, progress, provenance, and catalog-revision DTOs.
- Expose the versioned Data and credential endpoints defined by the owning API
  specification through thin handlers. Mutations carry command and correlation
  IDs plus expected catalog/schedule revisions where applicable.
- Treat events as progress and invalidation hints. Reconcile through REST after
  startup, reconnect, sequence gaps, or unknown event types.
- Preserve the Milestone 1a UI contract's generation, ordering, cancellation,
  validation, immutable publication, and visible state behavior when replacing
  simulator results with coordinator responses.

### Milestone 1b acceptance

Milestone 1b is complete only when:

- all Milestone 1a interaction, deterministic replay, lifecycle, responsive,
  and canonical visual tests remain passing against the coordinator-backed
  client where applicable;
- CSV/Parquet tests cover bounded previews, arbitrary mappings, timezone
  conversion, malformed schemas, duplicates, invalid OHLCV, create/append/range
  replacement, cancellation, stale revisions, and atomic rollback;
- Massive tests use recorded typed fixtures rather than paid network calls and
  cover Bearer authentication, ticker/aggregate pagination, empty intervals,
  partial pages, rate limits, entitlements, retries, cancellation,
  raw/split-adjusted policies, and provider request IDs;
- calendar tests cover normal sessions, weekends, holidays, daylight-saving
  transitions, and early closes for every supported interval;
- scheduler tests cover hourly/daily/manual execution, restart catch-up,
  missed-run coalescing, incomplete-bar exclusion, overlap correction,
  idempotency, queue bounds, cancellation, and shutdown;
- security tests prove tokens never enter logs, snapshots, serialization,
  errors, URLs, manifests, captures, events, fixtures, or API responses, and
  corrupt credential documents are quarantined without content disclosure;
- integration tests exercise UI client -> coordinator -> SQLite/canonical
  Parquet through restart, reconciliation, event sequence gaps, varied
  completion order, cancellation, and injected failures while retaining the
  last published revision; and
- `ruff format --check`, `ruff check`, `pyright`, the full pytest/property and
  lifecycle suites, vulnerability audit, exact-CPython Windows build and
  dependency compatibility gates, deterministic replay, leak checks, and a
  finding-free final review all pass.

Passing UI-only or simulator-only behavior does not complete Milestone 1b.
Later backend milestones are replanned only after this acceptance gate passes.
