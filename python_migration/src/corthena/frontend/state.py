"""Immutable Phase 2 frontend state and pure reducer."""

from __future__ import annotations

from dataclasses import dataclass, replace
from datetime import datetime
from enum import StrEnum
from typing import assert_never


class LoadState(StrEnum):
    """State of the simulator-backed startup snapshot."""

    IDLE = "idle"
    LOADING = "loading"
    READY = "ready"
    BUSY = "busy"
    FAILED = "failed"


class Workspace(StrEnum):
    """Closed Phase 3 workspace navigation identifiers."""

    DATA = "data"
    RESEARCH = "research"
    EXPERIMENTS = "experiments"
    JOBS = "jobs"
    RESULTS = "results"
    MODELS = "models"
    INFERENCE = "inference"


class ContextField(StrEnum):
    """Closed shell link-context fields that may be cycled."""

    DATASET = "dataset"
    SYMBOLS = "symbols"
    INTERVAL = "interval"


@dataclass(frozen=True, slots=True, order=True)
class SnapshotItem:
    """One immutable, render-neutral simulator result."""

    logical_index: int
    symbol: str
    value_micros: int

    def __post_init__(self) -> None:
        if self.logical_index < 0:
            raise ValueError("logical_index must be non-negative")
        if not self.symbol or self.symbol.strip() != self.symbol:
            raise ValueError("symbol must be non-empty and have no surrounding whitespace")


@dataclass(frozen=True, slots=True)
class Snapshot:
    """A deterministic snapshot returned through ``FrontendClient``."""

    request_id: str
    generation: int
    seed: int
    as_of: datetime
    items: tuple[SnapshotItem, ...]

    def __post_init__(self) -> None:
        _validate_identity(self.request_id, self.generation)
        if self.as_of.tzinfo is None:
            raise ValueError("as_of must be timezone-aware")
        indexes = tuple(item.logical_index for item in self.items)
        if len(indexes) != len(frozenset(indexes)):
            raise ValueError("snapshot logical indexes must be unique")


@dataclass(frozen=True, slots=True)
class AppState:
    """Published immutable frontend state owned by the UI thread."""

    generation: int = 0
    status: LoadState = LoadState.IDLE
    active_request_id: str | None = None
    snapshot: Snapshot | None = None
    error: str | None = None
    workspace: Workspace = Workspace.DATA
    ui_scale_percent: int = 100
    command_palette_open: bool = False
    settings_open: bool = False
    dataset_context_revision: int = 0
    symbols_context_revision: int = 0
    interval_context_revision: int = 0
    active_panel_index: int = 0
    toasts: tuple[str, ...] = ()
    critical_error: str | None = None

    def __post_init__(self) -> None:
        if self.ui_scale_percent not in (100, 125, 150, 175, 200):
            raise ValueError("ui_scale_percent must be a supported preset")
        revisions = (
            self.dataset_context_revision,
            self.symbols_context_revision,
            self.interval_context_revision,
        )
        if min(revisions) < 0 or self.active_panel_index < 0:
            raise ValueError("shell indexes must be non-negative")
        if any(not message for message in self.toasts):
            raise ValueError("toast messages must be non-empty")
        if self.critical_error == "":
            raise ValueError("critical_error must be non-empty when present")


@dataclass(frozen=True, slots=True)
class RequestSnapshot:
    """Ask the reducer to start a new generation-bound request."""

    request_id: str
    generation: int

    def __post_init__(self) -> None:
        _validate_identity(self.request_id, self.generation)


@dataclass(frozen=True, slots=True)
class SnapshotCompleted:
    """Apply a client result if it still identifies the active request."""

    snapshot: Snapshot


@dataclass(frozen=True, slots=True)
class SnapshotFailed:
    """Apply a typed failure if it still identifies the active request."""

    request_id: str
    generation: int
    message: str

    def __post_init__(self) -> None:
        _validate_identity(self.request_id, self.generation)
        if not self.message:
            raise ValueError("message must be non-empty")


@dataclass(frozen=True, slots=True)
class RuntimeBusy:
    """Report nonblocking queue saturation for an attempted request."""

    request_id: str
    generation: int

    def __post_init__(self) -> None:
        _validate_identity(self.request_id, self.generation)


@dataclass(frozen=True, slots=True)
class AdvanceGeneration:
    """Invalidate all work from earlier generations."""

    generation: int

    def __post_init__(self) -> None:
        if self.generation < 1:
            raise ValueError("generation must be positive")


@dataclass(frozen=True, slots=True)
class SelectWorkspace:
    """Select one shell workspace without performing domain work."""

    workspace: Workspace


@dataclass(frozen=True, slots=True)
class SetCommandPalette:
    """Open or close the shell command palette."""

    open: bool


@dataclass(frozen=True, slots=True)
class SetSettingsOpen:
    """Open or close the shell Settings overlay."""

    open: bool


@dataclass(frozen=True, slots=True)
class SetUIScale:
    """Select one supported shell UI scale preset."""

    scale_percent: int

    def __post_init__(self) -> None:
        if self.scale_percent not in (100, 125, 150, 175, 200):
            raise ValueError("scale_percent must be a supported preset")


@dataclass(frozen=True, slots=True)
class CycleLinkContext:
    """Cycle a canonical shell context field."""

    field: ContextField


@dataclass(frozen=True, slots=True)
class ActivatePanel:
    """Activate a shell panel by stable logical index."""

    panel_index: int

    def __post_init__(self) -> None:
        if self.panel_index < 0:
            raise ValueError("panel_index must be non-negative")


