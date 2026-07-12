---
name: verify-corthena-visualization-performance
description: "Verify Phase 5 charts and tables: transforms, LOD, bounded caches, virtualization, replay, races, benchmarks, and Raylib goldens."
---

# Verify Corthena Visualization Performance

Produce repeatable evidence that Phase 5 behavior is correct, deterministic,
bounded, race-free, and proportional to visible work.

## Ground verification

1. Read `python_migration/AGENTS.md`, `python_migration/specs/roadmap.md`,
   `python_migration/specs/frontend/visualization.md`, `python_migration/specs/frontend/foundation.md`, and
   `python_migration/specs/quality.md`.
2. Read `python_migration/specs/technology-stack.md` for native, dependency, or tooling checks.
3. Inspect the implementation, existing tests, benchmarks, and golden harness
   before selecting checks. Preserve unrelated changes.
4. When asked only to verify or audit, remain read-only. Add tests, benchmarks,
   or baselines only when the request includes implementation or gap closure.

## Verify numerical and interaction correctness

- Use hand-calculated cases for forward/inverse transforms, clipped and
  degenerate viewports, non-finite rejection, ticks, candles, continuous series,
  and every supported layer.
- Verify OHLC buckets preserve open, high, low, close, and stable source order.
  Verify continuous LOD preserves first, last, minimum, and maximum with stable
  source-index tie-breaking.
- Replay pan, zoom, box selection, crosshair, reset, visibility, keyboard, and
  linked-view inputs. Require identical state and event order across runs.
- Fuzz bounded parsers and transform inputs where malformed or extreme values
  can cross a serialization or numerical boundary.

## Verify bounded asynchronous behavior

- Test request deduplication, cancellation before and during work, channel
  closure, queue saturation, stale generations, and shutdown without leaks.
- Require immutable published buffers and run lifecycle/concurrency tests for caches,
  workers, reducers, and client-facing visualization paths.
- Test byte accounting and deterministic LRU eviction at exact boundaries,
  including entries larger than the budget and replacement of existing keys.

## Verify proportional work and virtualization

- Instrument operation counts so chart render preparation is bounded by
  viewport width after LOD. Do not rely only on noisy wall-clock thresholds.
- Test table windows at empty, first, middle, last, resized, pinned-column, and
  overscrolled positions. Assert measured/rendered cell counts depend on the
  visible window, not total rows or columns.
- Verify stable row-ID selection across sorting, filtering, pagination, inserts,
  removals, and updates with deterministic null ordering.
- Benchmark representative dense charts and large tables. Report `ns/op`,
  allocations, bytes, input size, viewport size, and work counts; avoid
  hardware-specific pass times in CI.

## Verify rendering evidence

- Capture golden images through Raylib on the locked UI thread at 1280x720 and
  1920x1080 with 100%, 150%, and 200% scaling.
- Record seed, viewport, scale, font assets, scenario clock, backend, and
  baseline version. Compare decoded RGBA with documented per-channel tolerance
  and maximum differing-pixel ratio.
- Keep filesystem reads/writes and image comparison outside the render frame;
  only Raylib capture occurs on the UI thread.

## Conclude the gate

- Run focused tests, then applicable configured `ruff format --check`, `ruff check`,
  `pyright`, `pytest`, `hypothesis`, `pytest-benchmark`, and vulnerability checks.
- Use `$python-windows-compat-gate` when native adapters, dependencies, the
  application shell, or toolchain changed.
- Report exact commands, results, skipped checks, benchmark context, golden
  differences, and residual risks. Do not mark Phase 5 complete when a required
  gate, proportional-work assertion, virtualization assertion, or golden matrix
  is missing.
