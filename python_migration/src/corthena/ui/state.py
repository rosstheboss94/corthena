"""Immutable Phase 2 UI state and pure reducer."""

from __future__ import annotations

from collections.abc import Callable
from dataclasses import dataclass, field, replace
from datetime import datetime
from enum import StrEnum
from typing import assert_never

from corthena.ui.docking import (
    DockPosition,
    Orientation,
    Panel,
    Split,
    TabStack,
    WorkspaceLayout,
    activate,
    close,
    dock,
    maximize,
    move,
    reopen,
    reorder,
    resize,
    restore,
    set_link_group,
)
from corthena.ui.research.actions import (
    RESEARCH_ACTION_TYPES,
    CancelResearch,
    ResearchAction,
    ResearchEffect,
)
from corthena.ui.research.models import (
    ResearchGroupState,
    ResearchLoadState,
    ResearchWorkspaceState,
    default_research_query,
)
from corthena.ui.research.reducer import reduce_research


class LoadState(StrEnum):
    """State of the simulator-backed startup snapshot."""

    IDLE = "idle"
    LOADING = "loading"
    READY = "ready"
    BUSY = "busy"
    FAILED = "failed"


class Workspace(StrEnum):
    """Closed Phase 3 workspace navigation identifiers."""

    DATA = "data"
    RESEARCH = "research"
    EXPERIMENTS = "experiments"
    JOBS = "jobs"
    RESULTS = "results"
    MODELS = "models"
    INFERENCE = "inference"


class ContextField(StrEnum):
    """Closed shell link-context fields that may be cycled."""

    DATASET = "dataset"
    SYMBOLS = "symbols"
    INTERVAL = "interval"


class PersistenceState(StrEnum):
    """Typed status of asynchronous preference/layout persistence."""

    IDLE = "idle"
    LOADING = "loading"
    SAVING = "saving"
    READY = "ready"
    FAILED = "failed"


def _default_workspace_layouts() -> tuple[tuple[Workspace, WorkspaceLayout], ...]:
    titles = ("Catalog", "Coverage", "Import Queue", "Dataset", "Import Logs")
    values: list[tuple[Workspace, WorkspaceLayout]] = []
    for workspace in Workspace:
        if workspace is Workspace.RESEARCH:
            research_titles = (
                ("ohlcv", "OHLCV"),
                ("features", "Features"),
                ("series", "Series"),
                ("target", "Target"),
                ("distributions", "Distributions"),
                ("rows", "Rows"),
            )
            panels = tuple(
                Panel(f"research-{name}", name, title) for name, title in research_titles
            )
            root = Split(
                "research-root",
                Orientation.VERTICAL,
                0.64,
                Split(
                    "research-primary",
                    Orientation.HORIZONTAL,
                    0.68,
                    TabStack("research-chart", panels[:1], panels[0].id),
                    TabStack("research-inspectors", panels[1:5], panels[1].id),
                ),
                TabStack("research-rows", panels[5:], panels[5].id),
            )
            values.append(
                (
                    workspace,
                    WorkspaceLayout(
                        0,
                        root,
                        link_groups=tuple((panel.id, "link-default-research") for panel in panels),
                    ),
                )
            )
            continue
        panels = tuple(
            Panel(f"{workspace.value}-{index}", title.lower().replace(" ", "-"), title)
            for index, title in enumerate(titles)
        )
        values.append(
            (
                workspace,
                WorkspaceLayout(0, TabStack(f"{workspace.value}-stack-root", panels, panels[0].id)),
            )
        )
    return tuple(values)


@dataclass(frozen=True, slots=True, order=True)
class SnapshotItem:
    """One immutable, render-neutral simulator result."""

    logical_index: int
    symbol: str
    value_micros: int

    def __post_init__(self) -> None:
        if self.logical_index < 0:
            raise ValueError("logical_index must be non-negative")
        if not self.symbol or self.symbol.strip() != self.symbol:
            raise ValueError("symbol must be non-empty and have no surrounding whitespace")


