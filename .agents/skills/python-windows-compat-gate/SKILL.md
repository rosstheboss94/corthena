---
name: python-windows-compat-gate
description: Validate Corthena's CPython/Cython toolchain, approved dependencies, Windows-native adapters, Raylib/Raygui bindings, storage, IPC, and quality gate.
---

# Python Windows Compatibility Gate

Run the Windows compatibility spike as an evidence-producing gate. Keep runtime
code, tools, tests, and extensions within the approved Python/Cython stack;
record unsupported combinations rather than silently changing architecture.

## Ground the gate

1. Read `python_migration/AGENTS.md`, `python_migration/specs/README.md`, the
   relevant roadmap route, `technology-stack.md`, and `quality.md`.
2. Read `system-architecture.md`,
   `decisions/0005-python-process-concurrency.md`, and `api.md` for process or
   public-protocol changes. Read `frontend/foundation.md` for shell or UI-thread work.
3. Treat `python_migration/specs/` as authoritative. Do not select versions or
   create a `pyproject.toml`/`uv.lock` until the spike passes.

## Prove compatibility

- Record Windows version and architecture, selected CPython candidate, `uv`,
  compiler/toolchain, Python Raylib/Raygui binding pair, and native dependency
  behavior. Select exact versions only in `pyproject.toml` and `uv.lock` after success.
- Admit only direct dependencies approved by `technology-stack.md`. Keep native,
  weakly typed, and library-specific values behind typed adapters.
- Build and import a minimal Cython extension. Prove fallback or failure behavior
  and keep its Python-facing API typed and stable.
- Open and close a minimal Raylib window, use one Raygui control, and capture a
  hidden smoke frame. Lock one UI OS thread before initialization and keep all
  Raylib/Raygui calls on that thread.
- Load the bundled fonts and icon atlas before native initialization. Keep I/O,
  network, decoding, persistence, and training off the UI thread.
- Start loopback HTTP and WebSocket endpoints with explicit timeouts,
  cancellation, queue ownership, shutdown, and lifecycle-leak checks.

## Report the evidence

- Run only configured checks after the project configuration exists: `ruff format
  --check`, `ruff check`, `pyright`, `pytest`, applicable property/benchmark and
  vulnerability checks, plus Windows Cython build/import tests.
- On success, report observed Python, `uv`, binding, compiler, Windows, and
  architecture versions; executed checks; results; and changed files.
- On failure, preserve the failure evidence. Do not lock versions, claim an
  unsupported binding pair works, or substitute another framework.
