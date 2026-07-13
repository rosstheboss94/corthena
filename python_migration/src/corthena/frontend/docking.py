"""Immutable dock trees, pure mutations, and deterministic geometry."""

from __future__ import annotations

import math
from collections.abc import Callable
from dataclasses import dataclass, replace
from enum import StrEnum
from typing import assert_never

MAX_DOCK_DEPTH = 64
MIN_SPLIT_RATIO = 0.05
MAX_SPLIT_RATIO = 0.95


class Orientation(StrEnum):
    HORIZONTAL = "horizontal"
    VERTICAL = "vertical"


class DockPosition(StrEnum):
    CENTER = "center"
    LEFT = "left"
    RIGHT = "right"
    TOP = "top"
    BOTTOM = "bottom"


@dataclass(frozen=True, slots=True)
class Panel:
    id: str
    panel_type: str
    title: str
    state_revision: int = 0

    def __post_init__(self) -> None:
        _identity(self.id, "panel id")
        _identity(self.panel_type, "panel type")
        _identity(self.title, "panel title")
        if self.state_revision < 0:
            raise ValueError("panel state revision must be non-negative")


@dataclass(frozen=True, slots=True)
class TabStack:
    id: str
    panels: tuple[Panel, ...]
    active_panel_id: str

    def __post_init__(self) -> None:
        _identity(self.id, "node id")
        if not self.panels:
            raise ValueError("tab stack must contain a panel")
        ids = tuple(panel.id for panel in self.panels)
        if len(ids) != len(frozenset(ids)) or self.active_panel_id not in ids:
            raise ValueError("tab stack panel ids must be unique and include the active id")


@dataclass(frozen=True, slots=True)
class Split:
    id: str
    orientation: Orientation
    ratio: float
    first: DockNode
    second: DockNode

    def __post_init__(self) -> None:
        _identity(self.id, "node id")
        if not math.isfinite(self.ratio) or not MIN_SPLIT_RATIO <= self.ratio <= MAX_SPLIT_RATIO:
            raise ValueError("split ratio is outside the persisted bounds")


DockNode = TabStack | Split


@dataclass(frozen=True, slots=True)
class WorkspaceLayout:
    revision: int
    root: DockNode
    hidden: tuple[Panel, ...] = ()
    maximized_panel_id: str | None = None
    link_groups: tuple[tuple[str, str], ...] = ()

    def __post_init__(self) -> None:
        if self.revision < 0:
            raise ValueError("layout revision must be non-negative")
        validate_layout(self)


@dataclass(frozen=True, slots=True)
class Rect:
    x: float
    y: float
    width: float
    height: float

    def __post_init__(self) -> None:
        if not all(math.isfinite(value) for value in (self.x, self.y, self.width, self.height)):
            raise ValueError("rectangle values must be finite")
        if self.width < 0 or self.height < 0:
            raise ValueError("rectangle dimensions must be non-negative")

    def contains(self, x: float, y: float) -> bool:
        return self.x <= x <= self.x + self.width and self.y <= y <= self.y + self.height


@dataclass(frozen=True, slots=True)
class DockGeometry:
    nodes: tuple[tuple[str, Rect], ...]
    splitters: tuple[tuple[str, Rect], ...]

    def node(self, node_id: str) -> Rect:
        return dict(self.nodes)[node_id]


