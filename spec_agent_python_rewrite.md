# Libraries and Main ADR Decisions

## Python/Cython Migration Note

This note has been promoted into authoritative specification work. The current
source of truth is `specs/python-migration.md`, `specs/technology-stack.md`,
ADR 0002, ADR 0005, ADR 0006, and the owning living specs. Keep this file only
as historical research context when comparing Go dependencies to Python
replacements.

## Popular Python/Cython Baseline

These were the likely core replacements considered for the Python/Cython stack:

| Responsibility | Python/Cython candidates |
|---|---|
| Runtime and packaging | CPython, `uv`, `pyproject.toml`, uv-managed virtual environments |
| Native kernels | Cython, NumPy C API, typed memoryviews |
| Numeric arrays | `numpy` |
| Scientific algorithms | `scipy` |
| Standard ML models and metrics | `scikit-learn`, `pytorch` |
| Columnar data and Arrow IPC/Parquet | `pyarrow` |
| Dataframes and tabular workflows | `pandas` |
| Database access | `sqlite3` standard library, `apsw`, or `sqlalchemy-core` behind repositories |
| UI/windowing | Python Raylib bindings, with Python Raygui bindings where UI controls are needed |
| HTTP and services | `fastapi`, `uvicorn`, `httpx` |
| WebSocket client/server | `websockets`, `websocket-client`, or framework-native WebSocket support |
| Validation and typed boundaries | Strong type hints, dataclasses or `pydantic`, and `pyright` as the type-checking gate |
| Testing | `pytest`, `hypothesis`, `pytest-benchmark` |
| Formatting, linting, and type checking | `ruff`, `black`, `pyright` |
| Vulnerability scanning | `pip-audit`, `osv-scanner`, `safety` |
| Build artifacts | `cibuildwheel`, `setuptools`, `cython`, platform wheels |

## Direct Libraries

github.com/apache/arrow-go/v18 v18.6.0
github.com/coder/websocket v1.8.15
github.com/gen2brain/raylib-go/raygui v0.0.0-20260619180708-8f9e96aca992
github.com/gen2brain/raylib-go/raylib v0.60.0
golang.org/x/sys v0.46.0
modernc.org/sqlite v1.53.0

## Direct Library Python Equivalents

| Go library | Current role | Python/Cython equivalent candidates |
|---|---|---|
| `github.com/apache/arrow-go/v18` | Arrow, Parquet, IPC, columnar data, compression integration | `pyarrow` for Arrow/Parquet/IPC and `pandas` for tabular workflows |
| `github.com/coder/websocket` | WebSocket event stream | FastAPI WebSockets, `websockets`, or `websocket-client` where a standalone client is needed |
| `github.com/gen2brain/raylib-go/raygui` | Immediate-mode UI controls over Raylib | Python Raygui bindings |
| `github.com/gen2brain/raylib-go/raylib` | Windowing, rendering, input, images | Python Raylib bindings such as `raylib-python-cffi` |
| `golang.org/x/sys` | Windows syscalls and memory/process adapters | `pywin32`, `ctypes`, `cffi`, `mmap`, `psutil` for process inspection where needed |
| `modernc.org/sqlite` | Pure-Go SQLite driver | `sqlite3` standard library for baseline use, `apsw` for lower-level SQLite control, or `sqlalchemy-core` behind typed repositories |

## Tool Packages

golang.org/x/vuln/cmd/govulncheck
honnef.co/go/tools/cmd/staticcheck

## Tool Package Python Equivalents

| Go tool | Current role | Python/Cython equivalent candidates |
|---|---|---|
| `golang.org/x/vuln/cmd/govulncheck` | Vulnerability scanning | `pip-audit`, `osv-scanner`, `safety` |
| `honnef.co/go/tools/cmd/staticcheck` | Static analysis | `ruff`, `pyright`, `bandit` for security-focused checks |

## Former Go ADR Decisions

ADR 0001: Use a loopback coordinator as the authoritative scheduler and metadata writer, a separate Raylib process, and isolated processes for active training jobs.

Former ADR 0002: Implement histogram regression trees, random forests,
squared-error gradient boosting, metrics, ranking, and portfolio calculations
as first-party kernels over typed contiguous slices and versioned memory-mapped
files.

ADR 0003: Keep the logical catalog mutable, record revision and content fingerprints for every run, and retain exact materialized arrays while a run is active or paused.

ADR 0004: Use one Raylib OS window with a custom retained dock tree, top workspace tabs, typed panel state, named layouts, and configurable panel link groups.

Former ADR 0005: Implement the application, public client, CLI, tests, tools,
and feature-extension model in the previous runtime. Use processes for role and
training-job isolation, with bounded pools for parallel work inside each
process.

ADR 0006: Maintain one approved Go technology stack, prefer the standard library, and admit a direct dependency only for a distinct responsibility after the compatibility and quality gates pass.

## Accepted Python/Cython ADR Decision Themes

These themes are now represented by accepted ADRs and living specs. Prefer the
authoritative files over this summary.

Accepted Python ADR theme 0001: Replace the prior runtime, tooling, and extension
model decisions with a Python/Cython application stack. CPython is the runtime,
`uv` owns dependency resolution and lockfiles, and transitive libraries are not
curated manually unless promoted to direct project dependencies.

Proposed Python ADR 0002: Stop implementing first-party tree and model
algorithms as the default path. `scikit-learn` and `pytorch` own model fitting,
prediction, metrics, and supported estimator behavior where their libraries
provide the needed capability.

Proposed Python ADR 0003: Corthena continues to own orchestration and audit
boundaries around library-owned model code. Corthena emits job progress, tracks
pause and resume state, validates model/data compatibility, controls artifact
promotion, and preserves immutable registry and audit metadata.

Proposed Python ADR 0004: Corthena owns checkpoint policy rather than assuming
all estimators expose the same mechanism. `scikit-learn` `partial_fit` is used
only for estimators that support it. PyTorch training is checkpointed through
library-native model, optimizer, scheduler, epoch, and training-state objects.

Proposed Python ADR 0005: Corthena wraps library artifacts with project
manifests. `scikit-learn` artifacts use a standard library format such as
`joblib` or `skops`; PyTorch artifacts use state-dict-style saved artifacts.
Both artifact families are wrapped with manifests, hashes, feature and data
fingerprints, run metadata, compatibility metadata, and immutable registry
promotion.

Proposed Python ADR 0006: FastAPI owns loopback HTTP and WebSocket service
boundaries, with DTO validation before domain conversion and typed public
contracts at process boundaries.

Proposed Python ADR 0007: Pyright is the strong type-checking gate for project
boundaries. Public APIs, DTOs, repositories, artifact manifests, and adapter
boundaries require explicit type hints and validated conversion points.
