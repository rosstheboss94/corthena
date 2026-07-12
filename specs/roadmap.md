# Frontend-First Implementation Roadmap

**Status:** In progress
**Owner:** Project  
**Last updated:** 2026-07-11
**Related:** [Technology stack](technology-stack.md), [Product](product.md), [Quality](quality.md), [Frontend foundation](frontend/foundation.md), [Frontend workspaces](frontend/workspaces.md), [Frontend visualization](frontend/visualization.md)

Phase 0 compatibility code now exists. Build the frontend against deterministic dummy data before implementing the real coordinator and research engine. Behavioral requirements remain in their owning specifications.

## 0. Go and native compatibility

**Implementation status:** Complete. The Go 1.25.11 toolchain, approved modules, typed compatibility adapters, native checks, tests, static analysis, race detector, and vulnerability scan pass on Windows/amd64.

1. Create the Go module with the selected Go 1.25.11 toolchain.
2. Verify Arrow/Parquet, SQLite, WebSocket, Windows adapters, `raylib-go`, Raygui, and development tools.
3. Exercise a minimal Raylib/Raygui window on a locked OS thread.
4. Verify Arrow, Parquet, SQLite WAL, typed memory mapping, loopback HTTP, WebSocket, and worker-pipe round trips.
5. Stop if the required Go modules or Windows compiler/native stack fail the compatibility gate.

**Done when:** all approved dependencies pass `specs/quality.md` and exact versions are recorded only in `go.mod` and `go.sum`.

## 1. Strict frontend scaffold

**Implementation status:** Complete. The four commands, bundled assets, typed Raylib/Raygui, Arrow, file-dialog, and Windows adapters, UI-thread enforcement, smoke launch, and required quality gates pass on Windows/amd64.

1. Create the command and package layout defined by the technology stack.
2. Configure formatting, static analysis, tests, race checks, vulnerability checks, and module tool pinning.
3. Add bundled assets and typed adapters for Raylib, Raygui, Arrow, file dialogs, and Windows APIs.
4. Lock the UI goroutine to its OS thread before Raylib initialization and enforce thread ownership.

**Done when:** the empty app launches, all configured checks pass, and native values do not escape adapters.

## 2. Typed frontend architecture and simulator

**Implementation status:** Implemented; completion is blocked by `govulncheck` on Go 1.25.11 standard-library vulnerability GO-2026-5856, which the tool reports fixed in Go 1.25.12.

1. Define `AppState`, `UIAction`, `UIEffect`, immutable client messages, `LinkContext`, panel state, and dock-node variants.
2. Implement pure reducers and a narrow `FrontendClient` interface.
3. Implement a seeded `DemoCoordinator` behind that interface with datasets, jobs, results, models, inference, delays, and failures.
4. Run simulator and persistence effects in owned background goroutines.
5. Inject clocks, ID sources, and seeds for repeatable demos and tests.

**Done when:** the same action sequence produces identical state and panels do not import simulator details.

## 3. Application shell

**Implementation status:** Implemented; hidden smoke launches pass at 1280x720 and 1920x1080, but completion is blocked by `govulncheck` on Go 1.25.11 standard-library vulnerability GO-2026-5856, which the tool reports fixed in Go 1.25.12.

Build the single Raylib window, seven workspace tabs, global context, command palette, component indicators, status bar, modals, toasts, connection states, and specified visual tokens.

**Done when:** the shell works at 1280×720 and 1920×1080 from 100% through 200% Windows scaling.

## 4. Docking and reusable controls

**Implementation status:** Implemented; core docking, typed UI-size preferences, the global Settings modal, keyboard shortcuts, responsive navigation, atomic preference recovery, unit and native input-replay tests, the resolution/scale matrix, hidden smoke launches, static analysis, and race checks pass. Completion is blocked by `govulncheck` on Go 1.25.11 standard-library vulnerability GO-2026-5856, which the tool reports fixed in Go 1.25.12.

Implement pure dock geometry, nested splits, tab stacks, drag targets, focus and pointer state, clipping, link groups, atomic versioned layout persistence, migrations, and invalid-layout fallback.

### Readability and settings follow-up

1. Add typed global frontend preferences with UI-size presets at 100%, 125%, 150%, 175%, and 200%; default to 125% and make `Ctrl+0` restore that default.
2. Add a global Settings modal opened from a top-right gear or `Ctrl+,`; provide live, autosaved preset buttons plus `Ctrl+Plus` and `Ctrl+Minus` stepping through the presets.
3. Apply UI sizing to typography, spacing, rows, controls, hit targets, sidebars, tabs, dock chrome, modal geometry, and dock minimum sizes. Compute effective scale as Windows DPI scale multiplied by the selected preset and clamped to the supported 1.0-2.0 range.
4. Replace undersized ad hoc metrics with readable roles: 12-pixel captions, 13-pixel body and data text, 14-pixel controls, and 18-pixel headings before scaling; load bundled fonts at a sufficiently high atlas size for sharp enlarged rendering.
5. Keep navigation, context, status, dock panels, and the Settings modal usable without overlap at 1280x720 and 1920x1080. Collapse lower-priority top-bar detail into the status bar when horizontal space is constrained.
6. Persist preferences in a strict, versioned document under the Corthena user configuration directory using background I/O, coalesced saves, atomic replacement, invalid-document quarantine, and default fallback. A late startup load must not overwrite a user change.
7. Keep Settings and command-palette overlays mutually exclusive, preserve critical-error precedence, surface persistence failures as retryable toasts, and update `specs/frontend/foundation.md` with the resulting behavior.

