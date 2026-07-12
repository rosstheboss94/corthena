# Superseded Phase 0 `cp314t` Compatibility Evidence

**Status:** In progress  
**Observed:** 2026-07-12 on Windows AMD64

The selected exact runtime is CPython `3.14.2+freethreaded` with the
`cp314t-win_amd64` ABI. This removes CFFI's explicit rejection of free-threaded
CPython 3.13.

The approved `raylib==6.0.1.0` package has no Windows `cp314t` wheel. Its PyPI
source archive begins a CFFI source build but omits the native `raylib-c`,
`raygui`, and `physac` submodule contents required by its own include paths, so
the build fails at `#include "raylib.h"`. An exact upstream Git checkout was
also evaluated, but recursive submodule retrieval did not complete within the
compatibility-spike deadline and was not selected as a dependency source.

Phase 0 remains in progress until the approved binding has a reproducible
Windows `cp314t` build. No alternate UI framework or floating source dependency
was substituted.

## Passing evidence

- CFFI 2.1.0, NumPy 2.5.1, PyArrow 23.0.1, Cython 3.2.8, Pydantic Core
  2.46.4, and the Corthena extension import without enabling the GIL.
- The Cython extension builds as `_compat.cp314t-win_amd64.pyd`, declares
  `Py_MOD_GIL_NOT_USED`, and passes concurrent invocation.
- Four CPU-bound Python threads produce a CPU/wall ratio of approximately
  3.98 with the GIL disabled.
- `-X gil=1` reports degraded health and emits the required warning.
- Ruff format/lint, focused Pyright, nine non-UI tests, lock validation, and
  the installed-environment vulnerability audit pass.

## Remaining gate

The full `uv sync`, full Pyright/pytest run, incremental `pyray` GIL audit,
hidden Raylib/Raygui capture, asset load, and UI lifecycle cleanup remain
blocked by the binding source-build failure above.