def validate_layout(layout: WorkspaceLayout) -> None:
    node_ids: set[str] = set()
    visible: set[str] = set()

    def visit(node: DockNode, depth: int) -> None:
        if depth > MAX_DOCK_DEPTH:
            raise ValueError("dock tree exceeds maximum depth")
        if node.id in node_ids:
            raise ValueError("dock node ids must be unique")
        node_ids.add(node.id)
        match node:
            case TabStack(panels=panels):
                for panel in panels:
                    if panel.id in visible:
                        raise ValueError("visible panel ids must be unique")
                    visible.add(panel.id)
            case Split(first=first, second=second):
                visit(first, depth + 1)
                visit(second, depth + 1)
            case _ as unreachable:
                assert_never(unreachable)

    visit(layout.root, 0)
    hidden_ids = tuple(panel.id for panel in layout.hidden)
    if len(hidden_ids) != len(frozenset(hidden_ids)) or visible.intersection(hidden_ids):
        raise ValueError("hidden panel ids must be unique and not visible")
    if layout.maximized_panel_id is not None and layout.maximized_panel_id not in visible:
        raise ValueError("maximized panel must be visible")
    linked = tuple(panel_id for panel_id, _ in layout.link_groups)
    known = visible.union(hidden_ids)
    if len(linked) != len(frozenset(linked)) or not frozenset(linked).issubset(known):
        raise ValueError("link groups must identify unique known panels")
    if any(not group or group.strip() != group for _, group in layout.link_groups):
        raise ValueError("link group ids must be valid")


def activate(layout: WorkspaceLayout, panel_id: str) -> WorkspaceLayout:
    _find_panel(layout.root, panel_id)
    return _finish(
        layout,
        _map_stack(layout.root, panel_id, lambda stack: replace(stack, active_panel_id=panel_id)),
    )


def reorder(layout: WorkspaceLayout, panel_id: str, index: int) -> WorkspaceLayout:
    if index < 0:
        raise ValueError("tab index must be non-negative")

    _find_panel(layout.root, panel_id)

    def change(stack: TabStack) -> TabStack:
        panels = list(stack.panels)
        panel = panels.pop(next(i for i, item in enumerate(panels) if item.id == panel_id))
        panels.insert(min(index, len(panels)), panel)
        return replace(stack, panels=tuple(panels))

    return _finish(layout, _map_stack(layout.root, panel_id, change))


def move(
    layout: WorkspaceLayout, panel_id: str, target_stack_id: str, index: int
) -> WorkspaceLayout:
    if index < 0:
        raise ValueError("tab index must be non-negative")
    if _stack_for_panel(layout.root, panel_id) == target_stack_id:
        return reorder(layout, panel_id, index)
    root, panel = _detach(layout.root, panel_id)
    if root is None:
        raise ValueError("cannot move the final visible panel")
    root = _insert(root, target_stack_id, panel, index)
    return _finish(layout, root)


def dock(
    layout: WorkspaceLayout,
    panel_id: str,
    target_stack_id: str,
    position: DockPosition,
    *,
    split_id: str | None = None,
    new_stack_id: str | None = None,
) -> WorkspaceLayout:
    if position is DockPosition.CENTER:
        return move(layout, panel_id, target_stack_id, 2**31 - 1)
    if split_id is None or new_stack_id is None:
        raise ValueError("directional docking requires stable split and stack ids")
    root, panel = _detach(layout.root, panel_id)
    if root is None:
        raise ValueError("cannot split the final visible panel")
    inserted = TabStack(new_stack_id, (panel,), panel.id)

    def replace_target(node: DockNode) -> DockNode:
        if node.id == target_stack_id:
            before = position in (DockPosition.LEFT, DockPosition.TOP)
            orientation = (
                Orientation.HORIZONTAL
                if position in (DockPosition.LEFT, DockPosition.RIGHT)
                else Orientation.VERTICAL
            )
            return Split(
                split_id,
                orientation,
                0.5,
                inserted if before else node,
                node if before else inserted,
            )
        if isinstance(node, Split):
            return replace(
                node, first=replace_target(node.first), second=replace_target(node.second)
            )
        return node

    changed = replace_target(root)
    if changed == root:
        raise KeyError(target_stack_id)
    return _finish(layout, changed)


def close(layout: WorkspaceLayout, panel_id: str) -> WorkspaceLayout:
    root, panel = _detach(layout.root, panel_id)
    if root is None:
        raise ValueError("cannot close the final visible panel")
    return _finish(layout, root, hidden=(*layout.hidden, panel), maximized_panel_id=None)


