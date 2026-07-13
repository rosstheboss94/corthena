"""Strict versioned frontend documents and bounded background persistence."""

from __future__ import annotations

import json
import os
import threading
from collections.abc import Mapping
from dataclasses import dataclass
from pathlib import Path
from typing import TypeIs

from corthena.frontend.docking import Orientation, Panel, Split, TabStack, WorkspaceLayout

PREFERENCES_VERSION = 1
LAYOUTS_VERSION = 2
MAX_DOCUMENT_BYTES = 1_048_576


@dataclass(frozen=True, slots=True)
class Preferences:
    revision: int = 0
    ui_scale_percent: int = 125

    def __post_init__(self) -> None:
        if self.revision < 0 or self.ui_scale_percent not in (100, 125, 150, 175, 200):
            raise ValueError("invalid preferences")


@dataclass(frozen=True, slots=True)
class NamedLayout:
    name: str
    layout: WorkspaceLayout

    def __post_init__(self) -> None:
        if not self.name or self.name.strip() != self.name or len(self.name) > 80:
            raise ValueError("invalid layout name")


@dataclass(frozen=True, slots=True)
class LayoutCollection:
    revision: int = 0
    layouts: tuple[NamedLayout, ...] = ()

    def __post_init__(self) -> None:
        names = tuple(item.name for item in self.layouts)
        if self.revision < 0 or len(names) != len(frozenset(names)):
            raise ValueError("invalid named layout collection")


def encode_preferences(value: Preferences) -> bytes:
    return _canonical(
        {
            "version": PREFERENCES_VERSION,
            "revision": value.revision,
            "ui_scale_percent": value.ui_scale_percent,
        }
    )


def decode_preferences(payload: bytes) -> Preferences:
    record = _record(_json(payload), frozenset({"version", "revision", "ui_scale_percent"}))
    version = _integer(record, "version")
    if version == 0:
        return Preferences(_integer(record, "revision"), 125)
    elif version != PREFERENCES_VERSION:
        raise ValueError("incompatible preferences version")
    return Preferences(_integer(record, "revision"), _integer(record, "ui_scale_percent"))


def encode_layouts(value: LayoutCollection) -> bytes:
    return _canonical(
        {
            "version": LAYOUTS_VERSION,
            "revision": value.revision,
            "layouts": [
                {"name": item.name, "layout": _encode_layout(item.layout)} for item in value.layouts
            ],
        }
    )


def decode_layouts(payload: bytes) -> LayoutCollection:
    record = _record(_json(payload), frozenset({"version", "revision", "layouts"}))
    version = _integer(record, "version")
    if version not in (1, LAYOUTS_VERSION):
        raise ValueError("incompatible layouts version")
    raw = record["layouts"]
    if not _is_list(raw):
        raise ValueError("layouts must be a list")
    layouts: list[NamedLayout] = []
    for item in raw:
        named = _record(item, frozenset({"name", "layout"}))
        layouts.append(
            NamedLayout(
                _string(named, "name"), _decode_layout(named["layout"], legacy=version == 1)
            )
        )
    return LayoutCollection(_integer(record, "revision"), tuple(layouts))


class DocumentStore:
    """Filesystem adapter for atomic replacement, quarantine, and recovery."""

    def __init__(self, directory: Path) -> None:
        self._directory = directory

    def load_preferences(self) -> Preferences:
        return self._load("preferences.json", decode_preferences, Preferences())

    def load_layouts(self) -> LayoutCollection:
        return self._load("layouts.json", decode_layouts, LayoutCollection())

    def save_preferences(self, value: Preferences) -> None:
        self._replace("preferences.json", encode_preferences(value))

    def save_layouts(self, value: LayoutCollection) -> None:
        self._replace("layouts.json", encode_layouts(value))

    def _load[Value](self, name: str, decode: object, fallback: Value) -> Value:
        path = self._directory / name
        try:
            payload = path.read_bytes()
            if not callable(decode):
                raise TypeError("decoder must be callable")
            value = decode(payload)
            if not isinstance(value, type(fallback)):
                raise ValueError("decoded document has wrong type")
            return value
        except FileNotFoundError:
            return fallback
        except OSError, ValueError, TypeError, json.JSONDecodeError:
            self._quarantine(path)
            return fallback

    def _replace(self, name: str, payload: bytes) -> None:
        if len(payload) > MAX_DOCUMENT_BYTES:
            raise ValueError("document exceeds maximum size")
        self._directory.mkdir(parents=True, exist_ok=True)
        target = self._directory / name
        temporary = self._directory / f".{name}.tmp"
        try:
            with temporary.open("wb") as stream:
                stream.write(payload)
                stream.flush()
                os.fsync(stream.fileno())
            os.replace(temporary, target)
        finally:
            temporary.unlink(missing_ok=True)

    def _quarantine(self, path: Path) -> None:
        if not path.exists():
            return
        candidate = path.with_suffix(path.suffix + ".invalid")
        index = 0
        while candidate.exists():
            index += 1
            candidate = path.with_suffix(path.suffix + f".invalid.{index}")
        os.replace(path, candidate)


