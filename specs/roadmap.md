# Frontend-First Implementation Roadmap

**Status:** In progress
**Owner:** Project  
**Last updated:** 2026-07-05  
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

1. Create the command and package layout defined by the technology stack.
2. Configure formatting, static analysis, tests, race checks, vulnerability checks, and module tool pinning.
3. Add bundled assets and typed adapters for Raylib, Raygui, Arrow, file dialogs, and Windows APIs.
4. Lock the UI goroutine to its OS thread before Raylib initialization and enforce thread ownership.

**Done when:** the empty app launches, all configured checks pass, and native values do not escape adapters.

## 2. Typed frontend architecture and simulator

1. Define `AppState`, `UIAction`, `UIEffect`, immutable client messages, `LinkContext`, panel state, and dock-node variants.
2. Implement pure reducers and a narrow `FrontendClient` interface.
3. Implement a seeded `DemoCoordinator` behind that interface with datasets, jobs, results, models, inference, delays, and failures.
4. Run simulator and persistence effects in owned background goroutines.
5. Inject clocks, ID sources, and seeds for repeatable demos and tests.

**Done when:** the same action sequence produces identical state and panels do not import simulator details.

## 3. Application shell

Build the single Raylib window, seven workspace tabs, global context, command palette, component indicators, status bar, modals, toasts, connection states, and specified visual tokens.

**Done when:** the shell works at 1280×720 and 1920×1080 from 100% through 200% Windows scaling.

## 4. Docking and reusable controls

Implement pure dock geometry, nested splits, tab stacks, drag targets, focus and pointer state, clipping, link groups, atomic versioned layout persistence, migrations, and invalid-layout fallback.

**Done when:** docking passes unit and input-replay tests and survives reload, migration, resolution, and DPI changes.

## 5. Charts and tables

Implement double-precision transforms, required chart layers, interaction, viewport-width level of detail, byte-bounded caches, cancellation, generation tokens, and virtualized tables with stable row IDs.

**Done when:** chart work scales with viewport width, tables render only visible cells, and normal interaction targets 60 FPS.

## 6. Polished Research vertical slice

Build the OHLCV chart, feature browser, series inspector, target preview, distributions, row table, link synchronization, and deterministic loading/error/reconnect/cancellation scenarios.

**Done when:** Research is polished, interactive, deterministic, leakage-safe, and exercises the complete client boundary.

## 7. Data and Experiments

Build simulated catalog/import workflows and the experiment list, configuration tree, property editor, validation, resource estimate, local autosave, and immutable submission flow.

**Done when:** a user can complete a simulated dataset-to-experiment workflow.

## 8. Jobs and Results

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
