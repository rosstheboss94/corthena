# Specification Index

**Status:** Authoritative
**Owner:** Project
**Last updated:** 2026-07-23
**Related:** [General specifications](general/README.md), [Page specifications](pages/README.md), [History](history/README.md)

This is the stable entry point for Corthena's living specifications. Start with
the [general index](general/README.md), then choose exactly one end-to-end page
index from [pages](pages/README.md). Shared supporting records are deliberately
separate from living authority.

## Living specification namespaces

- [General](general/README.md) — product, architecture, contracts, runtime,
  API transport, quality, and shared UI rules.
- [Pages](pages/README.md) — Data, Research, Experiments, Jobs, Results, Models,
  and Inference end-to-end ownership.

## Supporting records

- [Architectural decisions](decisions/README.md) explain accepted choices but
  do not replace living specifications.
- [Examples](examples/agent-facing-contracts.md) are non-authoritative.
- [Missing changes](missing/README.md) record behavior not yet authoritative;
  retain unrelated future notes here.
- [History](history/README.md) preserves accepted migration and phase evidence
  and is excluded from normal task routing.

## Authority and maintenance

Normative behavior has one owner. Consumers link to that owner instead of
restating it. Every authoritative document has a title, status, owner, update
date, and relevant related links. The user's current request takes precedence,
while conflicts between implementation and specification are reported rather
than silently resolved.

## Minimum reading

1. Read [general/contract.md](general/contract.md) before every task.
2. Read [general/design-pattern.md](general/design-pattern.md) for architecture
   or Python/Cython work.
3. Read the applicable [page index](pages/README.md).
4. Add [general/API](general/api.md), [concurrency](general/concurrency-and-parallelism.md),
   [technology](general/technology-stack.md), [quality](general/quality/README.md),
   or focused UI documents only when the task crosses those boundaries.
