# Results Workspace

Default panels are the run browser and filters, metric/fold comparison,
equity/drawdown charts, fold timeline, IC/prediction distributions,
prediction/market overlay, and configuration diff.

Test metrics are explicitly labeled and never merged into validation selection
metrics. Completed run summaries and details are immutable and include metric
partitions, fold stability, equity/drawdown, chronological windows,
distributions, overlays, configuration, and reference-backtest disclosures.
Queries are generation ordered, filterable, cancellable, and retain the last
immutable comparison during refresh.

Comparison requests cross `UIClientProtocol` with explicit request,
correlation, comparison, workspace, and generation identities. Results remain
immutable; stale or cancelled completions cannot replace the current
comparison.
