# Model Artifacts and Compatibility

An artifact directory contains canonical versioned JSON, checksummed
library-native model payloads, typed Arrow/NumPy array files where needed,
checksums for every file and manifest payload, schema/engine versions, model
kind/configuration, dependency versions, feature schema, target, training
fingerprint/cutoff, seeds and generator version, deterministic ordering
metadata, build revision, and feature implementation fingerprints.

Manifest encoding is stable and rejects non-finite JSON values. Completion
writes a sibling temporary directory, flushes and validates it, then promotes
atomically; only validated completed artifacts are indexed. Loading fails
closed on incompatible schema, engine, feature/target, array type/dimension,
checksum, model invariant, unknown required field, Arrow schema, NumPy dtype,
or library compatibility rule. Completed artifacts are immutable.