**Done when:** docking still passes unit and input-replay tests; the default 125% interface and every supported preset are readable and operable at both target resolutions; settings survive restart and invalid-file recovery; keyboard and pointer controls pass deterministic tests; and formatting, build, tests, vet, Staticcheck, race checks, hidden smoke launches, and vulnerability scanning have been run.

## 5. Charts and tables

**Implementation status:** Implemented. First-party double-precision chart transforms, clipping, ticks, all specified layers, viewport-width LOD, interaction replay, immutable render buffers, byte-bounded LRU caching, Arrow decoding, cancellation, deduplication, generation-safe workers, stable-ID virtual tables, typed sorting/filtering, pagination, bounded copying, Raylib primitive adaptation, RGBA capture/comparison, proportional-work tests, benchmarks, static analysis, and race checks pass. Completion remains blocked by the required Raylib golden baseline matrix and the Go 1.25.11 `govulncheck` finding GO-2026-5856, which the tool reports fixed in Go 1.25.12.

Implement double-precision transforms, required chart layers, interaction, viewport-width level of detail, byte-bounded caches, cancellation, generation tokens, and virtualized tables with stable row IDs.

**Done when:** chart work scales with viewport width, tables render only visible cells, and normal interaction targets 60 FPS.

## 6. Polished Research vertical slice

**Implementation status:** Implemented. The six-panel linked Research workspace, typed client/effect path, deterministic normal/loading/empty/failure/degraded/reconnecting/recovered/canceled/saturated scenarios, leakage-safe features and forward targets, stable virtual rows, interaction replay, cancellation and stale-generation safety, responsive rendering, benchmarks, and the 36-entry Raylib Research golden matrix pass on Windows/amd64. Completion is blocked only by `govulncheck` on Go 1.25.11 standard-library vulnerability GO-2026-5856, which the tool reports fixed in Go 1.25.12.

Build the OHLCV chart, feature browser, series inspector, target preview, distributions, row table, link synchronization, and deterministic loading/error/reconnect/cancellation scenarios.

**Done when:** Research is polished, interactive, deterministic, leakage-safe, and exercises the complete client boundary.

## 7. Data and Experiments

**Implementation status:** Implemented. The typed simulated catalog supports validated CSV and Parquet append or UTC range-replacement imports, atomic revision and fingerprint publication, deterministic failure/cancellation/saturation scenarios, and synchronized dataset selection. Experiments provide immutable definitions, a sectioned typed editor, compiled-feature metadata, leakage-safe validation, deterministic resource estimates, strict revision-aware background draft autosave and recovery, idempotent immutable submission, benchmarks, race coverage, and a deterministic 60-entry Raylib golden matrix across both target resolutions and 100%, 150%, and 200% UI scales. Completion is blocked only by `govulncheck` on Go 1.25.11 standard-library vulnerability GO-2026-5856, which the tool reports fixed in Go 1.25.12.

Build simulated catalog/import workflows and the experiment list, configuration tree, property editor, validation, resource estimate, local autosave, and immutable submission flow.

**Done when:** a user can complete a simulated dataset-to-experiment workflow.

## 8. Jobs and Results

**Implementation status:** Implemented. Typed generation-ordered Jobs and Results workflows, a 259-row stable-ID virtual queue, selected-job progress, live metrics, CPU leases and worker resources, process health, durable checkpoints, structured logs, legal pause/resume/cancel controls, immutable multi-run comparison, validation-versus-test metrics, equity/drawdown, fold timelines, IC and prediction distributions, prediction/market overlays, configuration diffs, lifecycle and stale-generation tests, race-safe bounded effects, and the deterministic 60-entry Raylib Phase 8 golden matrix pass on Windows/amd64. Completion is blocked only by `govulncheck` on Go 1.25.11 standard-library vulnerability GO-2026-5856, which the tool reports fixed in Go 1.25.12.

Build the virtualized queue, progress, metrics, resources, process status, checkpoints, logs, valid job controls, run comparisons, charts, distributions, overlays, and configuration diff.

**Done when:** deterministic jobs cover success, pause/resume, cancellation, interruption, and failure with immutable results.

## 9. Models and Inference

Build the immutable model registry, alias history, artifact metadata, feature importance, tree inspector, compatibility checks, rankings, score distributions, prediction history, and export status.

**Done when:** the dummy workflow reaches a registered model and inference output.

## 10. Backend-swap readiness

Keep dummy behavior behind `FrontendClient`; add reusable client contract tests; verify cancellation, deduplication, reconnect, reconciliation, and stale-generation rejection; and complete adapter, reducer, simulator, docking, chart, table, race, and golden-image tests.

**Done when:** the real Go client can replace `DemoCoordinator` without workspace or panel changes.

## Acceptance criteria

- The application, libraries, tests, tools, and extension model use Go exclusively.
- All seven workspaces operate with deterministic dummy scenarios.
- No filesystem, database, network, decoding, or simulator work runs on the render thread.
- Shared inputs are read-only; tasks own mutable outputs.
- Completed runs and model artifacts are immutable.
- Commands are enabled from typed state.
- Repeating a seed and action sequence produces identical state and screenshots.
- Formatting, static analysis, tests, race checks, and vulnerability checks pass.

## After the frontend

1. Implement the coordinator, health reporting, worker protocol, and repositories.
2. Implement imports, catalog revisions, compiled feature registry, targets, materialization, and leakage-safe splits.
3. Implement first-party Go models, deterministic training, Arrow artifacts, checkpoints, and recovery.
4. Implement evaluation, backtests, final refits, aliases, Arrow queries, and inference.
5. Replace the simulator with the real Go client using the shared contract suite.
