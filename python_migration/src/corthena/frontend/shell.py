"""Render-neutral one-to-one projection of the legacy Go application shell."""

from __future__ import annotations

from dataclasses import dataclass
from enum import StrEnum

from corthena.frontend.state import (
    ActivatePanel,
    AppState,
    ContextField,
    CycleLinkContext,
    SelectWorkspace,
    UIAction,
    Workspace,
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
    active_panel_index = state.active_panel_index % len(_PANELS)
    panels = tuple(
        PanelView(title, index == active_panel_index, ActivatePanel(index))
        for index, title in enumerate(_PANELS)
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
            DatasetRowView("US equities daily", "ready", "958328", "16", True),
            DatasetRowView("Index watchlist hourly", "validation", "219733", "7", False),
        ),
        content_bounds=Rect(0, top_height, width, max(0, height - top_height - status_height)),
        dataset_name=dataset_name,
        dataset_id=dataset_id,
        symbols=symbols,
        interval=interval,
        date_range="2020-07-09 to 2026-07-09",
        run_id="run-demo-complete",
        model_id="model-demo-champion",
        connection="connected",
        cache="96 MB gen 69",
        cpu_slots=10,
        worker_detail="workers pending",
        fps=fps,
        command_palette_open=state.command_palette_open,
        settings_open=state.settings_open,
        toasts=state.toasts,
        critical_error=state.critical_error,
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
    content = view.content_bounds
    host_x = left_width + 9 * scale
    host_y = content.y + 8 * scale
    host_width = content.width - left_width - 17 * scale
    tab_right = host_x + host_width - 335 * scale
    tab_width = min(
        132 * scale, max(56 * scale, (tab_right - host_x - 3 * scale) / len(view.panels))
    )
    tab_x = host_x + 3 * scale
    for panel in view.panels:
        width = min(tab_width, max(0, tab_right - tab_x))
        if _contains(Rect(tab_x, host_y + 3 * scale, width, 27 * scale), x, y):
            return panel.action
        tab_x += tab_width + scale
    return None


def _contains(bounds: Rect, x: float, y: float) -> bool:
    return bounds.x <= x <= bounds.x + bounds.width and bounds.y <= y <= bounds.y + bounds.height


__all__ = [
    "CONTEXT_BAR_HEIGHT",
    "PANEL_HEADER_HEIGHT",
    "SHELL_RENDER_ORDER",
    "STATUS_BAR_HEIGHT",
    "TOP_NAV_HEIGHT",
    "ComponentView",
    "DatasetRowView",
    "PanelView",
    "Rect",
    "ShellRegion",
    "ShellView",
    "WorkspaceTabView",
    "action_at",
    "project_shell",
]
