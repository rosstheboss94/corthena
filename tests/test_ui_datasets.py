from __future__ import annotations

from dataclasses import replace
from datetime import UTC, datetime
from threading import Event

import pytest

from corthena.ui.data_experiments.actions import (
    AddDatasetFeatureStep,
    MoveDatasetFeatureStep,
    Phase7Completed,
    RemoveDatasetFeatureStep,
    RequestPhase7,
    SetDataIngestionView,
    SetDatasetWizardStep,
)
from corthena.ui.data_experiments.deterministic import DataExperimentsDemo
from corthena.ui.data_experiments.models import (
    AdjustmentPolicy,
    DataExperimentsState,
    DataIngestionView,
    DatasetWizardStep,
    ImportMode,
    ImportRequest,
    Phase7Request,
    Phase7Workspace,
    SourceKind,
    SubmissionRequest,
)
from corthena.ui.data_experiments.reducer import reduce_data_experiments
from corthena.ui.datasets.models import (
    ColumnType,
    CrossSectionalMethod,
    CrossSectionalStep,
    DatasetBuildRequest,
    DatasetBuildState,
    DatasetSaveRequest,
    LaggedReturnStep,
    RollingStatistic,
    RollingStatisticStep,
    SourceColumn,
    SourceDefinition,
    SourceFamily,
    SourceProvider,
)
from corthena.ui.datasets.serialization import decode_feature_step, encode_feature_step
from corthena.ui.datasets.workflow import validate_dataset_version

FIXED_CLOCK = datetime(2026, 7, 22, 12, tzinfo=UTC)


def _snapshot(demo: DataExperimentsDemo):
    return demo.load(Phase7Request("datasets-1", 1, Phase7Workspace.DATA), Event())


def test_legacy_catalog_migrates_to_sources_recipes_builds_and_binding() -> None:
    snapshot = _snapshot(DataExperimentsDemo(107, FIXED_CLOCK))
    assert len(snapshot.catalog) == len(snapshot.sources) == len(snapshot.dataset_definitions)
    assert all(build.state is DatasetBuildState.READY for build in snapshot.dataset_builds)
    assert snapshot.draft.dataset_binding is not None
    assert snapshot.draft.dataset_binding.dataset_version == snapshot.draft.dataset_revision


def test_source_family_capability_rejects_options_during_recipe_validation() -> None:
    demo_snapshot = _snapshot(DataExperimentsDemo(107, FIXED_CLOCK))
    version = demo_snapshot.dataset_versions[0]
    options = SourceDefinition(
        version.source_id,
        version.source_revision,
        "Options future extension",
        SourceFamily.OPTIONS,
        SourceProvider.EXISTING,
        (SourceColumn("contract", ColumnType.SYMBOL),),
        "sha256:options-schema-v1",
    )
    validation = validate_dataset_version(version, options)
    assert {item.code for item in validation.diagnostics} >= {"unsupported_source_family"}


def test_closed_steps_round_trip_and_reject_unknown_or_partial_documents() -> None:
    steps = (
        LaggedReturnStep("ret_5", periods=5),
        RollingStatisticStep("mean_20", "close", 20, RollingStatistic.MEAN),
        CrossSectionalStep("rank", "ret_5", CrossSectionalMethod.RANK),
    )
    assert tuple(decode_feature_step(encode_feature_step(step)) for step in steps) == steps
    with pytest.raises(ValueError, match="unknown feature step"):
        decode_feature_step({"kind": "expression", "source": "close / volume"})
    with pytest.raises(ValueError, match="fields"):
        decode_feature_step({"kind": "lagged_return", "output_name": "ret"})


def test_validation_reports_dependency_collisions_bounds_and_accumulated_lookback() -> None:
    snapshot = _snapshot(DataExperimentsDemo(107, FIXED_CLOCK))
    source = snapshot.sources[0]
    base = snapshot.dataset_versions[0]
    valid = replace(
        base,
        steps=(
            LaggedReturnStep("ret_5", periods=5),
            RollingStatisticStep("ret_mean_20", "ret_5", 20, RollingStatistic.MEAN),
        ),
    )
    validation = validate_dataset_version(valid, source)
    assert validation.valid
    assert tuple(column.lookback for column in validation.columns) == (5, 24)

    invalid = replace(
        base,
        steps=(
            RollingStatisticStep("bad_window", "future_output", 1, RollingStatistic.MEAN),
            LaggedReturnStep("close", periods=5000),
        ),
    )
    codes = {item.code for item in validate_dataset_version(invalid, source).diagnostics}
    assert codes >= {"dependency_order", "parameter_bounds", "output_collision"}


