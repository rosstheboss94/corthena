"""Strict JSON-compatible codecs for the closed Phase 2 messages."""

from __future__ import annotations

from datetime import datetime
from typing import TypeIs

from corthena.frontend.state import (
    ActivatePanel,
    AdvanceGeneration,
    CancelRequest,
    ContextField,
    CycleLinkContext,
    LoadSnapshot,
    RequestSnapshot,
    RuntimeBusy,
    SelectWorkspace,
    SetCommandPalette,
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

JsonScalar = str | int | bool | None
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
