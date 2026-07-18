# Raylib Visual System

**Status:** Authoritative  
**Owner:** UI  
**Last updated:** 2026-07-18
**Related:** [UI foundation](foundation.md), [Workspaces](workspaces.md), [Visualization](visualization.md), [Visualization quality](../quality-visualization.md), [Migration baseline](../migration-baseline.md)

This document owns the visual language for every Corthena Raylib shell,
workspace, control, dock, chart, table, modal, notification, interaction state,
and responsive layout. Owning subsystem specifications define behavior and
content; this document defines their shared visual presentation.

## Design authority

The accepted Python workstation and its retained cutover evidence are the
design authority. Canonical PNG files and adjacent JSON manifests under
`tests/goldens/*-golden/` are normative visual evidence.
Phase 1--9 JPEG files under `screenshots/` are design references
for hierarchy and intent; JPEG compression makes them unsuitable as baselines.

Preserve the dense, restrained, dark trading-workstation character.
Consistency, readability, visible focus, and accessibility refinements are
permitted only when they preserve product behavior. An intentional departure
from canonical design behavior requires explicit approval and a coordinated
update to this specification, the owning subsystem specification, and every
affected manifest entry and PNG. Never weaken tolerances to absorb drift.

## Tokens

All visual literals cross one centralized token/native-style boundary. Render
and workspace code consume named semantic tokens or shared primitives; they do
not repeat raw colors, font names, spacing values, or state styling.

### Color

| Token | Value | Required use |
|---|---:|---|
| `surface.canvas` | `#0B0D10` | Window canvas, deepest chart field |
| `surface.panel` | `#11151A` | Primary panels, bars, modal body |
| `surface.raised` | `#171C22` | Controls, headers, restrained selected fills |
| `border.divider` | `#252B33` | One-pixel separators, outlines, chart grid |
| `text.primary` | `#D6DCE5` | Primary labels and values |
| `text.muted` | `#7E8896` | Metadata and inactive navigation |
| `accent.cyan` | `#3CC8C8` | Active navigation, links, current context, focus |
| `accent.purple` | `#9B7CF6` | Model, experiment, and comparison identity |
| `semantic.positive` | `#4CC38A` | Success, gains, healthy/complete state |
| `semantic.negative` | `#EF6B73` | Errors, losses, destructive state |
| `semantic.warning` | `#D8B45A` | Warning, degraded, attention state |

Derive hover, pressed, disabled, selection, overlay, and chart-series variants
from these named colors at the centralized boundary. Prefer divider outlines
and restrained alpha fills over large saturated regions. Cyan identifies active
or linked context; purple identifies model or comparison context. Semantic
colors communicate meaning only and always pair with text, icon, pattern, or
shape so status is never color-only.

### Typography

- Use bundled Inter for navigation, labels, controls, prose, and headings.
- Use bundled JetBrains Mono for identifiers, timestamps, numeric values, table
  cells, axes, metrics, code-like expressions, and comparison data.
- Use logical text roles of 12 px for metadata/axes, 13 px for dense body and
  controls, 14 px for emphasized body and panel titles, and 18 px for workspace
  or modal titles at 100% effective scale.
- Preserve font asset fingerprints in golden metadata. Use tabular alignment
  for comparable numeric values and do not simulate weight by overdraw.
- Scale from logical roles; do not select arbitrary per-control font sizes to
  compensate for layout defects.

### Spacing, lines, and shape

Use a 4 px logical spacing grid. Insets, gaps, heights, and widths use grid
multiples unless a one-pixel device separator or pixel-aligned chart mark is
required. Default to compact density: 4 px related-item gaps, 8 px control and
cell insets, 12–16 px panel insets, and 24 px only between major groups.
Separators and ordinary outlines are one device pixel after snapping. Keep
corners and shadows restrained; elevation comes primarily from the three
surface levels and precise dividers.

Icons come from the bundled, fingerprinted atlas. Align icons optically with
their text role, maintain a square hit target, and label ambiguous icons.

## Composition and shared geometry

Compute geometry from logical metrics and shared primitives. The established
vertical order is top navigation, context bar, workspace host, and status bar.
The workspace host contains the rail and central docking surface. Do not draw a
second competing application frame inside a workspace.

### Application chrome

- **Top navigation:** keep product identity and workspace navigation on the
  panel surface. Show active workspace with cyan text/indicator and a restrained
  raised fill. Keep utilities aligned to the trailing edge.
- **Context bar:** show current dataset, symbol, range, run, model, or connection
  context. Separate groups with one-pixel dividers and use muted labels with
  primary or monospaced values.
- **Rail:** use compact icon-and-label destinations in full mode and icons with
  accessible names/tooltips in compact mode. Apply the cyan active rule.
- **Status bar:** reserve the bottom strip for connection, job, clock, and
  diagnostic status. Keep it subordinate and pair semantics with labels/icons.

### Docks, panels, and tabs