UIAction = (
    RequestSnapshot
    | SnapshotCompleted
    | SnapshotFailed
    | RuntimeBusy
    | AdvanceGeneration
    | SelectWorkspace
    | SetCommandPalette
    | SetSettingsOpen
    | SetUIScale
    | CycleLinkContext
    | ActivatePanel
)


@dataclass(frozen=True, slots=True)
class LoadSnapshot:
    """Load one snapshot off the UI thread."""

    request_id: str
    generation: int

    def __post_init__(self) -> None:
        _validate_identity(self.request_id, self.generation)


@dataclass(frozen=True, slots=True)
class CancelRequest:
    """Cancel superseded work by request identity."""

    request_id: str

    def __post_init__(self) -> None:
        _validate_identity(self.request_id, 0)


UIEffect = LoadSnapshot | CancelRequest


class InvariantViolationError(RuntimeError):
    """Raised when a supposedly closed variant reaches an exhaustive switch."""


def _validate_identity(request_id: str, generation: int) -> None:
    if not request_id or request_id.strip() != request_id:
        raise ValueError("request_id must be non-empty and have no surrounding whitespace")
    if generation < 0:
        raise ValueError("generation must be non-negative")


def reduce(state: AppState, action: UIAction) -> tuple[AppState, tuple[UIEffect, ...]]:
    """Purely reduce one closed action into a new state and ordered effects."""
    match action:
        case RequestSnapshot(request_id=request_id, generation=generation):
            _validate_identity(request_id, generation)
            if generation != state.generation:
                return state, ()
            effects: tuple[UIEffect, ...] = (LoadSnapshot(request_id, generation),)
            if state.active_request_id is not None and state.active_request_id != request_id:
                effects = (CancelRequest(state.active_request_id), *effects)
            return (
                replace(
                    state,
                    status=LoadState.LOADING,
                    active_request_id=request_id,
                    error=None,
                ),
                effects,
            )
        case SnapshotCompleted(snapshot=snapshot):
            _validate_identity(snapshot.request_id, snapshot.generation)
            if (
                snapshot.generation != state.generation
                or snapshot.request_id != state.active_request_id
            ):
                return state, ()
            stable = replace(snapshot, items=tuple(sorted(snapshot.items)))
            return (
                replace(
                    state,
                    status=LoadState.READY,
                    active_request_id=None,
                    snapshot=stable,
                    error=None,
                ),
                (),
            )
        case SnapshotFailed(request_id=request_id, generation=generation, message=message):
            _validate_identity(request_id, generation)
            if generation != state.generation or request_id != state.active_request_id:
                return state, ()
            return (
                replace(
                    state,
                    status=LoadState.FAILED,
                    active_request_id=None,
                    error=message,
                ),
                (),
            )
        case RuntimeBusy(request_id=request_id, generation=generation):
            _validate_identity(request_id, generation)
            if generation != state.generation or request_id != state.active_request_id:
                return state, ()
            return (
                replace(
                    state,
                    status=LoadState.BUSY,
                    active_request_id=None,
                    error="effect queue is busy",
                ),
                (),
            )
        case AdvanceGeneration(generation=generation):
            if generation <= state.generation:
                raise ValueError("generation must advance monotonically")
            effects = (
                (CancelRequest(state.active_request_id),)
                if state.active_request_id is not None
                else ()
            )
            return (
                AppState(
                    generation=generation,
                    workspace=state.workspace,
                    ui_scale_percent=state.ui_scale_percent,
                    command_palette_open=state.command_palette_open,
                    settings_open=state.settings_open,
                    dataset_context_revision=state.dataset_context_revision,
                    symbols_context_revision=state.symbols_context_revision,
                    interval_context_revision=state.interval_context_revision,
                    active_panel_index=state.active_panel_index,
                    toasts=state.toasts,
                    critical_error=state.critical_error,
                ),
                effects,
            )
        case SelectWorkspace(workspace=workspace):
            return replace(state, workspace=workspace, command_palette_open=False), ()
        case SetCommandPalette(open=open_value):
            return replace(state, command_palette_open=open_value, settings_open=False), ()
        case SetSettingsOpen(open=open_value):
            return replace(state, settings_open=open_value, command_palette_open=False), ()
        case SetUIScale(scale_percent=scale_percent):
            return replace(state, ui_scale_percent=scale_percent), ()
        case CycleLinkContext(field=ContextField.DATASET):
            return replace(state, dataset_context_revision=state.dataset_context_revision + 1), ()
        case CycleLinkContext(field=ContextField.SYMBOLS):
            return replace(state, symbols_context_revision=state.symbols_context_revision + 1), ()
        case CycleLinkContext(field=ContextField.INTERVAL):
            return replace(state, interval_context_revision=state.interval_context_revision + 1), ()
        case CycleLinkContext(field=field):
            raise InvariantViolationError(f"unknown ContextField: {field!r}")
        case ActivatePanel(panel_index=panel_index):
            return replace(state, active_panel_index=panel_index), ()
        case _ as unreachable:
            try:
                assert_never(unreachable)
            except AssertionError as error:
                raise InvariantViolationError(f"unknown UIAction: {type(unreachable)!r}") from error


__all__ = [
    "ActivatePanel",
    "AdvanceGeneration",
    "AppState",
    "CancelRequest",
    "ContextField",
    "CycleLinkContext",
    "InvariantViolationError",
    "LoadSnapshot",
    "LoadState",
    "RequestSnapshot",
    "RuntimeBusy",
    "SelectWorkspace",
    "SetCommandPalette",
    "SetSettingsOpen",
    "SetUIScale",
    "Snapshot",
    "SnapshotCompleted",
    "SnapshotFailed",
    "SnapshotItem",
    "UIAction",
    "UIEffect",
    "Workspace",
    "reduce",
]
