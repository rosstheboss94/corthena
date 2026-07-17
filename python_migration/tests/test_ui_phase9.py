from __future__ import annotations

import json
import time
from dataclasses import FrozenInstanceError
from datetime import UTC, datetime
from pathlib import Path
from threading import Event

import pytest

from corthena.ui.effects import EffectsRuntime
from corthena.ui.models_inference.actions import (
    InferenceCompleted,
    Phase9Completed,
    RequestInference,
    RequestPhase9,
)
from corthena.ui.models_inference.deterministic import ModelsInferenceDemo
from corthena.ui.models_inference.models import (
    AliasCommand,
    InferenceMode,
    InferenceQuery,
    Phase9LoadState,
    Phase9Request,
    Phase9Scenario,
    Phase9Workspace,
    TreeArrays,
)
from corthena.ui.models_inference.reducer import reduce_models_inference
from corthena.ui.serialization import decode_action, decode_effect, encode_action, encode_effect
from corthena.ui.shell import project_shell
from corthena.ui.simulator import DeterministicSimulator, SimulatorConfig
from corthena.ui.state import AppState, SelectWorkspace, Workspace, reduce

FIXED_CLOCK = datetime(2026, 7, 10, 12, tzinfo=UTC)


def _request(
    workspace: Phase9Workspace = Phase9Workspace.MODELS,
    scenario: Phase9Scenario = Phase9Scenario.MODELS_NORMAL,
    generation: int = 1,
) -> Phase9Request:
    return Phase9Request(
        f"phase9-{workspace.value}-{generation:020d}", generation, workspace, scenario
    )


def _query(generation: int = 1, fingerprint: str = "dataset-fingerprint-phase9") -> InferenceQuery:
    return InferenceQuery(
        f"phase9-inference-{generation:020d}",
        generation,
        "champion",
        "dataset-us-equities",
        fingerprint,
        InferenceMode.HISTORICAL,
        datetime(2026, 6, 1, tzinfo=UTC),
        datetime(2026, 7, 1, tzinfo=UTC),
    )


def test_registry_is_stable_immutable_and_final_refit_only() -> None:
    first = ModelsInferenceDemo(309, FIXED_CLOCK).load(_request(), Event())
    second = ModelsInferenceDemo(309, FIXED_CLOCK).load(_request(), Event())
    assert first == second
    assert tuple(item.model_id for item in first.models) == tuple(
        sorted(item.model_id for item in first.models)
    )
    assert all(item.final_refit for item in first.models)
    assert all(len(checksum) == 64 for item in first.models for _, checksum in item.artifact.files)
    with pytest.raises(FrozenInstanceError):
        first.models[0].model_id = "mutated"  # type: ignore[misc]


@pytest.mark.parametrize(
    "tree",
    (
        TreeArrays(
            (1, -1, -1),
            (2, -1, -1),
            (0, -1, -1),
            (0.0, 0.0, 0.0),
            (0.0, 1.0, 2.0),
            (True, False, False),
            1,
        ),
    ),
)
def test_tree_arrays_accept_valid_indexed_structure(tree: TreeArrays) -> None:
    assert tree.left == (1, -1, -1)


def test_tree_arrays_reject_cycles_bounds_and_leaf_conflicts() -> None:
    with pytest.raises(ValueError, match="cycle"):
        TreeArrays((1, 0), (1, 0), (0, 0), (0.0, 0.0), (0.0, 0.0), (True, True), 1)
    with pytest.raises(ValueError, match="feature index"):
        TreeArrays(
            (1, -1, -1),
            (2, -1, -1),
            (2, -1, -1),
            (0.0, 0.0, 0.0),
            (0.0, 0.0, 0.0),
            (True, False, False),
            1,
        )
    with pytest.raises(ValueError, match="leaf"):
        TreeArrays((-1,), (-1,), (0,), (0.0,), (0.0,), (False,), 1)


def test_alias_assignment_is_transactional_confirmed_and_idempotent() -> None:
    demo = ModelsInferenceDemo(309, FIXED_CLOCK)
    command = AliasCommand(
        "promote-1", "correlation-promote-1", 1, "champion", "model-phase9-b", True
    )
    first = demo.assign_alias(command, Event())
    replay = demo.assign_alias(command, Event())
    snapshot = demo.load(_request(generation=2), Event())
    assert first.event.previous_model_id == "model-phase9-a"
    assert replay.replayed
    assert dict(snapshot.aliases)["champion"] == "model-phase9-b"
    assert snapshot.alias_history == (first.event,)
    with pytest.raises(ValueError, match="confirmation"):
        AliasCommand("promote-2", "correlation-promote-2", 1, "champion", "model-phase9-a", False)