Dock headers use the panel or raised surface, a 14 px title, and trailing shared
actions. Drag targets, splitters, close controls, and overflow controls retain
stable hit regions. Active tabs use primary text plus a cyan edge; inactive tabs
use muted text. Unsaved, loading, failed, and disabled states remain
distinguishable without color alone. Clip panel content and tab labels to their
declared rectangles. Splitters and dock outlines remain one pixel when idle and
gain a clear hot/active treatment during manipulation.

### Controls and interaction states

Every interactive primitive defines default, hot, focused, active/pressed,
selected, and disabled states; toggles additionally define checked and
indeterminate states. Focus remains visible independently of hover on dark and
raised surfaces. Hot may raise contrast; pressed also changes edge, position,
or fill. Disabled retains legibility and does not respond. Destructive actions
use negative semantics and explicit wording. Maintain usable logical hit targets
even when painted controls are dense.

### Tables

Use a raised header, one-pixel structure where needed, and restrained
alternation or hover fills. Keep identifiers and numbers monospaced; align text
left and comparable numbers right. Pinned columns and sort state require a
divider plus icon/text cue. Selection, keyboard focus, validation failure,
loading, empty, and disabled rows are explicit and not color-only. Clip cell
content with deterministic ellipsis or typed tooltip behavior. Virtualization
must not change row height or visual ordering.

### Charts

Use canvas for plots, divider for low-emphasis axes/grid, and muted monospaced
labels. Semantic positive/negative colors are reserved for directional meaning;
use cyan for active/linked series and purple for model/comparison series.
Additional series variants are centrally derived and distinguishable by line
style, marker, fill pattern, or label as well as color. Clip every layer,
crosshair, tooltip anchor, and selection to its plot or overlay. Keep axes
readable, fills restrained, and dense marks subordinate to selected values. LOD
changes must not alter visual meaning or stable draw order.

### Modals, menus, tooltips, and notifications

Draw transient layers above workspace content in this order: dimming scrim,
modal or menu, tooltip, then toast/critical notification. A modal traps focus,
has an 18 px title, bounded content, explicit actions, and a visible close path;
background content is inert. Menus and tooltips stay inside the live viewport.
Toasts use semantic icon and text, never color alone, and stack without covering
critical navigation or status. Errors remain available long enough to read.

## Responsive and DPI behavior

Read live framebuffer, window, monitor DPI, and user density-preset metrics at
layout time. Effective scale is monitor DPI scale multiplied by preset scale.
Apply it once to logical geometry and typography, snap painted edges to device
pixels, and never infer current size from startup constants or a golden name.

- At 1920×1080, use full navigation when labels and minimum panel extents fit;
  preserve the multi-panel hierarchy and use extra space for data, not inflated
  chrome.
- At 1280×720, allow compact navigation, shorter context values, collapsed
  secondary actions, and constrained panel arrangements. Preserve the primary
  workflow, active context, status, and keyboard access.
- Between and beyond required viewports, choose modes from measured minimum
  extents. Never shrink below readable type/hit targets, overlap panels, draw
  off-screen controls, or silently omit essential actions.
- Constrained panels clip or scroll and expose overflow actions; dialogs clamp
  to the viewport with scrollable bodies. Dock minimums prevent unusable
  content rectangles.
- Recompute after resize, DPI transition, preset change, navigation-mode change,
  font load, and dock revision. Hit testing and clipping use painted rectangles.

The required acceptance matrix covers 1280×720 and 1920×1080 at every scale
and scenario recorded by the applicable manifest. A feature may add cases in
its owning specification; it may not omit recorded cases.

## Rendering architecture and draw order

Render functions consume immutable, typed, render-neutral view models and emit
only typed `UIAction` values from input handling. Keep Raylib/Raygui colors,
rectangles, fonts, handles, and weakly typed values inside the native/style
adapter. Shared primitives own token resolution, state styling, clipping, text
measurement, and pixel snapping.

Maintain deterministic draw order: canvas, application chrome, dock surfaces,
panel content, local interaction decoration, then transient layers. Balance
every clip begin/end on all paths and prevent cross-panel drawing. Native calls
stay on the locked UI OS thread. Do not perform I/O, decoding, database,
network, training, or blocking work during layout or draw.

## Golden governance

Identify the owning subsystem and canonical manifest before implementation.
Capture through Raylib on the UI thread using the manifest's exact viewport,
scale, seed, scenario clock, fixture/state, layout revision, asset fingerprint,
backend, dependency/build identity, and hidden-frame setup. Compare decoded
RGBA pixels with its recorded channel tolerance and maximum differing ratio.

Do not use JPEG baselines, hand-edit captures, waive failures, skip a recorded
scenario, or increase tolerances to make drift pass. Diagnose differences as an
intended change, nondeterminism, environment/asset mismatch, or defect.
Intentional change updates the owning spec and manifest entry together and
regenerates the PNG under review; unintentional change fixes implementation.

## Review checklist

- Named tokens and shared primitives replace local visual literals.
- Typography, spacing, separators, hierarchy, and density preserve identity.
- Applicable interaction and system states are readable and not color-only.
- Layout uses live viewport/effective scale and clips at both required sizes.
- Draw order, UI-thread ownership, render-neutral models, and native containment
  remain stable.
- Every applicable manifest case passes at unchanged recorded tolerance.
