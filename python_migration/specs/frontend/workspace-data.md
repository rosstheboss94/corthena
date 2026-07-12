# Data Workspace

Default panels are the catalog table, coverage timeline, import and validation
queue, dataset inspector, and import logs.

Users import files, choose append or range replacement, inspect validation
failures, and select the active dataset context. Requests use typed source
kind, adjustment policy, symbols, interval, mode, and optional UTC replacement
bounds. Validation completes before a catalog mutation. A successful import
atomically publishes exactly one new dataset revision and implementation
fingerprint; rejected duplicate, malformed, or out-of-range input remains in
the queue and logs without changing the prior revision. Selection uses stable
identities and refreshes dependent workspace context.
