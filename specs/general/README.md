# General Specifications

**Status:** Authoritative index  
**Owner:** Architecture  
**Last updated:** 2026-07-23  
**Related:** [Specification index](../README.md), [Page specifications](../pages/README.md)

This folder owns rules shared by two or more pages or by a process-wide
boundary. Page-specific lifecycle, API resources, and panel behavior belong in
`specs/pages/<page>/`. Supporting decisions, examples, missing changes, and
historical evidence are not living specifications.

## Documents

- [Product](product.md)
- [Roadmap](roadmap.md)
- [Agent-facing contract](contract.md)
- [Design pattern](design-pattern.md)
- [System architecture](system-architecture.md)
- [Concurrency and parallelism](concurrency-and-parallelism.md)
- [Technology stack](technology-stack.md)
- [API](api.md)
- [Quality index](quality/README.md)
- [UI index](ui/README.md)

## Cross-cutting ownership

Product, roadmap, agent contracts, design patterns, system architecture,
concurrency, technology, API, quality, and shared UI ownership are defined by
the documents listed above.

## Reading routes

Start with `contract.md`, then read `design-pattern.md` and the applicable page
index. Add the API, concurrency, technology, quality, or focused UI documents
listed above only when the task crosses those boundaries.
