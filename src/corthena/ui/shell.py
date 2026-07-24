"""Render-neutral one-to-one projection of the legacy Go application shell."""

from __future__ import annotations

from dataclasses import dataclass
from enum import StrEnum

from corthena.ui.data_experiments.models import Phase7LoadState, Phase7WorkspaceState
from corthena.ui.docking import (
    DockPosition,
    Split,
    TabStack,
    WorkspaceLayout,
    calculate_geometry,
)
from corthena.ui.docking import Rect as DockRect
from corthena.ui.jobs_results.models import (
    JobDetail,
    JobsWorkspaceState,
    ResultsWorkspaceState,
    RunComparison,
)
from corthena.ui.models_inference.models import (
    InferenceWorkspaceState,
    ModelsWorkspaceState,
)
from corthena.ui.research.models import ResearchGroupState
from corthena.ui.state import (
    ActivateDockPanel,
    ActivatePanel,
    AppState,
    ContextField,
    CycleLinkContext,
    SelectWorkspace,
    UIAction,
    Workspace,
    workspace_layout,
)

TOP_NAV_HEIGHT = 36
CONTEXT_BAR_HEIGHT = 40
PANEL_HEADER_HEIGHT = 32
STATUS_BAR_HEIGHT = 26


class ShellRegion(StrEnum):
    """Stable Go-compatible back-to-front render order."""

    BACKGROUND = "background"
    WORKSPACE_TABS = "workspace_tabs"
    GLOBAL_CONTEXT = "global_context"
    COMPONENT_STATUS = "component_status"
    CONTENT_HOST = "content_host"
    STATUS_BAR = "status_bar"
    TOAST_OVERLAY = "toast_overlay"
    MODAL_OVERLAY = "modal_overlay"


SHELL_RENDER_ORDER = tuple(ShellRegion)


@dataclass(frozen=True, slots=True)
class Rect:
    """Native-independent physical-pixel rectangle."""

    x: float
    y: float
    width: float
    height: float

    def __post_init__(self) -> None:
        if self.width < 0 or self.height < 0:
            raise ValueError("rectangle dimensions must be non-negative")


@dataclass(frozen=True, slots=True)
class WorkspaceTabView:
    """One compact or full workspace navigation tab."""

    label: str
    bounds: Rect
    selected: bool
    action: SelectWorkspace


@dataclass(frozen=True, slots=True)
class PanelView:
    """One shell panel descriptor used by the rail and dock header."""

    title: str
    selected: bool
    action: ActivatePanel


@dataclass(frozen=True, slots=True)
class ComponentView:
    """One canonical component-health row."""

    title: str
    detail: str
    color: tuple[int, int, int, int]


@dataclass(frozen=True, slots=True)
class DatasetRowView:
    """One deterministic catalog row in the Phase 3 Data panel."""

    name: str
    status: str
    rows: str
    revision: str
    selected: bool


@dataclass(frozen=True, slots=True)
class DockTabView:
    panel_id: str
    title: str
    bounds: Rect
    active: bool
    action: ActivateDockPanel


@dataclass(frozen=True, slots=True)
class DockStackView:
    stack_id: str
    bounds: Rect
    header_bounds: Rect
    body_bounds: Rect
    tabs: tuple[DockTabView, ...]
    active_panel_id: str


@dataclass(frozen=True, slots=True)
class DockSplitterView:
    split_id: str
    bounds: Rect


@dataclass(frozen=True, slots=True)
class DockDropTargetView:
    """One transient directional docking target and its resulting preview."""

    position: DockPosition
    bounds: Rect
    preview_bounds: Rect
    hot: bool


