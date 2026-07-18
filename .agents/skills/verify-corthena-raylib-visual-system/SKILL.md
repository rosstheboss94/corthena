---
name: verify-corthena-raylib-visual-system
description: Verify or review Corthena Raylib shell, workspace, control, docking, chart, table, modal, notification, interaction-state, or responsive visual changes. Use to audit visual-system compliance, accessibility, clipping, scaling, stable draw order, and canonical golden-image evidence.
---

# Verify Corthena Raylib Visual System

Audit visual changes against the authoritative system and canonical PNG matrix.
Treat missing evidence as a verification failure, not permission to infer pass.

## Establish the contract

1. Read `AGENTS.md`,
   `specs/design-pattern.md`,
   `specs/ui/raylib-visual-system.md`, the owning
   subsystem specification, and `specs/quality.md`.
2. Locate the applicable canonical manifest and PNG family under
   `tests/goldens/`. Record its viewport, scale, scenario,
   seed, clock, fixture/state, layout, asset fingerprint, backend, build
   identity, channel tolerance, and maximum differing-pixel ratio.
3. Inspect the change and workspace without modifying implementation. Use JPEG
   screenshots only for design intent, never as baselines.

## Audit implementation

- Find raw or duplicated colors, typography, spacing, geometry, and state style
  outside the centralized token/native-style boundary.
- Check Inter/JetBrains Mono roles, logical sizes, the 4 px grid, one-pixel
  separators, compact hierarchy, restrained fills, icon and numeric alignment.
- Check default, hot, focused, active/pressed, selected, checked,
  indeterminate, disabled, loading, empty, failure, degraded, and recovered
  states where applicable. Reject color-only status and invisible focus.
- Check live viewport metrics, DPI × preset scaling applied once, pixel
  snapping, compact/full navigation, minimum extents, overflow, hit testing,
  and clipping at both required viewports and every recorded scale.
- Check modal focus trapping and inert background, transient layering, toast
  placement, table virtualization, chart LOD meaning, tooltip containment, and
  balanced clip/scissor scopes.
- Check immutable typed render-neutral view models, typed actions, native-value
  containment, locked UI-thread calls, no blocking draw work, and deterministic
  stable draw order.
- Confirm unrelated visuals remain unchanged unless an explicitly approved
  design change updates the owning spec and affected manifest entries.

## Verify canonical goldens

Run every applicable manifest case at its recorded viewport, scale, seed,
scenario clock, fixtures/state, layout revision, assets, backend, build inputs,
and hidden-frame configuration. Compare decoded RGBA output using the exact
recorded channel tolerance and maximum differing-pixel ratio.

Reject manual screenshot waivers, JPEG baselines, edited captures, skipped
comparisons, missing state cases, nondeterministic inputs, raw style duplication,
and tolerance inflation. Classify failures as implementation drift,
nondeterminism, environment/asset mismatch, or intentional design change. An
intentional change passes only after explicit approval and coordinated
owning-spec, manifest, and PNG updates.

## Report findings

Lead with actionable defects ordered by severity and cite files and lines.
Then list the manifest matrix exercised, commands run, and checks that could not
run. State whether token use, accessibility, responsive behavior, native
containment, draw order, and all golden comparisons passed. Do not claim overall
compliance while a required case is skipped or failing.
