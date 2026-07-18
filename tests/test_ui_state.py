from __future__ import annotations

from dataclasses import FrozenInstanceError
from datetime import UTC, datetime

import pytest
from hypothesis import given
from hypothesis import strategies as st

from corthena.ui.serialization import (
    decode_action,
    decode_effect,
    encode_action,
    encode_effect,
)
from corthena.ui.state import (
    ActivatePanel,
    AdvanceGeneration,
    AppState,
    CancelRequest,
    ContextField,
    CycleLinkContext,
    LoadSnapshot,
    LoadState,
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
    reduce,
)


def snapshot(request_id: str = "request-1", generation: int = 0) -> Snapshot:
    return Snapshot(
        request_id,
        generation,
        17,
        datetime(2026, 7, 12, 14, tzinfo=UTC),
        (SnapshotItem(1, "MSFT", 2), SnapshotItem(0, "AAPL", 1)),
    )


def test_reducer_supersedes_and_rejects_stale_or_wrong_generation() -> None:
    state, effects = reduce(AppState(), RequestSnapshot("request-1", 0))
    assert state.status is LoadState.LOADING
    assert effects == (LoadSnapshot("request-1", 0),)
    state, effects = reduce(state, RequestSnapshot("request-2", 0))
    assert effects == (CancelRequest("request-1"), LoadSnapshot("request-2", 0))
    assert reduce(state, SnapshotCompleted(snapshot("request-1")))[0] == state
    assert reduce(state, SnapshotCompleted(snapshot("request-2", 1)))[0] == state
    state, _ = reduce(state, SnapshotCompleted(snapshot("request-2")))
    assert state.status is LoadState.READY
    assert state.active_request_id is None
    assert state.snapshot is not None
    assert tuple(item.logical_index for item in state.snapshot.items) == (0, 1)


def test_generation_advance_cancels_and_resets_state() -> None:
    loading, _ = reduce(AppState(), RequestSnapshot("old", 0))
    next_state, effects = reduce(loading, AdvanceGeneration(1))
    assert next_state == AppState(generation=1)
    assert effects == (CancelRequest("old"),)
    with pytest.raises(ValueError, match="advance"):
        reduce(next_state, AdvanceGeneration(1))


def test_published_state_and_snapshot_are_immutable_and_validated() -> None:
    state = AppState()
    with pytest.raises(FrozenInstanceError):
        AppState.__setattr__(state, "generation", 2)
    with pytest.raises(ValueError, match="timezone-aware"):
        Snapshot("request", 0, 1, datetime(2026, 7, 12), ())
    with pytest.raises(ValueError, match="unique"):
        Snapshot(
            "request",
            0,
            1,
            datetime(2026, 7, 12, tzinfo=UTC),
            (SnapshotItem(0, "A", 1), SnapshotItem(0, "B", 2)),
        )


@given(st.text(min_size=1).filter(lambda value: value.strip() == value))
def test_request_reduction_is_pure_and_repeatable(request_id: str) -> None:
    original = AppState()
    left = reduce(original, RequestSnapshot(request_id, 0))
    right = reduce(original, RequestSnapshot(request_id, 0))
    assert left == right
    assert original == AppState()


@pytest.mark.parametrize(
    "action",
    [
        RequestSnapshot("request-1", 0),
        SnapshotCompleted(snapshot()),
        SnapshotFailed("request-1", 0, "failed"),
        RuntimeBusy("request-1", 0),
        AdvanceGeneration(1),
        SelectWorkspace(Workspace.RESEARCH),
        SetCommandPalette(True),
        SetSettingsOpen(True),
        SetUIScale(150),
        CycleLinkContext(ContextField.DATASET),
        ActivatePanel(3),
    ],
)
def test_all_action_variants_round_trip(action: UIAction) -> None:
    assert decode_action(encode_action(action)) == action


@pytest.mark.parametrize("effect", [LoadSnapshot("request-1", 0), CancelRequest("request-1")])
def test_all_effect_variants_round_trip(effect: UIEffect) -> None:
    assert decode_effect(encode_effect(effect)) == effect


@pytest.mark.parametrize(
    "message",
    [
        {"type": "unknown"},
        {"type": "request_snapshot", "request_id": "x", "generation": 0, "extra": 1},
        {"type": 7},
    ],
)
def test_action_decoder_rejects_unknown_or_invalid_messages(message: object) -> None:
    with pytest.raises(ValueError):
        decode_action(message)


def test_effect_decoder_rejects_unknown_discriminator() -> None:
    with pytest.raises(ValueError, match="unknown"):
        decode_effect({"type": "network_request"})


def test_workspace_navigation_is_pure_and_preserved_across_generation() -> None:
    selected, effects = reduce(AppState(), SelectWorkspace(Workspace.RESEARCH))
    assert selected.workspace is Workspace.RESEARCH
    assert effects == ()
    advanced, _ = reduce(selected, AdvanceGeneration(1))
    assert advanced.workspace is Workspace.RESEARCH


def test_shell_actions_are_typed_exclusive_and_deterministic() -> None:
    state, _ = reduce(AppState(), SetCommandPalette(True))
    assert state.command_palette_open and not state.settings_open
    state, _ = reduce(state, SetSettingsOpen(True))
    assert state.settings_open and not state.command_palette_open
    state, _ = reduce(state, SetUIScale(150))
    state, _ = reduce(state, CycleLinkContext(ContextField.SYMBOLS))
    state, _ = reduce(state, ActivatePanel(4))
    assert (
        state.ui_scale_percent,
        state.symbols_context_revision,
        state.active_panel_index,
    ) == (150, 1, 4)
    with pytest.raises(ValueError, match="supported preset"):
        SetUIScale(110)
    with pytest.raises(ValueError, match="non-negative"):
        ActivatePanel(-1)
