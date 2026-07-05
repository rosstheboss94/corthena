# Frontend Workspaces

**Status:** Authoritative  
**Owner:** Frontend  
**Last updated:** 2026-07-04  
**Related:** [Technology stack](../technology-stack.md), [Foundation](foundation.md), [Visualization](visualization.md)

All workspaces consume typed state and emit `UIAction` values. Panels do not call repositories, the network, the filesystem, workers, or the simulator directly.

## Data

Default panels are the catalog table, coverage timeline, import and validation queue, dataset inspector, and import logs.

Users can import files, choose append or range replacement, inspect validation failures, and select the active dataset context.

## Research

Default panels are the primary OHLCV chart, feature browser, series inspector, target preview, feature/target distributions, and row-level data table.

Linked panels synchronize dataset, symbols, interval, and visible time range through configurable link groups.

## Experiments

Default panels are the experiment list, searchable configuration section tree, compact property table/editor, contextual inspector, validation summary, and resource estimate.

The editor is panel-based rather than a wizard. It configures dataset, features, target, split, model, portfolio, and optional sweep. Drafts autosave through background effects. Submission validates through the coordinator and creates an immutable experiment definition.

Custom feature selection shows compiled registry name, semantic version, lookback, output schema, and implementation fingerprint. The UI never accepts source paths or runtime scripts.

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
