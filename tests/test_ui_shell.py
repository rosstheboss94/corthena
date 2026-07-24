from __future__ import annotations

from dataclasses import FrozenInstanceError

import pytest

from corthena.ui.native import RaylibUIAdapter
from corthena.ui.shell import (
    SHELL_RENDER_ORDER,
    ShellRegion,
    action_at,
    project_shell,
)
from corthena.ui.state import (
    ActivatePanel,
    AppState,
    ContextField,
    CycleLinkContext,
    SelectWorkspace,
    SetCommandPalette,
    SetUIScale,
    Workspace,
    reduce,
)


def test_shell_projects_all_regions_in_stable_order() -> None:
    view = project_shell(AppState())
    assert view.render_order == SHELL_RENDER_ORDER == tuple(ShellRegion)
    assert tuple(tab.action.workspace for tab in view.tabs) == tuple(Workspace)
    assert sum(tab.selected for tab in view.tabs) == 1
    assert view.tabs[0].selected
    assert view.content_bounds.width == 1280


@pytest.mark.parametrize("workspace", tuple(Workspace))
def test_every_workspace_tab_emits_a_closed_navigation_action(workspace: Workspace) -> None:
    view = project_shell(AppState())
    tab = next(item for item in view.tabs if item.action.workspace is workspace)
    action = action_at(view, tab.bounds.x, tab.bounds.y)
    assert action == SelectWorkspace(workspace)
    assert action is not None
    selected, effects = reduce(AppState(), action)
    assert selected.workspace is workspace
    assert effects == ()


def test_projection_is_immutable_repeatable_and_rejects_unsupported_viewports() -> None:
    left = project_shell(AppState())
    right = project_shell(AppState())
    assert left == right
    with pytest.raises(FrozenInstanceError):
        left.__setattr__("dataset_name", "changed")
    with pytest.raises(ValueError, match="640x360"):
        project_shell(AppState(), width=320, height=200)
    assert action_at(left, -1, -1) is None


def test_canonical_go_phase3_fixture_and_compact_geometry() -> None:
    view = project_shell(
        AppState(ui_scale_percent=100), width=1280, height=720, dpi_scale=1, fps=60
    )
    assert (view.dataset_name, view.dataset_id) == ("US equities daily", "dataset-us-equities")
    assert view.symbols == "AAPL, MSFT, NVDA, AMD"
    assert view.date_range == "2020-07-09 to 2026-07-09"
    assert tuple(tab.label for tab in view.tabs) == ("D", "R", "E", "J", "Rs", "M", "I")
    assert view.content_bounds == type(view.content_bounds)(0, 76, 1280, 618)
    assert tuple(row.rows for row in view.datasets) == ("958328", "219733")


def test_scale_and_context_projection_follow_closed_state() -> None:
    state, _ = reduce(AppState(), SetUIScale(150))
    state, _ = reduce(state, CycleLinkContext(ContextField.DATASET))
    view = project_shell(state, width=1920, height=1080)
    assert view.scale == 1.5
    assert view.dataset_name == "Index watchlist hourly"
    assert view.symbols == "AAPL, MSFT, NVDA, AMD"
    assert view.interval == "1d"


def test_context_and_dock_header_hit_regions_are_specific() -> None:
    view = project_shell(AppState(ui_scale_percent=100))
    assert action_at(view, 20, 50) == CycleLinkContext(ContextField.DATASET)
    assert action_at(view, 240, 50) == CycleLinkContext(ContextField.SYMBOLS)
    assert action_at(view, 490, 50) == CycleLinkContext(ContextField.INTERVAL)
    assert action_at(view, 280, 98) == ActivatePanel(0)
    assert action_at(view, 412, 98) == ActivatePanel(1)


def test_go_modal_hit_regions_emit_closed_actions() -> None:
    adapter = RaylibUIAdapter()
    settings = project_shell(AppState(settings_open=True, ui_scale_percent=100))
    assert adapter.settings_click_actions(settings, 480, 351) == [SetUIScale(125)]
    palette = project_shell(AppState(command_palette_open=True))
    assert adapter.command_click_actions(palette, 350, 260) == [
        SelectWorkspace(Workspace.DATA),
        SetCommandPalette(False),
    ]


def test_toast_and_critical_error_projection_is_immutable() -> None:
    state = AppState(toasts=("saved", "reconciled"), critical_error="coordinator unavailable")
    view = project_shell(state)
    assert view.toasts == ("saved", "reconciled")
    assert view.critical_error == "coordinator unavailable"
    with pytest.raises(ValueError, match="toast"):
        AppState(toasts=("",))
    with pytest.raises(ValueError, match="critical_error"):
        AppState(critical_error="")