@dataclass(frozen=True, slots=True)
class ShellView:
    """Complete immutable Go-compatible shell view for one frame."""

    viewport: Rect
    scale: float
    ui_scale_percent: int
    tabs: tuple[WorkspaceTabView, ...]
    panels: tuple[PanelView, ...]
    components: tuple[ComponentView, ...]
    datasets: tuple[DatasetRowView, ...]
    content_bounds: Rect
    dock_stacks: tuple[DockStackView, ...]
    dock_splitters: tuple[DockSplitterView, ...]
    hidden_panel_ids: tuple[str, ...]
    maximized_panel_id: str | None
    layout_revision: int
    dataset_name: str
    dataset_id: str
    symbols: str
    interval: str
    date_range: str
    run_id: str
    model_id: str
    connection: str
    cache: str
    cpu_slots: int
    worker_detail: str
    fps: int
    command_palette_open: bool
    settings_open: bool
    toasts: tuple[str, ...]
    critical_error: str | None
    research_group: ResearchGroupState | None
    research_groups: tuple[tuple[str, ResearchGroupState], ...]
    phase7_workspace: Phase7WorkspaceState | None
    phase8_workspace: JobsWorkspaceState | ResultsWorkspaceState | None
    selected_job: JobDetail | None
    run_comparison: RunComparison | None
    phase9_workspace: ModelsWorkspaceState | InferenceWorkspaceState | None
    render_order: tuple[ShellRegion, ...] = SHELL_RENDER_ORDER


_WORKSPACES = (
    Workspace.DATA,
    Workspace.RESEARCH,
    Workspace.EXPERIMENTS,
    Workspace.JOBS,
    Workspace.RESULTS,
    Workspace.MODELS,
    Workspace.INFERENCE,
)
_COMPACT_LABELS = ("D", "R", "E", "J", "Rs", "M", "I")
_PANELS = ("Catalog", "Coverage", "Import Queue", "Dataset", "Import Logs")


