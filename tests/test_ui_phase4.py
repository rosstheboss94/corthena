from __future__ import annotations

import threading
import time
from dataclasses import FrozenInstanceError
from pathlib import Path

import pytest

from corthena.ui.controls import (
    ControlEventKind,
    ControlState,
    FrameInput,
    PointerBehavior,
    Widget,
    WidgetId,
    replay,
    route,
)
from corthena.ui.docking import (
    DockPosition,
    Orientation,
    Panel,
    Rect,
    Split,
    TabStack,
    WorkspaceLayout,
    activate,
    calculate_geometry,
    close,
    dock,
    maximize,
    move,
    reopen,
    reorder,
    resize,
    restore,
)
from corthena.ui.persistence import (
    DocumentStore,
    LayoutCollection,
    NamedLayout,
    PersistenceWorker,
    Preferences,
    decode_layouts,
    decode_preferences,
    encode_layouts,
    encode_preferences,
)
from corthena.ui.serialization import decode_action, encode_action
from corthena.ui.shell import project_dock_drop_targets, project_shell
from corthena.ui.state import (
    ActivateDockPanel,
    AppState,
    CloseDockPanel,
    DockPanel,
    ReopenDockPanel,
    ResetWorkspaceLayout,
    SetDockMaximized,
    reduce,
    workspace_layout,
)


def _layout() -> WorkspaceLayout:
    catalog = Panel("panel-catalog", "catalog", "Catalog", 7)
    coverage = Panel("panel-coverage", "coverage", "Coverage", 3)
    queue = Panel("panel-queue", "queue", "Import Queue", 2)
    return WorkspaceLayout(
        0,
        Split(
            "split-root",
            Orientation.HORIZONTAL,
            0.6,
            TabStack("stack-left", (catalog, coverage), catalog.id),
            TabStack("stack-right", (queue,), queue.id),
        ),
    )


def test_dock_mutations_are_pure_stable_and_collapse_empty_splits() -> None:
    original = _layout()
    activated = activate(original, "panel-coverage")
    ordered = reorder(activated, "panel-coverage", 0)
    moved = move(ordered, "panel-coverage", "stack-right", 1)
    assert original.revision == 0
    assert moved.revision == 3
    assert isinstance(original.root, Split)
    assert isinstance(original.root.first, TabStack)
    assert original.root.first.active_panel_id == "panel-catalog"
    assert isinstance(moved.root, Split)
    assert isinstance(moved.root.second, TabStack)
    assert tuple(panel.id for panel in moved.root.second.panels) == (
        "panel-queue",
        "panel-coverage",
    )

    closed = close(moved, "panel-catalog")
    assert isinstance(closed.root, TabStack)
    assert closed.hidden[0].state_revision == 7
    reopened = reopen(closed, "panel-catalog", "stack-right")
    assert reopened.hidden == ()
    assert isinstance(reopened.root, TabStack)
    assert reopened.root.panels[-1].id == "panel-catalog"
    with pytest.raises(ValueError, match="final visible"):
        close(WorkspaceLayout(0, TabStack("only", (Panel("p", "p", "P"),), "p")), "p")


def test_directional_dock_maximize_resize_and_restore() -> None:
    layout = _layout()
    docked = dock(
        layout,
        "panel-coverage",
        "stack-right",
        DockPosition.BOTTOM,
        split_id="split-new",
        new_stack_id="stack-new",
    )
    assert isinstance(docked.root, Split)
    maximized = maximize(docked, "panel-coverage")
    resized = resize(maximized, "split-new", 2.0)
    assert resized.maximized_panel_id == "panel-coverage"
    assert isinstance(resized.root, Split)
    restored = restore(resized)
    assert restored.maximized_panel_id is None
    with pytest.raises(FrozenInstanceError):
        layout.__setattr__("revision", 99)


@pytest.mark.parametrize("viewport", ((1280, 720), (1920, 1080)))
@pytest.mark.parametrize("preset", (100, 125, 150, 175, 200))
def test_geometry_is_deterministic_snapped_and_minimum_bounded(
    viewport: tuple[int, int], preset: int
) -> None:
    width, height = viewport
    layout = _layout()
    left = calculate_geometry(layout.root, Rect(0, 0, width, height), preset_percent=preset)
    right = calculate_geometry(layout.root, Rect(0, 0, width, height), preset_percent=preset)
    assert left == right
    assert left.node("stack-left").width >= 120
    assert left.node("stack-right").width >= 120
    assert left.node("split-root") == Rect(0, 0, width, height)
    assert all(
        value == round(value)
        for _, rect in left.nodes
        for value in (rect.x, rect.y, rect.width, rect.height)
    )


def test_widget_ids_clipping_capture_focus_cancellation_and_replay() -> None:
    root = WidgetId.root("dock").child("header")
    first = Widget(root.child_index(0), Rect(0, 0, 30, 30), Rect(0, 0, 20, 20))
    drag = Widget(
        root.child_index(1), Rect(10, 0, 30, 30), Rect(0, 0, 50, 50), PointerBehavior.DRAG
    )
    assert drag.id.descends_from(root)
    pressed = route(ControlState(), FrameInput(15, 10, pressed=True, down=True), (first, drag))
    assert pressed.state.captured == drag.id
    released = route(pressed.state, FrameInput(100, 100, released=True), (first, drag))
    assert tuple(event.kind for event in released.events) == (
        ControlEventKind.RELEASE,
        ControlEventKind.ACTIVATE,
    )
    frames = (
        (FrameInput(focus_next=True), (first, drag)),
        (FrameInput(activate=True), (first, drag)),
    )
    assert replay(ControlState(), frames) == replay(ControlState(), frames)
    cancelled = route(pressed.state, FrameInput(cancel=True), (first, drag))
    assert cancelled.events[-1].kind is ControlEventKind.CANCEL
    lost_level = route(pressed.state, FrameInput(100, 100), (first, drag))
    assert lost_level.events[-1].kind is ControlEventKind.CANCEL


