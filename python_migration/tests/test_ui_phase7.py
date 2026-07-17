from __future__ import annotations

import json
from dataclasses import replace
from datetime import UTC, datetime
from pathlib import Path
from threading import Event

import pytest

from corthena.ui.client.errors import RequestCancelledError
from corthena.ui.data_experiments.actions import (
    Phase7Completed,
    RequestPhase7,
)
from corthena.ui.data_experiments.deterministic import DataExperimentsDemo
from corthena.ui.data_experiments.models import (
    AdjustmentPolicy,
    DraftSaveRequest,
    ImportMode,
    ImportRequest,
    ImportState,
    Phase7LoadState,
    Phase7Request,
    Phase7Scenario,
    Phase7Workspace,
    SourceKind,
    SubmissionRequest,
)
from corthena.ui.data_experiments.reducer import reduce_data_experiments
from corthena.ui.shell import project_shell
from corthena.ui.state import AppState, SelectWorkspace, Workspace, reduce

FIXED_CLOCK = datetime(2026, 7, 10, 12, tzinfo=UTC)


def _request(
    workspace: Phase7Workspace = Phase7Workspace.DATA,
    generation: int = 1,
    scenario: Phase7Scenario = Phase7Scenario.NORMAL,
) -> Phase7Request:
    return Phase7Request(
        f"phase7-{workspace.value}-{generation:020d}", generation, workspace, scenario
    )


def _demo() -> DataExperimentsDemo:
    return DataExperimentsDemo(107, FIXED_CLOCK)


def test_phase7_snapshots_are_deterministic_typed_and_stably_ordered() -> None:
    first = _demo().load(_request(), Event())
    second = _demo().load(_request(), Event())
    assert first == second
    assert tuple(item.dataset_id for item in first.catalog) == (
        "dataset-us-equities",
        "dataset-index-watchlist",
    )
    assert first.draft.dataset_id == "dataset-us-equities"
    assert first.evaluation.valid
    assert first.evaluation.estimate.rows > 0
    assert all(item.content_fingerprint.startswith("sha256:") for item in first.catalog)


def test_atomic_import_commits_one_revision_and_rejections_preserve_catalog() -> None:
    demo = _demo()
    snapshot = demo.load(_request(), Event())
    entry = next(item for item in snapshot.catalog if item.dataset_id == "dataset-us-equities")
    base = ImportRequest(
        "import-command-1",
        "import-request-1",
        1,
        entry.dataset_id,
        entry.revision,
        SourceKind.CSV,
        "bars.csv",
        entry.symbols,
        entry.interval,
        AdjustmentPolicy.SPLIT_AND_DIVIDEND,
        ImportMode.APPEND,
    )
    rejected = demo.run_import(
        replace(base, command_id="bad", correlation_id="bad", scenario="duplicate"), Event()
    )
    assert rejected.state is ImportState.REJECTED
    assert rejected.catalog_entry == entry
    accepted = demo.run_import(base, Event())
    assert accepted.state is ImportState.READY
    assert accepted.catalog_entry.revision == entry.revision + 1
    assert accepted.catalog_entry.content_fingerprint != entry.content_fingerprint
    late = demo.run_import(replace(base, command_id="late", correlation_id="late"), Event())
    assert late.state is ImportState.REJECTED
    assert late.catalog_entry == accepted.catalog_entry


def test_invalid_draft_remains_evaluable_but_cannot_submit() -> None:
    demo = _demo()
    snapshot = demo.load(_request(Phase7Workspace.EXPERIMENTS), Event())
    invalid = replace(
        snapshot.draft,
        revision=2,
        features=(),
        target_horizon=10,
        purge_bars=2,
        cpu_limit=0,
    )
    evaluation = demo.evaluate("evaluation-2", 1, invalid, Event())
    assert not evaluation.valid
    assert {item.field for item in evaluation.diagnostics} >= {
        "features",
        "purge_bars",
        "cpu_limit",
    }
    with pytest.raises(ValueError, match="invalid"):
        demo.submit(SubmissionRequest("submit-2", "command-2", 1, invalid), Event())


