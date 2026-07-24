# Cross-Page Roadmap

**Status:** Authoritative  
**Owner:** Project  
**Last updated:** 2026-07-23  
**Related:** [Product](product.md), [Page specifications](../pages/README.md), [Data roadmap](../pages/data/roadmap.md)

The current delivery boundary is the Data page's two-stage ingestion effort.
Detailed Milestone 1a and 1b scope and acceptance live in the [Data roadmap](../pages/data/roadmap.md).

## Sequence

1. Accept the simulator-backed Data ingestion workflow and its backend-swappable
   UI contract.
2. Accept coordinator-backed real Data ingestion, durable revisions, schedules,
   reconciliation, and the exact Windows compatibility gates.
3. Replan Research, Experiments, Jobs, Results, Models, and Inference backend
   delivery after real Data ingestion is accepted. Their page specifications
   define current contracts and UI behavior but do not imply backend delivery.

The seven pages are Data, Research, Experiments, Jobs, Results, Models, and
Inference. Settings, shell navigation, layouts, charts, tables, and visual
tokens remain shared concerns under `general/ui/`.
