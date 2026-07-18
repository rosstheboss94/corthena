# Phase 0 Windows Compatibility Evidence (Superseded)

This CPython 3.13 GIL-enabled evidence is retained as superseded historical baseline only.
The current candidate and evidence are recorded in
`phase0-cp314t-compatibility.md`.

**Status:** Passed on 2026-07-12  
**Host:** Windows 11 Home 10.0.26200, AMD64  
**Selected runtime:** CPython 3.13.9  
**Environment tool:** uv 0.9.24  
**Compiler:** MSYS2 MinGW-w64 GCC 15.2.0  
**Native UI pair:** raylib Python CFFI binding 6.0.1.0, including its Raygui API  

The isolated pre-selection spike passed before `pyproject.toml` and `uv.lock`
were created. It proved a typed Cython extension build/import, staged bundled
Inter and JetBrains Mono fonts plus the Lucide atlas, one-thread Raylib/Raygui
ownership, a hidden PNG frame capture, cleanup, a loopback FastAPI/httpx HTTP
handshake, a WebSocket event handshake, bounded shutdown without a leaked
server thread, CSV/Parquet and Arrow IPC round trips, SQLite WAL with a
read-only reader, and a published read-only NumPy memory map.

Asset SHA-256 values observed by the passing spike:

- Inter Variable: `4989b125924991b90d05b2d16e0e388c48f7d5bb8b30539bbf9c755278d0ccaf`
- JetBrains Mono Regular: `a0bf60ef0f83c5ed4d7a75d45838548b1f6873372dfac88f71804491898d138f`
- Lucide atlas: `ba3ad4e3426424e315ad71d44cd5293f457eb137240d6a47168e43bc2d1b7217`

Unsupported toolchain evidence: no MSVC installation providing the x64 C/C++
tools was detected. The selected MinGW-w64 toolchain successfully produced and
imported a `cp313-win_amd64` extension linked against CPython 3.13.

The repository gate is `corthena-phase0-gate`. Quality results are recorded in
the implementation handoff; resolved dependency versions are owned by
`uv.lock`.
