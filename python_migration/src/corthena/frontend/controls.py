"""Deterministic, clipping-aware reusable control input routing."""

from __future__ import annotations

from dataclasses import dataclass
from enum import StrEnum

from corthena.frontend.docking import Rect


@dataclass(frozen=True, slots=True)
class WidgetId:
    encoded: str = ""

    @classmethod
    def root(cls, segment: str) -> WidgetId:
        return cls().child(segment)

    def child(self, segment: str) -> WidgetId:
        return WidgetId(f"{self.encoded}s{len(segment)}:{segment}")

    def child_index(self, index: int) -> WidgetId:
        if index < 0:
            raise ValueError("widget index must be non-negative")
        return WidgetId(f"{self.encoded}i{index};")

    def descends_from(self, ancestor: WidgetId) -> bool:
        return (
            bool(ancestor.encoded)
            and len(self.encoded) > len(ancestor.encoded)
            and self.encoded.startswith(ancestor.encoded)
        )


class PointerBehavior(StrEnum):
    CLICK = "click"
    DRAG = "drag"
    PASSIVE = "passive"


@dataclass(frozen=True, slots=True)
class Widget:
    id: WidgetId
    bounds: Rect
    clip: Rect
    behavior: PointerBehavior = PointerBehavior.CLICK
    focusable: bool = True
    enabled: bool = True

    def __post_init__(self) -> None:
        if not self.id.encoded:
            raise ValueError("widget id must be valid")

    def hit_test(self, x: float, y: float) -> bool:
        return self.enabled and self.bounds.contains(x, y) and self.clip.contains(x, y)


@dataclass(frozen=True, slots=True)
class FrameInput:
    x: float = 0
    y: float = 0
    pressed: bool = False
    down: bool = False
    released: bool = False
    focus_next: bool = False
    focus_previous: bool = False
    activate: bool = False
    cancel: bool = False

    def __post_init__(self) -> None:
        if self.pressed and self.released:
            raise ValueError("pointer cannot press and release in one frame")


@dataclass(frozen=True, slots=True)
class ControlState:
    hot: WidgetId = WidgetId()
    active: WidgetId = WidgetId()
    focused: WidgetId = WidgetId()
    captured: WidgetId = WidgetId()


class ControlEventKind(StrEnum):
    FOCUS = "focus"
    BLUR = "blur"
    PRESS = "press"
    DRAG = "drag"
    ACTIVATE = "activate"
    RELEASE = "release"
    CANCEL = "cancel"


@dataclass(frozen=True, slots=True)
class ControlEvent:
    kind: ControlEventKind
    widget_id: WidgetId


@dataclass(frozen=True, slots=True)
class RouteResult:
    state: ControlState
    events: tuple[ControlEvent, ...]


def route(previous: ControlState, frame: FrameInput, widgets: tuple[Widget, ...]) -> RouteResult:
    ids = tuple(widget.id for widget in widgets)
    if len(ids) != len(frozenset(ids)):
        raise ValueError("widget ids must be unique")
    by_id = {widget.id: widget for widget in widgets}
    events: list[ControlEvent] = []
    state = previous
    for owner in (state.active, state.focused, state.captured):
        if owner.encoded and (owner not in by_id or not by_id[owner].enabled):
            events.append(ControlEvent(ControlEventKind.CANCEL, owner))
            state = ControlState()
            break
    hot = next(
        (widget.id for widget in reversed(widgets) if widget.hit_test(frame.x, frame.y)), WidgetId()
    )
    state = ControlState(hot, state.active, state.focused, state.captured)
    if state.captured.encoded and not frame.down and not frame.released and not frame.cancel:
        events.append(ControlEvent(ControlEventKind.CANCEL, state.captured))
        state = ControlState(hot=hot, focused=state.focused)
    focusable = tuple(widget.id for widget in widgets if widget.enabled and widget.focusable)
    if frame.cancel:
        owner = state.captured if state.captured.encoded else state.active
        if owner.encoded:
            events.append(ControlEvent(ControlEventKind.CANCEL, owner))
        state = ControlState(hot=hot, focused=state.focused)
    if (frame.focus_next or frame.focus_previous) and focusable:
        step = -1 if frame.focus_previous else 1
        old = state.focused
        index = focusable.index(old) if old in focusable else (-1 if step > 0 else 0)
        focused = focusable[(index + step) % len(focusable)]
        if old.encoded and old != focused:
            events.append(ControlEvent(ControlEventKind.BLUR, old))
        if old != focused:
            events.append(ControlEvent(ControlEventKind.FOCUS, focused))
        state = ControlState(hot, state.active, focused, state.captured)
    if frame.pressed and hot.encoded:
        widget = by_id[hot]
        captured = hot if widget.behavior is PointerBehavior.DRAG else WidgetId()
        events.append(ControlEvent(ControlEventKind.PRESS, hot))
        state = ControlState(hot, hot, hot if widget.focusable else state.focused, captured)
    if frame.down and state.captured.encoded:
        events.append(ControlEvent(ControlEventKind.DRAG, state.captured))
    if frame.released and state.active.encoded:
        events.append(ControlEvent(ControlEventKind.RELEASE, state.active))
        if state.active == hot or state.captured.encoded:
            events.append(ControlEvent(ControlEventKind.ACTIVATE, state.active))
        state = ControlState(hot, focused=state.focused)
    if frame.activate and state.focused.encoded:
        events.append(ControlEvent(ControlEventKind.ACTIVATE, state.focused))
    return RouteResult(state, tuple(events))


def replay(
    initial: ControlState, frames: tuple[tuple[FrameInput, tuple[Widget, ...]], ...]
) -> RouteResult:
    state = initial
    events: list[ControlEvent] = []
    for frame, widgets in frames:
        result = route(state, frame, widgets)
        state = result.state
        events.extend(result.events)
    return RouteResult(state, tuple(events))


__all__ = [
    "ControlEvent",
    "ControlEventKind",
    "ControlState",
    "FrameInput",
    "PointerBehavior",
    "RouteResult",
    "Widget",
    "WidgetId",
    "replay",
    "route",
]