PersistValue = Preferences | LayoutCollection


@dataclass(frozen=True, slots=True)
class SaveCompletion:
    kind: str
    revision: int
    error: str | None = None


class PersistenceWorker:
    """One-owner bounded worker; each document kind has one coalescing slot."""

    def __init__(
        self,
        store: DocumentStore,
        *,
        shutdown_timeout: float = 2.0,
        completion_capacity: int = 16,
    ) -> None:
        if shutdown_timeout <= 0 or completion_capacity < 1:
            raise ValueError("shutdown timeout and completion capacity must be positive")
        self._store = store
        self._timeout = shutdown_timeout
        self._completion_capacity = completion_capacity
        self._condition = threading.Condition()
        self._pending: dict[str, PersistValue] = {}
        self._highest_revision = {"preferences": -1, "layouts": -1}
        self._completions: list[SaveCompletion] = []
        self._closed = False
        self._thread = threading.Thread(target=self._run, name="corthena-persistence", daemon=False)
        self._thread.start()

    def submit(self, value: PersistValue) -> bool:
        kind = "preferences" if isinstance(value, Preferences) else "layouts"
        with self._condition:
            if self._closed:
                return False
            if self._highest_revision[kind] >= value.revision:
                return False
            self._highest_revision[kind] = value.revision
            self._pending[kind] = value
            self._condition.notify()
        return True

    def cancel_pending(self, kind: str | None = None) -> int:
        """Cancel queued, not-yet-started replaceable saves without blocking."""
        if kind not in (None, "preferences", "layouts"):
            raise ValueError("unknown persistence document kind")
        with self._condition:
            keys = tuple(self._pending) if kind is None else (kind,)
            cancelled = sum(key in self._pending for key in keys)
            for key in keys:
                self._pending.pop(key, None)
            return cancelled

    def drain(self, maximum: int = 4) -> tuple[SaveCompletion, ...]:
        if maximum < 0:
            raise ValueError("maximum must be non-negative")
        with self._condition:
            values = tuple(self._completions[:maximum])
            del self._completions[:maximum]
        return values

    def close(self) -> None:
        with self._condition:
            if self._closed:
                return
            self._closed = True
            self._condition.notify_all()
        self._thread.join(self._timeout)
        if self._thread.is_alive():
            raise RuntimeError("persistence worker did not terminate")

    def _run(self) -> None:
        while True:
            with self._condition:
                while not self._pending and not self._closed:
                    self._condition.wait()
                if not self._pending and self._closed:
                    return
                kind = sorted(self._pending)[0]
                value = self._pending.pop(kind)
            error: str | None = None
            try:
                if isinstance(value, Preferences):
                    self._store.save_preferences(value)
                else:
                    self._store.save_layouts(value)
            except (OSError, ValueError) as failure:
                error = str(failure)
            with self._condition:
                if value.revision == self._highest_revision[kind]:
                    if len(self._completions) == self._completion_capacity:
                        del self._completions[0]
                    self._completions.append(SaveCompletion(kind, value.revision, error))


def _encode_layout(layout: WorkspaceLayout) -> dict[str, object]:
    return {
        "revision": layout.revision,
        "root": _encode_node(layout.root),
        "hidden": [_encode_panel(panel) for panel in layout.hidden],
        "maximized_panel_id": layout.maximized_panel_id,
        "link_groups": [list(item) for item in layout.link_groups],
    }


def _encode_node(node: TabStack | Split) -> dict[str, object]:
    if isinstance(node, TabStack):
        return {
            "type": "tabs",
            "id": node.id,
            "panels": [_encode_panel(panel) for panel in node.panels],
            "active_panel_id": node.active_panel_id,
        }
    return {
        "type": "split",
        "id": node.id,
        "orientation": node.orientation.value,
        "ratio": node.ratio,
        "first": _encode_node(node.first),
        "second": _encode_node(node.second),
    }


def _encode_panel(panel: Panel) -> dict[str, object]:
    return {
        "id": panel.id,
        "panel_type": panel.panel_type,
        "title": panel.title,
        "state_revision": panel.state_revision,
    }