def project_shell(
    state: AppState,
    *,
    width: int = 1280,
    height: int = 720,
    dpi_scale: float = 1.0,
    fps: int = 60,
) -> ShellView:
    """Project state using the exact legacy Go shell sizing rules."""
    if width < 640 or height < 360:
        raise ValueError("shell viewport must be at least 640x360")
    if dpi_scale <= 0:
        raise ValueError("dpi_scale must be positive")
    scale = min(2.0, max(1.0, dpi_scale * state.ui_scale_percent / 100))
    compact = width < 1500 * scale
    labels = _COMPACT_LABELS if compact else tuple(item.value.title() for item in _WORKSPACES)
    x = 56 * scale if compact else 116 * scale
    tabs: list[WorkspaceTabView] = []
    for workspace, label in zip(_WORKSPACES, labels, strict=True):
        tab_width = 44 * scale if compact else (len(label) * 7 + 36) * scale
        tabs.append(
            WorkspaceTabView(
                label,
                Rect(x, 4 * scale, tab_width, 24 * scale),
                state.workspace is workspace,
                SelectWorkspace(workspace),
            )
        )
        x += tab_width + 4 * scale
    top_height = (TOP_NAV_HEIGHT + CONTEXT_BAR_HEIGHT) * scale
    status_height = STATUS_BAR_HEIGHT * scale
    layout = workspace_layout(state)
    if state.workspace is Workspace.RESEARCH and compact:
        layout = _collapse_research_layout(layout)
    visible = _visible_stacks(layout.root)
    active_ids = frozenset(stack.active_panel_id for stack in visible)
    flat_panels = tuple(panel for stack in visible for panel in stack.panels)
    panels = tuple(
        PanelView(panel.title, panel.id in active_ids, ActivatePanel(index))
        for index, panel in enumerate(flat_panels)
    )
    datasets = (
        ("US equities daily", "dataset-us-equities"),
        ("Index watchlist hourly", "dataset-index-watchlist"),
    )
    symbols_options = ("AAPL, MSFT, NVDA, AMD", "AAPL")
    intervals = ("1d", "1h")
    dataset_name, dataset_id = datasets[state.dataset_context_revision % len(datasets)]
    symbols = symbols_options[state.symbols_context_revision % len(symbols_options)]
    interval = intervals[state.interval_context_revision % len(intervals)]
    content_bounds = Rect(0, top_height, width, max(0, height - top_height - status_height))
    left_width = 260 * scale if width >= 1100 * scale else 218 * scale
    dock_host = Rect(
        left_width + 9 * scale,
        content_bounds.y + 8 * scale,
        max(0, content_bounds.width - left_width - 17 * scale),
        max(0, content_bounds.height - 16 * scale),
    )
    dock_stacks, dock_splitters = _project_docks(layout, dock_host, scale)
    phase7 = (
        state.data_experiments.data
        if state.workspace is Workspace.DATA
        else state.data_experiments.experiments
        if state.workspace is Workspace.EXPERIMENTS
        else None
    )
    phase7_datasets = (
        tuple(
            DatasetRowView(
                item.name,
                item.status,
                str(item.row_count),
                str(item.revision),
                item.dataset_id == phase7.selected_dataset_id,
            )
            for item in phase7.snapshot.catalog
        )
        if phase7 is not None and phase7.snapshot is not None
        else ()
    )
    phase8 = (
        state.jobs_results.jobs
        if state.workspace is Workspace.JOBS
        else state.jobs_results.results
        if state.workspace is Workspace.RESULTS
        else None
    )
    phase9 = (
        state.models_inference.models
        if state.workspace is Workspace.MODELS
        else state.models_inference.inference
        if state.workspace is Workspace.INFERENCE
        else None
    )
    selected_job = (
        next(
            (
                item
                for item in state.jobs_results.jobs.snapshot.jobs
                if item.summary.job_id == state.jobs_results.jobs.selected_job_id
            ),
            None,
        )
        if state.jobs_results.jobs.snapshot is not None
        else None
    )
    return ShellView(
        viewport=Rect(0, 0, width, height),
        scale=scale,
        ui_scale_percent=state.ui_scale_percent,
        tabs=tuple(tabs),
        panels=panels,
        components=(
            ComponentView("Coordinator", "demo", (76, 195, 138, 255)),
            ComponentView("Catalog", "2 demo datasets", (76, 195, 138, 255)),
            ComponentView("Scheduler", "simulated queue", (216, 180, 90, 255)),
        ),
        datasets=(
            phase7_datasets
            if phase7 is not None and phase7.snapshot is not None
            else (
                DatasetRowView("US equities daily", "ready", "958328", "16", True),
                DatasetRowView("Index watchlist hourly", "validation", "219733", "7", False),
            )
        ),
        content_bounds=content_bounds,
        dock_stacks=dock_stacks,
        dock_splitters=dock_splitters,
        hidden_panel_ids=tuple(panel.id for panel in layout.hidden),
        maximized_panel_id=layout.maximized_panel_id,
        layout_revision=layout.revision,
        dataset_name=dataset_name,
        dataset_id=dataset_id,
        symbols=symbols,
        interval=interval,
        date_range="2020-07-09 to 2026-07-09",
        run_id="run-demo-complete",
        model_id="model-demo-champion",
        connection=(
            "degraded"
            if phase7 is not None and phase7.state is Phase7LoadState.DEGRADED
            else "connected"
        ),
        cache="96 MB gen 15" if phase7 is not None else "96 MB gen 69",
        cpu_slots=10,
        worker_detail="workers pending",
        fps=fps,
        command_palette_open=state.command_palette_open,
        settings_open=state.settings_open,
        toasts=state.toasts,
        critical_error=state.critical_error,
        research_group=(
            state.research.group("link-default-research")
            if state.workspace is Workspace.RESEARCH
            else None
        ),
        research_groups=tuple(
            (panel_id, group)
            for panel_id, group_id in layout.link_groups
            if (group := state.research.group(group_id)) is not None
        )
        if state.workspace is Workspace.RESEARCH
        else (),
        phase7_workspace=phase7,
        phase8_workspace=phase8,
        selected_job=selected_job,
        run_comparison=state.jobs_results.results.comparison,
        phase9_workspace=phase9,
    )