def reopen(layout: WorkspaceLayout, panel_id: str, target_stack_id: str) -> WorkspaceLayout:
    try:
        panel = next(item for item in layout.hidden if item.id == panel_id)
    except StopIteration as error:
        raise KeyError(panel_id) from error
    root = _insert(layout.root, target_stack_id, panel, 2**31 - 1)
    hidden = tuple(item for item in layout.hidden if item.id != panel_id)
    return _finish(layout, root, hidden=hidden)


def maximize(layout: WorkspaceLayout, panel_id: str) -> WorkspaceLayout:
    _find_panel(layout.root, panel_id)
    return _finish(layout, layout.root, maximized_panel_id=panel_id)


def restore(layout: WorkspaceLayout) -> WorkspaceLayout:
    return _finish(layout, layout.root, maximized_panel_id=None)


def set_link_group(layout: WorkspaceLayout, panel_id: str, group_id: str | None) -> WorkspaceLayout:
    _find_panel_or_hidden(layout, panel_id)
    links = tuple(item for item in layout.link_groups if item[0] != panel_id)
    if group_id is not None:
        _identity(group_id, "link group id")
        links = (*links, (panel_id, group_id))
    return replace(layout, revision=layout.revision + 1, link_groups=links)


def resize(layout: WorkspaceLayout, split_id: str, ratio: float) -> WorkspaceLayout:
    ratio = min(MAX_SPLIT_RATIO, max(MIN_SPLIT_RATIO, ratio))
    changed = False

    def visit(node: DockNode) -> DockNode:
        nonlocal changed
        if isinstance(node, Split):
            if node.id == split_id:
                changed = True
                return replace(node, ratio=ratio)
            return replace(node, first=visit(node.first), second=visit(node.second))
        return node

    root = visit(layout.root)
    if not changed:
        raise KeyError(split_id)
    return _finish(layout, root)


def calculate_geometry(
    root: DockNode,
    host: Rect,
    *,
    dpi_scale: float = 1.0,
    preset_percent: int = 100,
    minimum_extent: float = 120.0,
    splitter: float = 4.0,
) -> DockGeometry:
    if dpi_scale <= 0 or preset_percent not in (100, 125, 150, 175, 200):
        raise ValueError("invalid effective scale inputs")
    scale = min(2.0, max(1.0, dpi_scale * preset_percent / 100))
    nodes: list[tuple[str, Rect]] = []
    splitters: list[tuple[str, Rect]] = []

    def snap(value: float) -> float:
        return float(round(value * scale))

    def visit(node: DockNode, rect: Rect) -> None:
        physical = Rect(snap(rect.x), snap(rect.y), snap(rect.width), snap(rect.height))
        nodes.append((node.id, physical))
        if isinstance(node, Split):
            length = rect.width if node.orientation is Orientation.HORIZONTAL else rect.height
            available = max(0.0, length - splitter)
            low = min(minimum_extent, available / 2)
            first = min(max(available * node.ratio, low), max(low, available - low))
            if node.orientation is Orientation.HORIZONTAL:
                first_rect = Rect(rect.x, rect.y, first, rect.height)
                split_rect = Rect(rect.x + first, rect.y, splitter, rect.height)
                second_rect = Rect(
                    rect.x + first + splitter, rect.y, available - first, rect.height
                )
            else:
                first_rect = Rect(rect.x, rect.y, rect.width, first)
                split_rect = Rect(rect.x, rect.y + first, rect.width, splitter)
                second_rect = Rect(rect.x, rect.y + first + splitter, rect.width, available - first)
            splitters.append(
                (
                    node.id,
                    Rect(
                        snap(split_rect.x),
                        snap(split_rect.y),
                        snap(split_rect.width),
                        snap(split_rect.height),
                    ),
                )
            )
            visit(node.first, first_rect)
            visit(node.second, second_rect)

    visit(root, Rect(host.x / scale, host.y / scale, host.width / scale, host.height / scale))
    return DockGeometry(tuple(nodes), tuple(splitters))


