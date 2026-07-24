# Research Features and Targets

**Status:** Authoritative  
**Owner:** Research  
**Last updated:** 2026-07-23  
**Related:** [Data datasets](../data/datasets.md), [Research workspace](workspace.md), [General API](../../general/api.md)

Research computes and previews features and targets from an immutable Data
dataset binding. It does not own acquisition, catalog mutation, or recipe
publication.

## Feature computation

The supported built-in operations include lagged returns, rolling price and
volume statistics, ratios, volatility and range measures, and per-timestamp
cross-sectional ranks and z-scores. Ordered recipe steps are validated by Data;
Research resolves their descriptors and computes them on immutable input
views. Learned transforms and bins fit only on training observations, and
cross-sectional operations use only symbols present at the timestamp. Missing
values remain explicit NaNs until their lookback is available.

## Targets and leakage

Configure an N-bar forward simple or log return. Features use information
through bar `t` close; reference execution is at bar `t+1` open. The default
target is the configured forward open-to-open horizon. Rows without a valid
future target are excluded. Visible-range changes cannot change target
membership, and future observations never enter feature fitting, target
construction, or previews.

## Queries and preview

Typed generation-bound requests load OHLCV, feature, target, distribution, and
cursor-paginated row responses through `UIClientProtocol`. Linked queries carry
dataset binding, symbols, interval, visible range, request, correlation, and
generation identities. Superseded requests cancel; stale completions cannot
publish. Results are immutable and bounded.

