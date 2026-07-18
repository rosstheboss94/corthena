# Phase 0 Regular `cp314` Compatibility Evidence

**Status:** Complete  
**Runtime target:** regular Windows AMD64 CPython `3.14.2`  
**ABI target:** `cp314-win_amd64`

ADR 0008 supersedes the prior free-threaded compatibility selection. A clean
environment was recreated and the regular-ABI gate passed on 2026-07-12.

Required evidence includes a clean `uv sync`, lockfile validation, approved
native dependency imports, a `_compat.cp314-win_amd64.pyd` build/import, Python
fallback parity and deterministic concurrent calls, hidden Raylib/Raygui capture
with bundled assets and UI-thread ownership, loopback HTTP/WebSocket lifecycle,
storage and memory-map checks, cancellation and cleanup, bounded process/native
library pools under CPU leases, Ruff, Pyright, pytest/property/benchmark gates,
vulnerability scanning, and leak-free shutdown.

## Observed environment

- CPython `3.14.2`, regular build (`Py_GIL_DISABLED=0`)
- ABI/platform: `cp314-win_amd64` / `win_amd64`
- Windows `11` build `10.0.26200`, AMD64
- `uv 0.9.24`
- Extension: `_compat.cp314-win_amd64.pyd`
- Native imports: CFFI `2.1.0`, NumPy `2.5.1`, PyArrow `23.0.1`, Cython
  `3.2.8`, Pydantic Core `2.46.4`, and Raylib binding `6.0.1.0`

## Executed evidence

- `uv sync --all-extras --python 3.14.2` recreated `.venv` and installed the
  regular-ABI lock successfully.
- `uv lock --check`, `ruff format --check`, `ruff check`, and strict `pyright`
  passed.
- `pytest` passed all 9 collected tests, including runtime rejection, regular
  Cython build/fallback/concurrent-call behavior, loopback lifecycle, storage,
  memory map, Arrow, and hidden UI ownership/capture.
- `corthena-phase0-gate` reported healthy runtime/resource limits, imported all
  approved native dependencies, loaded the three bundled assets, captured and
  cleaned up the hidden Raylib window, completed HTTP/WebSocket and storage
  probes, and imported the regular Cython extension.
- `pip-audit` found no known vulnerabilities; the local unpublished `corthena`
  package was reported as not auditable through PyPI.

There are no admitted `nogil` performance kernels yet, so no hot-path benchmark
claim or cancellation-boundary benchmark applies to this compatibility probe.