@dataclass(frozen=True, slots=True)
class Snapshot:
    """A deterministic snapshot returned through ``UIClient``."""

    request_id: str
    generation: int
    seed: int
    as_of: datetime
    items: tuple[SnapshotItem, ...]

    def __post_init__(self) -> None:
        _validate_identity(self.request_id, self.generation)
        if self.as_of.tzinfo is None:
            raise ValueError("as_of must be timezone-aware")
        indexes = tuple(item.logical_index for item in self.items)
        if len(indexes) != len(frozenset(indexes)):
            raise ValueError("snapshot logical indexes must be unique")


@dataclass(frozen=True, slots=True)
class AppState:
    """Published immutable UI state owned by the UI thread."""

    generation: int = 0
    status: LoadState = LoadState.IDLE
    active_request_id: str | None = None
    snapshot: Snapshot | None = None
    error: str | None = None
    workspace: Workspace = Workspace.DATA
    ui_scale_percent: int = 100
    command_palette_open: bool = False
    settings_open: bool = False
    dataset_context_revision: int = 0
    symbols_context_revision: int = 0
    interval_context_revision: int = 0
    # Compatibility projection only; workspace_layouts is the layout authority.
    active_panel_index: int = 0
    workspace_layouts: tuple[tuple[Workspace, WorkspaceLayout], ...] = field(
        default_factory=_default_workspace_layouts
    )
    active_layout_name: str = "default"
    preferences_revision: int = 0
    persistence_state: PersistenceState = PersistenceState.IDLE
    toasts: tuple[str, ...] = ()
    critical_error: str | None = None
    research: ResearchWorkspaceState = field(default_factory=ResearchWorkspaceState)

    def __post_init__(self) -> None:
        if self.ui_scale_percent not in (100, 125, 150, 175, 200):
            raise ValueError("ui_scale_percent must be a supported preset")
        revisions = (
            self.dataset_context_revision,
            self.symbols_context_revision,
            self.interval_context_revision,
        )
        if min(revisions) < 0 or self.preferences_revision < 0 or self.active_panel_index < 0:
            raise ValueError("shell indexes must be non-negative")
        workspaces = tuple(workspace for workspace, _ in self.workspace_layouts)
        if len(workspaces) != len(Workspace) or frozenset(workspaces) != frozenset(Workspace):
            raise ValueError("workspace layouts must contain each workspace exactly once")
        if (
            not self.active_layout_name
            or self.active_layout_name.strip() != self.active_layout_name
        ):
            raise ValueError("active layout name must be valid")
        if any(not message for message in self.toasts):
            raise ValueError("toast messages must be non-empty")
        if self.critical_error == "":
            raise ValueError("critical_error must be non-empty when present")


@dataclass(frozen=True, slots=True)
class RequestSnapshot:
    """Ask the reducer to start a new generation-bound request."""

    request_id: str
    generation: int

    def __post_init__(self) -> None:
        _validate_identity(self.request_id, self.generation)


@dataclass(frozen=True, slots=True)
class SnapshotCompleted:
    """Apply a client result if it still identifies the active request."""

    snapshot: Snapshot


@dataclass(frozen=True, slots=True)
class SnapshotFailed:
    """Apply a typed failure if it still identifies the active request."""

    request_id: str
    generation: int
    message: str

    def __post_init__(self) -> None:
        _validate_identity(self.request_id, self.generation)
        if not self.message:
            raise ValueError("message must be non-empty")


@dataclass(frozen=True, slots=True)
class RuntimeBusy:
    """Report nonblocking queue saturation for an attempted request."""

    request_id: str
    generation: int

    def __post_init__(self) -> None:
        _validate_identity(self.request_id, self.generation)


@dataclass(frozen=True, slots=True)
class AdvanceGeneration:
    """Invalidate all work from earlier generations."""

    generation: int

    def __post_init__(self) -> None:
        if self.generation < 1:
            raise ValueError("generation must be positive")


