# Data Datasets and Recipes

**Status:** Authoritative  
**Owner:** Data  
**Last updated:** 2026-07-23  
**Related:** [Ingestion](ingestion.md), [Research features and targets](../research/features-and-targets.md), [Experiments](../experiments/definitions-and-materialization.md), [Data API](api.md)

## Source and dataset identity

Data distinguishes a typed source definition and immutable source snapshots
from a reusable dataset definition and immutable dataset versions/builds. A
source definition records acquisition identity and policy; each refresh creates
an immutable snapshot with source checksum or provider request identity. A
dataset definition references a source and an ordered transformation recipe;
published versions record the exact source snapshot, coverage, policies, and
content fingerprint.

Refreshing a source can mark the latest dataset definition stale, but it never
changes an immutable dataset version and never rebinds a pinned Research
session or submitted Experiment. Consumers select an explicit version or an
explicit latest binding and receive stale-latest state when the latest source
has advanced.

## Closed feature recipes

Recipes are ordered, validated members of a closed union of built-in feature
steps. They are not free-form code, dynamic expressions, or unvalidated plugin
payloads. Each step has a stable name, version, typed configuration, lookback,
output schema, and implementation fingerprint. The dataset version stores the
ordered recipe and resolved fingerprint; Research owns computation and preview
semantics described in [features and targets](../research/features-and-targets.md).

## Bindings and reproducibility

Research queries and Experiment drafts carry a pinned dataset binding containing
dataset identity, immutable version, catalog revision, content fingerprint,
feature recipe identities, target configuration, and generation/request
identity. A completed Experiment submission captures the same values
immutably. Dataset refreshes may update the catalog's latest pointer but cannot
alter an existing binding or materialization.

## Schema and policy selection

The New Dataset schema stage projects only a bounded active file preview. It
shows detected source names and types, editable canonical-role mappings, and
diagnostics. Canonical-role selection uses a bounded dropdown containing every
supported role; selecting a role applies it and closes the menu. The Selection
stage exposes interval, session, adjustment, and source-timezone controls as
editable validated values rather than a read-only summary.