def test_compatibility_fails_closed_and_publishes_no_predictions() -> None:
    result = ModelsInferenceDemo(309, FIXED_CLOCK).score(_query(fingerprint="wrong"), Event())
    assert not result.compatibility.compatible
    assert result.compatibility.issues[0].field == "dataset.fingerprint"
    assert result.predictions == ()
    assert result.checksum is None


def test_scoring_ranks_descending_with_symbol_tie_break_and_missing_rows() -> None:
    result = ModelsInferenceDemo(309, FIXED_CLOCK).score(_query(), Event())
    assert tuple(item.prediction.symbol_id for item in result.rankings) == (
        "AAPL",
        "MSFT",
        "NVDA",
        "SPY",
    )
    assert tuple(item.rank for item in result.rankings) == (1, 2, 3, None)
    assert result.predictions[-1].score is None and not result.predictions[-1].eligible
    assert result.checksum is not None


def test_reducer_rejects_stale_load_and_inference_completion() -> None:
    demo = ModelsInferenceDemo(309, FIXED_CLOCK)
    state, _ = reduce_models_inference(AppState().models_inference, RequestPhase9(_request()))
    snapshot = demo.load(_request(), Event())
    state, _ = reduce_models_inference(state, Phase9Completed(snapshot))
    assert state.models.state is Phase9LoadState.READY
    state, _ = reduce_models_inference(state, RequestInference(_query(1)))
    newer, _ = reduce_models_inference(state, RequestInference(_query(2)))
    stale, _ = reduce_models_inference(newer, InferenceCompleted(demo.score(_query(1), Event())))
    assert stale == newer


def test_effects_runtime_executes_phase9_through_client_boundary() -> None:
    simulator = DeterministicSimulator(SimulatorConfig(309, FIXED_CLOCK))
    with EffectsRuntime(simulator) as runtime:
        state, effects = reduce(AppState(), RequestPhase9(_request()))
        assert runtime.enqueue(effects[0]).state.value == "accepted"
        deadline = time.monotonic() + 2
        while time.monotonic() < deadline:
            actions = runtime.drain()
            if actions:
                state, _ = reduce(state, actions[0])
                break
            time.sleep(0.001)
        assert state.models_inference.models.state is Phase9LoadState.READY


def test_phase9_layouts_and_shell_projection_expose_required_panels() -> None:
    state, _ = reduce(AppState(), SelectWorkspace(Workspace.MODELS))
    model_ids = {tab.panel_id for stack in project_shell(state).dock_stacks for tab in stack.tabs}
    assert {
        "models-registry",
        "models-aliases",
        "models-artifact",
        "models-importance",
        "models-tree",
    } <= model_ids
    state, _ = reduce(state, SelectWorkspace(Workspace.INFERENCE))
    inference_ids = {
        tab.panel_id for stack in project_shell(state).dock_stacks for tab in stack.tabs
    }
    assert {
        "inference-selector",
        "inference-compatibility",
        "inference-rankings",
        "inference-distribution",
        "inference-history",
        "inference-export",
    } <= inference_ids


def test_phase9_legacy_golden_manifest_owns_complete_66_image_matrix() -> None:
    manifest = json.loads(
        (
            Path(__file__).parents[2]
            / "internal/app/workstation/testdata/phase9-golden/manifest.json"
        ).read_text(encoding="utf-8")
    )
    entries = manifest["entries"]
    assert len(entries) == 66
    assert {
        (item["golden"]["metadata"]["width"], item["golden"]["metadata"]["height"])
        for item in entries
    } == {(1280, 720), (1920, 1080)}
    assert {item["golden"]["metadata"]["scale_percent"] for item in entries} == {100, 150, 200}
    assert all(item["golden"]["channel_tolerance"] == 3 for item in entries)
    assert all(item["golden"]["max_different_ratio"] == 0.002 for item in entries)


def test_phase9_request_replay_codecs_are_strict_and_round_trip() -> None:
    action = RequestPhase9(_request())
    state, effects = reduce(AppState(), action)
    assert decode_action(encode_action(action)) == action
    assert decode_effect(encode_effect(effects[0])) == effects[0]
    assert state.models_inference.models.state is Phase9LoadState.LOADING