@dataclass(frozen=True, slots=True)
class SelectWorkspace:
    """Select one shell workspace without performing domain work."""

    workspace: Workspace


@dataclass(frozen=True, slots=True)
class SetCommandPalette:
    """Open or close the shell command palette."""

    open: bool


@dataclass(frozen=True, slots=True)
class SetSettingsOpen:
    """Open or close the shell Settings overlay."""

    open: bool


@dataclass(frozen=True, slots=True)
class SetUIScale:
    """Select one supported shell UI scale preset."""

    scale_percent: int

    def __post_init__(self) -> None:
        if self.scale_percent not in (100, 125, 150, 175, 200):
            raise ValueError("scale_percent must be a supported preset")


@dataclass(frozen=True, slots=True)
class CycleLinkContext:
    """Cycle a canonical shell context field."""

    field: ContextField


@dataclass(frozen=True, slots=True)
class ActivatePanel:
    """Activate a shell panel by stable logical index."""

    panel_index: int

    def __post_init__(self) -> None:
        if self.panel_index < 0:
            raise ValueError("panel_index must be non-negative")


@dataclass(frozen=True, slots=True)
class ActivateDockPanel:
    panel_id: str
    expected_revision: int


@dataclass(frozen=True, slots=True)
class ReorderDockPanel:
    panel_id: str
    index: int
    expected_revision: int


@dataclass(frozen=True, slots=True)
class MoveDockPanel:
    panel_id: str
    target_stack_id: str
    index: int
    expected_revision: int


@dataclass(frozen=True, slots=True)
class DockPanel:
    panel_id: str
    target_stack_id: str
    position: DockPosition
    split_id: str
    new_stack_id: str
    expected_revision: int


@dataclass(frozen=True, slots=True)
class ResizeDockSplit:
    split_id: str
    ratio: float
    expected_revision: int


@dataclass(frozen=True, slots=True)
class CloseDockPanel:
    panel_id: str
    expected_revision: int


@dataclass(frozen=True, slots=True)
class ReopenDockPanel:
    panel_id: str
    target_stack_id: str
    expected_revision: int


@dataclass(frozen=True, slots=True)
class SetDockMaximized:
    panel_id: str | None
    expected_revision: int


@dataclass(frozen=True, slots=True)
class SetPanelLinkGroup:
    panel_id: str
    group_id: str | None
    expected_revision: int


@dataclass(frozen=True, slots=True)
class ApplyWorkspaceLayout:
    workspace: Workspace
    layout: WorkspaceLayout
    expected_revision: int
    layout_name: str = "default"


@dataclass(frozen=True, slots=True)
class ResetWorkspaceLayout:
    expected_revision: int


UIAction = (
    RequestSnapshot
    | SnapshotCompleted
    | SnapshotFailed
    | RuntimeBusy
    | AdvanceGeneration
    | SelectWorkspace
    | SetCommandPalette
    | SetSettingsOpen
    | SetUIScale
    | CycleLinkContext
    | ActivatePanel
    | ActivateDockPanel
    | ReorderDockPanel
    | MoveDockPanel
    | DockPanel
    | ResizeDockSplit
    | CloseDockPanel
    | ReopenDockPanel
    | SetDockMaximized
    | SetPanelLinkGroup
    | ApplyWorkspaceLayout
    | ResetWorkspaceLayout
    | ResearchAction
)


@dataclass(frozen=True, slots=True)
class LoadSnapshot:
    """Load one snapshot off the UI thread."""

    request_id: str
    generation: int

    def __post_init__(self) -> None:
        _validate_identity(self.request_id, self.generation)


@dataclass(frozen=True, slots=True)
class CancelRequest:
    """Cancel superseded work by request identity."""

    request_id: str

    def __post_init__(self) -> None:
        _validate_identity(self.request_id, 0)


UIEffect = LoadSnapshot | CancelRequest | ResearchEffect


