# Phase 0 `cp313t` Compatibility Evidence

**Status:** Failed; Phase 0 remains in progress  
**Observed:** 2026-07-12 on Windows AMD64

The exact `3.13.9+freethreaded` interpreter installed successfully and reports:

- `sys._is_gil_enabled() == False`
- `Py_GIL_DISABLED == 1`
- `SOABI == "cp313t-win_amd64"`

`uv sync --all-extras --python 3.13.9+freethreaded` cannot create the approved
environment. `raylib==6.0.1.0` requires `cffi==2.1.0`; CFFI's build backend
fails before compilation with: `CFFI does not support the free-threaded build
of CPython 3.13. Upgrade to free-threaded 3.14 or newer to use CFFI with the
free-threaded build.` Python 3.14 is outside the approved exact-runtime scope.

The first Cython `cp313t` build exposed setuptools' MinGW selection of
`python313` instead of the installed `python313t` import library. The local
build command now corrects that ABI library name. The extension subsequently
built as `_compat.cp313t-win_amd64.pyd`; concurrent invocation remains part of
the focused test evidence.

No Raylib/CFFI version was substituted, no unsupported dependency was locked,
and Phase 0 was not marked complete. The full UI, storage, loopback, native
import, degraded-GIL, and lifecycle gate remains blocked on an approved
CPython-3.13t-compatible Raylib binding path.