def action_at(view: ShellView, x: float, y: float) -> UIAction | None:
    """Map pointer input to the shell's closed navigation and panel actions."""
    for tab in view.tabs:
        if _contains(tab.bounds, x, y):
            return tab.action
    scale = view.scale
    context_y = 36 * scale
    context_x = 12 * scale
    context_values = (
        (ContextField.DATASET, view.dataset_name),
        (ContextField.SYMBOLS, view.symbols),
        (ContextField.INTERVAL, view.interval),
    )
    for field, value in context_values:
        width = (72 + len(value) * 7 + 32) * scale
        if _contains(Rect(context_x, context_y, width, 40 * scale), x, y):
            return CycleLinkContext(field)
        context_x += width
    left_width = 260 * view.scale if view.viewport.width >= 1100 * view.scale else 218 * view.scale
    panel_y = view.content_bounds.y + 34 * view.scale
    for index, panel in enumerate(view.panels):
        bounds = Rect(
            10 * view.scale,
            panel_y + index * 24 * view.scale,
            left_width - 20 * view.scale,
            22 * view.scale,
        )
        if _contains(bounds, x, y):
            return panel.action
    for stack in view.dock_stacks:
        for tab in stack.tabs:
            if _contains(tab.bounds, x, y):
                for index, panel in enumerate(view.panels):
                    if panel.title == tab.title:
                        return ActivatePanel(index)
                return tab.action
    return None


def _visible_stacks(node: TabStack | Split) -> tuple[TabStack, ...]:
    if isinstance(node, TabStack):
        return (node,)
    return (*_visible_stacks(node.first), *_visible_stacks(node.second))


def _collapse_research_layout(layout: WorkspaceLayout) -> WorkspaceLayout:
    panels = tuple(panel for stack in _visible_stacks(layout.root) for panel in stack.panels)
    active = next(
        (stack.active_panel_id for stack in _visible_stacks(layout.root) if stack.active_panel_id),
        panels[0].id,
    )
    return WorkspaceLayout(
        layout.revision,
        TabStack("research-responsive", panels, active),
        layout.hidden,
        layout.maximized_panel_id,
        layout.link_groups,
    )


def _project_docks(
    layout: WorkspaceLayout, host: Rect, scale: float
) -> tuple[tuple[DockStackView, ...], tuple[DockSplitterView, ...]]:
    geometry = calculate_geometry(
        layout.root,
        DockRect(host.x, host.y, host.width, host.height),
        minimum_extent=120 * scale,
    )
    node_rects = dict(geometry.nodes)
    stacks: list[DockStackView] = []
    for stack in _visible_stacks(layout.root):
        raw = node_rects[stack.id]
        bounds = Rect(raw.x, raw.y, raw.width, raw.height)
        header_height = min(PANEL_HEADER_HEIGHT * scale, bounds.height)
        header = Rect(bounds.x, bounds.y, bounds.width, header_height)
        body = Rect(
            bounds.x + scale,
            bounds.y + header_height,
            max(0, bounds.width - 2 * scale),
            max(0, bounds.height - header_height - scale),
        )
        button_size = min(20 * scale, header.height)
        button_gap = 2 * scale
        tab_right = bounds.x + bounds.width - 3 * scale
        tab_right -= button_size + button_gap
        tab_right -= button_size + button_gap
        tab_right -= min(96 * scale, max(0, header.width * 0.28)) + button_gap
        tab_width = max(
            56 * scale,
            min(132 * scale, (tab_right - bounds.x - 3 * scale) / len(stack.panels)),
        )
        tabs = tuple(
            DockTabView(
                panel.id,
                panel.title,
                Rect(
                    bounds.x + 3 * scale + index * (tab_width + scale),
                    bounds.y + 3 * scale,
                    min(tab_width, bounds.width),
                    max(0, header_height - 5 * scale),
                ),
                panel.id == stack.active_panel_id,
                ActivateDockPanel(panel.id, layout.revision),
            )
            for index, panel in enumerate(stack.panels)
        )
        stacks.append(DockStackView(stack.id, bounds, header, body, tabs, stack.active_panel_id))
    splitters = tuple(
        DockSplitterView(split_id, Rect(rect.x, rect.y, rect.width, rect.height))
        for split_id, rect in geometry.splitters
    )
    if layout.maximized_panel_id is not None:
        stacks = [
            DockStackView(
                stack.stack_id,
                host,
                Rect(host.x, host.y, host.width, PANEL_HEADER_HEIGHT * scale),
                Rect(
                    host.x,
                    host.y + PANEL_HEADER_HEIGHT * scale,
                    host.width,
                    max(0, host.height - PANEL_HEADER_HEIGHT * scale),
                ),
                stack.tabs,
                stack.active_panel_id,
            )
            for stack in stacks
            if any(tab.panel_id == layout.maximized_panel_id for tab in stack.tabs)
        ]
        splitters = ()
    return tuple(stacks), splitters


