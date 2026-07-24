# Page Specifications

**Status:** Authoritative index  
**Owner:** Product and Architecture  
**Last updated:** 2026-07-23  
**Related:** [Specification index](../README.md), [General specifications](../general/README.md)

Each page folder owns one end-to-end top-level workspace: its domain lifecycle,
page-specific API resources, panels, commands, states, and acceptance rules.
Shared transport, concurrency, UI primitives, and quality rules remain in
`specs/general/` and are linked rather than copied.

| Page | Owner | Index |
|---|---|---|
| Data | acquisition, catalog, datasets, ingestion, schedules | [Data](data/README.md) |
| Research | feature/target queries and previews | [Research](research/README.md) |
| Experiments | definitions, drafts, estimates, bindings, submission | [Experiments](experiments/README.md) |
| Jobs | execution lifecycle, resources, checkpoints | [Jobs](jobs/README.md) |
| Results | evaluation, backtests, metrics, comparisons | [Results](results/README.md) |
| Models | estimators, artifacts, registry, aliases | [Models](models/README.md) |
| Inference | compatibility, scoring, prediction history, export | [Inference](inference/README.md) |

