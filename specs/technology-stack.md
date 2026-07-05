# Technology Stack

**Status:** Authoritative  
**Owner:** Architecture  
**Last updated:** 2026-07-05  
**Related:** [System architecture](system-architecture.md), [Quality](quality.md), [ADR 0002](decisions/0002-go-tree-engine-and-artifacts.md), [ADR 0005](decisions/0005-go-hybrid-concurrency.md), [ADR 0006](decisions/0006-minimal-curated-go-dependencies.md)

This document owns approved direct dependencies and their responsibilities. The initial compatibility baseline is Go 1.25.7. The selected toolchain and exact module versions belong in `go.mod` and `go.sum` once the compatibility gate passes.

## Dependency policy

- Prefer the standard library when it provides a clear, maintainable solution.
- Add a direct dependency only when it materially reduces implementation risk or cost.
- Transitive packages are not application APIs unless explicitly promoted here.
- Keep native dependencies behind narrow typed adapters.
- Do not duplicate the same responsibility across multiple libraries.
- The application, build scripts, generators, tests, and extension model use Go exclusively.

## Runtime, build, and layout

| Responsibility | Technology |
|---|---|
| Runtime and compiler | Go toolchain selected by `go.mod` and the `toolchain` directive |
| Dependency resolution and checksums | Go modules, `go.mod`, and `go.sum` |
| Build | `go build` with first-party build scripts only when needed |
| Native binding boundary | cgo only inside approved adapter packages |
| Commands | `cmd/workstation`, `cmd/coordinator`, `cmd/worker`, `cmd/trading-research` |
| Internal implementation | `internal/...` packages |
| Supported library surface | `client` and `contract` packages |
| Application-data paths | `os.UserConfigDir` plus a first-party application subdirectory helper |
| Configuration and JSON | Standard-library typed structs and `encoding/json` |
| Identifiers, dates, hashing, and atomic files | Standard library |

The Windows UI build uses cgo because the approved Raygui binding requires it. The compatibility gate records the supported Windows C toolchain. Coordinator and worker code must not import Raylib or Raygui.

## Numerical, data, and storage

| Responsibility | Technology |
|---|---|
| Numerical computation and custom ML | First-party Go kernels over typed contiguous slices |
| Columnar data, CSV, Parquet, Arrow IPC, and Zstandard | `github.com/apache/arrow-go/v18` |
| Metadata database | `database/sql` with `modernc.org/sqlite`, in WAL mode |
| Database migrations | Numbered SQL files and `PRAGMA user_version` |
| Training matrices | Versioned raw little-endian typed files through a Windows memory-map adapter |
| Models and checkpoints | JSON manifests plus checksummed Arrow IPC array files |
| Checksums and atomic writes | Standard library |
| Windows memory mapping and process integration | `golang.org/x/sys/windows` behind typed adapters |

Do not add an external ML implementation, dataframe framework, ORM, numerical runtime, or backtesting framework. Apache Arrow is a boundary and storage library; estimator fitting, split search, metrics, ranking, and portfolio accounting remain first-party code.

## Services, clients, and CLI

| Responsibility | Technology |
|---|---|
| Loopback API and HTTP client | `net/http` |
| Routing | Go `http.ServeMux` method/path patterns |
| DTO validation and serialization | Typed structs, explicit validators, and `encoding/json` |
| Event stream | `github.com/coder/websocket` |
| CLI | Standard-library `flag.FlagSet` subcommands |
| Database access | Typed repositories over `database/sql` |
| Process-safe structured logging | `log/slog` JSON handlers and coordinator ingestion |
| Retry/backoff | Small first-party bounded-backoff utility |

Do not add a web framework, ORM, external message broker, distributed task queue, dependency-injection framework, or second logging facade.

## Concurrency

Use processes for role and job isolation. Within a process use goroutines, channels, `context`, `sync`, `sync/atomic`, and bounded worker pools. The coordinator owns CPU-slot leases. Do not add a distributed runtime or hidden numerical thread pool.

## Frontend

| Responsibility | Technology |
|---|---|
| Windowing, rendering, input, and basic controls | `github.com/gen2brain/raylib-go/raylib` and `github.com/gen2brain/raylib-go/raygui` |
| Docking, charts, tables, and file browser | First-party Go components |
| Networking | `net/http` and the approved WebSocket package on background goroutines |
| Fonts | Bundled Inter and JetBrains Mono |
| Icons | Bundled Lucide-derived atlas |
| Golden-image comparison | Raylib image APIs plus first-party pixel comparison |

Do not add another UI or charting framework. All Raylib and Raygui calls remain on the locked UI OS thread.

## Development and testing

| Responsibility | Technology |
|---|---|
| Formatting | `gofmt` |
| Compiler and unit/integration tests | Go toolchain and `go test` |
| Static analysis | `go vet` and `honnef.co/go/tools/cmd/staticcheck` |
| Vulnerability analysis | `golang.org/x/vuln/cmd/govulncheck` |
| Race analysis | Go race detector on supported packages and platforms |
| Property and parser testing | Native fuzz tests and table-driven tests |
| Coverage | Go coverage profiles |
| Timeouts and leak checks | Test contexts, explicit deadlines, and first-party goroutine lifecycle assertions |

Do not add a second test runner, assertion framework, mocking framework, linter aggregator, or code generator unless this document is revised.

## Dependency admission

A new direct dependency requires:

1. A responsibility not reasonably covered by the standard library or approved stack.
2. Successful installation, compilation, and tests with the selected Go toolchain on Windows.
3. Race, goroutine-lifecycle, and native-thread behavior verification where applicable.
4. Windows support and an acceptable license.
5. A typed public API or narrow typed adapter.
6. Tests for its failure and cancellation boundaries.
7. Updated `go.mod`, `go.sum`, and this document.
8. An ADR when it changes a foundational architectural choice.

The compatibility spike chooses exact versions and records them only in module files.