def _contains(bounds: Rect, x: float, y: float) -> bool:
    return bounds.x <= x <= bounds.x + bounds.width and bounds.y <= y <= bounds.y + bounds.height


def project_dock_drop_targets(
    stack: DockStackView, x: float, y: float, scale: float
) -> tuple[DockDropTargetView, ...]:
    """Project five snapped docking targets for a captured tab drag."""
    size = max(32.0, round(40 * scale))
    gap = max(4.0, round(6 * scale))
    center_x = round(stack.bounds.x + stack.bounds.width / 2 - size / 2)
    center_y = round(stack.bounds.y + stack.bounds.height / 2 - size / 2)
    locations = (
        (DockPosition.CENTER, center_x, center_y),
        (DockPosition.LEFT, center_x - size - gap, center_y),
        (DockPosition.RIGHT, center_x + size + gap, center_y),
        (DockPosition.TOP, center_x, center_y - size - gap),
        (DockPosition.BOTTOM, center_x, center_y + size + gap),
    )
    targets: list[DockDropTargetView] = []
    for position, target_x, target_y in locations:
        bounds = Rect(target_x, target_y, size, size)
        match position:
            case DockPosition.CENTER:
                preview = stack.bounds
            case DockPosition.LEFT:
                preview = Rect(
                    stack.bounds.x,
                    stack.bounds.y,
                    stack.bounds.width / 2,
                    stack.bounds.height,
                )
            case DockPosition.RIGHT:
                preview = Rect(
                    stack.bounds.x + stack.bounds.width / 2,
                    stack.bounds.y,
                    stack.bounds.width / 2,
                    stack.bounds.height,
                )
            case DockPosition.TOP:
                preview = Rect(
                    stack.bounds.x,
                    stack.bounds.y,
                    stack.bounds.width,
                    stack.bounds.height / 2,
                )
            case DockPosition.BOTTOM:
                preview = Rect(
                    stack.bounds.x,
                    stack.bounds.y + stack.bounds.height / 2,
                    stack.bounds.width,
                    stack.bounds.height / 2,
                )
        targets.append(DockDropTargetView(position, bounds, preview, _contains(bounds, x, y)))
    return tuple(targets)


__all__ = [
    "CONTEXT_BAR_HEIGHT",
    "PANEL_HEADER_HEIGHT",
    "SHELL_RENDER_ORDER",
    "STATUS_BAR_HEIGHT",
    "TOP_NAV_HEIGHT",
    "ComponentView",
    "DatasetRowView",
    "DockDropTargetView",
    "DockSplitterView",
    "DockStackView",
    "DockTabView",
    "PanelView",
    "Rect",
    "ShellRegion",
    "ShellView",
    "WorkspaceTabView",
    "action_at",
    "project_dock_drop_targets",
    "project_shell",
]
