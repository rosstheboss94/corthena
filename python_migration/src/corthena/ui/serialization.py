"""Strict JSON-compatible codecs for the closed Phase 2 messages."""

from __future__ import annotations

from datetime import datetime
from threading import Event
from typing import TypeIs

from corthena.ui.docking import DockPosition
from corthena.ui.persistence import (
    LayoutCollection,
    NamedLayout,
    decode_layouts,
    encode_layouts,
)
from corthena.ui.research.actions import (
    CancelResearch,
    LoadResearch,
    RequestResearch,
    ResearchCancelled,
    ResearchCompleted,
    ResearchFailed,
    SelectResearchRow,
    SetResearchFeature,
    SetResearchRange,
    SetResearchScenario,
    SetResearchVisibility,
)
from corthena.ui.research.deterministic import build_research_snapshot
from corthena.ui.research.models import (
    BarInterval,
    ResearchQuery,
    ResearchScenario,
    ResearchSort,
    TargetSpec,
    TimeRange,
)
from corthena.ui.state import (
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


def _boolean(record: dict[str, object], field: str) -> bool:
    value = record[field]
    if not isinstance(value, bool):
        raise ValueError(f"{field} must be a boolean")
    return value


def _encode_research_query(query: ResearchQuery) -> dict[str, JsonValue]:
    return {
        "correlation_id": query.correlation_id,
        "group_id": query.group_id,
        "generation": query.generation,
        "dataset_id": query.dataset_id,
        "symbols": list(query.symbols),
        "interval": query.interval.value,
        "start": query.time_range.start.isoformat(),
        "end": query.time_range.end.isoformat(),
        "selected_features": list(query.selected_features),
        "target_kind": query.target.kind,
        "target_horizon_bars": query.target.horizon_bars,
        "target_log_return": query.target.log_return,
        "resolution": query.resolution,
        "cursor": query.cursor,
        "page_size": query.page_size,
        "sort": query.sort.value,
        "filter": query.filter,
        "scenario": query.scenario.value,
    }


_RESEARCH_QUERY_FIELDS = frozenset(
    {
        "correlation_id",
        "group_id",
        "generation",
        "dataset_id",
        "symbols",
        "interval",
        "start",
        "end",
        "selected_features",
        "target_kind",
        "target_horizon_bars",
        "target_log_return",
        "resolution",
        "cursor",
        "page_size",
        "sort",
        "filter",
        "scenario",
    }
)


def _string_tuple(record: dict[str, object], field: str) -> tuple[str, ...]:
    values = record[field]
    if not _is_list(values) or not all(isinstance(value, str) for value in values):
        raise ValueError(f"{field} must be a list of strings")
    return tuple(value for value in values if isinstance(value, str))


def _decode_research_query(value: object) -> ResearchQuery:
    record = _object(value, _RESEARCH_QUERY_FIELDS)
    try:
        interval = BarInterval(_string(record, "interval"))
        sort = ResearchSort(_string(record, "sort"))
        scenario = ResearchScenario(_string(record, "scenario"))
    except ValueError as error:
        raise ValueError("unknown Research query enum value") from error
    return ResearchQuery(
        correlation_id=_string(record, "correlation_id"),
        group_id=_string(record, "group_id"),
        generation=_integer(record, "generation"),
        dataset_id=_string(record, "dataset_id"),
        symbols=_string_tuple(record, "symbols"),
        interval=interval,
        time_range=TimeRange(
            datetime.fromisoformat(_string(record, "start")),
            datetime.fromisoformat(_string(record, "end")),
        ),
        selected_features=_string_tuple(record, "selected_features"),
        target=TargetSpec(
            _string(record, "target_kind"),
            _integer(record, "target_horizon_bars"),
            _boolean(record, "target_log_return"),
        ),
        resolution=_integer(record, "resolution"),
        cursor=_string(record, "cursor"),
        page_size=_integer(record, "page_size"),
        sort=sort,
        filter=_string(record, "filter"),
        scenario=scenario,
    )


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
        case RequestResearch(query=query):
            return {"type": "request_research", "query": _encode_research_query(query)}
        case ResearchCompleted(snapshot=snapshot):
            return {
                "type": "research_completed",
                "query": _encode_research_query(snapshot.query),
                "replay_seed": snapshot.replay_seed,
                "replay_clock": snapshot.replay_clock.isoformat(),
            }
        case ResearchFailed(group_id=group_id, generation=generation, message=message, busy=busy):
            return {
                "type": "research_failed",
                "group_id": group_id,
                "generation": generation,
                "message": message,
                "busy": busy,
            }
        case ResearchCancelled(group_id=group_id, generation=generation):
            return {
                "type": "research_cancelled",
                "group_id": group_id,
                "generation": generation,
            }
        case SetResearchFeature(group_id=group_id, feature=feature):
            return {"type": "set_research_feature", "group_id": group_id, "feature": feature}
        case SetResearchScenario(group_id=group_id, scenario=scenario):
            return {
                "type": "set_research_scenario",
                "group_id": group_id,
                "scenario": scenario.value,
            }
        case SetResearchVisibility(
            group_id=group_id,
            show_ohlcv=show_ohlcv,
            show_feature=show_feature,
            show_target=show_target,
        ):
            return {
                "type": "set_research_visibility",
                "group_id": group_id,
                "show_ohlcv": show_ohlcv,
                "show_feature": show_feature,
                "show_target": show_target,
            }
        case SelectResearchRow(group_id=group_id, row_id=row_id, toggle=toggle):
            return {
                "type": "select_research_row",
                "group_id": group_id,
                "row_id": row_id,
                "toggle": toggle,
            }
        case SetResearchRange(group_id=group_id, source_panel_id=panel_id, time_range=time_range):
            return {
                "type": "set_research_range",
                "group_id": group_id,
                "source_panel_id": panel_id,
                "start": time_range.start.isoformat(),
                "end": time_range.end.isoformat(),
            }


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
    if kind == "request_research":
        record = _object(value, frozenset({"type", "query"}))
        return RequestResearch(_decode_research_query(record["query"]))
    if kind == "research_completed":
        record = _object(
            value,
            frozenset({"type", "query", "replay_seed", "replay_clock"}),
        )
        return ResearchCompleted(
            build_research_snapshot(
                _integer(record, "replay_seed"),
                datetime.fromisoformat(_string(record, "replay_clock")),
                _decode_research_query(record["query"]),
                Event(),
            )
        )
    if kind == "research_failed":
        record = _object(value, frozenset({"type", "group_id", "generation", "message", "busy"}))
        return ResearchFailed(
            _string(record, "group_id"),
            _integer(record, "generation"),
            _string(record, "message"),
            _boolean(record, "busy"),
        )
    if kind == "research_cancelled":
        record = _object(value, frozenset({"type", "group_id", "generation"}))
        return ResearchCancelled(_string(record, "group_id"), _integer(record, "generation"))
    if kind == "set_research_feature":
        record = _object(value, frozenset({"type", "group_id", "feature"}))
        return SetResearchFeature(_string(record, "group_id"), _string(record, "feature"))
    if kind == "set_research_scenario":
        record = _object(value, frozenset({"type", "group_id", "scenario"}))
        return SetResearchScenario(
            _string(record, "group_id"),
            ResearchScenario(_string(record, "scenario")),
        )
    if kind == "set_research_visibility":
        record = _object(
            value,
            frozenset({"type", "group_id", "show_ohlcv", "show_feature", "show_target"}),
        )
        return SetResearchVisibility(
            _string(record, "group_id"),
            _boolean(record, "show_ohlcv"),
            _boolean(record, "show_feature"),
            _boolean(record, "show_target"),
        )
    if kind == "select_research_row":
        record = _object(value, frozenset({"type", "group_id", "row_id", "toggle"}))
        return SelectResearchRow(
            _string(record, "group_id"),
            _string(record, "row_id"),
            _boolean(record, "toggle"),
        )
    if kind == "set_research_range":
        record = _object(
            value,
            frozenset({"type", "group_id", "source_panel_id", "start", "end"}),
        )
        return SetResearchRange(
            _string(record, "group_id"),
            _string(record, "source_panel_id"),
            TimeRange(
                datetime.fromisoformat(_string(record, "start")),
                datetime.fromisoformat(_string(record, "end")),
            ),
        )
    raise ValueError(f"unknown action discriminator: {kind!r}")


def encode_effect(effect: UIEffect) -> dict[str, JsonValue]:
    """Encode an effect with a stable discriminator."""
    match effect:
        case LoadSnapshot(request_id=request_id, generation=generation):
            return {"type": "load_snapshot", "request_id": request_id, "generation": generation}
        case CancelRequest(request_id=request_id):
            return {"type": "cancel_request", "request_id": request_id}
        case LoadResearch(query=query):
            return {"type": "load_research", "query": _encode_research_query(query)}
        case CancelResearch(request_id=request_id, group_id=group_id, generation=generation):
            return {
                "type": "cancel_research",
                "request_id": request_id,
                "group_id": group_id,
                "generation": generation,
            }


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
    if kind == "load_research":
        record = _object(value, frozenset({"type", "query"}))
        return LoadResearch(_decode_research_query(record["query"]))
    if kind == "cancel_research":
        record = _object(value, frozenset({"type", "request_id", "group_id", "generation"}))
        return CancelResearch(
            _string(record, "request_id"),
            _string(record, "group_id"),
            _integer(record, "generation"),
        )
    raise ValueError(f"unknown effect discriminator: {kind!r}")


__all__ = ["JsonValue", "decode_action", "decode_effect", "encode_action", "encode_effect"]