def _decode_layout(value: object, *, legacy: bool = False) -> WorkspaceLayout:
    fields = {"revision", "root", "hidden", "maximized_panel_id"}
    if not legacy:
        fields.add("link_groups")
    record = _record(value, frozenset(fields))
    raw_hidden = record["hidden"]
    if not _is_list(raw_hidden):
        raise ValueError("hidden must be a list")
    maximum = record["maximized_panel_id"]
    if maximum is not None and not isinstance(maximum, str):
        raise ValueError("maximized_panel_id must be a string or null")
    links: tuple[tuple[str, str], ...] = ()
    if not legacy:
        raw_links = record["link_groups"]
        if not _is_list(raw_links):
            raise ValueError("link_groups must be a list")
        parsed: list[tuple[str, str]] = []
        for item in raw_links:
            if not _is_list(item) or len(item) != 2:
                raise ValueError("link group entries must be pairs")
            left, right = item
            if not isinstance(left, str) or not isinstance(right, str):
                raise ValueError("link group entries must be strings")
            parsed.append((left, right))
        links = tuple(parsed)
    return WorkspaceLayout(
        _integer(record, "revision"),
        _decode_node(record["root"]),
        tuple(_decode_panel(item) for item in raw_hidden),
        maximum,
        links,
    )


def _decode_node(value: object) -> TabStack | Split:
    if not _is_record(value) or not isinstance(value.get("type"), str):
        raise ValueError("dock node requires a discriminator")
    if value["type"] == "tabs":
        record = _record(value, frozenset({"type", "id", "panels", "active_panel_id"}))
        raw = record["panels"]
        if not _is_list(raw):
            raise ValueError("panels must be a list")
        return TabStack(
            _string(record, "id"),
            tuple(_decode_panel(item) for item in raw),
            _string(record, "active_panel_id"),
        )
    if value["type"] == "split":
        record = _record(
            value, frozenset({"type", "id", "orientation", "ratio", "first", "second"})
        )
        ratio = record["ratio"]
        if not isinstance(ratio, int | float) or isinstance(ratio, bool):
            raise ValueError("ratio must be numeric")
        return Split(
            _string(record, "id"),
            Orientation(_string(record, "orientation")),
            float(ratio),
            _decode_node(record["first"]),
            _decode_node(record["second"]),
        )
    raise ValueError("unknown dock node discriminator")


def _decode_panel(value: object) -> Panel:
    record = _record(value, frozenset({"id", "panel_type", "title", "state_revision"}))
    return Panel(
        _string(record, "id"),
        _string(record, "panel_type"),
        _string(record, "title"),
        _integer(record, "state_revision"),
    )


def _canonical(value: object) -> bytes:
    return (
        json.dumps(value, sort_keys=True, separators=(",", ":"), ensure_ascii=False) + "\n"
    ).encode()


def _json(payload: bytes) -> object:
    if len(payload) > MAX_DOCUMENT_BYTES:
        raise ValueError("document exceeds maximum size")
    value: object = json.loads(payload, object_pairs_hook=_unique_object)
    return value


def _unique_object(pairs: list[tuple[str, object]]) -> dict[str, object]:
    record: dict[str, object] = {}
    for key, value in pairs:
        if key in record:
            raise ValueError(f"duplicate field: {key}")
        record[key] = value
    return record


def _is_record(value: object) -> TypeIs[dict[str, object]]:
    return _is_dict(value) and all(isinstance(key, str) for key in value)


def _is_dict(value: object) -> TypeIs[dict[object, object]]:
    return isinstance(value, dict)


def _is_list(value: object) -> TypeIs[list[object]]:
    return isinstance(value, list)


def _record(value: object, fields: frozenset[str]) -> dict[str, object]:
    if not _is_record(value) or frozenset(value) != fields:
        raise ValueError("document has missing or unknown fields")
    return dict(value)


def _integer(record: Mapping[str, object], field: str) -> int:
    value = record[field]
    if not isinstance(value, int) or isinstance(value, bool):
        raise ValueError(f"{field} must be an integer")
    return value


def _string(record: Mapping[str, object], field: str) -> str:
    value = record[field]
    if not isinstance(value, str):
        raise ValueError(f"{field} must be a string")
    return value


__all__ = [
    "LAYOUTS_VERSION",
    "MAX_DOCUMENT_BYTES",
    "PREFERENCES_VERSION",
    "DocumentStore",
    "LayoutCollection",
    "NamedLayout",
    "PersistenceWorker",
    "Preferences",
    "SaveCompletion",
    "decode_layouts",
    "decode_preferences",
    "encode_layouts",
    "encode_preferences",
]