def test_documents_round_trip_strictly_migrate_and_quarantine(tmp_path: Path) -> None:
    preferences = Preferences(4, 150)
    assert decode_preferences(encode_preferences(preferences)) == preferences
    collection = LayoutCollection(8, (NamedLayout("analysis", _layout()),))
    assert decode_layouts(encode_layouts(collection)) == collection
    assert decode_preferences(b'{"version":0,"revision":2,"ui_scale_percent":100}') == Preferences(
        2, 125
    )
    with pytest.raises(ValueError, match="unknown"):
        decode_preferences(b'{"version":1,"revision":0,"ui_scale_percent":125,"extra":1}')
    with pytest.raises(ValueError, match="duplicate"):
        decode_preferences(b'{"version":1,"version":1,"revision":0,"ui_scale_percent":125}')

    store = DocumentStore(tmp_path)
    (tmp_path / "preferences.json").write_text("broken", encoding="utf-8")
    assert store.load_preferences() == Preferences()
    assert (tmp_path / "preferences.json.invalid").exists()
    store.save_preferences(preferences)
    store.save_layouts(collection)
    assert store.load_preferences() == preferences
    assert store.load_layouts() == collection


def test_persistence_worker_coalesces_rejects_stale_and_shuts_down(tmp_path: Path) -> None:
    worker = PersistenceWorker(DocumentStore(tmp_path), completion_capacity=1)
    assert worker.submit(Preferences(1, 100))
    assert worker.submit(Preferences(3, 150))
    assert not worker.submit(Preferences(2, 125))
    deadline = time.monotonic() + 2
    completions = ()
    while not completions and time.monotonic() < deadline:
        completions = worker.drain()
        threading.Event().wait(0.01)
    worker.close()
    assert completions
    assert not worker.submit(Preferences(4, 175))
    assert DocumentStore(tmp_path).load_preferences().revision == 3
    assert not any(thread.name == "corthena-persistence" for thread in threading.enumerate())


def test_persistence_worker_cancel_and_completion_capacity_are_bounded(tmp_path: Path) -> None:
    worker = PersistenceWorker(DocumentStore(tmp_path), completion_capacity=1)
    assert worker.submit(Preferences(1, 100))
    worker.cancel_pending("preferences")
    for revision in range(2, 12):
        assert worker.submit(LayoutCollection(revision))
        time.sleep(0.01)
    worker.close()
    assert len(worker.drain(4)) <= 1
    with pytest.raises(ValueError, match="unknown"):
        worker.cancel_pending("other")


def test_live_state_uses_revision_checked_workspace_layouts() -> None:
    state = AppState()
    initial = workspace_layout(state)
    stale, _ = reduce(state, ActivateDockPanel("data-catalog", initial.revision + 1))
    assert stale is state
    state, _ = reduce(state, ActivateDockPanel("data-catalog", initial.revision))
    assert workspace_layout(state).revision == 1
    state, _ = reduce(state, CloseDockPanel("data-import-logs", 1))
    assert workspace_layout(state).hidden[0].id == "data-import-logs"
    state, _ = reduce(state, ReopenDockPanel("data-import-logs", "data-stack-root", 2))
    state, _ = reduce(state, SetDockMaximized("data-catalog", 3))
    view = project_shell(state)
    assert len(view.dock_stacks) == 1
    assert view.maximized_panel_id == "data-catalog"
    assert view.dock_stacks[0].body_bounds == view.dock_stacks[0].body_bounds
    state, _ = reduce(state, ResetWorkspaceLayout(4))
    assert workspace_layout(state).revision == 5


def test_directional_action_round_trip_and_replay_are_deterministic() -> None:
    action = DockPanel(
        "data-catalog",
        "data-stack-root",
        DockPosition.RIGHT,
        "split-r1-data-catalog",
        "stack-r1-data-catalog",
        0,
    )
    assert decode_action(encode_action(action)) == action
    left, _ = reduce(AppState(), action)
    right, _ = reduce(AppState(), decode_action(encode_action(action)))
    assert left == right
    view = project_shell(left)
    assert len(view.dock_stacks) == 2
    assert len(view.dock_splitters) == 1
    assert all(stack.body_bounds.width >= 0 for stack in view.dock_stacks)


@pytest.mark.parametrize("preset", (100, 125, 150, 175, 200))
def test_drag_drop_targets_are_snapped_distinct_and_show_exact_preview(preset: int) -> None:
    view = project_shell(AppState(ui_scale_percent=preset), width=1920, height=1080)
    stack = view.dock_stacks[0]
    initial = project_dock_drop_targets(stack, stack.bounds.x, stack.bounds.y, view.scale)
    assert tuple(target.position for target in initial) == tuple(DockPosition)
    assert not any(target.hot for target in initial)
    assert len(frozenset(target.bounds for target in initial)) == 5

    left = next(target for target in initial if target.position is DockPosition.LEFT)
    hovered = project_dock_drop_targets(
        stack,
        left.bounds.x + left.bounds.width / 2,
        left.bounds.y + left.bounds.height / 2,
        view.scale,
    )
    hot = tuple(target for target in hovered if target.hot)
    assert len(hot) == 1
    assert hot[0].position is DockPosition.LEFT
    assert hot[0].preview_bounds.width == stack.bounds.width / 2
    assert all(
        value == round(value)
        for target in hovered
        for value in (target.bounds.x, target.bounds.y, target.bounds.width, target.bounds.height)
    )
