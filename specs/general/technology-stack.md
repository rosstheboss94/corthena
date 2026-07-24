# Technology Stack

**Status:** Authoritative
**Owner:** Architecture
**Last updated:** 2026-07-18
**Related:** [Concurrency and parallelism](concurrency-and-parallelism.md), [System architecture](system-architecture.md), [Quality](quality/README.md), [Python migration](../history/migration/python-migration.md), [ADR 0002](../decisions/0002-python-library-estimators-and-artifacts.md), [ADR 0008](../decisions/0008-regular-cpython-concurrency.md), [ADR 0006](../decisions/0006-curated-python-dependencies.md)

This document owns approved direct dependencies and their responsibilities for
the Python/Cython codebase. `pyproject.toml` owns direct dependency declarations
and tool configuration. `uv.lock` owns resolved versions accepted by the
compatibility gate.

## Dependency Policy

- Prefer the Python standard library when it provides a clear, maintainable
  solution.
- Add a direct dependency only when it materially reduces implementation risk
  or cost.
- Transitive packages are not application APIs unless explicitly promoted here.
- Keep native, weakly typed, and library-specific values behind narrow typed
  adapters.
- Do not duplicate the same responsibility across multiple libraries.
- Use Cython only for measured hot paths or native adapters, not as the default
  implementation language for ordinary modules.
- The application, build scripts, generators, tests, and extension model use
  Python unless a measured native adapter requires Cython.

## Runtime, Build, and Layout

| Responsibility | Technology |
|---|---|
| Runtime | Exact regular CPython `3.14.2` Windows AMD64 (`cp314-win_amd64`) |
| Environment and dependency resolution | `uv`, `pyproject.toml`, and `uv.lock` |
| Packaging and entry points | `pyproject.toml` project scripts |
| Native extension build | `setuptools`, `cython`, and Windows compiler tooling approved by the compatibility gate |
| Commands | Python module entry points for workstation UI, coordinator, worker, and CLI |
| Internal implementation | `corthena/...` packages |
| Supported library surface | typed `corthena.client` and `corthena.contracts` packages |
| Application-data paths | `platformdirs` behind a first-party path helper |
| Configuration and JSON | dataclasses or Pydantic DTOs with explicit validators |
| Identifiers, dates, hashing, and atomic files | Python standard library |

The Windows UI build may use native Raylib/Raygui bindings. The compatibility
gate records the supported Windows compiler and native runtime. Coordinator and
worker code must not import Raylib or Raygui.

## Numerical, Data, and Storage

| Responsibility | Technology |
|---|---|
| Numerical arrays and vectorized kernels | `numpy` |
| Scientific utilities | `scipy` only for approved numerical routines |
| Standard ML estimators and metrics | `scikit-learn` |
| Neural model training and inference | PyTorch |
| Hot first-party kernels | Cython over typed memoryviews after benchmark evidence |
| Columnar data, Parquet, Arrow IPC, and compression | `pyarrow` |
| Tabular workflow convenience | `pandas` behind adapters, not as a public contract type |
| Metadata database | `sqlite3` standard library initially; `apsw` may be admitted for explicit SQLite control |
| Market-session calendar | `exchange_calendars` behind a typed adapter, pending the milestone-specific admission gate below |
| Database migrations | Numbered SQL files and `PRAGMA user_version` |
| Training matrices | Versioned raw little-endian typed files or NumPy memory maps through adapters |
| Models and checkpoints | Project manifests plus checksummed library artifacts and array files |
| Windows process/native integration | standard library, `ctypes`, `cffi`, or `pywin32` behind typed adapters |

Corthena owns orchestration, validation, leakage checks, manifests, artifact
promotion, registry compatibility, audit metadata, and user-visible workflow
state. scikit-learn and PyTorch own estimator internals where approved.

## Services, Clients, and CLI

| Responsibility | Technology |
|---|---|
| Loopback API | FastAPI |
| ASGI server | Uvicorn |
| HTTP client | `httpx` |
| DTO validation and serialization | Pydantic or dataclasses with explicit validators |
| Event stream | FastAPI WebSockets or `websockets` behind adapters |
| CLI | `argparse` or Typer after explicit admission |
| Database access | typed repositories over `sqlite3` or approved SQLite adapter |
| Process-safe structured logging | Python `logging` JSON formatting behind first-party helpers |
| Retry/backoff | small first-party bounded-backoff utility |

Massive historical ingestion uses `httpx` directly with a first-party typed
connector. A Massive SDK is not approved.

