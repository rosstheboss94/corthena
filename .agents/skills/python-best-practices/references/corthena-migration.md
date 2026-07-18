# Corthena Migration Rules

Use `AGENTS.md` as the authoritative routing index. Read the
owning specification rather than duplicating it here.

## Authoritative sources

- Route and ownership: `AGENTS.md`,
  `specs/README.md`, and relevant
  `specs/routing/phase-*.md`.
- Migration and approved stack: `specs/python-migration.md`,
  `specs/migration-baseline.md`, and
  `specs/technology-stack.md`.
- Quality and verification: `specs/quality.md` plus applicable
  `specs/quality-*.md` documents.
- Native UI: `specs/ui/foundation.md`; use
  `specs/ui/workspaces.md` or
  `specs/ui/visualization.md` when those behaviors are involved.

## Non-negotiables

- Treat the root Python/Cython implementation and `tests/goldens/` evidence as
  authoritative after the accepted repository cutover.
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