def _identity(value: str, label: str) -> None:
    if not value or value.strip() != value:
        raise ValueError(f"{label} must be non-empty without surrounding whitespace")


def _finish(
    layout: WorkspaceLayout,
    root: DockNode,
    *,
    hidden: tuple[Panel, ...] | None = None,
    maximized_panel_id: str | None | object = ...,
) -> WorkspaceLayout:
    maximum = layout.maximized_panel_id if maximized_panel_id is ... else maximized_panel_id
    assert maximum is None or isinstance(maximum, str)
    return WorkspaceLayout(
        layout.revision + 1,
        root,
        layout.hidden if hidden is None else hidden,
        maximum,
        layout.link_groups,
    )


def _find_panel(node: DockNode, panel_id: str) -> Panel:
    if isinstance(node, TabStack):
        for panel in node.panels:
            if panel.id == panel_id:
                return panel
    else:
        for child in (node.first, node.second):
            try:
                return _find_panel(child, panel_id)
            except KeyError:
                pass
    raise KeyError(panel_id)


def _find_panel_or_hidden(layout: WorkspaceLayout, panel_id: str) -> Panel:
    try:
        return _find_panel(layout.root, panel_id)
    except KeyError:
        for panel in layout.hidden:
            if panel.id == panel_id:
                return panel
    raise KeyError(panel_id)


def _map_stack(
    node: DockNode, panel_id: str, transform: Callable[[TabStack], TabStack]
) -> DockNode:
    if isinstance(node, TabStack):
        if any(panel.id == panel_id for panel in node.panels):
            return transform(node)
        return node
    first = _map_stack(node.first, panel_id, transform)
    second = _map_stack(node.second, panel_id, transform)
    return replace(node, first=first, second=second)


def _contains_panel(node: DockNode, panel_id: str) -> bool:
    try:
        _find_panel(node, panel_id)
    except KeyError:
        return False
    return True


def _stack_for_panel(node: DockNode, panel_id: str) -> str:
    if isinstance(node, TabStack):
        if any(panel.id == panel_id for panel in node.panels):
            return node.id
    else:
        for child in (node.first, node.second):
            try:
                return _stack_for_panel(child, panel_id)
            except KeyError:
                pass
    raise KeyError(panel_id)


def _detach(node: DockNode, panel_id: str) -> tuple[DockNode | None, Panel]:
    if isinstance(node, TabStack):
        panel = _find_panel(node, panel_id)
        remaining = tuple(item for item in node.panels if item.id != panel_id)
        if not remaining:
            return None, panel
        active = node.active_panel_id if node.active_panel_id != panel_id else remaining[0].id
        return replace(node, panels=remaining, active_panel_id=active), panel
    if _contains_panel(node.first, panel_id):
        first, panel = _detach(node.first, panel_id)
        return (node.second if first is None else replace(node, first=first)), panel
    if _contains_panel(node.second, panel_id):
        second, panel = _detach(node.second, panel_id)
        return (node.first if second is None else replace(node, second=second)), panel
    raise KeyError(panel_id)


def _insert(node: DockNode, stack_id: str, panel: Panel, index: int) -> DockNode:
    if isinstance(node, TabStack):
        if node.id != stack_id:
            raise KeyError(stack_id)
        panels = list(node.panels)
        panels.insert(min(index, len(panels)), panel)
        return replace(node, panels=tuple(panels), active_panel_id=panel.id)
    try:
        return replace(node, first=_insert(node.first, stack_id, panel, index))
    except KeyError:
        return replace(node, second=_insert(node.second, stack_id, panel, index))


__all__ = [
    "MAX_DOCK_DEPTH",
    "DockGeometry",
    "DockNode",
    "DockPosition",
    "Orientation",
    "Panel",
    "Rect",
    "Split",
    "TabStack",
    "WorkspaceLayout",
    "activate",
    "calculate_geometry",
    "close",
    "dock",
    "maximize",
    "move",
    "reopen",
    "reorder",
    "resize",
    "restore",
    "set_link_group",
    "validate_layout",
]
