# Corthena Migration Rules

Use `python_migration/AGENTS.md` as the authoritative routing index. Read the
owning specification rather than duplicating it here.

## Authoritative sources

- Route and ownership: `python_migration/AGENTS.md`,
  `python_migration/specs/README.md`, and relevant
  `python_migration/specs/routing/phase-*.md`.
- Migration and approved stack: `python_migration/specs/python-migration.md`,
  `python_migration/specs/migration-baseline.md`, and
  `python_migration/specs/technology-stack.md`.
- Quality and verification: `python_migration/specs/quality.md` plus applicable
  `python_migration/specs/quality-*.md` documents.
- Native UI: `python_migration/specs/ui/foundation.md`; use
  `python_migration/specs/ui/workspaces.md` or
  `python_migration/specs/ui/visualization.md` when those behaviors are involved.

## Non-negotiables

- Keep the pre-migration root implementation available as the parity reference
  until Python rewrite parity is accepted.
- Use CPython and `uv`; use Cython only for measured hot paths or native
  adapters; admit dependencies only through the approved-stack process.
- Keep all Raylib/Raygui calls on one locked UI OS thread; keep blocking work
  off the render thread.
- Validate DTOs before domain conversion; contain weak/native values in typed
  adapters; keep public boundaries explicit and typed.
- Preserve deterministic results, prevent future-data leakage, and make
  completed runs, artifacts, and published buffers immutable.
- Pass cancellation through blocking boundaries; define process, thread, queue,
  and mutable-library-object ownership explicitly.