class InvariantViolationError(RuntimeError):
    """Raised when a supposedly closed variant reaches an exhaustive switch."""


def _validate_identity(request_id: str, generation: int) -> None:
    if not request_id or request_id.strip() != request_id:
        raise ValueError("request_id must be non-empty and have no surrounding whitespace")
    if generation < 0:
        raise ValueError("generation must be non-negative")


def workspace_layout(state: AppState, workspace: Workspace | None = None) -> WorkspaceLayout:
    """Return the immutable layout for a workspace (the active workspace by default)."""
    selected = state.workspace if workspace is None else workspace
    return dict(state.workspace_layouts)[selected]


def _replace_workspace_layout(
    state: AppState, workspace: Workspace, layout: WorkspaceLayout, *, name: str | None = None
) -> AppState:
    values = tuple(
        (item, layout if item is workspace else current)
        for item, current in state.workspace_layouts
    )
    return replace(
        state,
        workspace_layouts=values,
        active_layout_name=state.active_layout_name if name is None else name,
        persistence_state=PersistenceState.SAVING,
    )


def _mutate_active_layout(
    state: AppState,
    expected_revision: int,
    mutation: Callable[[WorkspaceLayout], WorkspaceLayout],
) -> tuple[AppState, tuple[UIEffect, ...]]:
    current = workspace_layout(state)
    if expected_revision != current.revision:
        return state, ()
    changed = mutation(current)
    return _replace_workspace_layout(state, state.workspace, changed), ()


