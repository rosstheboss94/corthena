---
name: review-corthena-code
description: Review Corthena code, tests, specifications, diffs, commits, ranges, or workspace changes for actionable defects. Use when Codex is asked to perform a code review, audit an implementation, inspect a proposed change, check specification compliance, or assess correctness, safety, determinism, concurrency, Windows compatibility, API boundaries, Raylib UI-thread behavior, or test coverage in the Corthena repository.
---

# Review Corthena Code

Perform an evidence-based, read-only review. Prioritize defects that can affect correctness, safety, contracts, determinism, operability, or maintainability. Omit taste-level comments and do not modify files unless the user separately requests fixes.

## Establish the review target

1. Treat the directory containing the repository `AGENTS.md` and `specs/` as the workspace root.
2. Use an explicit user-supplied file set, diff, commit, or range when provided.
3. Otherwise inspect staged, unstaged, and untracked changes within the workspace root.
4. Confine every search, status, and diff operation to the workspace root. Never enumerate or review paths above it.
5. Detect the Git top level before relying on Git. If it is above the workspace root, use workspace-relative pathspecs for every Git command and ignore all reported paths outside the workspace. If Git cannot represent a useful workspace-local change set, request an explicit target instead of reviewing the parent repository.
6. Preserve unrelated changes and keep the review read-only. Do not run formatters or generators in write mode.

## Ground the review

1. Read the repository `AGENTS.md`.
2. Read `specs/quality.md` and only the owning specifications selected by the routing table in `AGENTS.md`.
3. Read `specs/README.md` when the change crosses subsystem boundaries or ownership is unclear.
4. Read `specs/technology-stack.md` for dependency, packaging, build, extension, or tooling changes.
5. Read `specs/api.md` for public or process-boundary changes, plus the owning domain specification.
6. Inspect `screenshots/` only for visual-design changes.
7. Treat living specifications as canonical. Report code/specification conflicts rather than silently choosing one.
8. Check whether changed behavior or a public contract also updates its canonical specification. Require an ADR only for a decision with meaningful alternatives and lasting consequences.

## Inspect the change

Read the complete target diff and enough surrounding code, tests, types, callers, and specifications to establish behavior. Trace affected data and control flow across package or process boundaries. Check for missing changes outside the edited lines.

Apply the checks relevant to the target:

- Preserve future-data isolation in features, targets, splits, evaluation, and execution timing.
- Preserve deterministic results across goroutine counts, worker counts, task completion orders, serialization, seeds, clocks, and ID sources.
- Verify goroutine ownership, termination, cancellation, channel closure and buffering, lock boundaries, immutable shared inputs, and task-owned mutable outputs.
- Keep Raylib and Raygui calls on the locked UI OS thread. Keep filesystem, database, network, decoding, simulation, and training work off the render thread.
- Keep completed runs and model artifacts immutable.
- Enforce concrete typed models, validated DTOs, explicit serialized fields, exhaustive enum handling, contextual errors, and machine-testable causes.
- Keep native, `unsafe`, Arrow, SQLite, Windows handle, and weakly typed conversions inside narrow adapters.
- Verify resource ownership, idempotent cleanup, bounded waits, deadlines, and failure behavior.
- Enforce the approved Go-only stack and dependency responsibilities. Flag unapproved dependencies, external runtime ML, hidden thread pools, and duplicate frameworks.
- Check backward compatibility, storage and artifact versioning, migrations, protocol framing, and rejection of corrupt or incompatible inputs when applicable.
- Check tests for the changed behavior, failure paths, leakage, cancellation, determinism, races, goroutine leaks, and UI-thread enforcement according to risk.
- Verify that domain behavior does not move into UI render functions.

Report a finding only when the change introduces or exposes a concrete problem and the evidence supports it. Do not report speculative concerns, generic hardening ideas, pre-existing defects unrelated to the target, or stylistic preferences.

## Verify proportionally

Run focused, non-mutating checks that materially confirm or reject suspected defects. Use only commands defined by project files that currently exist.

- Run owning package tests for implementation changes.
- Add race-enabled tests for concurrency-sensitive packages.
- Add broader compilation, `go vet`, Staticcheck, vulnerability, integration, determinism, or UI checks only when the changed surface warrants them and their configuration exists.
- Use diff/check modes for formatting tools; never rewrite reviewed files.
- Record commands that could not run and why. Do not claim unexecuted checks passed.
- Do not invent setup, test, lint, build, or launch commands while the repository remains specification-only.

## Report findings first

Order findings by severity:

- `P0`: Causes catastrophic loss, compromise, or a broadly unusable system and requires immediate blocking.
- `P1`: Causes incorrect results, data loss, leakage, deadlock, serious races, broken contracts, or a major feature failure.
- `P2`: Causes a bounded functional, reliability, compatibility, or maintainability defect that should be fixed.
- `P3`: Causes a low-impact but concrete defect with a clear correction.

For each finding:

1. Write a concise imperative title prefixed with its priority.
2. Link the smallest useful file and line location in the changed code.
3. Explain the failure condition and user or system impact.
4. Cite the relevant code path, specification rule, or verification evidence.
5. Give a bounded remediation direction without supplying an unrelated redesign.

After findings, list verification performed and residual risks or untested areas. If no qualifying findings exist, state that explicitly and still report verification limits. Keep summaries secondary to findings.
