# Phase 0 Task Route

Non-authoritative navigation aid; canonical behavior remains in linked specs.

- Required: `AGENTS.md`, Phase 0 in `roadmap.md`, `design-pattern.md`,
  `python-migration.md`, `technology-stack.md`, `quality.md`,
  `quality-common.md`, `system-architecture.md`,
  `decisions/0008-regular-cpython-concurrency.md`, `frontend/foundation.md`,
  and `api.md`.
- Conditional: read the focused frontend, API, or quality specification when
  the compatibility evidence exercises its owned behavior.
- Scope: collect Windows compatibility-spike evidence for exact Windows AMD64
  regular CPython `3.14.2`, its `cp314` ABI, `uv`,
  Windows architecture, compiler/toolchain, approved Python Raylib/Raygui
  bindings, a typed Cython extension, locked UI-thread ownership, bundled
  assets, hidden-frame capture, a loopback HTTP/WebSocket handshake, and
  lifecycle cleanup.
- Command and version restrictions: do not introduce commands or select exact
  package versions. Create `pyproject.toml` and `uv.lock`, and record exact
  versions in them, only after the compatibility spike passes. Record
  unsupported combinations as failure evidence.
- Exclude: Phase 1+ shell or workspace behavior; coordinator, worker, data,
  experiment, model, inference, and real-backend implementation work.
