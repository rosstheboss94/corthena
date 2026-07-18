# UI Async Effects and Ownership

Background workers handle HTTP, Arrow decoding, WebSocket events, layout I/O,
clipboard, and file dialogs. They use bounded typed queues/channels and never
retain Raylib values or invoke render callbacks. Each frame drains a bounded
message count, reduces actions, computes dock geometry, routes input, renders,
and enqueues nonblocking effects.

No filesystem, database, network, decoding, or training work runs on the render
thread. Render-thread sends are nonblocking; replaceable effects coalesce or a
typed busy state is shown. Demo preparation, feature/target calculation, LOD,
sorting, filtering, pagination, imports, experiment evaluation/submission,
and draft persistence run on bounded workers. Superseding or hidden requests
cancel by link group/workflow; generation checks reject stale completions.
Internal demo behavior does not define coordinator HTTP endpoints.

`UIClientProtocol` is the backend-swappable boundary for the accepted
simulator workflows. Its typed operations cover Research queries; Phase 7
snapshot loading, import, draft evaluation, autosave, and immutable submission;
Phase 8 snapshot loading, idempotent job commands, and immutable comparisons;
and Phase 9 registry loading, confirmed alias assignment, historical/latest
scoring, and export preparation. Requests and results carry the applicable
request, command, correlation, workspace, generation, and comparison identities.
The simulator implements this contract today; a future coordinator-backed
adapter must preserve its cancellation, ordering, validation, and immutable
publication behavior.
