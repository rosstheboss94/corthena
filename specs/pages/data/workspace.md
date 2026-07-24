# Data Workspace

**Status:** Authoritative
**Owner:** UI
**Last updated:** 2026-07-18

The operational Data workspace retains Catalog, Coverage, Import Queue,
Dataset Inspector, and Import Logs and adds New Dataset, Import File, Pull from
Massive, and Schedules flows. Stable dataset, import, schedule, and provider
identities survive refresh and reconciliation.

## Boundary and state

The workspace loads immutable snapshots and submits typed commands through
`UIClientProtocol`. The UI never reads a selected data file, calls Massive,
writes credentials, or mutates SQLite, Parquet, manifests, or the catalog
directly. Background effects perform client calls; the Raylib thread only
reduces bounded results and renders typed state.

State distinguishes loading, preview, ready, confirming, queued, running,
cancelling, cancelled, validating, validation-failed, authentication-failed,
entitlement-failed, rate-limited, failed, reconnecting, reconciling, recovered,
and complete outcomes. Keep the last accepted catalog snapshot visible where
safe. Stale generations and catalog revisions cannot advance visible state.

## Massive API token settings

Settings -> API Tokens provides Massive save, replace, test, and delete
actions. Show whether a token exists and the last typed connection-test result,
but never display or return the saved token. Before save or replacement, warn
explicitly that the credential is stored in plaintext in a separate
access-restricted application-data file and is not encrypted.

Password entry is owned by a dedicated transient secret-entry buffer, not
published `AppState`, and is excluded from snapshots, events, replay
serialization, logs, captures, tooltips, errors, URLs, and clipboard history.
Close, cancel, successful submission, and terminal failure clear the buffer.
Save and test are the only commands that may carry the secret; delete uses
credential identity only.

## File import wizard

The first-party asynchronous file browser selects CSV or Parquet paths without
blocking the render thread. Selection submits a coordinator preview request;
only a bounded typed preview returns to the UI.

The wizard presents automatic detection and editable mapping for timestamp,
symbol, open, high, low, close, volume, source timezone, interval, session
policy, and adjustment policy. Users choose dataset creation, append, or
explicit symbol/range replacement, review validation counts and representative
typed failures, then confirm against the expected catalog revision.

## Massive pull wizard

The Massive flow provides paginated searchable US-stock discovery and explicit
symbol selection, requested UTC range, one of 1m/5m/15m/1h/1d, regular or all
sessions, raw or provider split-adjusted policy, and create/append/range-replace
mode. It shows authentication, entitlement, provider request ID, rate-limit and
retry timing, progress, cancellation, and empty-range outcomes without exposing
request credentials or provider-native payloads.

## Schedules

Schedules can be created, edited, enabled, disabled, run manually, and deleted.
Each schedule records dataset, symbols, interval, session and adjustment policy,
hourly or daily cadence, and next/last safe execution status. The UI explains
that schedules run while the coordinator is open and that startup coalesces
missed executions into one bounded catch-up range.

## Catalog publication

Every file import, Massive pull, and scheduled correction is generation-safe
and cancellable. Only an accepted response for the expected catalog revision
may publish exactly one newer catalog revision and content fingerprint.
Rejected, failed, cancelled, stale, unauthorized, rate-limited, or malformed
work remains visible in queue/log history and leaves the prior revision active.