def reduce(state: AppState, action: UIAction) -> tuple[AppState, tuple[UIEffect, ...]]:
    """Purely reduce one closed action into a new state and ordered effects."""
    if isinstance(action, RESEARCH_ACTION_TYPES):
        research_action: ResearchAction = action
        research, effects = reduce_research(state.research, research_action)
        return replace(state, research=research), effects
    match action:
        case RequestSnapshot(request_id=request_id, generation=generation):
            _validate_identity(request_id, generation)
            if generation != state.generation:
                return state, ()
            effects: tuple[UIEffect, ...] = (LoadSnapshot(request_id, generation),)
            if state.active_request_id is not None and state.active_request_id != request_id:
                effects = (CancelRequest(state.active_request_id), *effects)
            return (
                replace(
                    state,
                    status=LoadState.LOADING,
                    active_request_id=request_id,
                    error=None,
                ),
                effects,
            )
        case SnapshotCompleted(snapshot=snapshot):
            _validate_identity(snapshot.request_id, snapshot.generation)
            if (
                snapshot.generation != state.generation
                or snapshot.request_id != state.active_request_id
            ):
                return state, ()
            stable = replace(snapshot, items=tuple(sorted(snapshot.items)))
            return (
                replace(
                    state,
                    status=LoadState.READY,
                    active_request_id=None,
                    snapshot=stable,
                    error=None,
                ),
                (),
            )
        case SnapshotFailed(request_id=request_id, generation=generation, message=message):
            _validate_identity(request_id, generation)
            if generation != state.generation or request_id != state.active_request_id:
                return state, ()
            return (
                replace(
                    state,
                    status=LoadState.FAILED,
                    active_request_id=None,
                    error=message,
                ),
                (),
            )
        case RuntimeBusy(request_id=request_id, generation=generation):
            _validate_identity(request_id, generation)
            if generation != state.generation or request_id != state.active_request_id:
                return state, ()
            return (
                replace(
                    state,
                    status=LoadState.BUSY,
                    active_request_id=None,
                    error="effect queue is busy",
                ),
                (),
            )
        case AdvanceGeneration(generation=generation):
            if generation <= state.generation:
                raise ValueError("generation must advance monotonically")
            effects = (
                (CancelRequest(state.active_request_id),)
                if state.active_request_id is not None
                else ()
            )
            return (
                AppState(
                    generation=generation,
                    workspace=state.workspace,
                    ui_scale_percent=state.ui_scale_percent,
                    command_palette_open=state.command_palette_open,
                    settings_open=state.settings_open,
                    dataset_context_revision=state.dataset_context_revision,
                    symbols_context_revision=state.symbols_context_revision,
                    interval_context_revision=state.interval_context_revision,
                    active_panel_index=state.active_panel_index,
                    workspace_layouts=state.workspace_layouts,
                    active_layout_name=state.active_layout_name,
                    preferences_revision=state.preferences_revision,
                    persistence_state=state.persistence_state,
                    toasts=state.toasts,
                    critical_error=state.critical_error,
                    research=state.research,
                ),
                effects,
            )
        case SelectWorkspace(workspace=workspace):
            selected = replace(state, workspace=workspace, command_palette_open=False)
            effects: tuple[UIEffect, ...] = ()
            if state.workspace is Workspace.RESEARCH and workspace is not Workspace.RESEARCH:
                effects = tuple(
                    CancelResearch(
                        group.query.request_id,
                        group.group_id,
                        group.generation,
                    )
                    for group in state.research.groups
                    if group.query is not None and group.state is ResearchLoadState.LOADING
                )
            if workspace is Workspace.RESEARCH:
                group = state.research.group("link-default-research")
                if group is None or group.query is None:
                    query = default_research_query(scenario=state.research.scenario)
                    research = replace(
                        state.research,
                        groups=(
                            ResearchGroupState(
                                query.group_id,
                                scenario=query.scenario,
                                query=query,
                            ),
                        ),
                    )
                    selected = replace(selected, research=research)
            return selected, effects
        case SetCommandPalette(open=open_value):
            return replace(state, command_palette_open=open_value, settings_open=False), ()
        case SetSettingsOpen(open=open_value):
            return replace(state, settings_open=open_value, command_palette_open=False), ()
        case SetUIScale(scale_percent=scale_percent):
            return (
                replace(
                    state,
                    ui_scale_percent=scale_percent,
                    preferences_revision=state.preferences_revision + 1,
                    persistence_state=PersistenceState.SAVING,
                ),
                (),
            )
        case CycleLinkContext(field=ContextField.DATASET):
            return replace(state, dataset_context_revision=state.dataset_context_revision + 1), ()
        case CycleLinkContext(field=ContextField.SYMBOLS):
            return replace(state, symbols_context_revision=state.symbols_context_revision + 1), ()
        case CycleLinkContext(field=ContextField.INTERVAL):
            return replace(state, interval_context_revision=state.interval_context_revision + 1), ()
        case CycleLinkContext(field=field):
            raise InvariantViolationError(f"unknown ContextField: {field!r}")
        case ActivatePanel(panel_index=panel_index):
            current = workspace_layout(state)
            visible = tuple(_visible_panel_ids(current.root))
            if not visible:
                return state, ()
            changed, effects = _mutate_active_layout(
                state,
                current.revision,
                lambda layout: activate(layout, visible[panel_index % len(visible)]),
            )
            return replace(changed, active_panel_index=panel_index), effects
        case ActivateDockPanel(panel_id=panel_id, expected_revision=revision):
            return _mutate_active_layout(state, revision, lambda layout: activate(layout, panel_id))
        case ReorderDockPanel(panel_id=panel_id, index=index, expected_revision=revision):
            return _mutate_active_layout(
                state, revision, lambda layout: reorder(layout, panel_id, index)
            )
        case MoveDockPanel(
            panel_id=panel_id,
            target_stack_id=target,
            index=index,
            expected_revision=revision,
        ):
            return _mutate_active_layout(
                state, revision, lambda layout: move(layout, panel_id, target, index)
            )
        case DockPanel(
            panel_id=panel_id,
            target_stack_id=target,
            position=position,
            split_id=split_id,
            new_stack_id=stack_id,
            expected_revision=revision,
        ):
            return _mutate_active_layout(
                state,
                revision,
                lambda layout: dock(
                    layout,
                    panel_id,
                    target,
                    position,
                    split_id=split_id,
                    new_stack_id=stack_id,
                ),
            )
        case ResizeDockSplit(split_id=split_id, ratio=ratio, expected_revision=revision):
            return _mutate_active_layout(
                state, revision, lambda layout: resize(layout, split_id, ratio)
            )
        case CloseDockPanel(panel_id=panel_id, expected_revision=revision):
            return _mutate_active_layout(state, revision, lambda layout: close(layout, panel_id))
        case ReopenDockPanel(panel_id=panel_id, target_stack_id=target, expected_revision=revision):
            return _mutate_active_layout(
                state, revision, lambda layout: reopen(layout, panel_id, target)
            )
        case SetDockMaximized(panel_id=panel_id, expected_revision=revision):
            return _mutate_active_layout(
                state,
                revision,
                restore if panel_id is None else lambda layout: maximize(layout, panel_id),
            )
        case SetPanelLinkGroup(panel_id=panel_id, group_id=group_id, expected_revision=revision):
            updated, effects = _mutate_active_layout(
                state, revision, lambda layout: set_link_group(layout, panel_id, group_id)
            )
            if (
                updated is state
                or updated.workspace is not Workspace.RESEARCH
                or group_id is None
                or updated.research.group(group_id) is not None
            ):
                return updated, effects
            query = default_research_query(
                scenario=updated.research.scenario,
                group_id=group_id,
            )
            groups = tuple(
                sorted(
                    (
                        *updated.research.groups,
                        ResearchGroupState(
                            group_id,
                            scenario=query.scenario,
                            query=query,
                        ),
                    ),
                    key=lambda group: group.group_id,
                )
            )
            return replace(updated, research=replace(updated.research, groups=groups)), effects
        case ApplyWorkspaceLayout(
            workspace=workspace_value,
            layout=layout,
            expected_revision=revision,
            layout_name=name,
        ):
            if workspace_layout(state, workspace_value).revision != revision:
                return state, ()
            return _replace_workspace_layout(state, workspace_value, layout, name=name), ()
        case ResetWorkspaceLayout(expected_revision=revision):
            if workspace_layout(state).revision != revision:
                return state, ()
            default = dict(_default_workspace_layouts())[state.workspace]
            default = replace(default, revision=revision + 1)
            return _replace_workspace_layout(state, state.workspace, default, name="default"), ()
        case _ as unreachable:
            try:
                assert_never(unreachable)
            except AssertionError as error:
                raise InvariantViolationError(f"unknown UIAction: {type(unreachable)!r}") from error


