---
name: build-corthena-raylib-ui
description: Build or change Corthena's Windows Raylib ui scaffold, adapters, UI-thread ownership, frame loop, assets, or shell launch.
---

# Build Corthena Raylib UI

Implement ui changes without leaking native types, moving domain behavior
into rendering code, or violating the locked-thread and asynchronous-work
boundaries.

## Ground the change

1. Read `AGENTS.md`.
2. Read the requested phase in `specs/general/roadmap.md`,
   `specs/general/technology-stack.md`, `specs/general/quality/README.md`, and
   `specs/general/ui/README.md`.
3. Read `specs/general/ui/workspaces.md` or
   `specs/general/ui/visualization.md` only when the task reaches those
   behaviors. Read `specs/general/api.md` for public client or process-boundary work.
4. Inspect `screenshots/` only for an explicitly visual-design task.
5. Inspect the current workspace and preserve unrelated changes.
6. Treat living specifications as canonical and report conflicts rather than
   choosing silently.

## Keep the scaffold strict

- Create only commands and package ownership required by the current roadmap
  phase. For Phase 1, create the named workstation project entry point and the
  smallest owned `corthena.ui` package needed for an empty frame loop.
  Do not implement Phase 2+ state, effects, simulator, docking, workspace,
  visualization, persistence, client, or domain behavior.
- For Phase 12, create the Python command/package surfaces defined by the
  technology stack, but do not implement later shell, docking, simulator, or
  domain behavior.
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

## Build the Phase 1 workstation

- Treat `specs/history/routing/phase-1.md` as the Phase 1 reading and evidence map.
- Add one named workstation project script whose callable delegates immediately
  to owned ui startup code. Do not reuse the Phase 0 compatibility-gate
  command as the workstation entry point.
- Put Raylib, Raygui, and Windows conversions behind a narrow typed native
  adapter. Keep native structs, handles, pointers, and weakly typed binding
  values out of the entry point and ui state.
- Validate bundled Inter and JetBrains Mono fonts, Lucide-derived icon data,
  and applicable license notices before any native initialization.
- Make the empty loop bounded under smoke-test configuration, render at least
  one frame, and clean up deterministically on success, initialization failure,
  frame failure, and cancellation. Cleanup must be safe to call once ownership
  has been acquired and must not hide the original failure.
- Do not add queues, background work, or state/effect abstractions unless the
  empty scaffold actually needs them. If it does, define bounded ownership,
  cancellation, backpressure, and shutdown without implementing Phase 2
  behavior.

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
