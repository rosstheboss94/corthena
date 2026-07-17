# Phase 7 UI client workflows

**Status:** Missing from authoritative specifications
**Recorded:** 2026-07-15
**Affected:** `corthena.ui.client.UIClient`, simulator and effects runtime, Data and Experiments workspaces

## What changed

The internal UI client contract now exposes typed operations for loading the
Phase 7 snapshot, importing data, evaluating an experiment draft, autosaving a
draft revision, and submitting an immutable experiment definition.

## Why

Data and Experiments panels require the same simulator-replaceable asynchronous
boundary, generation checks, cancellation, and deterministic replay behavior as
the existing UI workflows.