def _visible_panel_ids(node: object) -> tuple[str, ...]:
    from corthena.ui.docking import Split, TabStack

    if isinstance(node, TabStack):
        return tuple(panel.id for panel in node.panels)
    if isinstance(node, Split):
        return (*_visible_panel_ids(node.first), *_visible_panel_ids(node.second))
    raise InvariantViolationError("unknown dock node")


__all__ = [
    "ActivateDockPanel",
    "ActivatePanel",
    "AdvanceGeneration",
    "AppState",
    "ApplyWorkspaceLayout",
    "CancelRequest",
    "CloseDockPanel",
    "ContextField",
    "CycleLinkContext",
    "DockPanel",
    "InvariantViolationError",
    "LoadSnapshot",
    "LoadState",
    "MoveDockPanel",
    "PersistenceState",
    "ReopenDockPanel",
    "ReorderDockPanel",
    "RequestSnapshot",
    "ResetWorkspaceLayout",
    "ResizeDockSplit",
    "RuntimeBusy",
    "SelectWorkspace",
    "SetCommandPalette",
    "SetDockMaximized",
    "SetPanelLinkGroup",
    "SetSettingsOpen",
    "SetUIScale",
    "Snapshot",
    "SnapshotCompleted",
    "SnapshotFailed",
    "SnapshotItem",
    "UIAction",
    "UIEffect",
    "Workspace",
    "reduce",
    "workspace_layout",
]
