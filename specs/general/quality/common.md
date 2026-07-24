# Common Quality Gates

Use `ruff format --check`, `ruff check`, `pyright`, `pytest`, `hypothesis`,
`pytest-benchmark` for hot paths, vulnerability scanning with `pip-audit` or
`osv-scanner`, and Windows Cython extension builds. Do
not add another runner, linter aggregator, mocking, or assertion framework
without revising the technology stack.

Source and tests must compile; exported identifiers need documentation and
ownership. Prefer concrete immutable boundary structs and consumer-owned
interfaces. Avoid `Any`, untyped dict payloads, reflection-style models, and
unchecked casts. Validate unknown-field JSON before domain conversion, preserve
typed error causes, and make resource ownership/cancellation/idempotent
cleanup explicit. Keep native values, Cython pointers, Arrow builders, SQLite
rows, tensors, library estimators, and Windows handles in adapters.

The Python migration Windows gate covers approved dependencies,
Raylib/Raygui UI-thread window/control, CSV/Parquet and Arrow IPC round trips,
SQLite WAL and readers, mapped typed matrices, loopback/WebSocket/client/worker
handshake, Cython build/import checks, and all applicable quality tools. Exact
resolved versions live only in `uv.lock`.

The gate runs in a clean regular `3.14.2` environment, rejects
`Py_GIL_DISABLED=1`, verifies the `cp314` extension tag, and imports every
approved native dependency. It tests process and native-library pool bounds
against coordinator CPU leases, including nested pools and stable reductions,
and rejects free-threaded or wrong-version CPython.


**Status:** Authoritative
**Owner:** Engineering
**Last updated:** 2026-07-23
**Related:** [Quality index](README.md), [General index](../README.md)
