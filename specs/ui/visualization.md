# UI Visualization

**Status:** Authoritative  
**Owner:** UI  
**Last updated:** 2026-07-12
**Related:** [Technology stack](../technology-stack.md), [Foundation](foundation.md), [API](../api.md), [Quality](../quality.md)

## Technology constraints

Render with approved Python Raylib primitives and use first-party Python
kernels for transforms, level-of-detail aggregation, and golden-image
comparison, with Cython only for measured hot paths. Transfer dense data with
PyArrow and Arrow IPC. Do not add another chart, image-processing, dataframe,
or numerical framework.

## Chart types and layers

Render directly with Raylib primitives:

- Candlestick and volume.
- Line and area.
- Histogram and scatter.
- Equity and drawdown.
- Heatmap.
- Feature importance.

Supported layers include OHLCV, features, predictions, portfolio trades, and train/validation/test regions.

V1 excludes persistent annotations, drawing tools, replay, order flow, liquidation maps, depth displays, and alerts.

## Interaction

- Pointer pan and wheel zoom.
- Box range selection.
- Crosshair and typed tooltip.
- Series visibility and reset-to-fit.
- Linked time axes and symbol/range propagation.
- Configurable link groups so comparison panels can remain independent.

Coordinate transforms use `float64` until final conversion to Raylib `float32` draw values. Conversion rejects non-finite or out-of-range coordinates and clips geometry to the viewport.

## Level of detail

Chart requests specify series, range, and target resolution. Dense responses use Arrow IPC.

- Bucket data by horizontal pixel range.
- Preserve OHLC semantics for candles.
- Preserve first, last, minimum, and maximum samples for continuous series with stable source-index tie-breaking.
- Keep render work proportional to viewport width rather than source rows.
- Cache chart frames by query and resolution with a byte-bounded LRU.
- Use generation tokens so stale responses cannot replace newer views.
- Perform Arrow decoding and aggregation off the render thread; publish immutable render-ready buffers.

## Tables

Virtualize rows and columns. Support sortable and resizable headers, pinned identifier columns, keyboard and pointer selection, copying selected cells/rows, server-side filters and pagination, and stable row IDs across updates.

Only visible cells are measured and rendered. Sorting and filtering use explicit typed column behavior and deterministic null ordering.

## Asynchronous client

Owned background workers provide request deduplication, cancellation tokens,
generation tokens, bounded caches, and WebSocket reconnect with bounded backoff.
They send immutable typed messages to the render thread through bounded queues.

Connection states are `offline`, `connecting`, `synchronized`, and `degraded`. REST reconciliation follows startup, reconnect, sequence gaps, and unknown events.

## Golden images

Capture named scenarios through Raylib image APIs on the UI thread. Compare
decoded RGBA pixels with first-party Python code using documented per-channel
tolerance and maximum differing-pixel ratio. Store metadata beside each image:
phase, scenario, viewport, scale factor, seed, scenario clock, dataset fixture,
layout name, serialized app state revision, font/icon asset versions, rendering
backend, dependency versions, build revision, and dirty-build flag.

## Performance

Normal chart interaction targets 60 FPS on the reference workstation. Rendering must not scale linearly with million-point inputs after level-of-detail processing, and table rendering must not scale with total row count. Per-frame allocations in stable interaction paths are measured and bounded.
