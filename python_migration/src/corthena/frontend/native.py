"""Typed adapter containing all Phase 1 Raylib and Raygui values."""

# pyright: reportAttributeAccessIssue=false, reportUnknownArgumentType=false
# pyright: reportUnknownMemberType=false, reportUnknownVariableType=false

from __future__ import annotations

import threading
from dataclasses import dataclass
from typing import TYPE_CHECKING, Protocol

if TYPE_CHECKING:
    from pyray import Font, Texture, Vector2

    from corthena.frontend.phase5b import VisualizationView

from corthena.frontend.assets import FrontendAssets
from corthena.frontend.controls import (
    ControlEventKind,
    ControlState,
    FrameInput,
    PointerBehavior,
    Widget,
    WidgetId,
    route,
)
from corthena.frontend.docking import DockPosition
from corthena.frontend.docking import Rect as ControlRect
from corthena.frontend.phase5b import (
    ChartAction,
    InteractionKind,
)
from corthena.frontend.phase5b import (
    Point as VisualizationPoint,
)
from corthena.frontend.phase5b import (
    Rect as VisualizationRect,
)
from corthena.frontend.phase5b import (
    Transform as VisualizationTransform,
)
from corthena.frontend.shell import (
    DockDropTargetView,
    ShellRegion,
    ShellView,
    action_at,
    project_dock_drop_targets,
)
from corthena.frontend.state import (
    DockPanel,
    ResizeDockSplit,
    SetCommandPalette,
    SetSettingsOpen,
    SetUIScale,
    UIAction,
)


class UiThreadViolationError(RuntimeError):
    """Raised before a native call when the UI OS thread does not own it."""


class NativeFrontend(Protocol):
    """Native-free lifecycle surface consumed by frontend startup."""

    @property
    def owner_thread_id(self) -> int: ...

    def initialize(self, assets: FrontendAssets, *, hidden: bool) -> None: ...

    def should_close(self) -> bool: ...

    def frame_metrics(self) -> FrameMetrics: ...

    def render_frame(self) -> None: ...

    def render_shell(self, view: ShellView) -> tuple[UIAction, ...]: ...

    def render_visualization(self, view: VisualizationView) -> tuple[ChartAction, ...]: ...

    def capture_rgba(self) -> CapturedFrame: ...

    def close(self) -> None: ...


@dataclass(frozen=True, slots=True)
class WindowSize:
    """Native-independent initial window dimensions."""

    width: int = 1280
    height: int = 720


@dataclass(frozen=True, slots=True)
class CapturedFrame:
    """Immutable native-free pixels captured on the UI owner thread."""

    width: int
    height: int
    rgba: bytes

    def __post_init__(self) -> None:
        if self.width < 1 or self.height < 1 or len(self.rgba) != self.width * self.height * 4:
            raise ValueError("captured RGBA dimensions and byte length do not agree")


@dataclass(frozen=True, slots=True)
class FrameMetrics:
    """Native-free live viewport inputs sampled once for a render frame."""

    width: int
    height: int
    dpi_scale: float
    fps: int

    def __post_init__(self) -> None:
        if self.width < 1 or self.height < 1:
            raise ValueError("frame dimensions must be positive")
        if self.dpi_scale <= 0:
            raise ValueError("dpi_scale must be positive")
        if self.fps < 0:
            raise ValueError("fps must be non-negative")


class RaylibFrontendAdapter:
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

    @property
    def owner_thread_id(self) -> int:
        """Return the locked Windows OS thread identifier."""
        return self._owner_thread_id

    def _assert_owner(self) -> None:
        if threading.get_native_id() != self._owner_thread_id:
            raise UiThreadViolationError("Raylib call attempted outside the locked UI OS thread")

    def initialize(self, assets: FrontendAssets, *, hidden: bool) -> None:
        """Initialize a window and load validated assets on the owner thread."""
        self._assert_owner()
        import pyray as rl

        if self._window_open:
            raise RuntimeError("frontend adapter is already initialized")
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
            raise RuntimeError("a bundled frontend asset failed to load")
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
            raise RuntimeError("frontend adapter is not initialized")
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
            raise RuntimeError("frontend adapter is not initialized")
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
            raise RuntimeError("frontend adapter is not initialized")
        if self._inter_font is None or self._mono_font is None:
            raise RuntimeError("frontend fonts are not initialized")
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
            raise RuntimeError("frontend visualization resources are not initialized")
        import pyray as rl

        from corthena.frontend.native_visualization import draw_visualization

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
        component_y = y + 10 * scale
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
            self._rect(
                rl, 10 * scale, component_y, left_width - 20 * scale, 24 * scale, (23, 28, 34, 255)
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
        global_y = bounds.y + bounds.height - 104 * scale
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
        self._small_line(rl, 10 * scale, global_y + 24 * scale, "Dataset", view.dataset_id, scale)
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
            if selected_panel.title == "Catalog":
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
        self._rect(rl, button_x, y, 134 * scale, 20 * scale, (23, 28, 34, 255))
        self._outline(rl, button_x, y, 134 * scale, 20 * scale, (37, 43, 51, 255))
        self._text(
            rl,
            self._inter_font,
            "Scenario: normal",
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
            raise RuntimeError("frontend adapter is not initialized")
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
    "CapturedFrame",
    "FrameMetrics",
    "NativeFrontend",
    "RaylibFrontendAdapter",
    "UiThreadViolationError",
    "WindowSize",
]
