"""Strict JSON-compatible codecs for the closed Phase 2 messages."""

from __future__ import annotations

from datetime import datetime
from typing import TypeIs

from corthena.frontend.docking import DockPosition
from corthena.frontend.persistence import (
    LayoutCollection,
    NamedLayout,
    decode_layouts,
    encode_layouts,
)
from corthena.frontend.state import (
    ActivateDockPanel,
    ActivatePanel,
    AdvanceGeneration,
    ApplyWorkspaceLayout,
    CancelRequest,
    CloseDockPanel,
    ContextField,
    CycleLinkContext,
    DockPanel,
    LoadSnapshot,
    MoveDockPanel,
    ReopenDockPanel,
    ReorderDockPanel,
    RequestSnapshot,
    ResetWorkspaceLayout,
    ResizeDockSplit,
    RuntimeBusy,
    SelectWorkspace,
    SetCommandPalette,
    SetDockMaximized,
    SetPanelLinkGroup,
    SetSettingsOpen,
    SetUIScale,
    Snapshot,
    SnapshotCompleted,
    SnapshotFailed,
    SnapshotItem,
    UIAction,
    UIEffect,
    Workspace,
)

JsonScalar = str | int | float | bool | None
JsonValue = JsonScalar | list["JsonValue"] | dict[str, "JsonValue"]


def _is_dict(value: object) -> TypeIs[dict[object, object]]:
    return isinstance(value, dict)


def _is_object(value: object) -> TypeIs[dict[str, object]]:
    return _is_dict(value) and all(isinstance(key, str) for key in value)


def _is_list(value: object) -> TypeIs[list[object]]:
    return isinstance(value, list)


def _object(value: object, fields: frozenset[str]) -> dict[str, object]:
    if not _is_object(value):
        raise ValueError("message must be an object with string keys")
    record = dict(value)
    unknown = frozenset(record) - fields
    missing = fields - frozenset(record)
    if unknown or missing:
        raise ValueError(
            f"invalid fields; missing={sorted(missing)!r}, unknown={sorted(unknown)!r}"
        )
    return record


def _string(record: dict[str, object], field: str) -> str:
    value = record[field]
    if not isinstance(value, str):
        raise ValueError(f"{field} must be a string")
    return value


def _integer(record: dict[str, object], field: str) -> int:
    value = record[field]
    if not isinstance(value, int) or isinstance(value, bool):
        raise ValueError(f"{field} must be an integer")
    return value


def encode_action(action: UIAction) -> dict[str, JsonValue]:
    """Encode an action with a stable discriminator."""
    match action:
        case RequestSnapshot(request_id=request_id, generation=generation):
            return {"type": "request_snapshot", "request_id": request_id, "generation": generation}
        case SnapshotCompleted(snapshot=snapshot):
            return {
                "type": "snapshot_completed",
                "request_id": snapshot.request_id,
                "generation": snapshot.generation,
                "seed": snapshot.seed,
                "as_of": snapshot.as_of.isoformat(),
                "items": [
                    {
                        "logical_index": item.logical_index,
                        "symbol": item.symbol,
                        "value_micros": item.value_micros,
                    }
                    for item in snapshot.items
                ],
            }
        case SnapshotFailed(request_id=request_id, generation=generation, message=message):
            return {
                "type": "snapshot_failed",
                "request_id": request_id,
                "generation": generation,
                "message": message,
            }
        case RuntimeBusy(request_id=request_id, generation=generation):
            return {"type": "runtime_busy", "request_id": request_id, "generation": generation}
        case AdvanceGeneration(generation=generation):
            return {"type": "advance_generation", "generation": generation}
        case SelectWorkspace(workspace=workspace):
            return {"type": "select_workspace", "workspace": workspace.value}
        case SetCommandPalette(open=open_value):
            return {"type": "set_command_palette", "open": open_value}
        case SetSettingsOpen(open=open_value):
            return {"type": "set_settings_open", "open": open_value}
        case SetUIScale(scale_percent=scale_percent):
            return {"type": "set_ui_scale", "scale_percent": scale_percent}
        case CycleLinkContext(field=field):
            return {"type": "cycle_link_context", "field": field.value}
        case ActivatePanel(panel_index=panel_index):
            return {"type": "activate_panel", "panel_index": panel_index}
        case ActivateDockPanel(panel_id=panel_id, expected_revision=revision):
            return {
                "type": "activate_dock_panel",
                "panel_id": panel_id,
                "expected_revision": revision,
            }
        case ReorderDockPanel(panel_id=panel_id, index=index, expected_revision=revision):
            return {
                "type": "reorder_dock_panel",
                "panel_id": panel_id,
                "index": index,
                "expected_revision": revision,
            }
        case MoveDockPanel(
            panel_id=panel_id, target_stack_id=target, index=index, expected_revision=revision
        ):
            return {
                "type": "move_dock_panel",
                "panel_id": panel_id,
                "target_stack_id": target,
                "index": index,
                "expected_revision": revision,
            }
        case DockPanel(
            panel_id=panel_id,
            target_stack_id=target,
            position=position,
            split_id=split_id,
            new_stack_id=stack_id,
            expected_revision=revision,
        ):
            return {
                "type": "dock_panel",
                "panel_id": panel_id,
                "target_stack_id": target,
                "position": position.value,
                "split_id": split_id,
                "new_stack_id": stack_id,
                "expected_revision": revision,
            }
        case ResizeDockSplit(split_id=split_id, ratio=ratio, expected_revision=revision):
            return {
                "type": "resize_dock_split",
                "split_id": split_id,
                "ratio": ratio,
                "expected_revision": revision,
            }
        case CloseDockPanel(panel_id=panel_id, expected_revision=revision):
            return {"type": "close_dock_panel", "panel_id": panel_id, "expected_revision": revision}
        case ReopenDockPanel(panel_id=panel_id, target_stack_id=target, expected_revision=revision):
            return {
                "type": "reopen_dock_panel",
                "panel_id": panel_id,
                "target_stack_id": target,
                "expected_revision": revision,
            }
        case SetDockMaximized(panel_id=panel_id, expected_revision=revision):
            return {
                "type": "set_dock_maximized",
                "panel_id": panel_id,
                "expected_revision": revision,
            }
        case SetPanelLinkGroup(panel_id=panel_id, group_id=group_id, expected_revision=revision):
            return {
                "type": "set_panel_link_group",
                "panel_id": panel_id,
                "group_id": group_id,
                "expected_revision": revision,
            }
        case ApplyWorkspaceLayout(
            workspace=workspace, layout=layout, expected_revision=revision, layout_name=name
        ):
            payload = encode_layouts(LayoutCollection(0, (NamedLayout(name, layout),))).decode()
            return {
                "type": "apply_workspace_layout",
                "workspace": workspace.value,
                "layout_document": payload,
                "expected_revision": revision,
            }
        case ResetWorkspaceLayout(expected_revision=revision):
            return {"type": "reset_workspace_layout", "expected_revision": revision}


