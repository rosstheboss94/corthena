# Go workstation visual baselines

These JPEGs were captured from the current Go reference application with
`go run .\\cmd\\workstation`, using a deterministic seed, fixed clock, a
1280x720 viewport, and 30 hidden render frames. They are quick inspection
aids, not normative parity targets; the Go manifest-backed PNG baselines are
canonical. They are migration references,
not historical binaries: phases 1–5 show the current shell carrying forward
the relevant UI foundation, while phases 6–9 show their completed workspaces.

Phase 0 has no screenshot because it is a Go/native compatibility baseline
with no user-facing UI scope.

| Phase | Image | Visible control surface |
| --- | --- | --- |
| 1 | `1/1.jpg` | Raylib workstation shell: top workspace navigation, contextual dataset/symbol/interval controls, panel chrome, status rail. |
| 2 | `2/1.jpg` | Typed simulator-backed Data catalog: deterministic dataset selection, coverage/import queue tabs, component health. |
| 3 | `3/1.jpg` | Application shell: global context, workspace navigation, command/settings entries, and operational status. |
| 4 | `4/1.jpg` | Dockable Data workspace layout: panel tabs, workspace panel navigation, selected dataset context, and persisted-layout chrome. |
| 5 | `5/1.jpg` | Research visualization surface: chart/table workspace and linked research controls used for chart and table behavior. |
| 6 | `6/1.jpg` | Research vertical slice: OHLCV/feature/target panels, linked selection controls, deterministic simulator state. |
| 7 | `7/1.jpg` | Data catalog/import controls and validation-oriented dataset state. |
| 7 | `7/2.jpg` | Experiment draft, estimate, validation, autosave/submission-oriented workspace. |
| 8 | `8/1.jpg` | Jobs queue: lifecycle telemetry and pause/resume/cancel/checkpoint controls. |
| 8 | `8/2.jpg` | Results: immutable run comparison, result charts, and selected-run context. |
| 9 | `9/1.jpg` | Models registry: registered models, artifacts, aliases, provenance, and tree inspection controls. |
| 9 | `9/2.jpg` | Inference: model/dataset compatibility, scoring output, rankings/distributions/history, and export controls. |
