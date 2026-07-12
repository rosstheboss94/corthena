# Frontend Workspaces

**Status:** Authoritative  
**Owner:** Frontend  
**Last updated:** 2026-07-11
**Related:** [Technology stack](../technology-stack.md), [Foundation](foundation.md), [Visualization](visualization.md)

All workspaces consume typed state and emit `UIAction` values. Panels do not call repositories, the network, the filesystem, workers, or the simulator directly.

## Data

Default panels are the catalog table, coverage timeline, import and validation queue, dataset inspector, and import logs.

Users can import files, choose append or range replacement, inspect validation failures, and select the active dataset context.

Import requests use typed source kind, adjustment policy, symbols, interval,
mode, and optional UTC replacement bounds. Validation completes before a
catalog mutation. A successful import atomically publishes exactly one new
dataset revision and implementation fingerprint; rejected duplicate,
malformed, or out-of-range input remains visible in the queue and logs without
changing the prior revision. Dataset selection uses stable identities and
refreshes dependent workspace context without exposing mutable simulator data.

Catalog queries and imports run through bounded effects with cancellation,
correlation, and generation identities. Panels retain the last accepted
snapshot while rendering loading, empty, failure/retry, degraded, recovered,
canceled, and saturated states in place.

## Research

Default panels are the primary OHLCV chart, feature browser, series inspector, target preview, feature/target distributions, and row-level data table.

Linked panels synchronize dataset, symbols, interval, and visible time range through configurable link groups.

The default Research layout keeps the OHLCV chart primary, groups the four
inspectors in a side tab stack, and places the virtualized row table below. At
constrained logical widths the six panels collapse into one operable tab stack
without changing their stable IDs or link-group assignments.

The chart provides candlestick and volume rendering, selected-feature and
forward-target overlays, train/validation/test regions, crosshair values,
wheel zoom, shift-drag pan, box range selection, layer visibility, and reset.
Feature selection, row selection, typed sorting and filtering, and cursor
pagination retain stable identities across refreshed generations.

Every link group owns an independent, generation-ordered Research request and
selection state. Only dataset, symbols, interval, and visible range propagate
through the source panel's supported link scopes; comparison groups remain
unchanged. Panels retain stale data during a refresh and render loading, empty,
failure/retry, degraded, recovered, canceled, and saturated states in place.

Feature values retain explicit missing prefixes until their declared lookback
is available. Target previews use the configured forward open-to-open horizon,
exclude rows without a dataset-valid future target, and do not change feature,
target, or split membership when the visible range changes.

## Experiments

Default panels are the experiment list, searchable configuration section tree, compact property table/editor, contextual inspector, validation summary, and resource estimate.

The editor is panel-based rather than a wizard. It configures dataset, features, target, split, model, portfolio, and optional sweep. Drafts autosave through background effects. Submission validates through the coordinator and creates an immutable experiment definition.

Custom feature selection shows compiled registry name, semantic version, lookback, output schema, and implementation fingerprint. The UI never accepts source paths or runtime scripts.

Draft validation is typed and section-aware. It checks the selected dataset
revision and fingerprint, unique compiled features, forward target, walk-forward
split, purge of at least the target horizon, bounded model and sweep settings,
finite portfolio values, and CPU limits. Resource estimates are deterministic
for the same validated draft. Invalid drafts remain editable and autosavable
but cannot be submitted.

Local drafts use a strict schema-versioned document, revision-aware coalesced
background writes, atomic replacement, unknown-field rejection, invalid-file
quarantine, and default fallback. A late load or stale save cannot overwrite a
newer edit. Submission is idempotent by command identity and captures an
immutable experiment definition with the accepted dataset revision,
fingerprint, compiled feature identities, and complete configuration; later
catalog changes do not rewrite accepted definitions.

## Jobs

Default panels are the virtualized job queue, selected-job stage/progress view, live metrics, worker and CPU-slot resources, process/component and checkpoint status, and structured logs.

Users can pause, resume, or cancel only when allowed by the typed job state. Interrupted jobs require explicit resume.

## Results

Default panels are the run browser and filters, metric and fold comparison, equity and drawdown charts, fold timeline, IC and prediction distributions, prediction/market overlay, and configuration diff.

Test metrics are visually distinct from selection metrics to discourage test-set tuning.

## Models

Default panels are the immutable model registry, alias and promotion history, artifact metadata, feature importance, and tree structure inspector.

Alias assignment requires explicit confirmation and never deletes the prior model.

## Inference

Default panels are the model and alias selector, dataset/range selector, ranked scored symbols, score distribution, prediction history, and export status.

Historical or latest-snapshot scoring displays model, engine, feature-registry, lookback, and data compatibility before submission.

## Navigation and commands

Top tabs activate workspaces. A searchable command palette and shortcuts provide keyboard access to navigation, panel opening, layout switching, job actions, and chart reset. Commands are enabled from current typed state rather than failing after invocation.