def decode_action(value: object) -> UIAction:
    """Validate and decode an action, rejecting unknown discriminators and fields."""
    if not _is_object(value) or not isinstance(value.get("type"), str):
        raise ValueError("action requires a string type discriminator")
    kind = value["type"]
    if kind in {"request_snapshot", "runtime_busy"}:
        record = _object(value, frozenset({"type", "request_id", "generation"}))
        cls = RequestSnapshot if kind == "request_snapshot" else RuntimeBusy
        return cls(_string(record, "request_id"), _integer(record, "generation"))
    if kind == "snapshot_failed":
        record = _object(value, frozenset({"type", "request_id", "generation", "message"}))
        return SnapshotFailed(
            _string(record, "request_id"),
            _integer(record, "generation"),
            _string(record, "message"),
        )
    if kind == "advance_generation":
        record = _object(value, frozenset({"type", "generation"}))
        return AdvanceGeneration(_integer(record, "generation"))
    if kind == "select_workspace":
        record = _object(value, frozenset({"type", "workspace"}))
        try:
            workspace = Workspace(_string(record, "workspace"))
        except ValueError as error:
            raise ValueError("unknown workspace") from error
        return SelectWorkspace(workspace)
    if kind in {"set_command_palette", "set_settings_open"}:
        record = _object(value, frozenset({"type", "open"}))
        open_value = record["open"]
        if not isinstance(open_value, bool):
            raise ValueError("open must be a boolean")
        cls = SetCommandPalette if kind == "set_command_palette" else SetSettingsOpen
        return cls(open_value)
    if kind == "set_ui_scale":
        record = _object(value, frozenset({"type", "scale_percent"}))
        return SetUIScale(_integer(record, "scale_percent"))
    if kind == "cycle_link_context":
        record = _object(value, frozenset({"type", "field"}))
        try:
            field = ContextField(_string(record, "field"))
        except ValueError as error:
            raise ValueError("unknown context field") from error
        return CycleLinkContext(field)
    if kind == "activate_panel":
        record = _object(value, frozenset({"type", "panel_index"}))
        return ActivatePanel(_integer(record, "panel_index"))
    if kind in {"activate_dock_panel", "close_dock_panel"}:
        record = _object(value, frozenset({"type", "panel_id", "expected_revision"}))
        cls = ActivateDockPanel if kind == "activate_dock_panel" else CloseDockPanel
        return cls(_string(record, "panel_id"), _integer(record, "expected_revision"))
    if kind == "reorder_dock_panel":
        record = _object(value, frozenset({"type", "panel_id", "index", "expected_revision"}))
        return ReorderDockPanel(
            _string(record, "panel_id"),
            _integer(record, "index"),
            _integer(record, "expected_revision"),
        )
    if kind == "move_dock_panel":
        record = _object(
            value, frozenset({"type", "panel_id", "target_stack_id", "index", "expected_revision"})
        )
        return MoveDockPanel(
            _string(record, "panel_id"),
            _string(record, "target_stack_id"),
            _integer(record, "index"),
            _integer(record, "expected_revision"),
        )
    if kind == "dock_panel":
        record = _object(
            value,
            frozenset(
                {
                    "type",
                    "panel_id",
                    "target_stack_id",
                    "position",
                    "split_id",
                    "new_stack_id",
                    "expected_revision",
                }
            ),
        )
        return DockPanel(
            _string(record, "panel_id"),
            _string(record, "target_stack_id"),
            DockPosition(_string(record, "position")),
            _string(record, "split_id"),
            _string(record, "new_stack_id"),
            _integer(record, "expected_revision"),
        )
    if kind == "resize_dock_split":
        record = _object(value, frozenset({"type", "split_id", "ratio", "expected_revision"}))
        ratio = record["ratio"]
        if not isinstance(ratio, int | float) or isinstance(ratio, bool):
            raise ValueError("ratio must be numeric")
        return ResizeDockSplit(
            _string(record, "split_id"), float(ratio), _integer(record, "expected_revision")
        )
    if kind == "reopen_dock_panel":
        record = _object(
            value, frozenset({"type", "panel_id", "target_stack_id", "expected_revision"})
        )
        return ReopenDockPanel(
            _string(record, "panel_id"),
            _string(record, "target_stack_id"),
            _integer(record, "expected_revision"),
        )
    if kind == "set_dock_maximized":
        record = _object(value, frozenset({"type", "panel_id", "expected_revision"}))
        panel = record["panel_id"]
        if panel is not None and not isinstance(panel, str):
            raise ValueError("panel_id must be a string or null")
        return SetDockMaximized(panel, _integer(record, "expected_revision"))
    if kind == "set_panel_link_group":
        record = _object(value, frozenset({"type", "panel_id", "group_id", "expected_revision"}))
        panel = record["panel_id"]
        group = record["group_id"]
        if not isinstance(panel, str):
            raise ValueError("panel_id must be a string")
        if group is not None and not isinstance(group, str):
            raise ValueError("group_id must be a string or null")
        return SetPanelLinkGroup(panel, group, _integer(record, "expected_revision"))
    if kind == "apply_workspace_layout":
        record = _object(
            value, frozenset({"type", "workspace", "layout_document", "expected_revision"})
        )
        collection = decode_layouts(_string(record, "layout_document").encode())
        if len(collection.layouts) != 1:
            raise ValueError("layout document must contain exactly one layout")
        named = collection.layouts[0]
        return ApplyWorkspaceLayout(
            Workspace(_string(record, "workspace")),
            named.layout,
            _integer(record, "expected_revision"),
            named.name,
        )
    if kind == "reset_workspace_layout":
        record = _object(value, frozenset({"type", "expected_revision"}))
        return ResetWorkspaceLayout(_integer(record, "expected_revision"))
    if kind == "snapshot_completed":
        fields = frozenset({"type", "request_id", "generation", "seed", "as_of", "items"})
        record = _object(value, fields)
        raw_items = record["items"]
        if not _is_list(raw_items):
            raise ValueError("items must be a list")
        items: list[SnapshotItem] = []
        for raw_item in raw_items:
            item = _object(raw_item, frozenset({"logical_index", "symbol", "value_micros"}))
            items.append(
                SnapshotItem(
                    _integer(item, "logical_index"),
                    _string(item, "symbol"),
                    _integer(item, "value_micros"),
                )
            )
        snapshot = Snapshot(
            _string(record, "request_id"),
            _integer(record, "generation"),
            _integer(record, "seed"),
            datetime.fromisoformat(_string(record, "as_of")),
            tuple(items),
        )
        return SnapshotCompleted(snapshot)
    raise ValueError(f"unknown action discriminator: {kind!r}")


def encode_effect(effect: UIEffect) -> dict[str, JsonValue]:
    """Encode an effect with a stable discriminator."""
    match effect:
        case LoadSnapshot(request_id=request_id, generation=generation):
            return {"type": "load_snapshot", "request_id": request_id, "generation": generation}
        case CancelRequest(request_id=request_id):
            return {"type": "cancel_request", "request_id": request_id}


def decode_effect(value: object) -> UIEffect:
    """Validate and decode an effect."""
    if not _is_object(value) or not isinstance(value.get("type"), str):
        raise ValueError("effect requires a string type discriminator")
    kind = value["type"]
    if kind == "load_snapshot":
        record = _object(value, frozenset({"type", "request_id", "generation"}))
        return LoadSnapshot(_string(record, "request_id"), _integer(record, "generation"))
    if kind == "cancel_request":
        record = _object(value, frozenset({"type", "request_id"}))
        return CancelRequest(_string(record, "request_id"))
    raise ValueError(f"unknown effect discriminator: {kind!r}")


__all__ = ["JsonValue", "decode_action", "decode_effect", "encode_action", "encode_effect"]
