# Corthena

Corthena is a Windows-first trading research workstation being migrated to a
typed Python/Cython modular monolith. The Python implementation currently
contains the Phase 0 compatibility scaffold: exact-runtime validation, a Cython
extension, Raylib/Raygui smoke capture, loopback HTTP/WebSocket checks, and
storage compatibility checks.

The workstation, coordinator, worker, and research CLI entry points are not yet
implemented in this migration tree. The currently runnable entry point is the
Phase 0 compatibility gate described below. See
[`specs/roadmap.md`](specs/roadmap.md) for implementation status.

## Requirements

- Windows 11 on AMD64
- [`uv`](https://docs.astral.sh/uv/)
- Exact regular CPython `3.14.2` (`cp314-win_amd64`)
- A Windows C compiler compatible with the configured Cython build
- A working desktop/OpenGL environment for the Raylib smoke gate

Free-threaded CPython and every Python patch version other than `3.14.2` are
unsupported. The required version is pinned in [`.python-version`](.python-version).

## Setup

Open PowerShell in this directory:

```powershell
cd C:\Users\torra\Desktop\Projects\corthena\python_migration
```

Install the pinned Python runtime through `uv`, then create the local virtual
environment and install runtime plus development dependencies:

```powershell
uv python install 3.14.2
uv sync --all-extras --python 3.14.2
```

You do not need to activate `.venv`; `uv run` executes commands in the managed
environment. To confirm the selected interpreter and ABI:

```powershell
uv run python -c "import platform, sysconfig; print(platform.python_version()); print(sysconfig.get_config_var('SOABI'))"
```

Expected output includes `3.14.2` and `cp314-win_amd64`.

## Run the current project

Run the Phase 0 compatibility gate:

```powershell
uv run corthena-phase0-gate
```

The gate validates the runtime and approved native imports, imports the Cython
extension, opens a hidden Raylib window on the UI thread, loads bundled assets,
captures a smoke frame, exercises loopback HTTP/WebSocket behavior, verifies
SQLite/Arrow/memory-map storage, and cleans up its resources. It prints JSON
evidence to standard output and exits nonzero on failure.

There is currently no normal workstation launch command. Add one to
`[project.scripts]` in `pyproject.toml` when the corresponding implementation
phase lands, then document it here.

## Build

Build the source distribution and regular-ABI Windows wheel through the declared
PEP 517 build configuration:

```powershell
uv build
```

Build artifacts are written to `dist/`. On the supported runtime, the wheel is
tagged `cp314-cp314-win_amd64` and contains
`_compat.cp314-win_amd64.pyd`.

Do not invoke `setup.py` directly; build dependencies are intentionally managed
in an isolated environment by `uv build`.

## Tests and quality checks

Run the full test suite:

```powershell
uv run pytest
```

Run the standard local quality gate:

```powershell
uv lock --check
uv run ruff format --check
uv run ruff check
uv run pyright
uv run pytest
uv build
uv run pip-audit
```

Format source files after an intentional formatting change:

```powershell
uv run ruff format
```

Run a focused test file or test:

```powershell
uv run pytest tests\test_runtime.py
uv run pytest tests\test_runtime.py::test_supported_runtime_is_healthy
```

Tests using Hypothesis or `pytest-benchmark` are collected through `pytest` when
present. Benchmarks are required only for measured hot paths and migration
performance comparisons.

## Dependency and lockfile workflow

`pyproject.toml` declares direct dependencies and tool configuration;
`uv.lock` owns exact resolved versions. After an approved dependency change:

```powershell
uv lock
uv sync --all-extras
uv lock --check
```

Do not hand-edit `uv.lock`. New direct dependencies must first satisfy the
admission rules in [`specs/technology-stack.md`](specs/technology-stack.md).

## Recreate the environment

If the environment uses the wrong interpreter or becomes stale, remove only the
local `.venv` and recreate it:

```powershell
Remove-Item -LiteralPath .venv -Recurse -Force
uv sync --all-extras --python 3.14.2
```

## Project layout

```text
python_migration/
|-- src/corthena/       Python package and Cython adapter
|-- tests/              Runtime, native, UI, loopback, and storage tests
|-- specs/              Authoritative behavior and architecture specifications
|-- docs/               Compatibility evidence
|-- screenshots/        Migration visual references
|-- pyproject.toml      Package, dependencies, entry points, and tool settings
|-- uv.lock             Reproducible dependency resolution
|-- setup.py            Isolated Cython extension build definition
`-- .python-version     Exact supported Python version
```

## Development rules

This is a specification-first migration. Read [`AGENTS.md`](AGENTS.md) and the
route-specific owning specifications before changing behavior. In particular:

- Keep Raylib/Raygui calls on the locked UI OS thread.
- Keep blocking I/O, decoding, database, network, and training work off the
  render thread.
- Use bounded processes for pure-Python CPU parallelism and threads for I/O and
  orchestration.
- Publish shared arrays and buffers as read-only and preserve deterministic
  results across worker counts and completion orders.
- Use Cython only for measured hot paths or native adapters, with parity tests
  and benchmark evidence.

The existing Go implementation above this migration directory remains the
reference implementation until the Python rewrite reaches accepted parity.

## More documentation

- [`specs/README.md`](specs/README.md) — specification index
- [`specs/roadmap.md`](specs/roadmap.md) — delivery status and sequencing
- [`specs/python-migration.md`](specs/python-migration.md) — migration policy
- [`specs/technology-stack.md`](specs/technology-stack.md) — approved stack
- [`specs/quality.md`](specs/quality.md) — quality-gate index
- [`docs/phase0-cp314-compatibility.md`](docs/phase0-cp314-compatibility.md) —
  successful regular-CPython compatibility evidence
