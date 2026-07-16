from __future__ import annotations

import json
from dataclasses import replace
from datetime import UTC, datetime
from pathlib import Path
from threading import Event

import pytest

from corthena.ui.client.errors import RequestCancelledError
from corthena.ui.research.actions import (
    RequestResearch,
    ResearchCompleted,
    SetResearchFeature,
    SetResearchRange,
)
from corthena.ui.research.deterministic import build_research_snapshot
from corthena.ui.research.models import (
    ResearchLoadState,
    ResearchScenario,
    ResearchWorkspaceState,
    default_research_query,
    pan_range,
    select_range,
    zoom_range,
)
from corthena.ui.research.reducer import reduce_research
from corthena.ui.serialization import decode_action, encode_action
from corthena.ui.shell import project_shell
from corthena.ui.state import (
    AppState,
    SelectWorkspace,
    SetPanelLinkGroup,
    Workspace,
    reduce,
    workspace_layout,
)

FIXED_CLOCK = datetime(2026, 7, 10, 12, tzinfo=UTC)


def _snapshot(
    scenario: ResearchScenario = ResearchScenario.NORMAL,
    *,
    generation: int = 1,
    group_id: str = "link-default-research",
):
    return build_research_snapshot(
        42,
        FIXED_CLOCK,
        default_research_query(generation, scenario=scenario, group_id=group_id),
        Event(),
    )


def test_research_data_is_deterministic_leakage_safe_and_immutable() -> None:
    first = _snapshot()
    second = _snapshot()
    assert first == second
    assert first.bars
    feature = first.features[0]
    assert all(value.value is None for value in feature.values[: feature.descriptor.lookback])
    horizon = first.target.spec.horizon_bars
    assert all(value.value is None for value in first.target.values[-horizon:])
    assert all(
        value.target_timestamp is None or value.target_timestamp > value.timestamp
        for value in first.target.values
    )
    assert first.target.excluded_rows == horizon + 1
    assert first.frame is not None
    assert first.frame.work_count >= len(first.bars)


def test_scenario_results_cover_empty_degraded_and_recovered_states() -> None:
    assert not _snapshot(ResearchScenario.EMPTY).bars
    assert _snapshot(ResearchScenario.DEGRADED).degraded
    recovered = _snapshot(ResearchScenario.RECOVERED)
    state, _ = reduce_research(ResearchWorkspaceState(), RequestResearch(recovered.query))
    state, effects = reduce_research(state, ResearchCompleted(recovered))
    assert effects == ()
    group = state.group(recovered.query.group_id)
    assert group is not None
    assert group.state is ResearchLoadState.RECOVERED


def test_cancelled_generation_stops_before_publication() -> None:
    cancellation = Event()
    cancellation.set()
    with pytest.raises(RequestCancelledError):
        build_research_snapshot(42, FIXED_CLOCK, default_research_query(), cancellation)


def test_stale_completion_cannot_replace_a_newer_generation() -> None:
    first = _snapshot(generation=1)
    state, _ = reduce_research(ResearchWorkspaceState(), RequestResearch(first.query))
    state, _ = reduce_research(
        state,
        RequestResearch(default_research_query(2)),
    )
    unchanged, effects = reduce_research(state, ResearchCompleted(first))
    assert unchanged == state
    assert effects == ()


def test_link_groups_order_generations_independently() -> None:
    state, _ = reduce_research(
        ResearchWorkspaceState(), RequestResearch(default_research_query(group_id="group-a"))
    )
    state, _ = reduce_research(state, RequestResearch(default_research_query(group_id="group-b")))
    state, effects = reduce_research(state, SetResearchFeature("group-a", "volatility_20"))
    group_a = state.group("group-a")
    group_b = state.group("group-b")
    assert group_a is not None and group_b is not None
    assert group_a.generation == 2
    assert group_b.generation == 1
    assert len(effects) == 2


def test_linked_range_helpers_are_deterministic_and_bounded() -> None:
    original = default_research_query().time_range
    selected = select_range(original, 0.25, 0.75)
    assert selected == select_range(original, 0.25, 0.75)
    assert selected.start > original.start and selected.end < original.end
    assert pan_range(selected, 0.1).start > selected.start
    assert (
        zoom_range(selected, 0.5, 0.5).end - zoom_range(selected, 0.5, 0.5).start
        == (selected.end - selected.start) / 2
    )


def test_research_actions_and_completed_results_round_trip_for_replay() -> None:
    snapshot = _snapshot()
    actions = (
        RequestResearch(snapshot.query),
        ResearchCompleted(snapshot),
        SetResearchRange(snapshot.query.group_id, "research-ohlcv", snapshot.query.time_range),
    )
    for action in actions:
        assert decode_action(encode_action(action)) == action


def test_research_workspace_projects_six_panels_and_compact_tabs() -> None:
    state, effects = reduce(AppState(), SelectWorkspace(Workspace.RESEARCH))
    assert effects == ()
    wide = project_shell(state, width=1920, height=1080)
    compact = project_shell(state, width=900, height=600)
    wide_panels = {tab.panel_id for stack in wide.dock_stacks for tab in stack.tabs}
    compact_panels = {tab.panel_id for stack in compact.dock_stacks for tab in stack.tabs}
    assert wide_panels == compact_panels
    assert wide_panels == {
        "research-ohlcv",
        "research-features",
        "research-series",
        "research-target",
        "research-distributions",
        "research-rows",
    }
    assert len(compact.dock_stacks) == 1


def test_panel_link_groups_project_independent_research_state() -> None:
    state, _ = reduce(AppState(), SelectWorkspace(Workspace.RESEARCH))
    revision = workspace_layout(state).revision
    state, effects = reduce(
        state,
        SetPanelLinkGroup("research-features", "link-secondary", revision),
    )
    assert effects == ()
    secondary = state.research.group("link-secondary")
    assert secondary is not None
    view = project_shell(state, width=1920, height=1080)
    groups = dict(view.research_groups)
    assert groups["research-features"].group_id == "link-secondary"
    assert groups["research-ohlcv"].group_id == "link-default-research"


def test_canonical_research_manifest_owns_the_complete_36_case_matrix() -> None:
    manifest_path = (
        Path(__file__).parents[2]
        / "internal"
        / "app"
        / "workstation"
        / "testdata"
        / "research-golden"
        / "manifest.json"
    )
    manifest = json.loads(manifest_path.read_text(encoding="utf-8"))
    entries = manifest["entries"]
    assert len(entries) == 36
    assert {entry["metadata"]["scenario"] for entry in entries} == {
        "normal",
        "linked_selection",
        "loading",
        "failure",
        "degraded",
        "recovered",
    }
    assert {(entry["metadata"]["width"], entry["metadata"]["height"]) for entry in entries} == {
        (1280, 720),
        (1920, 1080),
    }
    assert {entry["metadata"]["scale_percent"] for entry in entries} == {100, 150, 200}
    assert all(entry["channel_tolerance"] == 3 for entry in entries)
    assert all(entry["max_different_ratio"] == 0.002 for entry in entries)


def test_query_validation_rejects_unbounded_or_ambiguous_inputs() -> None:
    query = default_research_query()
    with pytest.raises(ValueError):
        replace(query, symbols=("AAPL", "AAPL"))
    with pytest.raises(ValueError):
        replace(query, page_size=501)
    with pytest.raises(ValueError):
        replace(query, filter="x" * 129)
