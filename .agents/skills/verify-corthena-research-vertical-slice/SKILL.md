---
name: verify-corthena-research-vertical-slice
description: Verify or add verification for Corthena roadmap Phase 6's Research vertical slice. Use when testing, auditing, profiling, or closing acceptance gaps in Research panels, linked dataset/symbol/interval/range behavior, typed research client requests, deterministic demo data, leakage-safe feature and target presentation, loading/error/reconnect/cancellation scenarios, interaction replay, or Research golden images.
---

# Verify Corthena Research Vertical Slice

Produce repeatable evidence that Research is polished, interactive,
deterministic, leakage-safe, and exercised through the complete typed client
boundary.

## Ground verification

1. Read `AGENTS.md`, Phase 6 in `specs/roadmap.md`,
   `specs/frontend/workspaces.md`, `specs/frontend/foundation.md`,
   `specs/frontend/visualization.md`, `specs/data-and-features.md`,
   `specs/api.md`, and `specs/quality.md`.
2. Inspect Research app state, actions/effects, client types, simulator data,
   panel renderers, Phase 5 kernels, existing tests, benchmarks, and golden
   harness before selecting checks. Preserve unrelated changes.
3. Stay read-only when asked only to audit. Add tests, benchmarks, scenarios,
   or baselines only when implementation or acceptance-gap closure is in scope.

## Verify the typed client workflow

- Exercise every Research request through `FrontendClient` and the effects
  runtime; fail if a panel imports or type-switches on simulator details.
- Validate dataset, symbol, interval, range, resolution, series, target, cursor,
  correlation, and generation fields at their owning boundaries.
- Assert response slices and render buffers are owned, immutable after
  publication, chronologically ordered, and stable-ID keyed.
- Test deduplication, cancellation before and during work, stale generation
  rejection, queue saturation, bounded draining, channel closure, and shutdown
  without leaks. Run these paths with the race detector.

## Replay linked panel behavior

- Replay dataset, symbol, interval, pan, zoom, box selection, crosshair, series
  visibility, feature selection, row selection, pagination, and reset inputs.
- Require identical state, actions, effects, requests, visible buffers, and
  event order across repeated seeds and completion orders.
- Verify panels in one link group receive only supported scopes and independent
  groups remain unchanged. Confirm a stale source panel cannot overwrite a
  newer linked range.
- Cover keyboard and pointer paths, selection persistence, responsive layout,
  loading, empty, failure, retry, degraded/reconnecting, recovered, canceled,
  and busy states for all six Research panel types.

## Prove leakage safety

- Use hand-calculated rows to verify rolling lookbacks use only available past
  values and preserve missing prefixes.
- Verify forward targets use only the configured future horizon, terminal rows
  without valid targets are excluded, and targets never enter feature values.
- Check timestamp, symbol, source-row ordering, interval boundaries, feature
  availability, and target timestamps across viewport and pagination edges.
- Verify linked range changes alter presentation only and never recompute split
  membership or expose future observations.

## Verify visible work and rendering

- Invoke `$verify-corthena-visualization-performance` for changed chart/table,
  cache, LOD, generation, or virtualization paths. Retain operation-count
  assertions rather than relying only on wall-clock thresholds.
- Measure stable Research interaction allocations and representative dense-data
  benchmarks. Report input size, viewport, work counts, `ns/op`, bytes, and
  allocations.
- Capture the full Research workspace through Raylib on the locked UI thread at
  1280x720 and 1920x1080 with 100%, 150%, and 200% scaling. Record seed, clock,
  font fingerprint, backend, scenario, tolerance, and baseline version.
- Include normal, linked-selection, loading, error/degraded, and recovered
  golden scenarios. Perform image I/O and decoded RGBA comparison off the
  render thread.

## Conclude the gate

- Run focused tests first, then `gofmt -l`, `go build ./...`, `go test ./...`,
  `go vet ./...`, Staticcheck, applicable `go test -race`, hidden smoke
  launches, and `govulncheck`.
- Use `$go-windows-compat-gate` only when dependencies, native adapters, the
  application shell, or toolchain changed.
- Report exact commands, scenario coverage, replay determinism, leakage cases,
  benchmark context, golden differences, skipped checks, and residual risks.
  Do not mark Phase 6 complete when any Research panel, client path, required
  scenario, leakage assertion, golden matrix entry, or quality gate is missing.
