# Specification Change Recording

**Status:** Missing from authoritative specifications  
**Recorded:** 2026-07-13  
**Affected:** `AGENTS.md`, `specs/README.md`, `specs/design-pattern.md`, specification maintenance workflow

## What changed

Agents no longer update an owning specification automatically when behavior or
a public contract changes. They record the divergence in a concise Markdown
note under `specs/missing/` instead.

## Why

Specification changes should remain an explicit user decision. Recording what
changed and why preserves visibility without silently rewriting the
authoritative requirements to match an implementation change.
