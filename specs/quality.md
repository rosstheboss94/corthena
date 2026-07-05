# Quality Specification

**Status:** Authoritative  
**Owner:** Engineering  
**Last updated:** 2026-07-04  
**Related:** [Technology stack](technology-stack.md), [System architecture](system-architecture.md), [Roadmap](roadmap.md)

## Verification toolchain

Once scaffolding exists, use `gofmt` for formatting, the compiler and `go test` for tests, `go vet` and Staticcheck for static analysis, `govulncheck` for reachable vulnerabilities, native fuzzing for property/parser tests, coverage profiles for coverage, and the race detector for concurrency-sensitive packages. Do not add another test runner, linter aggregator, mocking framework, or assertion framework without revising the technology stack.

## Type and API safety

- First-party source and tests must compile and pass all configured checks.
- Exported identifiers require documentation and deliberate package ownership.
- Prefer concrete immutable value structs at boundaries and small consumer-owned interfaces.
- Avoid `any`, `map[string]any`, reflection-driven decoding, and unchecked type assertions in domain and boundary code.
- Use dedicated string-backed types and validation for IDs, enum values, versions, and state machines.
- Validate JSON with unknown-field rejection before conversion to domain models.
- Return errors with operation context while preserving machine-testable causes.
- Keep native values, `unsafe`, Arrow builders, SQLite rows, and Windows handles inside adapters.
- Make resource ownership explicit with `Close`, cancellation, and idempotent cleanup contracts.

## Concurrency safety

- Every goroutine must have an owner, termination condition, and cancellation path.
- Every channel must document its sender, receiver, closer, and buffering policy.
- Do not hold locks across filesystem, network, process, database, or callback boundaries.
- Do not copy lock- or atomic-bearing structs after first use.
- Publish shared slices and mapped files only after they become immutable.
- Run race-enabled tests for coordinator, worker, client, scheduler, cache, and reducer packages.
- Use deterministic logical ordering for reductions; tests vary goroutine counts and completion order.
- Raylib and Raygui adapters assert locked UI-thread ownership in development and tests.

## Compatibility gate

Before full scaffolding, verify the selected Go toolchain and module versions on Windows:

- Compile all commands and approved direct dependencies.
- Open and close a minimal `raylib-go` window and exercise one Raygui control on the UI OS thread.
- Perform CSV-to-Parquet and Arrow IPC round trips, including Zstandard compression.
- Open SQLite in WAL mode, run migrations, and verify concurrent readers with the single coordinator writer.
- Map typed matrix files, run parallel read-only kernels, flush exclusive output ranges, and reopen them.
- Start the loopback endpoint, WebSocket feed, Go client, and a worker pipe handshake.
- Run formatting, static analysis, unit tests, applicable race tests, and vulnerability scanning.

The gate fails on unsupported Go, C compiler, Raylib/Raygui, Arrow, SQLite, or Windows adapter combinations. Exact accepted versions are recorded only in module files.

## Test requirements

- Hand-calculated table tests for histogram splits, missing routing, leaves, forests, boosting, metrics, and serialization.
- Fuzz tests for DTOs, framed worker messages, manifests, checkpoints, migrations, and layout documents.
- Interrupt/resume tests before and after node, tree, and stage boundaries.
- Corrupt, partial, incompatible, and previous-version checkpoint tests.
- Leakage tests for features, targets, purge, embargo, and execution timing.
- Import tests for missing bars, duplicates, corrections, and catalog changes during paused runs.
- Integration tests for API, Go client, CLI, crashes, stale heartbeats, pause-on-close, sweeps, aliases, and inference rejection.
- Concurrency stress tests across goroutine counts, worker counts, and completion orders, requiring identical artifacts and metrics.
- CPU-slot, nested-pool, cancellation, channel-closure, and goroutine-leak tests.
- Raylib main-thread enforcement tests.
- Docking, input replay, layout migration, chart transform, and large-table tests.
- Golden UI images at 1280×720 and 1920×1080 with 100%, 150%, and 200% scaling.

## Performance acceptance

- Maintain smooth 60 FPS during normal chart interaction on the reference workstation.
- Make chart rendering proportional to viewport width after level-of-detail processing.
- Virtualize large tables.
- Keep filesystem, decoding, database, and network work off the render thread.
- Run an out-of-CI synthetic benchmark near ten million rows and record throughput, allocation counts, garbage-collection pauses, and peak memory without hardware-specific pass times.
- Benchmark hot numerical kernels with representative missingness and row counts before accepting optimization-specific complexity.