def test_autosave_rejects_stale_revision_and_late_overwrite() -> None:
    demo = _demo()
    draft = demo.load(_request(Phase7Workspace.EXPERIMENTS), Event()).draft
    edited = replace(draft, revision=2, estimator_count=320)
    saved = demo.save(DraftSaveRequest("save-2", 1, edited, 0), Event())
    assert saved.saved_revision == 2
    with pytest.raises(ValueError, match="stale draft save"):
        demo.save(DraftSaveRequest("save-stale", 1, replace(edited, revision=3), 0), Event())
    with pytest.raises(ValueError, match="late draft save"):
        demo.save(DraftSaveRequest("save-late", 1, draft, 2), Event())


def test_submission_is_command_idempotent_and_immutable() -> None:
    demo = _demo()
    draft = demo.load(_request(Phase7Workspace.EXPERIMENTS), Event()).draft
    request = SubmissionRequest("submit-1", "submit-command-1", 1, draft)
    first = demo.submit(request, Event())
    assert demo.submit(request, Event()) is first
    assert first.draft.dataset_revision == draft.dataset_revision
    with pytest.raises(ValueError, match="different draft"):
        demo.submit(replace(request, draft=replace(draft, revision=2)), Event())


def test_cancellation_and_phase7_scenarios_are_explicit() -> None:
    cancellation = Event()
    cancellation.set()
    with pytest.raises(RequestCancelledError):
        _demo().load(_request(), cancellation)
    degraded = _demo().load(_request(scenario=Phase7Scenario.DEGRADED), Event())
    recovered = _demo().load(_request(scenario=Phase7Scenario.RECOVERED), Event())
    empty = _demo().load(_request(scenario=Phase7Scenario.EMPTY), Event())
    assert degraded.degraded
    assert not recovered.degraded
    assert not empty.catalog
    with pytest.raises(RuntimeError):
        _demo().load(_request(scenario=Phase7Scenario.FAILURE), Event())


def test_reducer_rejects_stale_completion_and_projects_status() -> None:
    first = _demo().load(_request(), Event())
    state, effects = reduce_data_experiments(
        AppState().data_experiments, RequestPhase7(first.request)
    )
    assert len(effects) == 1
    state, _ = reduce_data_experiments(state, RequestPhase7(_request(generation=2)))
    unchanged, effects = reduce_data_experiments(state, Phase7Completed(first))
    assert unchanged == state and effects == ()
    current = state.data
    assert current.state is Phase7LoadState.LOADING
    assert current.stale is False


def test_data_and_experiments_layouts_project_required_panels_responsively() -> None:
    state, _ = reduce(AppState(), SelectWorkspace(Workspace.DATA))
    data = project_shell(state, width=1920, height=1080)
    assert {tab.panel_id for stack in data.dock_stacks for tab in stack.tabs} == {
        "data-catalog",
        "data-coverage",
        "data-import-queue",
        "data-dataset",
        "data-import-logs",
    }
    state, _ = reduce(state, SelectWorkspace(Workspace.EXPERIMENTS))
    experiments = project_shell(state, width=1280, height=720)
    assert {tab.panel_id for stack in experiments.dock_stacks for tab in stack.tabs} == {
        "experiments-list",
        "experiments-configuration",
        "experiments-properties",
        "experiments-inspector",
        "experiments-validation",
        "experiments-resources",
    }


def test_canonical_phase7_manifest_owns_complete_60_case_matrix() -> None:
    path = (
        Path(__file__).parents[2] / "internal/app/workstation/testdata/phase7-golden/manifest.json"
    )
    manifest = json.loads(path.read_text(encoding="utf-8"))
    entries = manifest["entries"]
    assert len(entries) == 60
    assert {entry["workspace"] for entry in entries} == {"data", "experiments"}
    assert {entry["golden"]["metadata"]["scenario"].split("_", 1)[1] for entry in entries} == {
        "normal",
        "loading",
        "failure",
        "degraded",
        "recovered",
    }
    assert {
        (entry["golden"]["metadata"]["width"], entry["golden"]["metadata"]["height"])
        for entry in entries
    } == {(1280, 720), (1920, 1080)}
    assert {entry["golden"]["metadata"]["scale_percent"] for entry in entries} == {100, 150, 200}
    assert all(entry["golden"]["channel_tolerance"] == 3 for entry in entries)
    assert all(entry["golden"]["max_different_ratio"] == 0.002 for entry in entries)
