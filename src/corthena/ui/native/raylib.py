"""Typed adapter containing all Phase 1 Raylib and Raygui values."""

# pyright: reportAttributeAccessIssue=false, reportUnknownArgumentType=false
# pyright: reportUnknownMemberType=false, reportUnknownVariableType=false

from __future__ import annotations

import threading
from dataclasses import replace
from datetime import datetime
from itertools import pairwise
from typing import TYPE_CHECKING

if TYPE_CHECKING:
    from pyray import Font, Texture, Vector2

    from corthena.ui.phase5b import VisualizationView

from corthena.ui.assets import UIAssets
from corthena.ui.controls import (
    ControlEventKind,
    ControlState,
    FrameInput,
    PointerBehavior,
    Widget,
    WidgetId,
    route,
)
from corthena.ui.data_experiments.actions import (
    EditExperimentDraft,
    RequestDataImport,
    RequestDraftEvaluation,
    RequestDraftSave,
    RequestPhase7,
    RequestSubmission,
    SetPhase7Scenario,
)
from corthena.ui.data_experiments.models import (
    AdjustmentPolicy,
    DraftSaveRequest,
    ImportMode,
    ImportRequest,
    Phase7LoadState,
    Phase7Request,
    Phase7Scenario,
    Phase7Workspace,
    Phase7WorkspaceState,
    SourceKind,
    SubmissionRequest,
)
from corthena.ui.docking import DockPosition
from corthena.ui.docking import Rect as ControlRect
from corthena.ui.jobs_results.actions import RequestJobCommand, RequestPhase8, SetPhase8Scenario
from corthena.ui.jobs_results.models import (
    JobCommand,
    JobCommandKind,
    JobState,
    JobsWorkspaceState,
    MetricPartition,
    Phase8LoadState,
    Phase8Request,
    Phase8Scenario,
    Phase8Workspace,
)
from corthena.ui.models_inference.actions import RequestPhase9, SetPhase9Scenario
from corthena.ui.models_inference.models import (
    ModelsWorkspaceState,
    Phase9LoadState,
    Phase9Request,
    Phase9Scenario,
    Phase9Workspace,
)
from corthena.ui.native.models import CapturedFrame, FrameMetrics, WindowSize
from corthena.ui.phase5b import (
    ChartAction,
    InteractionKind,
)
from corthena.ui.phase5b import (
    Point as VisualizationPoint,
)
from corthena.ui.phase5b import (
    Rect as VisualizationRect,
)
from corthena.ui.phase5b import (
    Transform as VisualizationTransform,
)
from corthena.ui.research.actions import (
    RequestResearch,
    SelectResearchRow,
    SetResearchFeature,
    SetResearchRange,
    SetResearchScenario,
    SetResearchVisibility,
)
from corthena.ui.research.models import (
    ResearchGroupState,
    ResearchLoadState,
    ResearchQuery,
    ResearchScenario,
    ResearchSort,
    TimeRange,
    default_research_query,
    pan_range,
    select_range,
    zoom_range,
)
from corthena.ui.shell import (
    DockDropTargetView,
    ShellRegion,
    ShellView,
    action_at,
    project_dock_drop_targets,
)
from corthena.ui.shell import (
    Rect as ShellRect,
)
from corthena.ui.state import (
    DockPanel,
    ResizeDockSplit,
    SetCommandPalette,
    SetSettingsOpen,
    SetUIScale,
    UIAction,
)


class UiThreadViolationError(RuntimeError):
    """Raised before a native call when the UI OS thread does not own it."""


