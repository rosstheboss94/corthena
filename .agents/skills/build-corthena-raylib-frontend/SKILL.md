---
name: build-corthena-raylib-frontend
description: Build or change Corthena's Windows Raylib frontend scaffold, adapters, UI-thread ownership, frame loop, assets, or shell launch.
---

# Build Corthena Raylib Frontend

Implement frontend changes without leaking native types, moving domain behavior
into rendering code, or violating the locked-thread and asynchronous-work
boundaries.

## Ground the change

1. Read `python_migration/AGENTS.md`.
2. Read the requested phase in `python_migration/specs/roadmap.md`,
   `python_migration/specs/technology-stack.md`, `python_migration/specs/quality.md`, and
   `python_migration/specs/frontend/foundation.md`.
3. Read `python_migration/specs/frontend/workspaces.md` or
   `python_migration/specs/frontend/visualization.md` only when the task reaches those
   behaviors. Read `python_migration/specs/api.md` for public client or process-boundary work.
4. Inspect `python_migration/screenshots/` only for an explicitly visual-design task.
5. Inspect the current workspace and preserve unrelated changes.
6. Treat living specifications as canonical and report conflicts rather than
   choosing silently.

## Keep the scaffold strict

- Create only commands and package ownership required by the current roadmap
  phase. For Phase 12, create the Python command/package surfaces defined by the technology stack,
  but do not implement later shell, docking, simulator, or domain behavior.
- Keep application implementation under `corthena/...`; expose only the
  deliberate `corthena.client` and `corthena.contracts` library surfaces when their owning phase
  requires them.
- Use only approved direct dependencies. Prefer the standard library and do not
  add a second GUI, charting, test, logging, routing, or build framework.
- Keep Raylib, Raygui, Arrow, file-dialog, and Windows values inside narrow
  typed adapters. Do not expose native handles, pointers, Arrow builders, or
  Raylib structs to domain or UI-state packages.
- Bundle approved fonts and icon data with Python package resources. Validate required
  assets before native initialization and preserve applicable license notices.
- Keep command entry points small. Put startup, lifecycle, and adapter behavior
  in owned packages with focused tests.

## Enforce UI-thread ownership

- Lock the workstation UI OS thread before Raylib initialization.
- Initialize, poll, draw, use Raygui, and shut down Raylib only on that
  thread.
- Record the owner Windows thread in the native adapter. Check ownership before
  every native call and fail before reaching Raylib when ownership is wrong.
- Test owner-thread and wrong-thread behavior without making an off-thread
  Raylib call.
- Keep filesystem, persistence, dialogs, network, Arrow decoding, simulation,
  and training work off the render thread.
- Exchange immutable typed messages through bounded queues. Make sender,
  receiver, closer, buffering, cancellation, and backpressure ownership
  explicit. Never block the render thread on a send.

## Verify the result

- Test pure package behavior without initializing Raylib where possible.
- Add focused adapter tests for native-value containment, resource cleanup,
  asset validation, and UI-thread enforcement.
- Compile every command and launch the empty workstation through a bounded
  smoke check. Confirm initialization, at least one frame, and clean shutdown.
- Hand off verification to the applicable verifier and apply the focused
  quality route. Use `$python-windows-compat-gate` for native, dependency,
  toolchain, adapter, or application-shell changes.
- Mark the roadmap phase complete only after every done condition and required
  gate passes. Report commands that could not run and do not claim skipped
  checks passed.
