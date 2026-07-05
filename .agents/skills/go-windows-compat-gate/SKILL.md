---
name: go-windows-compat-gate
description: Validate this repository's Go toolchain and Windows-native compatibility gate. Use when implementing or rerunning roadmap Phase 0, verifying the Phase 1 frontend scaffold, changing the Go toolchain or approved dependencies, diagnosing cgo or Windows compiler failures, or verifying Raylib/Raygui, Arrow/Parquet, SQLite, memory mapping, loopback networking, WebSockets, worker pipes, commands, and Go quality tools together.
---

# Go Windows Compatibility Gate

Run the compatibility spike as an evidence-producing gate. Keep all repository code, tests, tools, and extensions in Go, and stop on unsupported combinations rather than silently changing the approved architecture.

## Ground the task

1. Read the repository `AGENTS.md`.
2. Read `specs/README.md`, the relevant Phase 0 or Phase 1 section of
   `specs/roadmap.md`, `specs/technology-stack.md`, and `specs/quality.md`.
3. Read an additional owning specification only when the requested work reaches that boundary:
   - Read `specs/system-architecture.md`, `specs/decisions/0005-go-hybrid-concurrency.md`, and `specs/api.md` for worker-process or public protocol changes.
   - Read `specs/frontend/foundation.md` for Raylib/Raygui application-shell or UI-thread changes.
4. Treat specifications as canonical. Report conflicts between specifications, project files, and observed tool behavior instead of choosing silently.
5. Do not load unrelated specifications or inspect `screenshots/`.

## Establish the gate

Before changing files:

- Inspect the current repository state and preserve unrelated changes.
- Check the installed Go version and relevant `go env` values, including `GOOS`, `GOARCH`, `CGO_ENABLED`, and `CC`.
- Confirm that the selected Go version matches the canonical specification exactly.
- Detect the Windows C compiler and native prerequisites without changing the machine.
- Build a result matrix covering every applicable Phase 0 check and any Phase 1 scaffold checks with prerequisite, verification method, result, and failure evidence.
- Do not invent project setup, test, lint, or launch commands until the files that define them exist.

When scaffolding is part of the request:

- Record the exact selected Go patch version in the `go.mod` `go` directive.
  Do not add a redundant same-version `toolchain` directive.
- Admit only direct dependencies approved by `specs/technology-stack.md`.
- Verify current version compatibility from authoritative upstream sources; do not select versions from memory.
- Record exact Go module and Go tool versions only in `go.mod` and `go.sum`.
- Keep native and weakly typed APIs behind narrow typed adapters.
- Add the smallest Go implementation and tests that exercise the gate; do not introduce another runtime, build system, test runner, linter aggregator, or scripting language.

## Verify compatibility

Exercise each applicable boundary:

- Compile all commands and approved direct dependencies.
- For the Phase 1 scaffold, verify bundled assets load, native values remain
  inside typed adapters, the empty workstation initializes and shuts down on
  its locked UI thread, and wrong-thread calls fail before reaching Raylib.
- Open and close a minimal Raylib window and use one Raygui control. Lock the goroutine to its OS thread before initialization and keep every Raylib/Raygui call on that thread.
- Round-trip typed sample data through CSV-to-Parquet and Arrow IPC with Zstandard, checking schema, values, nulls, and reopened output.
- Enable SQLite WAL mode, apply numbered migrations, and verify concurrent readers with one coordinator-owned writer.
- Map a typed little-endian matrix file, run parallel read-only work, write only task-owned output ranges, flush, close, reopen, and compare values.
- Start loopback HTTP and WebSocket endpoints with explicit deadlines, cancellation, channel ownership, shutdown, and leak checks.
- Exercise the worker-pipe handshake and its malformed-message, cancellation, peer-exit, and closure paths.
- Run `gofmt`, compilation, `go test`, `go vet`, Staticcheck, `govulncheck`, and applicable race-enabled tests after their project configuration exists.

Use deterministic fixtures, bounded waits, temporary directories, and explicit cleanup. Never perform filesystem, database, network, or training work on the Raylib render thread. Do not weaken tests, skip required checks, or replace an approved dependency merely to make the gate pass.

## Conclude the gate

Pass only when every required row in the result matrix succeeds on the selected Windows environment.

- On success, report the observed Go, module, tool, C compiler, Windows, and architecture versions; list executed checks and their results; and identify created or changed files.
- On failure, stop dependent work and report the exact failed combination, command or action, relevant output, and the next bounded investigation.
- Update a living specification only when required behavior or a public contract changes. Add an ADR only for a lasting decision with meaningful alternatives.
- Do not claim success for checks that were skipped, unavailable, or performed only by static inspection.
