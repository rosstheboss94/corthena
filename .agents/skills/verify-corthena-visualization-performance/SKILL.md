---
name: verify-corthena-visualization-performance
description: Verify Corthena Phase 5 charts and tables for numerical correctness, deterministic interactions and LOD, bounded caches and workers, generation races, virtualization, proportional work, benchmarks, lifecycle safety, and the dedicated Raylib golden matrix. Use for Phase 5 acceptance or regression audits; reject Phase 6 Research workflows and later domain scope.
---

# Verify Corthena Visualization Performance

## Establish the contract

1. Read `python_migration/AGENTS.md` and every required document in
   `python_migration/specs/routing/phase-5.md`.
2. Inspect the complete target diff and matching legacy Go chart, table,
   golden, effects, state, and native visualization references.
3. Record every input and tolerance in the dedicated `phase5-golden` manifest.
   Treat migration JPEGs only as design references.
4. When asked only to verify or audit, remain read-only. Add evidence only when
   implementation or gap closure is explicitly in scope.

## Audit numerical and interaction behavior

- Exercise hand-calculated forward/inverse transforms, clipping, ticks,
  degenerate viewports, non-finite and range rejection, and every generic layer.
- Verify candle buckets preserve open, high, low, close, volume, and stable
  ordering. Verify continuous LOD preserves first, last, minimum, and maximum
  with stable source-index tie-breaking.
- Replay pan, zoom, box selection, crosshair, reset, visibility, keyboard, and
  generic linked-axis inputs. Require identical state and event order.
- Fuzz bounded numerical and serialized boundaries where malformed or extreme
  values can enter typed domain values.

## Audit bounded concurrency and caches

- Test exact queue and byte limits, request deduplication, replacement,
  deterministic LRU eviction, oversized entries, saturation, cancellation
  before and during work, channel closure, stale generations, and failure paths.
- Vary worker counts, delays, completion orders, and pressure. Require immutable
  publication, stable logical results, bounded draining, bounded shutdown, and
  no reliance on the GIL.
- Repeat normal, cancellation, saturation, startup-failure, and injected-failure
  lifecycles; assert threads, handles, queues, buffers, and temporary resources
  return to baseline.

## Audit proportional work and virtualization

- Require operation counts proving chart preparation after source-range
  selection is bounded by viewport width, not source rows. Do not accept only
  wall-clock thresholds.
- Exercise empty, first, middle, last, resized, pinned-column, and overscrolled
  table windows. Require cell work bounded by visible rows, columns, and
  overscan, independent of total table size.
- Verify stable row-ID selection through sorting, filtering, pagination,
  insertion, removal, and update with deterministic typed null behavior.
- Run configured representative benchmarks and record input/viewport sizes,
  work counts, time, allocations, and bytes. Record the out-of-CI
  near-ten-million-row throughput and peak Python/native memory evidence.

## Audit visual and native behavior

- Use `$verify-corthena-raylib-visual-system` to audit tokens, shared primitives,
  typography, interaction states, accessibility, clipping, scaling, and stable
  draw order.
- Require render-neutral immutable view models, typed actions, native-value
  containment, locked UI-thread calls, balanced clipping, and no blocking work
  in layout or draw.
- Capture 1280x720 and 1920x1080 at 100%, 150%, and 200% using every exact
  manifest input. Compare decoded RGBA at unchanged recorded tolerances.

## Conclude acceptance

- Run focused tests and applicable configured format, lint, type, property,
  benchmark, vulnerability, Windows native build/import, and full test gates.
- Lead with actionable findings and cite the smallest useful locations. Report
  every command, benchmark context, golden case, failure, and skipped check.
- Keep Phase 5 Pending while any required layer, race, lifecycle, work-count,
  virtualization, benchmark, golden, Windows gate, or final review is missing
  or failing. Reject Phase 6+ workflow changes from this route.