class RaylibUIAdapter:
    """Own Raylib resources on the constructing Windows OS thread."""

    def __init__(self, size: WindowSize | None = None) -> None:
        self._owner_thread_id = threading.get_native_id()
        self._size = WindowSize() if size is None else size
        self._window_open = False
        self._inter_font: Font | None = None
        self._mono_font: Font | None = None
        self._atlas: Texture | None = None
        self._command_index = 0
        self._visualization_controls = ControlState()
        self._visualization_drag_start: VisualizationPoint | None = None
        self._visualization_selecting = False
        self._dock_drag: tuple[str, float, float] | None = None
        self._split_drag: str | None = None
        self._dock_targets: tuple[DockDropTargetView, ...] = ()
        self._research_drag: tuple[str, float, TimeRange, bool] | None = None

    @property
    def owner_thread_id(self) -> int:
        """Return the locked Windows OS thread identifier."""
        return self._owner_thread_id

    def _assert_owner(self) -> None:
        if threading.get_native_id() != self._owner_thread_id:
            raise UiThreadViolationError("Raylib call attempted outside the locked UI OS thread")

    def initialize(self, assets: UIAssets, *, hidden: bool) -> None:
        """Initialize a window and load validated assets on the owner thread."""
        self._assert_owner()
        import pyray as rl

        if self._window_open:
            raise RuntimeError("UI adapter is already initialized")
        if hidden:
            rl.set_config_flags(
                rl.FLAG_WINDOW_HIDDEN | rl.FLAG_WINDOW_RESIZABLE | rl.FLAG_VSYNC_HINT
            )
        else:
            rl.set_config_flags(rl.FLAG_WINDOW_RESIZABLE | rl.FLAG_VSYNC_HINT)
        rl.init_window(self._size.width, self._size.height, "Corthena")
        self._window_open = True
        self._inter_font = rl.load_font_ex(str(assets.inter_font), 48, None, 0)
        self._mono_font = rl.load_font_ex(str(assets.mono_font), 48, None, 0)
        self._atlas = rl.load_texture(str(assets.icon_atlas))
        if (
            self._inter_font.texture.id == 0
            or self._mono_font.texture.id == 0
            or self._atlas.id == 0
        ):
            raise RuntimeError("a bundled UI asset failed to load")
        rl.set_target_fps(60)
        rl.gui_set_font(self._inter_font)

    def should_close(self) -> bool:
        """Poll the window-close flag on the owner thread."""
        self._assert_owner()
        import pyray as rl

        return bool(rl.window_should_close())

    def frame_metrics(self) -> FrameMetrics:
        """Sample the current resizable viewport, DPI, and FPS on the UI thread."""
        self._assert_owner()
        import pyray as rl

        if not self._window_open:
            raise RuntimeError("UI adapter is not initialized")
        dpi: Vector2 = rl.get_window_scale_dpi()
        dpi_scale = max(float(dpi.x), float(dpi.y), 1.0)
        return FrameMetrics(
            width=int(rl.get_screen_width()),
            height=int(rl.get_screen_height()),
            dpi_scale=dpi_scale,
            fps=max(0, int(rl.get_fps())),
        )

    def render_frame(self) -> None:
        """Render one empty Phase 1 frame and a Raygui control."""
        self._assert_owner()
        import pyray as rl

        if not self._window_open:
            raise RuntimeError("UI adapter is not initialized")
        rl.begin_drawing()
        try:
            rl.clear_background(rl.Color(15, 23, 42, 255))
            rl.gui_label(rl.Rectangle(20, 20, 240, 28), "Corthena")
        finally:
            rl.end_drawing()

    def render_shell(self, view: ShellView) -> tuple[UIAction, ...]:
        """Render the typed shell with the legacy Go primitive sequence."""
        self._assert_owner()
        import pyray as rl

        if not self._window_open:
            raise RuntimeError("UI adapter is not initialized")
        if self._inter_font is None or self._mono_font is None:
            raise RuntimeError("UI fonts are not initialized")
        actions = self._shell_actions(view, rl)
        rl.begin_drawing()
        try:
            for region in view.render_order:
                match region:
                    case ShellRegion.BACKGROUND:
                        rl.clear_background(rl.Color(11, 13, 16, 255))
                    case ShellRegion.WORKSPACE_TABS:
                        self._draw_top_navigation(view, rl)
                    case ShellRegion.GLOBAL_CONTEXT:
                        self._draw_context_bar(view, rl)
                    case ShellRegion.COMPONENT_STATUS:
                        self._draw_left_rail(view, rl)
                    case ShellRegion.CONTENT_HOST:
                        self._draw_data_host(view, rl)
                        self._draw_dock_targets(rl)
                    case ShellRegion.STATUS_BAR:
                        self._draw_status_bar(view, rl)
                    case ShellRegion.TOAST_OVERLAY:
                        self._draw_toasts(view, rl)
                    case ShellRegion.MODAL_OVERLAY:
                        self._draw_modal(view, rl)
        finally:
            rl.end_drawing()
        return tuple(actions)

    def render_visualization(self, view: VisualizationView) -> tuple[ChartAction, ...]:
        """Render immutable generic chart/table primitives on the owner thread."""
        self._assert_owner()
        if not self._window_open or self._inter_font is None or self._mono_font is None:
            raise RuntimeError("UI visualization resources are not initialized")
        import pyray as rl

        from corthena.ui.native_visualization import draw_visualization

        actions = self._visualization_actions(view, rl)
        draw_visualization(view, self._inter_font, self._mono_font)
        return actions

    def _visualization_actions(
        self, view: VisualizationView, rl: object
    ) -> tuple[ChartAction, ...]:
        """Map pointer and keyboard input through the Phase 4 control router."""
        bounds = view.chart_bounds
        widget = Widget(
            WidgetId.root("phase5-chart"),
            ControlRect(bounds.min_x, bounds.min_y, bounds.width, bounds.height),
            ControlRect(bounds.min_x, bounds.min_y, bounds.width, bounds.height),
            PointerBehavior.DRAG,
        )
        mouse = VisualizationPoint(float(rl.get_mouse_x()), float(rl.get_mouse_y()))
        routed = route(
            self._visualization_controls,
            FrameInput(
                mouse.x,
                mouse.y,
                bool(rl.is_mouse_button_pressed(rl.MOUSE_BUTTON_LEFT)),
                bool(rl.is_mouse_button_down(rl.MOUSE_BUTTON_LEFT)),
                bool(rl.is_mouse_button_released(rl.MOUSE_BUTTON_LEFT)),
                focus_next=bool(rl.is_key_pressed(rl.KEY_TAB)),
                activate=bool(rl.is_key_pressed(rl.KEY_ENTER)),
                cancel=bool(rl.is_key_pressed(rl.KEY_ESCAPE)),
            ),
            (widget,),
        )
        self._visualization_controls = routed.state
        transform = VisualizationTransform(view.frame.data, bounds)
        data_mouse = transform.inverse(
            VisualizationPoint(
                min(max(mouse.x, bounds.min_x), bounds.max_x),
                min(max(mouse.y, bounds.min_y), bounds.max_y),
            )
        )
        actions: list[ChartAction] = []
        if bounds.contains(mouse):
            actions.append(ChartAction(InteractionKind.CROSSHAIR, anchor=data_mouse))
            wheel = float(rl.get_mouse_wheel_move())
            if wheel:
                actions.append(
                    ChartAction(
                        InteractionKind.ZOOM,
                        anchor=data_mouse,
                        factor=1.2 if wheel > 0 else 1 / 1.2,
                    )
                )
        for event in routed.events:
            if event.kind is ControlEventKind.PRESS:
                self._visualization_drag_start = data_mouse
                self._visualization_selecting = bool(
                    rl.is_key_down(rl.KEY_LEFT_SHIFT) or rl.is_key_down(rl.KEY_RIGHT_SHIFT)
                )
            elif event.kind is ControlEventKind.DRAG and not self._visualization_selecting:
                delta = rl.get_mouse_delta()
                actions.append(
                    ChartAction(
                        InteractionKind.PAN,
                        delta=VisualizationPoint(
                            -float(delta.x) * view.frame.data.width / bounds.width,
                            float(delta.y) * view.frame.data.height / bounds.height,
                        ),
                    )
                )
            elif event.kind is ControlEventKind.RELEASE:
                start = self._visualization_drag_start
                if (
                    self._visualization_selecting
                    and start is not None
                    and start.x != data_mouse.x
                    and start.y != data_mouse.y
                ):
                    actions.append(
                        ChartAction(
                            InteractionKind.SELECT,
                            selection=VisualizationRect(
                                min(start.x, data_mouse.x),
                                min(start.y, data_mouse.y),
                                max(start.x, data_mouse.x),
                                max(start.y, data_mouse.y),
                            ),
                        )
                    )
                self._visualization_drag_start = None
                self._visualization_selecting = False
        step_x, step_y = view.frame.data.width * 0.05, view.frame.data.height * 0.05
        key_actions = (
            (rl.KEY_LEFT, VisualizationPoint(-step_x, 0)),
            (rl.KEY_RIGHT, VisualizationPoint(step_x, 0)),
            (rl.KEY_UP, VisualizationPoint(0, step_y)),
            (rl.KEY_DOWN, VisualizationPoint(0, -step_y)),
        )
        actions.extend(
            ChartAction(InteractionKind.KEYBOARD_PAN, delta=delta)
            for key, delta in key_actions
            if rl.is_key_pressed(key)
        )
        if rl.is_key_pressed(rl.KEY_R):
            actions.append(ChartAction(InteractionKind.RESET))
        if rl.is_key_pressed(rl.KEY_V):
            actions.append(ChartAction(InteractionKind.TOGGLE_VISIBILITY, series_id="predictions"))
        if rl.is_key_pressed(rl.KEY_L):
            actions.append(ChartAction(InteractionKind.LINK_AXIS, linked_axis=view.frame.data))
        return tuple(actions)

    def _research_actions(
        self,
        view: ShellView,
        rl: object,
        mouse_x: float,
        mouse_y: float,
    ) -> tuple[UIAction, ...]:
        actions: list[UIAction] = []
        scenarios = tuple(ResearchScenario)
        groups_by_panel = dict(view.research_groups)
        seen_groups: set[str] = set()
        for stack in view.dock_stacks:
            active = next((tab for tab in stack.tabs if tab.active), None)
            if active is None or not active.panel_id.startswith("research-"):
                continue
            group = groups_by_panel.get(active.panel_id)
            if group is None or group.query is None:
                continue
            if group.state is ResearchLoadState.IDLE:
                if group.group_id not in seen_groups:
                    actions.append(RequestResearch(group.query))
                seen_groups.add(group.group_id)
                continue
            if group.group_id not in seen_groups and rl.is_key_pressed(rl.KEY_S):
                scenario_index = (scenarios.index(group.scenario) + 1) % len(scenarios)
                actions.append(SetResearchScenario(group.group_id, scenarios[scenario_index]))
            seen_groups.add(group.group_id)
            panel_type = active.panel_id.removeprefix("research-")
            body = stack.body_bounds
            scale = view.scale
            data_x = body.x + 14 * scale
            data_y = body.y + 58 * scale
            data_width = body.width - 28 * scale
            data_height = body.height - 72 * scale
            if panel_type != "ohlcv" and rl.is_mouse_button_pressed(rl.MOUSE_BUTTON_LEFT):
                scenario_x = data_x + data_width - 130 * scale
                if self._inside(
                    ShellRect(scenario_x, data_y - 28 * scale, 130 * scale, 20 * scale),
                    mouse_x,
                    mouse_y,
                ):
                    scenario_index = (scenarios.index(group.scenario) + 1) % len(scenarios)
                    actions.append(SetResearchScenario(group.group_id, scenarios[scenario_index]))
                    continue
            if panel_type == "ohlcv":
                actions.extend(
                    self._research_chart_actions(
                        group,
                        active.panel_id,
                        rl,
                        mouse_x,
                        mouse_y,
                        data_x,
                        data_y,
                        data_width,
                        data_height,
                        scale,
                    )
                )
            elif panel_type == "features" and group.snapshot is not None:
                if rl.is_mouse_button_pressed(rl.MOUSE_BUTTON_LEFT):
                    for index, series in enumerate(group.snapshot.features):
                        row = ShellRect(
                            data_x,
                            data_y + index * 46 * scale,
                            data_width,
                            42 * scale,
                        )
                        if self._inside(row, mouse_x, mouse_y):
                            if series.descriptor.name != group.selected_feature:
                                actions.append(
                                    SetResearchFeature(group.group_id, series.descriptor.name)
                                )
                            break
            elif (
                panel_type == "rows"
                and group.snapshot is not None
                and rl.is_mouse_button_pressed(rl.MOUSE_BUTTON_LEFT)
            ):
                table_y = data_y + 24 * scale
                toolbar_right = data_x + data_width
                query = group.query
                if toolbar_right - 56 * scale <= mouse_x <= toolbar_right:
                    if group.snapshot.rows.next_cursor:
                        actions.append(
                            RequestResearch(
                                self._next_research_query(
                                    group,
                                    cursor=group.snapshot.rows.next_cursor,
                                )
                            )
                        )
                elif toolbar_right - 116 * scale <= mouse_x:
                    offset = max(0, int(query.cursor or "0") - query.page_size)
                    actions.append(
                        RequestResearch(
                            self._next_research_query(
                                group,
                                cursor="" if offset == 0 else str(offset),
                            )
                        )
                    )
                elif toolbar_right - 194 * scale <= mouse_x:
                    sorts = tuple(ResearchSort)
                    next_sort = sorts[(sorts.index(query.sort) + 1) % len(sorts)]
                    actions.append(
                        RequestResearch(self._next_research_query(group, cursor="", sort=next_sort))
                    )
                elif toolbar_right - 280 * scale <= mouse_x:
                    actions.append(
                        RequestResearch(
                            self._next_research_query(
                                group,
                                cursor="",
                                filter_value=query.symbols[0] if not query.filter else "",
                            )
                        )
                    )
                else:
                    row_index = int((mouse_y - table_y) // (24 * scale)) - 1
                    rows = group.snapshot.rows.dataset.rows
                    if not 0 <= row_index < len(rows):
                        continue
                    control_down = bool(
                        rl.is_key_down(rl.KEY_LEFT_CONTROL) or rl.is_key_down(rl.KEY_RIGHT_CONTROL)
                    )
                    actions.append(
                        SelectResearchRow(
                            group.group_id,
                            rows[row_index].id,
                            toggle=control_down,
                        )
                    )
            if group.state in {
                ResearchLoadState.FAILED,
                ResearchLoadState.CANCELLED,
                ResearchLoadState.BUSY,
            } and rl.is_mouse_button_pressed(rl.MOUSE_BUTTON_LEFT):
                retry = ShellRect(data_x, data_y + 72 * scale, 82 * scale, 24 * scale)
                if self._inside(retry, mouse_x, mouse_y):
                    actions.append(SetResearchScenario(group.group_id, ResearchScenario.NORMAL))
        return tuple(actions)

    @staticmethod
    def _next_research_query(
        group: ResearchGroupState,
        *,
        cursor: str | None = None,
        sort: ResearchSort | None = None,
        filter_value: str | None = None,
    ) -> ResearchQuery:
        query = group.query
        if query is None:
            raise ValueError("Research group has no active query")
        generation = group.generation + 1
        return replace(
            query,
            generation=generation,
            correlation_id=f"research-{group.group_id}-{generation:020d}",
            cursor=query.cursor if cursor is None else cursor,
            sort=query.sort if sort is None else sort,
            filter=query.filter if filter_value is None else filter_value,
        )

    def _research_chart_actions(
        self,
        group: ResearchGroupState,
        panel_id: str,
        rl: object,
        mouse_x: float,
        mouse_y: float,
        x: float,
        y: float,
        width: float,
        height: float,
        scale: float,
    ) -> tuple[UIAction, ...]:
        actions: list[UIAction] = []
        query = group.query
        if query is None:
            return ()
        if rl.is_mouse_button_pressed(rl.MOUSE_BUTTON_LEFT):
            visibility_buttons = (
                (group.show_ohlcv, group.show_feature, group.show_target),
                (group.show_ohlcv, not group.show_feature, group.show_target),
                (group.show_ohlcv, group.show_feature, not group.show_target),
            )
            for index, visibility in enumerate(visibility_buttons):
                button = ShellRect(x + index * 73 * scale, y, 70 * scale, 22 * scale)
                if self._inside(button, mouse_x, mouse_y):
                    if index == 0:
                        visibility = (
                            not group.show_ohlcv,
                            group.show_feature,
                            group.show_target,
                        )
                    actions.append(SetResearchVisibility(group.group_id, *visibility))
                    return tuple(actions)
            reset = ShellRect(x + width - 60 * scale, y, 58 * scale, 22 * scale)
            if self._inside(reset, mouse_x, mouse_y):
                actions.append(
                    SetResearchRange(
                        group.group_id,
                        panel_id,
                        default_research_query().time_range,
                    )
                )
                return tuple(actions)
        plot = ShellRect(x, y + 24 * scale, width, max(0.0, height - 24 * scale))
        if not self._inside(plot, mouse_x, mouse_y):
            return tuple(actions)
        current = query.time_range
        wheel = float(rl.get_mouse_wheel_move())
        if wheel:
            anchor = (mouse_x - plot.x) / max(1.0, plot.width)
            actions.append(
                SetResearchRange(
                    group.group_id,
                    panel_id,
                    zoom_range(current, anchor, 1.2 if wheel > 0 else 1 / 1.2),
                )
            )
        if rl.is_mouse_button_pressed(rl.MOUSE_BUTTON_LEFT):
            panning = bool(rl.is_key_down(rl.KEY_LEFT_SHIFT) or rl.is_key_down(rl.KEY_RIGHT_SHIFT))
            self._research_drag = (panel_id, mouse_x, current, panning)
        if rl.is_mouse_button_released(rl.MOUSE_BUTTON_LEFT) and self._research_drag:
            drag_panel, start_x, original, panning = self._research_drag
            self._research_drag = None
            if drag_panel != panel_id or abs(mouse_x - start_x) < 3:
                return tuple(actions)
            if not panning:
                start_fraction = (start_x - plot.x) / max(1.0, plot.width)
                end_fraction = (mouse_x - plot.x) / max(1.0, plot.width)
                selected = select_range(original, start_fraction, end_fraction)
            else:
                selected = pan_range(original, -(mouse_x - start_x) / max(1.0, plot.width))
            actions.append(SetResearchRange(group.group_id, panel_id, selected))
        return tuple(actions)

    def _shell_actions(self, view: ShellView, rl: object) -> list[UIAction]:
        actions: list[UIAction] = []
        control_down: bool = bool(
            rl.is_key_down(rl.KEY_LEFT_CONTROL) or rl.is_key_down(rl.KEY_RIGHT_CONTROL)
        )
        if (
            not view.critical_error
            and control_down
            and (rl.is_key_pressed(rl.KEY_K) or rl.is_key_pressed(rl.KEY_P))
        ):
            actions.append(SetCommandPalette(True))
        if not view.critical_error and control_down and rl.is_key_pressed(rl.KEY_COMMA):
            actions.append(SetSettingsOpen(True))
        if control_down and (rl.is_key_pressed(rl.KEY_EQUAL) or rl.is_key_pressed(rl.KEY_KP_ADD)):
            actions.append(SetUIScale(min(200, view.ui_scale_percent + 25)))
        if control_down and (
            rl.is_key_pressed(rl.KEY_MINUS) or rl.is_key_pressed(rl.KEY_KP_SUBTRACT)
        ):
            actions.append(SetUIScale(max(100, view.ui_scale_percent - 25)))
        if control_down and (rl.is_key_pressed(rl.KEY_ZERO) or rl.is_key_pressed(rl.KEY_KP_0)):
            actions.append(SetUIScale(125))
        if view.command_palette_open:
            if rl.is_key_pressed(rl.KEY_DOWN):
                self._command_index = (self._command_index + 1) % 8
            if rl.is_key_pressed(rl.KEY_UP):
                self._command_index = (self._command_index - 1) % 8
            if (
                rl.is_key_pressed(rl.KEY_ENTER) or rl.is_key_pressed(rl.KEY_KP_ENTER)
            ) and self._command_index < len(view.tabs):
                actions.extend((view.tabs[self._command_index].action, SetCommandPalette(False)))
        if rl.is_key_pressed(rl.KEY_ESCAPE):
            self._dock_drag = None
            self._split_drag = None
            if view.command_palette_open:
                actions.append(SetCommandPalette(False))
            if view.settings_open:
                actions.append(SetSettingsOpen(False))
        mouse_x = float(rl.get_mouse_x())
        mouse_y = float(rl.get_mouse_y())
        actions.extend(self._research_actions(view, rl, mouse_x, mouse_y))
        actions.extend(self._phase7_actions(view, rl, control_down))
        actions.extend(self._phase8_actions(view, rl, mouse_x, mouse_y))
        actions.extend(self._phase9_actions(view, rl))
        self._dock_targets = ()
        if self._dock_drag is not None:
            _, start_x, start_y = self._dock_drag
            if (mouse_x - start_x) ** 2 + (mouse_y - start_y) ** 2 >= 36:
                target_stack = next(
                    (
                        stack
                        for stack in reversed(view.dock_stacks)
                        if self._inside(stack.bounds, mouse_x, mouse_y)
                    ),
                    None,
                )
                if target_stack is not None:
                    self._dock_targets = project_dock_drop_targets(
                        target_stack, mouse_x, mouse_y, view.scale
                    )
        if rl.is_mouse_button_pressed(rl.MOUSE_BUTTON_LEFT):
            if view.critical_error:
                return actions
            for splitter in view.dock_splitters:
                if self._inside(splitter.bounds, mouse_x, mouse_y):
                    self._split_drag = splitter.split_id
                    return actions
            for stack in view.dock_stacks:
                for tab in stack.tabs:
                    if self._inside(tab.bounds, mouse_x, mouse_y):
                        self._dock_drag = (tab.panel_id, mouse_x, mouse_y)
                        break
            if view.settings_open:
                actions.extend(self.settings_click_actions(view, mouse_x, mouse_y))
                return actions
            if view.command_palette_open:
                actions.extend(self.command_click_actions(view, mouse_x, mouse_y))
                return actions
            action = action_at(view, mouse_x, mouse_y)
            if action is not None and not (
                view.command_palette_open or view.settings_open or view.critical_error
            ):
                actions.append(action)
            scale = view.scale
            settings_x, settings_width, command_x, command_width = self._top_right_bounds(view)
            if (
                settings_x <= mouse_x <= settings_x + settings_width
                and 4 * scale <= mouse_y <= 32 * scale
            ):
                actions.append(SetSettingsOpen(True))
            if (
                command_x <= mouse_x < command_x + command_width
                and 4 * scale <= mouse_y <= 32 * scale
            ):
                actions.append(SetCommandPalette(True))
        if self._split_drag is not None and rl.is_mouse_button_down(rl.MOUSE_BUTTON_LEFT):
            ratio = min(0.95, max(0.05, mouse_x / max(1.0, view.viewport.width)))
            actions.append(ResizeDockSplit(self._split_drag, ratio, view.layout_revision))
        if rl.is_mouse_button_released(rl.MOUSE_BUTTON_LEFT):
            self._split_drag = None
            if self._dock_drag is not None:
                panel_id, start_x, start_y = self._dock_drag
                self._dock_drag = None
                if (mouse_x - start_x) ** 2 + (mouse_y - start_y) ** 2 >= 36:
                    target = next(
                        (
                            stack
                            for stack in reversed(view.dock_stacks)
                            if self._inside(stack.bounds, mouse_x, mouse_y)
                        ),
                        None,
                    )
                    hot = next((item for item in self._dock_targets if item.hot), None)
                    if target is not None and hot is not None:
                        position = hot.position
                        token = f"r{view.layout_revision + 1}-{panel_id}"
                        actions.append(
                            DockPanel(
                                panel_id,
                                target.stack_id,
                                position,
                                f"split-{token}",
                                f"stack-{token}",
                                view.layout_revision,
                            )
                        )
                self._dock_targets = ()
        return actions

    def _phase8_actions(
        self, view: ShellView, rl: object, mouse_x: float, mouse_y: float
    ) -> tuple[UIAction, ...]:
        current = view.phase8_workspace
        if current is None:
            return ()
        if current.state is Phase8LoadState.IDLE:
            workspace = (
                Phase8Workspace.JOBS
                if isinstance(current, JobsWorkspaceState)
                else Phase8Workspace.RESULTS
            )
            return (
                RequestPhase8(
                    Phase8Request(
                        f"phase8-{workspace.value}-00000000000000000001",
                        1,
                        workspace,
                        current.scenario,
                    )
                ),
            )
        if rl.is_key_pressed(rl.KEY_S):
            scenarios = (
                (
                    Phase8Scenario.JOBS_SUCCESS,
                    Phase8Scenario.JOBS_PAUSE_RESUME,
                    Phase8Scenario.JOBS_CANCELLATION,
                    Phase8Scenario.JOBS_INTERRUPTION,
                    Phase8Scenario.JOBS_FAILURE,
                    Phase8Scenario.JOBS_CHECKPOINT_INCOMPATIBLE,
                )
                if isinstance(current, JobsWorkspaceState)
                else (
                    Phase8Scenario.RESULTS_NORMAL,
                    Phase8Scenario.RESULTS_LOADING,
                    Phase8Scenario.RESULTS_FAILURE,
                    Phase8Scenario.RESULTS_DEGRADED,
                    Phase8Scenario.RESULTS_RECOVERED,
                    Phase8Scenario.RESULTS_EMPTY,
                )
            )
            scenario = scenarios[(scenarios.index(current.scenario) + 1) % len(scenarios)]
            workspace = (
                Phase8Workspace.JOBS
                if isinstance(current, JobsWorkspaceState)
                else Phase8Workspace.RESULTS
            )
            return (SetPhase8Scenario(workspace, scenario),)
        selected = view.selected_job
        if not isinstance(current, JobsWorkspaceState) or selected is None:
            return ()
        if selected.summary.state is JobState.RUNNING:
            allowed = (JobCommandKind.PAUSE, JobCommandKind.CANCEL)
        elif selected.summary.state in {JobState.PAUSED, JobState.INTERRUPTED}:
            allowed = (JobCommandKind.RESUME, JobCommandKind.CANCEL)
        elif selected.summary.state is JobState.QUEUED:
            allowed = (JobCommandKind.CANCEL,)
        else:
            allowed = ()
        chosen: JobCommandKind | None = None
        keys = (
            (JobCommandKind.PAUSE, rl.KEY_P),
            (JobCommandKind.RESUME, rl.KEY_R),
            (JobCommandKind.CANCEL, rl.KEY_C),
        )
        for kind, key in keys:
            if kind in allowed and rl.is_key_pressed(key):
                chosen = kind
                break
        if chosen is None and rl.is_mouse_button_pressed(rl.MOUSE_BUTTON_LEFT):
            progress = next(
                (stack for stack in view.dock_stacks if stack.active_panel_id == "jobs-progress"),
                None,
            )
            if progress is not None:
                button_y = progress.body_bounds.y + 106 * view.scale
                button_x = progress.body_bounds.x + 14 * view.scale
                for index, kind in enumerate(allowed):
                    bounds = ShellRect(
                        button_x + index * 82 * view.scale,
                        button_y,
                        72 * view.scale,
                        22 * view.scale,
                    )
                    if self._inside(bounds, mouse_x, mouse_y):
                        chosen = kind
                        break
        if chosen is None:
            return ()
        identity = f"{selected.summary.job_id}-{chosen.value}-g{current.generation:020d}"
        return (
            RequestJobCommand(
                JobCommand(
                    identity,
                    f"correlation-{identity}",
                    current.generation,
                    selected.summary.job_id,
                    chosen,
                )
            ),
        )

    def _phase9_actions(self, view: ShellView, rl: object) -> tuple[UIAction, ...]:
        current = view.phase9_workspace
        if current is None:
            return ()
        workspace = (
            Phase9Workspace.MODELS
            if isinstance(current, ModelsWorkspaceState)
            else Phase9Workspace.INFERENCE
        )
        if current.state is Phase9LoadState.IDLE:
            return (
                RequestPhase9(
                    Phase9Request(
                        f"phase9-{workspace.value}-00000000000000000001",
                        1,
                        workspace,
                        current.scenario,
                    )
                ),
            )
        if not rl.is_key_pressed(rl.KEY_S):
            return ()
        scenarios = (
            (
                Phase9Scenario.MODELS_NORMAL,
                Phase9Scenario.MODELS_LOADING,
                Phase9Scenario.MODELS_EMPTY,
                Phase9Scenario.MODELS_FAILURE,
                Phase9Scenario.MODELS_DEGRADED,
                Phase9Scenario.MODELS_RECOVERED,
            )
            if workspace is Phase9Workspace.MODELS
            else (
                Phase9Scenario.INFERENCE_NORMAL,
                Phase9Scenario.INFERENCE_LOADING,
                Phase9Scenario.INFERENCE_LATEST,
                Phase9Scenario.INFERENCE_INCOMPATIBLE,
                Phase9Scenario.INFERENCE_EMPTY,
                Phase9Scenario.INFERENCE_FAILURE,
                Phase9Scenario.INFERENCE_DEGRADED,
                Phase9Scenario.INFERENCE_RECOVERED,
            )
        )
        scenario = scenarios[(scenarios.index(current.scenario) + 1) % len(scenarios)]
        return (SetPhase9Scenario(workspace, scenario),)

    def _phase7_actions(
        self, view: ShellView, rl: object, control_down: bool
    ) -> tuple[UIAction, ...]:
        current = view.phase7_workspace
        if current is None:
            return ()
        selected = next(tab.action.workspace for tab in view.tabs if tab.selected)
        workspace = (
            Phase7Workspace.DATA if selected.value == "data" else Phase7Workspace.EXPERIMENTS
        )
        if current.state is Phase7LoadState.IDLE:
            return (
                RequestPhase7(
                    Phase7Request(
                        f"phase7-{workspace.value}-00000000000000000001",
                        1,
                        workspace,
                    )
                ),
            )
        scenarios = (
            Phase7Scenario.NORMAL,
            Phase7Scenario.LOADING,
            Phase7Scenario.FAILURE,
            Phase7Scenario.DEGRADED,
            Phase7Scenario.RECOVERED,
        )
        if rl.is_key_pressed(rl.KEY_S):
            scenario = scenarios[(scenarios.index(current.scenario) + 1) % len(scenarios)]
            return (SetPhase7Scenario(workspace, scenario),)
        if current.snapshot is None:
            return ()
        if workspace is Phase7Workspace.DATA and rl.is_key_pressed(rl.KEY_I):
            entry = current.snapshot.catalog[0]
            generation = current.generation
            return (
                RequestDataImport(
                    ImportRequest(
                        f"import-command-{generation:020d}",
                        f"import-{generation:020d}",
                        generation,
                        entry.dataset_id,
                        entry.revision,
                        SourceKind.CSV,
                        "demo-bars.csv",
                        entry.symbols,
                        entry.interval,
                        AdjustmentPolicy.SPLIT_AND_DIVIDEND,
                        ImportMode.APPEND,
                    )
                ),
            )
        if workspace is Phase7Workspace.EXPERIMENTS:
            draft = current.draft
            if draft is None:
                return ()
            if rl.is_key_pressed(rl.KEY_E):
                edited = replace(
                    draft, revision=draft.revision + 1, estimator_count=draft.estimator_count + 10
                )
                return (
                    EditExperimentDraft(edited),
                    RequestDraftEvaluation(
                        f"evaluate-{current.generation:020d}", current.generation, edited
                    ),
                )
            if control_down and rl.is_key_pressed(rl.KEY_S):
                return (
                    RequestDraftSave(
                        DraftSaveRequest(
                            f"save-{current.generation:020d}",
                            current.generation,
                            draft,
                            current.saved_revision,
                        )
                    ),
                )
            if control_down and rl.is_key_pressed(rl.KEY_ENTER):
                return (
                    RequestSubmission(
                        SubmissionRequest(
                            f"submit-{current.generation:020d}",
                            f"submit-command-{current.generation:020d}",
                            current.generation,
                            draft,
                        )
                    ),
                )
        return ()

    def _draw_dock_targets(self, rl: object) -> None:
        hot = next((target for target in self._dock_targets if target.hot), None)
        if hot is not None:
            self._rect(
                rl,
                hot.preview_bounds.x,
                hot.preview_bounds.y,
                hot.preview_bounds.width,
                hot.preview_bounds.height,
                (60, 200, 200, 42),
            )
            self._outline(
                rl,
                hot.preview_bounds.x,
                hot.preview_bounds.y,
                hot.preview_bounds.width,
                hot.preview_bounds.height,
                (60, 200, 200, 220),
            )
        for target in self._dock_targets:
            fill = (60, 200, 200, 235) if target.hot else (23, 28, 34, 235)
            edge = (214, 220, 229, 255) if target.hot else (60, 200, 200, 220)
            self._rect(
                rl,
                target.bounds.x,
                target.bounds.y,
                target.bounds.width,
                target.bounds.height,
                fill,
            )
            self._outline(
                rl,
                target.bounds.x,
                target.bounds.y,
                target.bounds.width,
                target.bounds.height,
                edge,
            )
            inset = target.bounds.width * 0.27
            marker_x = target.bounds.x + inset
            marker_y = target.bounds.y + inset
            marker_w = target.bounds.width - 2 * inset
            marker_h = target.bounds.height - 2 * inset
            if target.position is DockPosition.LEFT:
                marker_w /= 2
            elif target.position is DockPosition.RIGHT:
                marker_x += marker_w / 2
                marker_w /= 2
            elif target.position is DockPosition.TOP:
                marker_h /= 2
            elif target.position is DockPosition.BOTTOM:
                marker_y += marker_h / 2
                marker_h /= 2
            self._rect(rl, marker_x, marker_y, marker_w, marker_h, edge)

    @staticmethod
    def _inside(bounds: object, x: float, y: float) -> bool:
        return bool(
            hasattr(bounds, "x")
            and bounds.x <= x <= bounds.x + bounds.width
            and bounds.y <= y <= bounds.y + bounds.height
        )

    def settings_click_actions(self, view: ShellView, x: float, y: float) -> list[UIAction]:
        """Map a Settings-overlay click to closed actions."""
        scale = view.scale
        left, top, width, _ = self._modal_bounds(view, 620, 350)
        if (
            left + width - 88 * scale <= x <= left + width - 20 * scale
            and top + 14 * scale <= y <= top + 46 * scale
        ):
            return [SetSettingsOpen(False)]
        button_y = top + 166 * scale
        padding, gap = 20 * scale, 8 * scale
        button_width = (width - 2 * padding - 4 * gap) / 5
        for index, preset in enumerate((100, 125, 150, 175, 200)):
            button_x = left + padding + index * (button_width + gap)
            if button_x <= x <= button_x + button_width and button_y <= y <= button_y + 38 * scale:
                return [] if preset == view.ui_scale_percent else [SetUIScale(preset)]
        return []

    def command_click_actions(self, view: ShellView, x: float, y: float) -> list[UIAction]:
        """Map a command-palette click to closed actions."""
        scale = view.scale
        left, top, width, height = self._modal_bounds(view, 620, 420)
        row_y = top + 102 * scale
        for index in range(8):
            if row_y > top + height - 44 * scale:
                break
            if (
                left + 10 * scale <= x <= left + width - 10 * scale
                and row_y <= y <= row_y + 38 * scale
            ):
                self._command_index = index
                if index < len(view.tabs):
                    return [view.tabs[index].action, SetCommandPalette(False)]
                return [SetCommandPalette(False)]
            row_y += 40 * scale
        return []

    def _draw_top_navigation(self, view: ShellView, rl: object) -> None:
        scale = view.scale
        width = view.viewport.width
        self._rect(rl, 0, 0, width, 36 * scale, (17, 21, 26, 255))
        self._line(rl, 0, 36 * scale - 1, width, 36 * scale - 1, (37, 43, 51, 255))
        self._text(
            rl,
            self._inter_font,
            "C" if width < 1500 * scale else "Corthena",
            12 * scale,
            8 * scale,
            14,
            scale,
            (214, 220, 229, 255),
        )
        for tab in view.tabs:
            color = (23, 28, 34, 255) if tab.selected else (17, 21, 26, 255)
            self._rect(rl, tab.bounds.x, tab.bounds.y, tab.bounds.width, tab.bounds.height, color)
            if tab.selected:
                self._rect(
                    rl,
                    tab.bounds.x,
                    tab.bounds.y + tab.bounds.height - 2 * scale,
                    tab.bounds.width,
                    2 * scale,
                    (60, 200, 200, 255),
                )
            self._text(
                rl,
                self._inter_font,
                tab.label,
                tab.bounds.x + 10 * scale,
                tab.bounds.y + 5 * scale,
                11,
                scale,
                (214, 220, 229, 255) if tab.selected else (126, 136, 150, 255),
            )
        settings_x, settings_width, command_x, command_width = self._top_right_bounds(view)
        compact = width < 1500 * scale
        self._nav_button(
            rl,
            command_x,
            4 * scale,
            command_width,
            28 * scale,
            "Cmd" if compact else "Ctrl+K Command",
            view.command_palette_open,
            scale,
        )
        self._nav_button(
            rl,
            settings_x,
            4 * scale,
            settings_width,
            28 * scale,
            "Set" if compact else "Settings",
            view.settings_open,
            scale,
        )

    @staticmethod
    def _top_right_bounds(view: ShellView) -> tuple[float, float, float, float]:
        scale = view.scale
        compact = view.viewport.width < 1500 * scale
        settings_width = (52 if compact else 92) * scale
        command_width = (52 if compact else 132) * scale
        settings_x = view.viewport.width - 12 * scale - settings_width
        command_x = settings_x - 8 * scale - command_width
        return settings_x, settings_width, command_x, command_width

    def _draw_context_bar(self, view: ShellView, rl: object) -> None:
        scale = view.scale
        y = 36 * scale
        self._rect(rl, 0, y, view.viewport.width, 40 * scale, (11, 13, 16, 255))
        self._line(
            rl, 0, y + 40 * scale - 1, view.viewport.width, y + 40 * scale - 1, (37, 43, 51, 255)
        )
        x = 12 * scale
        x = self._context_item(rl, x, y, "Dataset", view.dataset_name, (60, 200, 200, 255), scale)
        x = self._context_item(rl, x, y, "Symbols", view.symbols, (155, 124, 246, 255), scale)
        x = self._context_item(rl, x, y, "Interval", view.interval, (214, 220, 229, 255), scale)
        if view.viewport.width >= 1100 * scale:
            self._context_item(rl, x, y, "Range", view.date_range, (214, 220, 229, 255), scale)

    def _draw_left_rail(self, view: ShellView, rl: object) -> None:
        scale = view.scale
        bounds = view.content_bounds
        left_width = 260 * scale if bounds.width >= 1100 * scale else 218 * scale
        self._rect(rl, 0, bounds.y, left_width, bounds.height, (17, 21, 26, 255))
        self._line(
            rl,
            left_width - 1,
            bounds.y,
            left_width - 1,
            bounds.y + bounds.height,
            (37, 43, 51, 255),
        )
        self._text(
            rl,
            self._inter_font,
            "Workspace Panels",
            10 * scale,
            bounds.y + 10 * scale,
            11,
            scale,
            (126, 136, 150, 255),
        )
        y = bounds.y + 34 * scale
        for panel in view.panels:
            if panel.selected:
                self._rect(
                    rl, 10 * scale, y, left_width - 20 * scale, 22 * scale, (23, 28, 34, 255)
                )
                self._rect(rl, 10 * scale, y, 2 * scale, 22 * scale, (60, 200, 200, 255))
            self._text(
                rl,
                self._inter_font,
                panel.title,
                18 * scale,
                y + 5 * scale,
                11,
                scale,
                (214, 220, 229, 255),
            )
            y += 24 * scale
        bottom = bounds.y + bounds.height
        global_y = bottom - 104 * scale
        show_global = y + 10 * scale <= global_y
        component_bottom = global_y if show_global else bottom
        component_y = y + 10 * scale
        if component_y + 54 * scale <= component_bottom:
            self._text(
                rl,
                self._inter_font,
                "Component Status",
                10 * scale,
                component_y,
                11,
                scale,
                (126, 136, 150, 255),
            )
            component_y += 24 * scale
            for component in view.components:
                if component_y + 30 * scale > component_bottom:
                    break
                self._rect(
                    rl,
                    10 * scale,
                    component_y,
                    left_width - 20 * scale,
                    24 * scale,
                    (23, 28, 34, 255),
                )
                self._rect(rl, 10 * scale, component_y, 3 * scale, 24 * scale, component.color)
                self._text(
                    rl,
                    self._inter_font,
                    component.title,
                    20 * scale,
                    component_y + 5 * scale,
                    11,
                    scale,
                    (214, 220, 229, 255),
                )
                self._text(
                    rl,
                    self._inter_font,
                    component.detail,
                    10 * scale + (left_width - 20 * scale) * 0.52,
                    component_y + 5 * scale,
                    10,
                    scale,
                    (126, 136, 150, 255),
                )
                component_y += 30 * scale
        if show_global:
            self._text(
                rl,
                self._inter_font,
                "Global Context",
                10 * scale,
                global_y,
                11,
                scale,
                (126, 136, 150, 255),
            )
            self._small_line(
                rl, 10 * scale, global_y + 24 * scale, "Dataset", view.dataset_id, scale
            )
            self._small_line(rl, 10 * scale, global_y + 44 * scale, "Run", view.run_id, scale)
            self._small_line(rl, 10 * scale, global_y + 64 * scale, "Model", view.model_id, scale)

    def _draw_data_host(self, view: ShellView, rl: object) -> None:
        scale = view.scale
        content = view.content_bounds
        left_width = 260 * scale if content.width >= 1100 * scale else 218 * scale
        self._rect(
            rl,
            left_width + scale,
            content.y,
            content.width - left_width - scale,
            content.height,
            (11, 13, 16, 255),
        )
        for stack in view.dock_stacks:
            bounds, header, body = stack.bounds, stack.header_bounds, stack.body_bounds
            self._rect(rl, bounds.x, bounds.y, bounds.width, bounds.height, (17, 21, 26, 255))
            self._outline(rl, bounds.x, bounds.y, bounds.width, bounds.height, (37, 43, 51, 255))
            self._rect(rl, header.x, header.y, header.width, header.height, (23, 28, 34, 255))
            self._line(
                rl,
                header.x,
                header.y + header.height - scale,
                header.x + header.width,
                header.y + header.height - scale,
                (37, 43, 51, 255),
            )
            for tab in stack.tabs:
                self._rect(
                    rl,
                    tab.bounds.x,
                    tab.bounds.y,
                    tab.bounds.width,
                    tab.bounds.height,
                    (11, 13, 16, 255) if tab.active else (17, 21, 26, 255),
                )
                if tab.active:
                    self._rect(
                        rl,
                        tab.bounds.x,
                        tab.bounds.y + tab.bounds.height - 2 * scale,
                        tab.bounds.width,
                        2 * scale,
                        (60, 200, 200, 255),
                    )
                self._text(
                    rl,
                    self._inter_font,
                    tab.title,
                    tab.bounds.x + 7 * scale,
                    tab.bounds.y + 8 * scale,
                    10,
                    scale,
                    (214, 220, 229, 255),
                )
            self._dock_header_buttons(rl, header.x, header.y, header.width, scale)
            self._draw_stack_body(rl, view, stack, body, scale)
        for splitter in view.dock_splitters:
            self._rect(
                rl,
                splitter.bounds.x,
                splitter.bounds.y,
                splitter.bounds.width,
                splitter.bounds.height,
                (37, 43, 51, 255),
            )

    def _draw_stack_body(
        self, rl: object, view: ShellView, stack: object, body: object, scale: float
    ) -> None:
        selected_panel = next(tab for tab in stack.tabs if tab.active)
        body_x, body_y, body_w, body_h = body.x, body.y, body.width, body.height
        rl.begin_scissor_mode(round(body_x), round(body_y), round(body_w), round(body_h))
        try:
            self._text(
                rl,
                self._inter_font,
                selected_panel.title,
                body_x + 14 * scale,
                body_y + 12 * scale,
                16,
                scale,
                (214, 220, 229, 255),
            )
            self._text(
                rl,
                self._inter_font,
                "Deterministic demo data",
                body_x + 14 * scale,
                body_y + 34 * scale,
                11,
                scale,
                (126, 136, 150, 255),
            )
            data_x, data_y = body_x + 14 * scale, body_y + 58 * scale
            data_w, data_h = body_w - 28 * scale, body_h - 72 * scale
            research_group = dict(view.research_groups).get(selected_panel.panel_id)
            if research_group is not None and selected_panel.panel_id.startswith("research-"):
                self._draw_research_panel(
                    rl,
                    selected_panel.panel_id.removeprefix("research-"),
                    research_group,
                    data_x,
                    data_y,
                    data_w,
                    data_h,
                    scale,
                )
            elif view.phase7_workspace is not None and selected_panel.panel_id.startswith(
                ("data-", "experiments-")
            ):
                self._draw_phase7_panel(
                    rl,
                    view,
                    selected_panel.panel_id,
                    data_x,
                    data_y,
                    data_w,
                    data_h,
                    scale,
                )
            elif view.phase8_workspace is not None and selected_panel.panel_id.startswith(
                ("jobs-", "results-")
            ):
                self._draw_phase8_panel(
                    rl,
                    view,
                    selected_panel.panel_id,
                    data_x,
                    data_y,
                    data_w,
                    data_h,
                    scale,
                )
            elif view.phase9_workspace is not None and selected_panel.panel_id.startswith(
                ("models-", "inference-")
            ):
                self._draw_phase9_panel(
                    rl,
                    view,
                    selected_panel.panel_id,
                    data_x,
                    data_y,
                    data_w,
                    data_h,
                    scale,
                )
            elif selected_panel.title == "Catalog":
                self._draw_catalog(rl, view, data_x, data_y, data_w, data_h, scale)
            else:
                self._outline(rl, data_x, data_y, data_w, data_h, (37, 43, 51, 255))
                self._text(
                    rl,
                    self._inter_font,
                    selected_panel.title,
                    data_x + 16 * scale,
                    data_y + 16 * scale,
                    12,
                    scale,
                    (126, 136, 150, 255),
                )
        finally:
            rl.end_scissor_mode()

    def _draw_phase9_panel(
        self,
        rl: object,
        view: ShellView,
        panel_id: str,
        x: float,
        y: float,
        width: float,
        height: float,
        scale: float,
    ) -> None:
        current = view.phase9_workspace
        if current is None:
            return
        control_width = 174 * scale
        control_x = x + width - control_width
        self._rect(rl, control_x, y, control_width, 20 * scale, (23, 28, 34, 255))
        self._outline(rl, control_x, y, control_width, 20 * scale, (37, 43, 51, 255))
        scenario = current.scenario.value.removeprefix("models_").removeprefix("inference_")
        self._text(
            rl,
            self._inter_font,
            f"Scenario: {scenario}",
            control_x + 6 * scale,
            y + 5 * scale,
            10,
            scale,
            (214, 220, 229, 255),
        )
        if current.state is Phase9LoadState.LOADING and not current.stale:
            self._text(
                rl,
                self._inter_font,
                "Preparing immutable model workflow...",
                x,
                y + 34 * scale,
                11,
                scale,
                (126, 136, 150, 255),
            )
            return
        if (
            current.state
            in {
                Phase9LoadState.FAILED,
                Phase9LoadState.CANCELLED,
                Phase9LoadState.BUSY,
            }
            and not current.stale
        ):
            self._outline(rl, x, y + 28 * scale, width, 64 * scale, (239, 107, 115, 255))
            self._text(
                rl,
                self._inter_font,
                current.error or "Phase 9 request failed",
                x + 12 * scale,
                y + 44 * scale,
                11,
                scale,
                (239, 107, 115, 255),
            )
            return
        snapshot = current.snapshot
        if snapshot is None:
            return
        content_y = y + 30 * scale
        lines: list[tuple[str, str]] = []
        if isinstance(current, ModelsWorkspaceState):
            selected = next(
                (item for item in snapshot.models if item.model_id == current.selected_model_id),
                snapshot.models[0] if snapshot.models else None,
            )
            if panel_id == "models-registry":
                lines = [(item.model_id, item.artifact.model_kind) for item in snapshot.models]
            elif panel_id == "models-aliases":
                lines = list(snapshot.aliases) + [
                    (event.alias, f"{event.previous_model_id or '-'} -> {event.model_id}")
                    for event in snapshot.alias_history[-4:]
                ]
            elif selected is not None and panel_id == "models-artifact":
                lines = [
                    (
                        "Schema / engine",
                        f"v{selected.artifact.schema_version} / {selected.artifact.engine_version}",
                    ),
                    ("Training cutoff", selected.artifact.training_cutoff.isoformat()),
                    ("Fingerprint", selected.artifact.training_fingerprint[:20]),
                    ("Build", selected.artifact.build_revision),
                    ("Status", selected.artifact.status.value),
                ]
            elif selected is not None and panel_id == "models-importance":
                lines = [
                    (feature.name, f"{importance:.3f}")
                    for feature, importance in zip(
                        selected.features, selected.feature_importance, strict=True
                    )
                ]
            elif selected is not None:
                tree = selected.trees[current.selected_tree_index]
                lines = [
                    (
                        f"Node {index}",
                        "leaf"
                        if left == -1
                        else f"f{tree.feature[index]} <= {tree.threshold[index]:.4f}",
                    )
                    for index, left in enumerate(tree.left)
                ]
        else:
            inference = current.inference
            if panel_id == "inference-selector":
                lines = [
                    ("Model / alias", current.selected_model_or_alias),
                    ("Dataset", "dataset-us-equities"),
                    ("Mode", current.scenario.value.removeprefix("inference_")),
                ]
            elif panel_id == "inference-compatibility":
                report = (
                    inference.compatibility if inference is not None else snapshot.compatibility
                )
                lines = (
                    [("Compatible", "yes" if report.compatible else "no")]
                    + [(item.field, item.message) for item in report.issues]
                    if report is not None
                    else [("Status", "Awaiting selection")]
                )
            elif panel_id == "inference-rankings" and inference is not None:
                lines = [
                    (
                        str(item.rank) if item.rank is not None else "-",
                        (
                            f"{item.prediction.symbol_id}  "
                            + (
                                str(item.prediction.score)
                                if item.prediction.score is not None
                                else "missing"
                            )
                        ),
                    )
                    for item in inference.rankings
                ]
            elif panel_id == "inference-distribution" and inference is not None:
                distribution = inference.distribution
                lines = (
                    [
                        (
                            f"{distribution.edges[index]:.2f}-{distribution.edges[index + 1]:.2f}",
                            str(count),
                        )
                        for index, count in enumerate(distribution.counts)
                    ]
                    if distribution is not None
                    else []
                )
            elif panel_id == "inference-history":
                lines = [
                    (item.inference_id, item.checksum[:16] if item.checksum else "incompatible")
                    for item in current.history
                ]
            elif panel_id == "inference-export":
                lines = [
                    ("State", current.export_state.value),
                    (
                        "Rows",
                        str(current.export_result.row_count) if current.export_result else "-",
                    ),
                    (
                        "Checksum",
                        current.export_result.checksum[:20] if current.export_result else "-",
                    ),
                ]
        for index, (label, value) in enumerate(lines[:10]):
            row_y = content_y + index * 30 * scale
            if index % 2:
                self._rect(rl, x, row_y, width, 28 * scale, (23, 28, 34, 180))
            self._text(
                rl,
                self._inter_font,
                label,
                x + 8 * scale,
                row_y + 8 * scale,
                10,
                scale,
                (214, 220, 229, 255),
            )
            self._text(
                rl,
                self._mono_font,
                value,
                x + width * 0.42,
                row_y + 8 * scale,
                9,
                scale,
                (126, 136, 150, 255),
            )

    def _draw_phase8_panel(
        self,
        rl: object,
        view: ShellView,
        panel_id: str,
        x: float,
        y: float,
        width: float,
        height: float,
        scale: float,
    ) -> None:
        current = view.phase8_workspace
        if current is None:
            return
        control_width = 152 * scale
        control_x = x + width - control_width
        self._rect(rl, control_x, y, control_width, 20 * scale, (23, 28, 34, 255))
        self._outline(rl, control_x, y, control_width, 20 * scale, (37, 43, 51, 255))
        self._text(
            rl,
            self._inter_font,
            f"Scenario: {current.scenario.value.removeprefix('jobs_').removeprefix('results_')}",
            control_x + 6 * scale,
            y + 5 * scale,
            10,
            scale,
            (214, 220, 229, 255),
        )
        if current.state is Phase8LoadState.LOADING and not current.stale:
            self._text(
                rl,
                self._inter_font,
                "Loading immutable workflow state...",
                x,
                y + 30 * scale,
                11,
                scale,
                (126, 136, 150, 255),
            )
            return
        if (
            current.state
            in {
                Phase8LoadState.FAILED,
                Phase8LoadState.CANCELLED,
                Phase8LoadState.BUSY,
            }
            and not current.stale
        ):
            self._outline(rl, x, y + 28 * scale, width, 72 * scale, (239, 107, 115, 255))
            self._text(
                rl,
                self._inter_font,
                current.error or "Phase 8 request failed",
                x + 12 * scale,
                y + 42 * scale,
                11,
                scale,
                (239, 107, 115, 255),
            )
            return
        snapshot = current.snapshot
        if snapshot is None:
            return
        content_y = y + 28 * scale
        if isinstance(current, JobsWorkspaceState):
            self._draw_phase8_jobs_panel(
                rl, view, panel_id, snapshot.jobs, x, content_y, width, height, scale
            )
        else:
            self._draw_phase8_results_panel(
                rl, view, panel_id, snapshot.runs, x, content_y, width, height, scale
            )

    def _draw_phase8_jobs_panel(
        self,
        rl: object,
        view: ShellView,
        panel_id: str,
        jobs: tuple[object, ...],
        x: float,
        y: float,
        width: float,
        height: float,
        scale: float,
    ) -> None:
        selected = view.selected_job
        if panel_id == "jobs-queue":
            columns = (("Job", 0.0), ("State", 0.50), ("Stage", 0.68), ("%", 0.90))
            self._rect(rl, x, y, width, 24 * scale, (23, 28, 34, 255))
            for label, ratio in columns:
                self._text(
                    rl,
                    self._inter_font,
                    label,
                    x + width * ratio + 6 * scale,
                    y + 7 * scale,
                    10,
                    scale,
                    (214, 220, 229, 255),
                )
            row_height = 25 * scale
            visible = max(0, min(len(jobs), int((height - 56 * scale) / row_height)))
            for index, raw in enumerate(jobs[:visible]):
                detail = raw
                summary = detail.summary
                row_y = y + 24 * scale + index * row_height
                self._outline(rl, x, row_y, width, row_height, (37, 43, 51, 255))
                values = (
                    summary.job_id,
                    summary.state.value,
                    summary.stage_title,
                    str(round(summary.progress * 100)),
                )
                for value, (_, ratio) in zip(values, columns, strict=True):
                    self._text(
                        rl,
                        self._mono_font,
                        value[:30],
                        x + width * ratio + 6 * scale,
                        row_y + 7 * scale,
                        9,
                        scale,
                        (214, 220, 229, 255),
                    )
            return
        if selected is None:
            self._text(
                rl, self._inter_font, "No job selected", x, y, 11, scale, (126, 136, 150, 255)
            )
            return
        if panel_id == "jobs-progress":
            allowed = []
            if selected.summary.state.value == "running":
                allowed = ["pause", "cancel"]
            elif selected.summary.state.value in {"paused", "interrupted"}:
                allowed = ["resume", "cancel"]
            elif selected.summary.state.value == "queued":
                allowed = ["cancel"]
            self._text(
                rl,
                self._mono_font,
                f"{selected.summary.state.value}  {selected.summary.stage_title}",
                x,
                y,
                10,
                scale,
                (60, 200, 200, 255),
            )
            for index, label in enumerate(allowed):
                button_x = x + index * 82 * scale
                self._rect(rl, button_x, y + 20 * scale, 72 * scale, 22 * scale, (23, 28, 34, 255))
                self._outline(
                    rl, button_x, y + 20 * scale, 72 * scale, 22 * scale, (37, 43, 51, 255)
                )
                self._text(
                    rl,
                    self._inter_font,
                    label,
                    button_x + 8 * scale,
                    y + 26 * scale,
                    10,
                    scale,
                    (214, 220, 229, 255),
                )
            for index, stage in enumerate(selected.stages):
                row_y = y + (48 + index * 38) * scale
                self._text(
                    rl, self._inter_font, stage.title, x, row_y, 10, scale, (214, 220, 229, 255)
                )
                self._text(
                    rl,
                    self._mono_font,
                    stage.state.value,
                    x + width - 92 * scale,
                    row_y,
                    9,
                    scale,
                    (60, 200, 200, 255),
                )
                self._outline(rl, x, row_y + 18 * scale, width, 7 * scale, (37, 43, 51, 255))
                self._rect(
                    rl,
                    x,
                    row_y + 18 * scale,
                    width * stage.completed_units / stage.total_units,
                    7 * scale,
                    (60, 200, 200, 255),
                )
        elif panel_id == "jobs-metrics":
            for index, metric in enumerate(selected.metrics):
                row_y = y + index * 26 * scale
                self._text(
                    rl, self._inter_font, metric.name, x, row_y, 10, scale, (60, 200, 200, 255)
                )
                self._text(
                    rl,
                    self._mono_font,
                    f"{metric.value:.4f}",
                    x + width - 90 * scale,
                    row_y,
                    10,
                    scale,
                    (214, 220, 229, 255),
                )
        elif panel_id in {"jobs-workers", "jobs-processes"}:
            worker = selected.worker
            lease = selected.lease
            rows = (
                ("Worker", worker.worker_id if worker else "not started"),
                ("PID", str(worker.process_id) if worker else "-"),
                ("State", worker.detail if worker else selected.summary.state.value),
                ("CPU lease", f"{lease.active_slots}/{lease.granted_slots}" if lease else "queued"),
            )
            for index, (label, value) in enumerate(rows):
                self._text(
                    rl,
                    self._inter_font,
                    label,
                    x,
                    y + index * 25 * scale,
                    10,
                    scale,
                    (214, 220, 229, 255),
                )
                self._text(
                    rl,
                    self._mono_font,
                    value,
                    x + 76 * scale,
                    y + index * 25 * scale,
                    10,
                    scale,
                    (214, 220, 229, 255),
                )
        elif panel_id == "jobs-checkpoints":
            checkpoint = selected.checkpoint
            rows = (
                ("Checkpoint", checkpoint.checkpoint_id or "not available"),
                ("Compatibility", checkpoint.compatibility.value),
                ("Committed", str(checkpoint.committed_sequence)),
                ("Previous valid", checkpoint.previous_valid_checkpoint_id or "none"),
                ("Detail", checkpoint.detail),
            )
            for index, (label, value) in enumerate(rows):
                self._text(
                    rl,
                    self._inter_font,
                    label,
                    x,
                    y + index * 24 * scale,
                    10,
                    scale,
                    (126, 136, 150, 255),
                )
                self._text(
                    rl,
                    self._mono_font,
                    value[:45],
                    x + 96 * scale,
                    y + index * 24 * scale,
                    9,
                    scale,
                    (214, 220, 229, 255),
                )
        elif panel_id == "jobs-logs":
            for index, item in enumerate(selected.logs[-8:]):
                row_y = y + index * 26 * scale
                self._text(
                    rl,
                    self._mono_font,
                    item.timestamp.strftime("%H:%M:%S"),
                    x,
                    row_y,
                    9,
                    scale,
                    (126, 136, 150, 255),
                )
                self._text(
                    rl,
                    self._mono_font,
                    item.level,
                    x + 58 * scale,
                    row_y,
                    9,
                    scale,
                    (60, 200, 200, 255),
                )
                self._text(
                    rl,
                    self._mono_font,
                    item.message[:48],
                    x + 104 * scale,
                    row_y,
                    9,
                    scale,
                    (214, 220, 229, 255),
                )

    def _draw_phase8_results_panel(
        self,
        rl: object,
        view: ShellView,
        panel_id: str,
        runs: tuple[object, ...],
        x: float,
        y: float,
        width: float,
        height: float,
        scale: float,
    ) -> None:
        if panel_id == "results-runs":
            for index, raw in enumerate(runs[:8]):
                run = raw
                row_y = y + index * 44 * scale
                self._text(
                    rl, self._mono_font, run.run_id, x, row_y, 10, scale, (214, 220, 229, 255)
                )
                self._text(
                    rl,
                    self._mono_font,
                    f"immutable  {run.completed_at:%Y-%m-%d %H:%M}",
                    x + 8 * scale,
                    row_y + 18 * scale,
                    9,
                    scale,
                    (76, 195, 138, 255),
                )
                self._outline(rl, x, row_y + 38 * scale, width, 1, (37, 43, 51, 255))
            return
        comparison = view.run_comparison
        if comparison is None:
            self._text(
                rl,
                self._inter_font,
                "Preparing immutable comparison...",
                x,
                y,
                10,
                scale,
                (126, 136, 150, 255),
            )
            return
        if panel_id == "results-metrics":
            run = comparison.runs[0]
            for index, metric in enumerate(run.metrics):
                row_y = y + index * 27 * scale
                test = metric.partition is MetricPartition.TEST
                color = (216, 180, 90, 255) if test else (60, 200, 200, 255)
                label = f"{'TEST ' if test else 'validation '}{metric.metric}"
                self._text(rl, self._mono_font, label, x, row_y, 9, scale, color)
                value = "missing" if metric.value is None else f"{metric.value:.4f}"
                self._text(rl, self._mono_font, value, x + width * 0.55, row_y, 9, scale, color)
                self._outline(rl, x, row_y + 18 * scale, width, 1, (37, 43, 51, 255))
            self._text(
                rl,
                self._inter_font,
                "Validation selects; TEST remains isolated from tuning.",
                x,
                y + min(height - 20 * scale, 190 * scale),
                9,
                scale,
                (216, 180, 90, 255),
            )
        elif panel_id in {"results-equity", "results-predictions"}:
            series = comparison.runs[0].equity.points
            if len(series) > 1:
                low = min(item.value for item in series)
                high = max(item.value for item in series)
                span = max(1.0, high - low)
                for left, right in pairwise(series):
                    x1 = x + width * left.logical_index / (len(series) - 1)
                    x2 = x + width * right.logical_index / (len(series) - 1)
                    y1 = y + height * 0.72 - (left.value - low) / span * height * 0.62
                    y2 = y + height * 0.72 - (right.value - low) / span * height * 0.62
                    rl.draw_line(round(x1), round(y1), round(x2), round(y2), (60, 200, 200, 255))
        elif panel_id == "results-folds":
            for index, fold in enumerate(comparison.runs[0].folds):
                label = (
                    f"fold {fold.fold}  train {fold.train_start:%Y-%m-%d}  "
                    f"test {fold.test_end:%Y-%m-%d}"
                )
                self._text(
                    rl,
                    self._mono_font,
                    label,
                    x,
                    y + index * 26 * scale,
                    9,
                    scale,
                    (214, 220, 229, 255),
                )
        elif panel_id == "results-distributions":
            distributions = (
                comparison.runs[0].ic_distribution,
                comparison.runs[0].prediction_distribution,
            )
            panel_width = width / 2 - 8 * scale
            for group, distribution in enumerate(distributions):
                origin = x + group * (panel_width + 16 * scale)
                self._text(
                    rl,
                    self._inter_font,
                    distribution.label,
                    origin,
                    y,
                    9,
                    scale,
                    (60, 200, 200, 255) if group == 0 else (155, 124, 246, 255),
                )
                peak = max(distribution.counts)
                bar_width = panel_width / len(distribution.counts)
                for index, count in enumerate(distribution.counts):
                    bar_height = (height - 36 * scale) * count / peak
                    self._rect(
                        rl,
                        origin + index * bar_width,
                        y + height - bar_height,
                        max(1, bar_width - 2 * scale),
                        bar_height,
                        (60, 200, 200, 210) if group == 0 else (155, 124, 246, 210),
                    )
        elif panel_id == "results-config-diff":
            if comparison.differences:
                for index, difference in enumerate(comparison.differences):
                    values = " | ".join(value for _, value in difference.values)
                    self._text(
                        rl,
                        self._mono_font,
                        difference.path,
                        x,
                        y + index * 26 * scale,
                        9,
                        scale,
                        (126, 136, 150, 255),
                    )
                    self._text(
                        rl,
                        self._mono_font,
                        values,
                        x + width * 0.48,
                        y + index * 26 * scale,
                        9,
                        scale,
                        (155, 124, 246, 255),
                    )
            else:
                for index, item in enumerate(comparison.runs[0].configuration):
                    self._text(
                        rl,
                        self._mono_font,
                        item.path,
                        x,
                        y + index * 26 * scale,
                        9,
                        scale,
                        (126, 136, 150, 255),
                    )
                    self._text(
                        rl,
                        self._mono_font,
                        item.value,
                        x + width * 0.48,
                        y + index * 26 * scale,
                        9,
                        scale,
                        (155, 124, 246, 255),
                    )

    def _draw_phase7_panel(
        self,
        rl: object,
        view: ShellView,
        panel_id: str,
        x: float,
        y: float,
        width: float,
        height: float,
        scale: float,
    ) -> None:
        current = view.phase7_workspace
        if current is None:
            return
        if panel_id != "data-catalog" or current.state in {
            Phase7LoadState.LOADING,
            Phase7LoadState.FAILED,
            Phase7LoadState.CANCELLED,
            Phase7LoadState.BUSY,
        }:
            catalog_control = panel_id == "data-catalog"
            control_width = (134 if catalog_control else 142) * scale
            control_x = x + width - (138 if catalog_control else 146) * scale
            self._rect(
                rl,
                control_x,
                y,
                control_width,
                20 * scale,
                (23, 28, 34, 255),
            )
            self._outline(
                rl,
                control_x,
                y,
                control_width,
                20 * scale,
                (37, 43, 51, 255),
            )
            self._text(
                rl,
                self._inter_font,
                f"Scenario: {current.scenario.value}",
                control_x + 6 * scale,
                y + 5 * scale,
                10,
                scale,
                (214, 220, 229, 255),
            )
        if current.state is Phase7LoadState.LOADING and not current.stale:
            if panel_id.startswith("data-"):
                box_y = y + 24 * scale
                self._rect(rl, x, box_y, width, height - 24 * scale, (11, 13, 16, 255))
                self._outline(rl, x, box_y, width, height - 24 * scale, (37, 43, 51, 255))
                self._text(
                    rl,
                    self._inter_font,
                    "Loading deterministic catalog and import state...",
                    x + 16 * scale,
                    box_y + 16 * scale,
                    11,
                    scale,
                    (126, 136, 150, 255),
                )
            else:
                self._draw_phase7_failure(
                    rl,
                    "cancelled",
                    "context cancelled",
                    "Retry keeps the local draft and requests a fresh generation.",
                    x,
                    y + 28 * scale,
                    scale,
                )
            return
        if (
            current.state
            in {Phase7LoadState.FAILED, Phase7LoadState.CANCELLED, Phase7LoadState.BUSY}
            and not current.stale
        ):
            workspace_name = "Data" if panel_id.startswith("data-") else "Experiments"
            detail = (
                "Retry requests a fresh immutable generation."
                if workspace_name == "Data"
                else "Retry keeps the local draft and requests a fresh generation."
            )
            self._draw_phase7_failure(
                rl,
                "Error",
                f"Deterministic {workspace_name} request failed",
                detail,
                x,
                y + 28 * scale,
                scale,
            )
            return
        snapshot = current.snapshot
        if snapshot is None:
            self._text(
                rl,
                self._inter_font,
                "Preparing typed workflow state",
                x,
                y,
                11,
                scale,
                (126, 136, 150, 255),
            )
            return
        if current.state is Phase7LoadState.EMPTY:
            self._text(
                rl,
                self._inter_font,
                "No workflow records are available",
                x,
                y,
                11,
                scale,
                (126, 136, 150, 255),
            )
            return
        if panel_id == "data-catalog":
            self._draw_catalog(rl, view, x, y, width, height, scale)
        elif panel_id == "data-coverage":
            for index, item in enumerate(snapshot.catalog):
                row_y = y + index * 54 * scale
                self._text(
                    rl, self._inter_font, item.name, x, row_y, 11, scale, (214, 220, 229, 255)
                )
                self._text(
                    rl,
                    self._mono_font,
                    f"{item.coverage.start.date()}  ----------------  {item.coverage.end.date()}",
                    x,
                    row_y + 20 * scale,
                    10,
                    scale,
                    (60, 200, 200, 255),
                )
        elif panel_id == "data-import-queue":
            rows = snapshot.imports[-12:]
            if not rows:
                self._text(
                    rl,
                    self._inter_font,
                    "Queue empty - press I to append demo bars",
                    x,
                    y,
                    11,
                    scale,
                    (126, 136, 150, 255),
                )
            for index, item in enumerate(rows):
                self._text(
                    rl,
                    self._mono_font,
                    f"{item.request.command_id}  {item.state.value}  rows {item.imported_rows}",
                    x,
                    y + index * 24 * scale,
                    10,
                    scale,
                    (214, 220, 229, 255),
                )
        elif panel_id == "data-dataset":
            selected = next(
                (
                    item
                    for item in snapshot.catalog
                    if item.dataset_id == current.selected_dataset_id
                ),
                snapshot.catalog[0],
            )
            lines = (
                ("Dataset", selected.name),
                ("ID", selected.dataset_id),
                ("Revision", str(selected.revision)),
                ("Fingerprint", selected.content_fingerprint),
                ("Symbols", ", ".join(selected.symbols)),
                ("Interval", selected.interval),
                ("Adjustment", selected.adjustment.value),
            )
            self._draw_phase7_fields(rl, lines, x, y, scale)
        elif panel_id == "data-import-logs":
            diagnostics = tuple(
                diagnostic for item in snapshot.imports for diagnostic in item.diagnostics
            )
            if not diagnostics:
                self._text(
                    rl,
                    self._inter_font,
                    "No validation diagnostics",
                    x,
                    y,
                    11,
                    scale,
                    (76, 195, 138, 255),
                )
            for index, item in enumerate(diagnostics[-16:]):
                self._text(
                    rl,
                    self._mono_font,
                    f"{item.code}: {item.message}",
                    x,
                    y + index * 22 * scale,
                    10,
                    scale,
                    (239, 107, 115, 255),
                )
        else:
            self._draw_experiment_panel(
                rl, panel_id, current, x, y + 28 * scale, width, height, scale
            )
        if current.stale or current.state in {Phase7LoadState.DEGRADED, Phase7LoadState.RECOVERED}:
            message = "refreshing" if current.stale else current.state.value
            badge_y = y + 4 * scale
            badge_width = (len(message) * 6 + 14) * scale
            self._rect(rl, x + 4 * scale, badge_y, badge_width, 19 * scale, (17, 21, 26, 235))
            self._outline(rl, x + 4 * scale, badge_y, badge_width, 19 * scale, (60, 200, 200, 255))
            self._text(
                rl,
                self._inter_font,
                message,
                x + 10 * scale,
                badge_y + 4 * scale,
                9,
                scale,
                (60, 200, 200, 255),
            )

    def _draw_phase7_failure(
        self,
        rl: object,
        title: str,
        message: str,
        detail: str,
        x: float,
        y: float,
        scale: float,
    ) -> None:
        self._text(
            rl,
            self._mono_font,
            title,
            x,
            y - 3 * scale,
            11,
            scale,
            (214, 220, 229, 255),
        )
        self._text(
            rl,
            self._mono_font,
            message,
            x,
            y + 20 * scale,
            11,
            scale,
            (214, 220, 229, 255),
        )
        self._text(
            rl,
            self._mono_font,
            detail,
            x,
            y + 44 * scale,
            11,
            scale,
            (214, 220, 229, 255) if "local draft" in detail else (126, 136, 150, 255),
        )
        self._rect(rl, x, y + 68 * scale, 84 * scale, 24 * scale, (23, 28, 34, 255))
        self._outline(rl, x, y + 68 * scale, 84 * scale, 24 * scale, (37, 43, 51, 255))
        self._text(
            rl,
            self._inter_font,
            "Retry",
            x + 6 * scale,
            y + 74 * scale,
            10,
            scale,
            (214, 220, 229, 255),
        )

    def _draw_experiment_panel(
        self,
        rl: object,
        panel_id: str,
        current: Phase7WorkspaceState,
        x: float,
        y: float,
        width: float,
        height: float,
        scale: float,
    ) -> None:
        snapshot = current.snapshot
        draft = current.draft
        evaluation = current.evaluation
        if snapshot is None or draft is None:
            return
        if panel_id == "experiments-list":
            definitions = snapshot.experiments
            if not definitions:
                self._text(
                    rl,
                    self._inter_font,
                    "Daily equity baseline",
                    x,
                    y,
                    11,
                    scale,
                    (214, 220, 229, 255),
                )
                self._text(
                    rl,
                    self._mono_font,
                    draft.draft_id,
                    x,
                    y + 20 * scale,
                    9,
                    scale,
                    (126, 136, 150, 255),
                )
            for index, item in enumerate(definitions):
                label = (
                    "Daily equity baseline"
                    if item.experiment_id == "experiment-demo-complete"
                    else "Daily equity baseline"
                    if item.experiment_id == "experiment-demo-forest"
                    else "Submitted experiment"
                )
                self._text(
                    rl,
                    self._inter_font,
                    label,
                    x + 6 * scale,
                    y + (2 + index * 38) * scale,
                    10,
                    scale,
                    (214, 220, 229, 255),
                )
                self._text(
                    rl,
                    self._mono_font,
                    item.experiment_id,
                    x + 6 * scale,
                    y + (18 + index * 38) * scale,
                    9,
                    scale,
                    (126, 136, 150, 255),
                )
        elif panel_id == "experiments-configuration":
            self._draw_phase7_fields(
                rl,
                (
                    ("Dataset", draft.dataset_id),
                    ("Features", str(len(draft.features))),
                    ("Target", f"{draft.target_horizon} bars"),
                    ("Split", f"{draft.train_bars}/{draft.validation_bars}/{draft.test_bars}"),
                    ("Model", draft.model_kind),
                    ("Portfolio", f"${draft.initial_capital:,.0f}"),
                    ("Sweep", str(len(draft.sweep_values))),
                ),
                x,
                y,
                scale,
            )
        elif panel_id == "experiments-properties":
            self._draw_phase7_fields(
                rl,
                (
                    ("Draft revision", str(draft.revision)),
                    ("Estimators", str(draft.estimator_count)),
                    ("Max depth", str(draft.max_depth)),
                    ("Purge bars", str(draft.purge_bars)),
                    ("CPU limit", str(draft.cpu_limit)),
                    ("Autosaved", str(current.saved_revision)),
                ),
                x,
                y,
                scale,
            )
        elif panel_id == "experiments-inspector":
            feature = draft.features[0]
            self._draw_phase7_fields(
                rl,
                (
                    ("Feature", feature.name),
                    ("Version", feature.semantic_version),
                    ("Lookback", str(feature.lookback)),
                    ("Schema", feature.output_schema),
                    ("Fingerprint", feature.fingerprint),
                    ("Dataset fingerprint", draft.dataset_fingerprint),
                ),
                x,
                y,
                scale,
            )
        elif panel_id == "experiments-validation":
            diagnostics = () if evaluation is None else evaluation.diagnostics
            if not diagnostics:
                self._text(
                    rl,
                    self._inter_font,
                    "Valid - ready for immutable submission",
                    x,
                    y,
                    11,
                    scale,
                    (76, 195, 138, 255),
                )
            for index, item in enumerate(diagnostics):
                self._text(
                    rl,
                    self._mono_font,
                    f"{item.field}: {item.message}",
                    x,
                    y + index * 24 * scale,
                    10,
                    scale,
                    (239, 107, 115, 255),
                )
        elif panel_id == "experiments-resources" and evaluation is not None:
            estimate = evaluation.estimate
            self._draw_phase7_fields(
                rl,
                (
                    ("Rows", f"{estimate.rows:,}"),
                    ("Feature bytes", f"{estimate.feature_bytes:,}"),
                    ("Peak bytes", f"{estimate.peak_bytes:,}"),
                    ("CPU seconds", f"{estimate.cpu_seconds:.3f}"),
                ),
                x,
                y,
                scale,
            )

    def _draw_phase7_fields(
        self, rl: object, fields: tuple[tuple[str, str], ...], x: float, y: float, scale: float
    ) -> None:
        for index, (label, value) in enumerate(fields):
            row_y = y + index * 26 * scale
            self._text(rl, self._inter_font, label, x, row_y, 10, scale, (126, 136, 150, 255))
            self._text(
                rl, self._mono_font, value, x + 132 * scale, row_y, 10, scale, (214, 220, 229, 255)
            )

    def _draw_research_panel(
        self,
        rl: object,
        panel_type: str,
        group: ResearchGroupState,
        x: float,
        y: float,
        width: float,
        height: float,
        scale: float,
    ) -> None:
        if panel_type != "ohlcv":
            self._research_scenario_control(rl, group, x, y - 28 * scale, width, scale)
        if group.state is ResearchLoadState.LOADING and not group.stale:
            self._text(
                rl,
                self._inter_font,
                "Loading deterministic Research data...",
                x,
                y,
                11,
                scale,
                (126, 136, 150, 255),
            )
            return
        if (
            group.state
            in {
                ResearchLoadState.FAILED,
                ResearchLoadState.CANCELLED,
                ResearchLoadState.BUSY,
            }
            and not group.stale
        ):
            self._draw_research_failure(rl, group, x, y, scale)
            return
        snapshot = group.snapshot
        if snapshot is None:
            self._text(
                rl,
                self._inter_font,
                "Preparing linked Research request",
                x,
                y,
                11,
                scale,
                (126, 136, 150, 255),
            )
            return
        if group.state is ResearchLoadState.EMPTY:
            self._text(
                rl,
                self._inter_font,
                "No rows match this Research query",
                x,
                y,
                11,
                scale,
                (126, 136, 150, 255),
            )
            return
        if panel_type == "ohlcv":
            self._draw_research_chart(rl, group, x, y, width, height, scale)
        elif panel_type == "features":
            self._draw_research_features(rl, group, x, y, width, scale)
        elif panel_type == "series":
            self._draw_research_series(rl, group, x, y, scale)
        elif panel_type == "target":
            self._draw_research_target(rl, group, x, y, scale)
        elif panel_type == "distributions":
            self._draw_research_distributions(rl, group, x, y, width, height, scale)
        elif panel_type == "rows":
            self._draw_research_rows(rl, group, x, y, width, height, scale)
        if group.stale or group.state in {
            ResearchLoadState.DEGRADED,
            ResearchLoadState.RECOVERED,
        }:
            message = "refreshing - showing stale generation" if group.stale else group.state.value
            color = (216, 180, 90, 255) if group.stale else (60, 200, 200, 255)
            badge_width = (len(message) * 6 + 14) * scale
            self._rect(rl, x + 4 * scale, y + 4 * scale, badge_width, 19 * scale, (17, 21, 26, 235))
            self._outline(rl, x + 4 * scale, y + 4 * scale, badge_width, 19 * scale, color)
            self._text(
                rl, self._inter_font, message, x + 10 * scale, y + 8 * scale, 9, scale, color
            )

    def _research_scenario_control(
        self,
        rl: object,
        group: ResearchGroupState,
        x: float,
        y: float,
        width: float,
        scale: float,
    ) -> None:
        button_x = x + width - 130 * scale
        self._rect(rl, button_x, y, 130 * scale, 20 * scale, (23, 28, 34, 255))
        self._outline(rl, button_x, y, 130 * scale, 20 * scale, (37, 43, 51, 255))
        self._text(
            rl,
            self._inter_font,
            f"Scenario: {group.scenario.value}",
            button_x + 6 * scale,
            y + 4 * scale,
            9,
            scale,
            (126, 136, 150, 255),
        )

    def _draw_research_failure(
        self,
        rl: object,
        group: ResearchGroupState,
        x: float,
        y: float,
        scale: float,
    ) -> None:
        lines = (
            group.state.value,
            group.error or "Research request failed",
            "Click Retry to request a fresh generation.",
        )
        for index, line in enumerate(lines):
            self._text(
                rl,
                self._mono_font if index else self._inter_font,
                line,
                x,
                y + index * 24 * scale,
                10,
                scale,
                (214, 220, 229, 255),
            )
        self._rect(rl, x, y + 72 * scale, 82 * scale, 24 * scale, (23, 28, 34, 255))
        self._outline(rl, x, y + 72 * scale, 82 * scale, 24 * scale, (216, 180, 90, 255))
        self._text(
            rl,
            self._inter_font,
            "Retry",
            x + 20 * scale,
            y + 77 * scale,
            10,
            scale,
            (214, 220, 229, 255),
        )

    def _draw_research_chart(
        self,
        rl: object,
        group: ResearchGroupState,
        x: float,
        y: float,
        width: float,
        height: float,
        scale: float,
    ) -> None:
        toolbar_height = 24 * scale
        labels = (
            ("Candles", group.show_ohlcv),
            ("Feature", group.show_feature),
            ("Target", group.show_target),
        )
        button_x = x
        for label, active in labels:
            self._rect(
                rl,
                button_x,
                y,
                70 * scale,
                22 * scale,
                (23, 28, 34, 255) if active else (17, 21, 26, 255),
            )
            self._outline(rl, button_x, y, 70 * scale, 22 * scale, (37, 43, 51, 255))
            self._text(
                rl,
                self._inter_font,
                label,
                button_x + 6 * scale,
                y + 5 * scale,
                9,
                scale,
                (214, 220, 229, 255),
            )
            button_x += 73 * scale
        self._rect(rl, x + width - 60 * scale, y, 58 * scale, 22 * scale, (23, 28, 34, 255))
        self._outline(rl, x + width - 60 * scale, y, 58 * scale, 22 * scale, (37, 43, 51, 255))
        self._text(
            rl,
            self._inter_font,
            "Reset",
            x + width - 48 * scale,
            y + 5 * scale,
            9,
            scale,
            (214, 220, 229, 255),
        )
        plot_y = y + toolbar_height
        plot_height = max(0.0, height - toolbar_height)
        self._rect(rl, x, plot_y, width, plot_height, (11, 13, 16, 255))
        self._outline(rl, x, plot_y, width, plot_height, (37, 43, 51, 255))
        for index in range(1, 6):
            grid_y = plot_y + plot_height * index / 6
            self._line(rl, x, grid_y, x + width, grid_y, (37, 43, 51, 130))
        snapshot = group.snapshot
        if snapshot is None or snapshot.frame is None:
            return
        self._draw_research_frame(
            rl,
            snapshot.frame,
            group,
            x,
            plot_y,
            width,
            plot_height,
            scale,
        )

    def _draw_research_frame(
        self,
        rl: object,
        frame: object,
        group: ResearchGroupState,
        x: float,
        y: float,
        width: float,
        height: float,
        scale: float,
    ) -> None:
        viewport = frame.viewport

        def point(value_x: float, value_y: float) -> tuple[float, float]:
            return (
                x + (value_x - viewport.min_x) / viewport.width * width,
                y + (value_y - viewport.min_y) / viewport.height * height,
            )

        colors = {
            "primary": (60, 200, 200, 255),
            "secondary": (155, 124, 246, 255),
            "positive": (76, 195, 138, 255),
            "negative": (239, 107, 115, 255),
            "warning": (216, 180, 90, 255),
            "muted": (126, 136, 150, 255),
            "train": (60, 200, 200, 28),
            "validation": (155, 124, 246, 28),
            "test": (216, 180, 90, 28),
        }
        for layer in frame.layers:
            if (
                (layer.id == "ohlcv" and not group.show_ohlcv)
                or (layer.id == "feature" and not group.show_feature)
                or (layer.id == "target" and not group.show_target)
            ):
                continue
            for item in layer.rects:
                first = point(item.bounds.min_x, item.bounds.min_y)
                second = point(item.bounds.max_x, item.bounds.max_y)
                self._rect(
                    rl,
                    first[0],
                    first[1],
                    second[0] - first[0],
                    second[1] - first[1],
                    colors[item.style.value],
                )
            for segment in layer.segments:
                first = point(segment.start.x, segment.start.y)
                second = point(segment.end.x, segment.end.y)
                self._line(
                    rl, first[0], first[1], second[0], second[1], colors[segment.style.value]
                )
            for marker in layer.markers:
                center = point(marker.center.x, marker.center.y)
                rl.draw_circle_v(
                    rl.Vector2(center[0], center[1]),
                    2.2 * scale,
                    rl.Color(*colors[marker.style.value]),
                )

    def _draw_research_features(
        self,
        rl: object,
        group: ResearchGroupState,
        x: float,
        y: float,
        width: float,
        scale: float,
    ) -> None:
        if group.snapshot is None:
            return
        for index, series in enumerate(group.snapshot.features):
            row_y = y + index * 46 * scale
            active = series.descriptor.name == group.selected_feature
            if active:
                self._rect(rl, x, row_y, width, 42 * scale, (23, 28, 34, 255))
            self._outline(rl, x, row_y, width, 42 * scale, (37, 43, 51, 255))
            color = (60, 200, 200, 255) if active else (214, 220, 229, 255)
            self._text(
                rl,
                self._inter_font,
                f"{series.descriptor.name}  v{series.descriptor.version}",
                x + 8 * scale,
                row_y + 6 * scale,
                11,
                scale,
                color,
            )
            detail = (
                f"lookback {series.descriptor.lookback}  missing {series.missing}  "
                f"{series.descriptor.description}"
            )
            self._text(
                rl,
                self._inter_font,
                detail,
                x + 8 * scale,
                row_y + 23 * scale,
                9,
                scale,
                (126, 136, 150, 255),
            )

    def _draw_research_series(
        self, rl: object, group: ResearchGroupState, x: float, y: float, scale: float
    ) -> None:
        if group.snapshot is None:
            return
        series = next(
            (
                item
                for item in group.snapshot.features
                if item.descriptor.name == group.selected_feature
            ),
            None,
        )
        if series is None:
            return
        latest = next(
            (value.value for value in reversed(series.values) if value.value is not None),
            None,
        )
        lines = (
            f"Selected series: {series.descriptor.name}",
            f"Semantic version: {series.descriptor.version}",
            f"Implementation: {series.descriptor.fingerprint}",
            f"Declared lookback: {series.descriptor.lookback} bars",
            f"Range: {series.minimum:.6f} to {series.maximum:.6f}",
            f"Explicit missing values: {series.missing}",
            f"Latest visible value: {'missing' if latest is None else f'{latest:.6f}'}",
        )
        self._research_text_block(rl, lines, x, y, scale)

    def _draw_research_target(
        self, rl: object, group: ResearchGroupState, x: float, y: float, scale: float
    ) -> None:
        if group.snapshot is None:
            return
        target = group.snapshot.target
        lines = (
            "Forward open-to-open target",
            f"Horizon: {target.spec.horizon_bars} bars",
            f"Return kind: {'log' if target.spec.log_return else 'simple'}",
            "Feature cutoff: bar t close",
            "Reference execution: bar t+1 open",
            f"Valid future targets: {target.valid_rows}",
            f"Excluded trailing rows: {target.excluded_rows}",
            "Viewport changes do not alter split membership.",
        )
        self._research_text_block(rl, lines, x, y, scale)

    def _research_text_block(
        self,
        rl: object,
        lines: tuple[str, ...],
        x: float,
        y: float,
        scale: float,
    ) -> None:
        for index, line in enumerate(lines):
            self._text(
                rl,
                self._mono_font,
                line,
                x,
                y + index * 22 * scale,
                10,
                scale,
                (214, 220, 229, 255),
            )

    def _draw_research_distributions(
        self,
        rl: object,
        group: ResearchGroupState,
        x: float,
        y: float,
        width: float,
        height: float,
        scale: float,
    ) -> None:
        if group.snapshot is None:
            return
        selected = next(
            (item for item in group.snapshot.distributions if item.name == group.selected_feature),
            None,
        )
        target = next(
            (item for item in group.snapshot.distributions if item.name == "target"), None
        )
        gap = 8 * scale
        histogram_width = (width - gap) / 2
        for offset, distribution in ((0.0, selected), (histogram_width + gap, target)):
            if distribution is None or not distribution.bins:
                continue
            maximum = max(1, max(item.count for item in distribution.bins))
            bar_width = histogram_width / len(distribution.bins)
            for index, item in enumerate(distribution.bins):
                bar_height = (height - 30 * scale) * item.count / maximum
                self._rect(
                    rl,
                    x + offset + index * bar_width + 1,
                    y + height - bar_height - 18 * scale,
                    max(1, bar_width - 2),
                    bar_height,
                    (60, 200, 200, 255),
                )
            self._text(
                rl,
                self._inter_font,
                f"{distribution.name} distribution",
                x + offset + 6 * scale,
                y + height - 14 * scale,
                9,
                scale,
                (126, 136, 150, 255),
            )

    def _draw_research_rows(
        self,
        rl: object,
        group: ResearchGroupState,
        x: float,
        y: float,
        width: float,
        height: float,
        scale: float,
    ) -> None:
        if group.snapshot is None:
            return
        page = group.snapshot.rows
        self._text(
            rl,
            self._inter_font,
            f"{page.total_rows} leakage-safe rows",
            x + 4 * scale,
            y + 5 * scale,
            9,
            scale,
            (126, 136, 150, 255),
        )
        controls = (("Filter", 86), ("Sort", 78), ("Prev", 60), ("Next", 56))
        control_x = x + width
        for label, logical_width in reversed(controls):
            button_width = logical_width * scale
            control_x -= button_width
            self._rect(
                rl,
                control_x + 2 * scale,
                y,
                button_width - 4 * scale,
                22 * scale,
                (23, 28, 34, 255),
            )
            self._outline(
                rl,
                control_x + 2 * scale,
                y,
                button_width - 4 * scale,
                22 * scale,
                (37, 43, 51, 255),
            )
            self._text(
                rl,
                self._inter_font,
                label,
                control_x + 12 * scale,
                y + 5 * scale,
                9,
                scale,
                (214, 220, 229, 255),
            )
        table_y = y + 24 * scale
        columns = page.dataset.columns[:6]
        column_width = width / len(columns)
        self._rect(rl, x, table_y, width, 24 * scale, (23, 28, 34, 255))
        for index, column in enumerate(columns):
            self._text(
                rl,
                self._inter_font,
                column.title,
                x + index * column_width + 4 * scale,
                table_y + 6 * scale,
                9,
                scale,
                (126, 136, 150, 255),
            )
        maximum_rows = max(0, int((height - 48 * scale) // (24 * scale)))
        for row_index, row in enumerate(page.dataset.rows[:maximum_rows]):
            row_y = table_y + (row_index + 1) * 24 * scale
            if row.id in group.selected_rows:
                self._rect(rl, x, row_y, width, 24 * scale, (23, 28, 34, 255))
                self._rect(rl, x, row_y, 2 * scale, 24 * scale, (60, 200, 200, 255))
            for column_index, cell in enumerate(row.cells[: len(columns)]):
                value = "--" if cell.value is None else str(cell.value)
                if isinstance(cell.value, float):
                    value = f"{cell.value:.4f}"
                elif isinstance(cell.value, datetime):
                    value = cell.value.strftime("%Y-%m-%d")
                self._text(
                    rl,
                    self._mono_font,
                    value[:22],
                    x + column_index * column_width + 4 * scale,
                    row_y + 6 * scale,
                    8,
                    scale,
                    (214, 220, 229, 255),
                )
            self._line(rl, x, row_y + 23 * scale, x + width, row_y + 23 * scale, (37, 43, 51, 160))

    def _draw_catalog(
        self,
        rl: object,
        view: ShellView,
        x: float,
        y: float,
        width: float,
        height: float,
        scale: float,
    ) -> None:
        button_x = x + width - 138 * scale
        scenario = (
            view.phase7_workspace.scenario.value if view.phase7_workspace is not None else "normal"
        )
        self._rect(rl, button_x, y, 134 * scale, 20 * scale, (23, 28, 34, 255))
        self._outline(rl, button_x, y, 134 * scale, 20 * scale, (37, 43, 51, 255))
        self._text(
            rl,
            self._inter_font,
            f"Scenario: {scenario}",
            button_x + 6 * scale,
            y + 5 * scale,
            10,
            scale,
            (214, 220, 229, 255),
        )
        table_y = y + 24 * scale
        self._rect(rl, x, table_y, width, 24 * scale, (23, 28, 34, 255))
        columns = (("Dataset", 0.0), ("Status", 0.42), ("Rows", 0.62), ("Revision", 0.79))
        for label, fraction in columns:
            self._text(
                rl,
                self._inter_font,
                label,
                x + width * fraction + 6 * scale,
                table_y + 6 * scale,
                10,
                scale,
                (126, 136, 150, 255),
            )
        for index, row in enumerate(view.datasets):
            row_y = table_y + 24 * scale + index * 30 * scale
            if row.selected:
                self._rect(rl, x, row_y, width, 30 * scale, (23, 28, 34, 255))
                self._rect(rl, x, row_y, 2 * scale, 30 * scale, (60, 200, 200, 255))
            self._text(
                rl,
                self._inter_font,
                row.name,
                x + 6 * scale,
                row_y + 8 * scale,
                10,
                scale,
                (214, 220, 229, 255),
            )
            status_color = (76, 195, 138, 255) if row.status == "ready" else (216, 180, 90, 255)
            self._text(
                rl,
                self._mono_font,
                row.status,
                x + width * 0.42 + 6 * scale,
                row_y + 8 * scale,
                9,
                scale,
                status_color,
            )
            self._text(
                rl,
                self._mono_font,
                row.rows,
                x + width * 0.62 + 6 * scale,
                row_y + 8 * scale,
                9,
                scale,
                (214, 220, 229, 255),
            )
            self._text(
                rl,
                self._mono_font,
                row.revision,
                x + width * 0.79 + 6 * scale,
                row_y + 8 * scale,
                9,
                scale,
                (155, 124, 246, 255),
            )
            self._line(rl, x, row_y + 29 * scale, x + width, row_y + 29 * scale, (37, 43, 51, 255))

    def _draw_status_bar(self, view: ShellView, rl: object) -> None:
        scale = view.scale
        y = view.viewport.height - 26 * scale
        self._rect(rl, 0, y, view.viewport.width, 26 * scale, (17, 21, 26, 255))
        self._line(rl, 0, y, view.viewport.width, y, (37, 43, 51, 255))
        parts = (
            f"health {view.connection}",
            f"UI {view.ui_scale_percent}%",
            f"selection {view.dataset_id} {view.symbols}",
            f"cache {view.cache}",
            f"CPU {view.cpu_slots} slots",
            view.worker_detail,
            f"FPS {view.fps}",
            "Ctrl+K command  Ctrl+, settings",
        )
        x = 10 * scale
        for part in parts:
            clipped = part if len(part) <= 34 else part[:31] + "..."
            self._text(
                rl, self._inter_font, clipped, x, y + 5 * scale, 10, scale, (126, 136, 150, 255)
            )
            x += (len(part) * 6 + 26) * scale
            if x > view.viewport.width - 120 * scale:
                break

    def _draw_modal(self, view: ShellView, rl: object) -> None:
        if not (view.command_palette_open or view.settings_open or view.critical_error):
            return
        self._rect(rl, 0, 0, view.viewport.width, view.viewport.height, (0, 0, 0, 168))
        if view.critical_error is not None:
            self._draw_critical_error(view, rl)
        elif view.settings_open:
            self._draw_settings_modal(view, rl)
        else:
            self._draw_command_palette(view, rl)

    def _draw_toasts(self, view: ShellView, rl: object) -> None:
        if not view.toasts:
            return
        scale = view.scale
        width = 360 * scale
        x = view.viewport.width - width - 12 * scale
        y = (36 + 40) * scale + 12 * scale
        for message in view.toasts[-3:]:
            self._rect(rl, x, y, width, 42 * scale, (23, 28, 34, 242))
            self._outline(rl, x, y, width, 42 * scale, (60, 200, 200, 255))
            self._rect(rl, x, y, 3 * scale, 42 * scale, (60, 200, 200, 255))
            clipped = message if len(message) <= 42 else message[:39] + "..."
            self._text(
                rl,
                self._inter_font,
                clipped,
                x + 12 * scale,
                y + 13 * scale,
                11,
                scale,
                (214, 220, 229, 255),
            )
            y += 48 * scale

    def _draw_critical_error(self, view: ShellView, rl: object) -> None:
        if view.critical_error is None:
            return
        scale = view.scale
        x, y, width, height = self._modal_bounds(view, 520, 220)
        self._rect(rl, x, y, width, height, (17, 21, 26, 255))
        self._outline(rl, x, y, width, height, (239, 107, 115, 255))
        self._text(
            rl,
            self._inter_font,
            "Critical Error",
            x + 18 * scale,
            y + 18 * scale,
            16,
            scale,
            (239, 107, 115, 255),
        )
        message = (
            view.critical_error
            if len(view.critical_error) <= 58
            else view.critical_error[:55] + "..."
        )
        self._text(
            rl,
            self._inter_font,
            message,
            x + 18 * scale,
            y + 52 * scale,
            12,
            scale,
            (214, 220, 229, 255),
        )
        self._text(
            rl,
            self._inter_font,
            "Coordinator actions are disabled until the error clears.",
            x + 18 * scale,
            y + 78 * scale,
            11,
            scale,
            (126, 136, 150, 255),
        )
        for button_x, label in ((x + 18 * scale, "Reconnect"), (x + 156 * scale, "Restart")):
            self._rect(
                rl, button_x, y + height - 42 * scale, 130 * scale, 24 * scale, (11, 13, 16, 255)
            )
            self._outline(
                rl, button_x, y + height - 42 * scale, 130 * scale, 24 * scale, (37, 43, 51, 255)
            )
            self._text(
                rl,
                self._inter_font,
                label,
                button_x + 12 * scale,
                y + height - 36 * scale,
                10,
                scale,
                (126, 136, 150, 255),
            )

    def _draw_settings_modal(self, view: ShellView, rl: object) -> None:
        scale = view.scale
        x, y, width, height = self._modal_bounds(view, 620, 350)
        self._rect(rl, x, y, width, height, (17, 21, 26, 255))
        self._outline(rl, x, y, width, height, (60, 200, 200, 255))
        padding = 20 * scale
        self._text(
            rl,
            self._inter_font,
            "Settings",
            x + padding,
            y + padding,
            18,
            scale,
            (214, 220, 229, 255),
        )
        self._nav_button(
            rl,
            x + width - 88 * scale,
            y + 14 * scale,
            68 * scale,
            32 * scale,
            "Close",
            False,
            scale,
        )
        content_y = y + 72 * scale
        self._text(
            rl,
            self._inter_font,
            "Appearance",
            x + padding,
            content_y,
            14,
            scale,
            (60, 200, 200, 255),
        )
        content_y += 32 * scale
        self._text(
            rl,
            self._inter_font,
            "Interface size",
            x + padding,
            content_y,
            13,
            scale,
            (214, 220, 229, 255),
        )
        content_y += 24 * scale
        self._text(
            rl,
            self._inter_font,
            "Scales text, controls, docking, spacing, and pointer targets together.",
            x + padding,
            content_y,
            12,
            scale,
            (126, 136, 150, 255),
        )
        content_y += 38 * scale
        gap = 8 * scale
        button_width = (width - 2 * padding - 4 * gap) / 5
        for index, preset in enumerate((100, 125, 150, 175, 200)):
            self._nav_button(
                rl,
                x + padding + index * (button_width + gap),
                content_y,
                button_width,
                38 * scale,
                f"{preset}%",
                preset == view.ui_scale_percent,
                scale,
            )
        content_y += 62 * scale
        self._text(
            rl,
            self._inter_font,
            f"Windows scale 100%   Effective UI scale {view.scale * 100:.0f}%",
            x + padding,
            content_y,
            12,
            scale,
            (126, 136, 150, 255),
        )
        self._text(
            rl,
            self._mono_font,
            "Ctrl+Plus / Ctrl+Minus adjust size   Ctrl+0 restores 125%",
            x + padding,
            content_y + 30 * scale,
            12,
            scale,
            (126, 136, 150, 255),
        )

    def _draw_command_palette(self, view: ShellView, rl: object) -> None:
        scale = view.scale
        x, y, width, height = self._modal_bounds(view, 620, 420)
        self._rect(rl, x, y, width, height, (17, 21, 26, 255))
        self._outline(rl, x, y, width, height, (155, 124, 246, 255))
        self._text(
            rl,
            self._inter_font,
            "Command Palette",
            x + 16 * scale,
            y + 14 * scale,
            16,
            scale,
            (214, 220, 229, 255),
        )
        self._text(
            rl,
            self._inter_font,
            "Navigate workspaces and inspect available shell commands",
            x + 16 * scale,
            y + 38 * scale,
            11,
            scale,
            (126, 136, 150, 255),
        )
        input_y = y + 62 * scale
        self._rect(rl, x + 16 * scale, input_y, width - 32 * scale, 28 * scale, (11, 13, 16, 255))
        self._outline(
            rl, x + 16 * scale, input_y, width - 32 * scale, 28 * scale, (37, 43, 51, 255)
        )
        self._text(
            rl,
            self._mono_font,
            "> workspace",
            x + 25 * scale,
            input_y + 7 * scale,
            11,
            scale,
            (126, 136, 150, 255),
        )
        active_title = next(tab.action.workspace.value.title() for tab in view.tabs if tab.selected)
        commands = [
            (
                f"Go to {tab.action.workspace.value.title()}",
                "Current workspace" if tab.selected else "Switch workspace",
            )
            for tab in view.tabs
        ]
        commands.append((f"Reset {active_title} layout", "Restore default docking"))
        row_y = y + 102 * scale
        for index, (title, detail) in enumerate(commands):
            if row_y > y + height - 44 * scale:
                break
            if index == self._command_index:
                self._rect(
                    rl, x + 10 * scale, row_y, width - 20 * scale, 38 * scale, (23, 28, 34, 255)
                )
                self._rect(rl, x + 10 * scale, row_y, 3 * scale, 38 * scale, (155, 124, 246, 255))
            self._text(
                rl,
                self._inter_font,
                title,
                x + 22 * scale,
                row_y + 7 * scale,
                12,
                scale,
                (214, 220, 229, 255),
            )
            self._text(
                rl,
                self._inter_font,
                detail,
                x + 230 * scale,
                row_y + 7 * scale,
                11,
                scale,
                (126, 136, 150, 255),
            )
            row_y += 40 * scale

    @staticmethod
    def _modal_bounds(
        view: ShellView, logical_width: float, logical_height: float
    ) -> tuple[float, float, float, float]:
        scale = view.scale
        margin = 16 * scale
        width = min(logical_width * scale, max(0, view.viewport.width - 2 * margin))
        height = min(logical_height * scale, max(0, view.viewport.height - 2 * margin))
        return (view.viewport.width - width) / 2, (view.viewport.height - height) / 2, width, height

    def _dock_header_buttons(
        self, rl: object, x: float, y: float, width: float, scale: float
    ) -> None:
        close_x = x + width - 23 * scale
        max_x = close_x - 22 * scale
        link_x = max_x - 98 * scale
        self._rect(rl, link_x, y + 3 * scale, 96 * scale, 18 * scale, (17, 21, 26, 255))
        self._outline(rl, link_x, y + 3 * scale, 96 * scale, 18 * scale, (37, 43, 51, 255))
        self._rect(
            rl, link_x + 5 * scale, y + 9.5 * scale, 5 * scale, 5 * scale, (60, 200, 200, 255)
        )
        self._text(
            rl,
            self._inter_font,
            "Default",
            link_x + 14 * scale,
            y + 7 * scale,
            9,
            scale,
            (214, 220, 229, 255),
        )
        for bx, label in ((max_x, "[]"), (close_x, "x")):
            self._rect(rl, bx, y + 3 * scale, 20 * scale, 18 * scale, (23, 28, 34, 255))
            self._outline(rl, bx, y + 3 * scale, 20 * scale, 18 * scale, (37, 43, 51, 255))
            self._text(
                rl,
                self._mono_font,
                label,
                bx + 5 * scale,
                y + 6 * scale,
                9,
                scale,
                (214, 220, 229, 255),
            )

    def _context_item(
        self,
        rl: object,
        x: float,
        y: float,
        label: str,
        value: str,
        color: tuple[int, int, int, int],
        scale: float,
    ) -> float:
        top = y + 6 * scale
        self._text(rl, self._inter_font, label, x, top + scale, 11, scale, (126, 136, 150, 255))
        value_x = x + 72 * scale
        self._text(rl, self._inter_font, value, value_x, top, 12, scale, color)
        return value_x + (len(value) * 7 + 32) * scale

    def _small_line(
        self, rl: object, x: float, y: float, label: str, value: str, scale: float
    ) -> None:
        self._text(rl, self._inter_font, label, x, y, 10, scale, (126, 136, 150, 255))
        self._text(
            rl, self._inter_font, value[:20], x + 58 * scale, y, 10, scale, (214, 220, 229, 255)
        )

    def _nav_button(
        self,
        rl: object,
        x: float,
        y: float,
        width: float,
        height: float,
        label: str,
        active: bool,
        scale: float,
    ) -> None:
        self._rect(rl, x, y, width, height, (23, 28, 34, 255) if active else (17, 21, 26, 255))
        self._text(
            rl,
            self._inter_font,
            label,
            x + 10 * scale,
            y + 5 * scale,
            11,
            scale,
            (214, 220, 229, 255) if active else (126, 136, 150, 255),
        )

    @staticmethod
    def _rect(
        rl: object,
        x: float,
        y: float,
        width: float,
        height: float,
        color: tuple[int, int, int, int],
    ) -> None:
        rl.draw_rectangle_rec(rl.Rectangle(x, y, width, height), rl.Color(*color))

    @staticmethod
    def _outline(
        rl: object,
        x: float,
        y: float,
        width: float,
        height: float,
        color: tuple[int, int, int, int],
    ) -> None:
        rl.draw_rectangle_lines_ex(rl.Rectangle(x, y, width, height), 1, rl.Color(*color))

    @staticmethod
    def _line(
        rl: object, x1: float, y1: float, x2: float, y2: float, color: tuple[int, int, int, int]
    ) -> None:
        rl.draw_line_ex(rl.Vector2(x1, y1), rl.Vector2(x2, y2), 1, rl.Color(*color))

    @staticmethod
    def _text(
        rl: object,
        font: object,
        value: str,
        x: float,
        y: float,
        size: float,
        scale: float,
        color: tuple[int, int, int, int],
    ) -> None:
        readable = 12 if size <= 10 else 13 if size <= 12 else 14 if size <= 14 else max(size, 18)
        rl.draw_text_ex(font, value, rl.Vector2(x, y), readable * scale, 0, rl.Color(*color))

    def capture_rgba(self) -> CapturedFrame:
        """Copy screen pixels and release native capture values on the owner thread."""
        self._assert_owner()
        import pyray as rl

        if not self._window_open:
            raise RuntimeError("UI adapter is not initialized")
        image = rl.load_image_from_screen()
        colors = None
        try:
            width, height = int(image.width), int(image.height)
            colors = rl.load_image_colors(image)
            rgba = bytes(
                channel
                for index in range(width * height)
                for channel in (
                    int(colors[index].r),
                    int(colors[index].g),
                    int(colors[index].b),
                    int(colors[index].a),
                )
            )
            return CapturedFrame(width, height, rgba)
        finally:
            if colors is not None:
                rl.unload_image_colors(colors)
            rl.unload_image(image)

    def close(self) -> None:
        """Release owned native resources exactly once on the owner thread."""
        self._assert_owner()
        if not self._window_open:
            return
        import pyray as rl

        first_error: BaseException | None = None
        if self._atlas is not None:
            try:
                rl.unload_texture(self._atlas)
            except BaseException as exc:
                first_error = exc
            self._atlas = None
        if self._mono_font is not None:
            try:
                rl.unload_font(self._mono_font)
            except BaseException as exc:
                if first_error is None:
                    first_error = exc
            self._mono_font = None
        if self._inter_font is not None:
            try:
                rl.unload_font(self._inter_font)
            except BaseException as exc:
                if first_error is None:
                    first_error = exc
            self._inter_font = None
        try:
            rl.close_window()
        except BaseException as exc:
            if first_error is None:
                first_error = exc
        finally:
            self._window_open = False
        if first_error is not None:
            raise first_error


__all__ = [
    "RaylibUIAdapter",
    "UiThreadViolationError",
]
