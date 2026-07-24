# Shared Workspace Contract

**Status:** Authoritative
**Owner:** UI
**Last updated:** 2026-07-23
**Related:** [UI foundation](README.md), [Page specifications](../../pages/README.md)

All workspaces consume typed state and emit `UIAction` values. Panels never
call repositories, network, filesystem, workers, or the simulator directly.
Page-specific panel composition and behavior live in the matching page index:

- [Data](../../pages/data/README.md)
- [Research](../../pages/research/README.md)
- [Experiments](../../pages/experiments/README.md)
- [Jobs](../../pages/jobs/README.md)
- [Results](../../pages/results/README.md)
- [Models](../../pages/models/README.md)
- [Inference](../../pages/inference/README.md)