Do not add an ORM, external message broker, distributed task queue,
dependency-injection framework, or second logging facade unless this document is
revised.

## Concurrency

[Concurrency and parallelism](concurrency-and-parallelism.md) owns execution
selection, CPU leases, adapter bounds, and lifecycle policy. This document owns
which process, async, threading, native, and Cython technologies may implement
that policy. Queues, cancellation events, async tasks, and library pools remain
behind explicit typed adapters; no distributed runtime or external broker is
approved.

## UI

| Responsibility | Technology |
|---|---|
| Windowing, rendering, input, and basic controls | Python Raylib bindings and Python Raygui bindings approved by compatibility testing |
| Docking, charts, tables, and file browser | First-party Python components, with Cython only for measured hot paths |
| Networking | `httpx` and approved WebSocket adapter on background workers |
| Fonts | Bundled Inter and JetBrains Mono |
| Icons | Bundled Lucide-derived atlas |
| Golden-image comparison | Raylib image APIs plus first-party pixel comparison, optionally Cython-optimized after measurement |

Do not add another UI or charting framework. All Raylib and Raygui calls remain
on the locked UI OS thread.

## Development and Testing

| Responsibility | Technology |
|---|---|
| Formatting | `ruff format --check` |
| Linting | `ruff check` |
| Type checking | `pyright` |
| Unit and integration tests | `pytest` |
| Property testing | `hypothesis` |
| Benchmarks | `pytest-benchmark` for hot paths and migration comparisons |
| Vulnerability analysis | `pip-audit` or `osv-scanner` |
| Cython validation | Windows extension build and focused import/runtime tests |
| Coverage | `pytest-cov` after admission |
| Timeouts and leak checks | Test deadlines plus first-party process/thread lifecycle assertions |

Do not add a second test runner, assertion framework, mocking framework, linter
aggregator, or code generator unless this document is revised.

## Dependency Admission

A new direct dependency requires:

1. A responsibility not reasonably covered by the standard library or approved stack.
2. Successful installation, native build if applicable, import, and tests on Windows.
3. Process, thread, native-library, cancellation, and determinism verification where applicable.
4. Windows support and an acceptable license.
5. A typed public API or narrow typed adapter.
6. Tests for failure, cancellation, and invalid input boundaries.
7. Updated `pyproject.toml`, `uv.lock`, and this document.
8. An ADR when it changes a foundational architectural choice.

The accepted compatibility spike chose exact versions recorded only in the
lock and project files. Dependency changes must rerun the applicable gate.

## Windows Compatibility Spike Acceptance

The initial Windows compatibility spike passed before `pyproject.toml` and
`uv.lock` selected the runtime and native versions. Its structured report
records the CPython version, Windows architecture, Python Raylib/Raygui binding
pair, compiler/toolchain, native dependency behavior, and locked versions.

The gate rejects every interpreter except regular Windows AMD64 CPython 3.14.2.
Every native dependency requires a validated `cp314` wheel or approved Windows
source build. The gate imports Raylib/CFFI, NumPy, PyArrow, Cython, Pydantic
Core, and later scientific/ML libraries and records versions plus pool,
cancellation, and shutdown behavior. A free-threaded or wrong-patch CPython
fails startup.

On the target Windows environment, the spike must prove all of the following:

- Raylib and Raygui ownership remains on one locked UI OS thread;
- a hidden smoke launch captures a frame successfully;
- the bundled Inter/JetBrains Mono fonts and Lucide-derived icon atlas load;
- the Cython extension builds and imports; and
- cleanup succeeds without leaked UI or worker lifecycle ownership.

A future unsuccessful compatibility rerun must not select a new binding pair or
lock new exact versions; record the failure evidence and continue evaluation.

## Market-calendar admission gate

`exchange_calendars` is a named candidate, not an approved application
dependency until it satisfies Dependency Admission and is declared in
`pyproject.toml` and `uv.lock`. On exact regular Windows AMD64 CPython 3.14.2,
the gate must install/import the resolved dependency and prove through the typed
adapter that supported US-equity calendars return correct normal sessions,
weekends, holidays, daylight-saving transitions, and early closes for 1m, 5m,
15m, 1h, and 1d ingestion. It must also verify deterministic results, bounded
use, cancellation/shutdown behavior where applicable, acceptable licensing,
and absence of an incompatible native/transitive dependency.

Real Data Ingestion cannot be accepted if this gate fails. A failure must not
silently substitute ad hoc holiday tables or downgrade regular-session
validation; record the evidence and leave the dependency unadmitted.
