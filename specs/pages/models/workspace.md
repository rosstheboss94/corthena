# Models Workspace

Default panels are the immutable model registry, alias and promotion history,
artifact metadata, feature importance, and tree structure inspector. Alias
assignment requires explicit confirmation and never deletes the prior model.
Read [artifact and compatibility rules](artifacts-and-registry.md) for metadata,
checksums, and tree validation.

Registry loads and confirmed alias assignments use typed
`UIClientProtocol` operations. Alias commands are idempotent and transactional;
generation checks prevent stale registry or alias results from publishing.

**Status:** Authoritative
**Owner:** ML
**Last updated:** 2026-07-23
**Related:** [models page index](README.md)
