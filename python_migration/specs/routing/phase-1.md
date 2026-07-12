# Phase 1 Task Route

Non-authoritative navigation aid; canonical behavior remains in linked specs.

- Required: `AGENTS.md`, Phase 1 in `roadmap.md`, `design-pattern.md`,
  `technology-stack.md`, `quality.md`, `quality-common.md`,
  `frontend/foundation.md`, and `frontend/foundation-shell-state.md`.
- Conditional: read `api.md` only if a public or process boundary is
  deliberately introduced. Do not read workspace or visualization specs for
  the empty scaffold.
- Implement with `$build-corthena-raylib-frontend`, then apply
  `$python-best-practices`. Verify with `$python-windows-compat-gate`, then use
  `$review-corthena-code` for the final specification and implementation audit.
- Scope: add the named workstation project entry point and the smallest owned
  `corthena.frontend` package needed for an empty Raylib/Raygui frame loop;
  validate bundled Inter and JetBrains Mono fonts and Lucide-derived icon data
  before native initialization; contain native values in a typed adapter; lock
  the UI OS thread before initialization and check ownership before every
  native call; render at least one frame through a bounded smoke launch; and
  perform deterministic, idempotent cleanup.
- Keep the entry point small. Put startup, lifecycle, asset validation, and
  native behavior in focused owned packages. Test asset failures, owner-thread
  and wrong-thread checks without off-thread Raylib calls, native-value
  containment, and cleanup.
- Exclude: Phase 2+ typed application state, effects runtime, simulator,
  docking, workspaces, charts, tables, persistence, coordinator/client
  behavior, and all domain workflows.
- Completion evidence: the named entry point and frontend package exist; source
  and tests compile; the bounded smoke launch proves initialization, at least
  one frame, and clean shutdown; focused tests pass; and all applicable
  configured Ruff, Pyright, pytest, vulnerability, and Windows compatibility
  checks pass. Keep Phase 1 Pending if any condition is missing or skipped.