def test_build_is_command_idempotent_and_source_refresh_only_marks_prior_build_stale() -> None:
    demo = DataExperimentsDemo(107, FIXED_CLOCK)
    snapshot = _snapshot(demo)
    version = snapshot.dataset_versions[0]
    source_snapshot = snapshot.source_snapshots[0]
    request = DatasetBuildRequest("build-command", "build-correlation", 2, version, source_snapshot)
    first = demo.build_dataset(request, Event())
    assert demo.build_dataset(request, Event()) is first

    catalog = snapshot.catalog[0]
    demo.run_import(
        ImportRequest(
            "refresh-command",
            "refresh-correlation",
            2,
            catalog.dataset_id,
            catalog.revision,
            SourceKind.CSV,
            "refresh.csv",
            catalog.symbols,
            catalog.interval,
            AdjustmentPolicy.SPLIT_AND_DIVIDEND,
            ImportMode.APPEND,
        ),
        Event(),
    )
    refreshed = demo.load(Phase7Request("datasets-3", 3, Phase7Workspace.DATA), Event())
    old = next(build for build in refreshed.dataset_builds if build.command_id == "build-command")
    assert old.state is DatasetBuildState.STALE
    assert old.binding == first.binding
    assert refreshed.draft.dataset_binding is not None


def test_save_is_idempotent_and_atomically_advances_the_latest_version_pointer() -> None:
    demo = DataExperimentsDemo(107, FIXED_CLOCK)
    snapshot = _snapshot(demo)
    definition = snapshot.dataset_definitions[0]
    prior = snapshot.dataset_versions[0]
    version = replace(
        prior,
        version=definition.latest_version + 1,
        recipe_fingerprint="sha256:recipe-version-19",
    )
    request = DatasetSaveRequest(
        "save-dataset-command",
        "save-dataset-correlation",
        2,
        definition.revision,
        definition,
        version,
    )
    first = demo.save_dataset(request, Event())
    assert demo.save_dataset(request, Event()) is first
    assert first.definition.latest_version == definition.latest_version + 1
    assert first.definition.revision == definition.revision + 1
    with pytest.raises(ValueError, match="reused"):
        demo.save_dataset(
            replace(request, version=replace(version, recipe_fingerprint="sha256:different")),
            Event(),
        )


def test_source_refresh_keeps_a_pinned_experiment_draft_submittable() -> None:
    demo = DataExperimentsDemo(107, FIXED_CLOCK)
    snapshot = _snapshot(demo)
    catalog = snapshot.catalog[0]
    demo.run_import(
        ImportRequest(
            "refresh-before-submit",
            "refresh-before-submit-correlation",
            2,
            catalog.dataset_id,
            catalog.revision,
            SourceKind.CSV,
            "refresh.csv",
            catalog.symbols,
            catalog.interval,
            AdjustmentPolicy.SPLIT_AND_DIVIDEND,
            ImportMode.APPEND,
        ),
        Event(),
    )
    submitted = demo.submit(
        SubmissionRequest("submit-pinned", "submit-pinned-command", 2, snapshot.draft),
        Event(),
    )
    assert submitted.draft.dataset_binding == snapshot.draft.dataset_binding


def test_dataset_wizard_preserves_and_reorders_the_editable_recipe() -> None:
    demo = DataExperimentsDemo(107, FIXED_CLOCK)
    request = Phase7Request("wizard-load", 1, Phase7Workspace.DATA)
    state, _ = reduce_data_experiments(DataExperimentsState(), RequestPhase7(request))
    state, _ = reduce_data_experiments(state, Phase7Completed(demo.load(request, Event())))
    state, _ = reduce_data_experiments(state, SetDataIngestionView(DataIngestionView.NEW_DATASET))
    state, _ = reduce_data_experiments(state, SetDatasetWizardStep(DatasetWizardStep.FEATURES))
    original_count = len(state.data.dataset_recipe_steps)
    state, _ = reduce_data_experiments(
        state, AddDatasetFeatureStep(LaggedReturnStep("ret_10", periods=10))
    )
    state, _ = reduce_data_experiments(state, MoveDatasetFeatureStep(original_count, -1))
    assert state.data.dataset_recipe_steps[-2].output_name == "ret_10"
    state, _ = reduce_data_experiments(state, RemoveDatasetFeatureStep(original_count - 1))
    assert len(state.data.dataset_recipe_steps) == original_count
