# Experiments Workspace

Default panels are the experiment list, searchable configuration tree,
property editor, contextual inspector, validation summary, and resource
estimate. The editor configures dataset, features, target, split, model,
portfolio, and optional sweep; drafts autosave through background effects and
submission creates an immutable experiment definition.

Validation checks dataset revision/fingerprint, unique compiled features,
forward target, walk-forward split, target-horizon purge, bounded model/sweep
settings, finite portfolio values, and CPU limits. Estimates are deterministic.
Invalid drafts remain editable and autosavable but cannot submit. Draft files
are schema-versioned, revision-aware, atomically replaced, and quarantine
invalid/unknown-field documents. Submission is command-idempotent and captures
the accepted revision, fingerprint, feature identities, and full config.

Draft evaluation, autosave, and immutable submission cross
`UIClientProtocol` as typed asynchronous operations. Revision and generation
checks prevent a late evaluation or autosave from replacing newer state, and
command identity makes repeated submission idempotent.

**Status:** Authoritative
**Owner:** Experiments
**Last updated:** 2026-07-23
**Related:** [experiments page index](README.md)
