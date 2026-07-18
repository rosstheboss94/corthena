# Inference Workspace

Default panels are the model/alias selector, dataset/range selector, ranked
scored symbols, score distribution, prediction history, and export status.
Historical or latest-snapshot scoring displays model, engine, feature-registry,
lookback, and data compatibility before submission. Read
[inference and registry rules](../inference.md) for scoring and artifact
requirements.

Historical/latest scoring and export preparation use typed
`UIClientProtocol` operations. Compatibility fails closed before prediction
publication, and cancellation or stale generations publish neither predictions
nor export-ready state.
