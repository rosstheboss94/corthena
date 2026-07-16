# Add agent-facing UI contract packages

**Status:** Missing from authoritative specifications
**Recorded:** 2026-07-13
**Affected:** `src/corthena/ui/client`, `src/corthena/ui/native`, `specs/contract.md`

## What changed

The UI snapshot-client and native-adapter boundaries now expose canonical
`protocol.py` modules with `Protocol`-suffixed contract names. Native-free
adapter values live in `native/models.py`, and the concrete Raylib adapter has
a signature-only `raylib.pyi` interface. Existing package-level import names
remain available as compatibility aliases.

## Why

The agent-facing contract rules require capability contracts to be isolated
from concrete implementations and concrete APIs to have a compact context
surface when directly constructed.
