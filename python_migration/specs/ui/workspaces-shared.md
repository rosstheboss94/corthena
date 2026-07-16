# Shared Workspace Rules

**Status:** Authoritative  
**Owner:** UI

Workspace queries use bounded typed effects with cancellation, correlation,
and generation identities. Panels retain the last accepted immutable snapshot
while rendering loading, empty, failure/retry, degraded, recovered, canceled,
and saturated states in place. Stable identities survive refresh and
reconciliation; stale completions are rejected.

Linked contexts propagate only explicitly supported scopes. Each link group
owns independent ordered request and selection state. Global reconciliation
must create a newer workspace generation and cannot overwrite richer state.

The default layouts remain operable at constrained logical widths: secondary
panels stack or collapse into a tab stack without changing panel IDs or link
groups. Commands are enabled from typed state.
