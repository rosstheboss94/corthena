from __future__ import annotations

import json
import time
from dataclasses import FrozenInstanceError
from datetime import UTC, datetime
from pathlib import Path
from threading import Event

import pytest

from corthena.ui.effects import EffectsRuntime, EnqueueState, RuntimeConfig
from corthena.ui.jobs_results.actions import (
    ComparisonCompleted,
    Phase8Completed,
    RequestComparison,
    RequestPhase8,
)
from corthena.ui.jobs_results.deterministic import JobsResultsDemo
from corthena.ui.jobs_results.models import (
    CheckpointCompatibility,
    ComparisonQuery,
    JobCommand,
    JobCommandKind,
    JobState,
    MetricPartition,
    Phase8LoadState,
    Phase8Request,
    Phase8Scenario,
    Phase8Workspace,
)
from corthena.ui.jobs_results.reducer import reduce_jobs_results
from corthena.ui.serialization import decode_action, decode_effect, encode_action, encode_effect
from corthena.ui.shell import project_shell
from corthena.ui.simulator import DeterministicSimulator, SimulatorConfig
from corthena.ui.state import AppState, SelectWorkspace, Workspace, reduce, workspace_layout

FIXED_CLOCK = datetime(2026, 7, 10, 12, tzinfo=UTC)


def _request(
    workspace: Phase8Workspace = Phase8Workspace.JOBS,
    scenario: Phase8Scenario = Phase8Scenario.JOBS_SUCCESS,
    generation: int = 1,
) -> Phase8Request:
    return Phase8Request(
        f"phase8-{workspace.value}-{generation:020d}",
        generation,
        workspace,
        scenario,
    )


def _demo() -> JobsResultsDemo:
    return JobsResultsDemo(208, FIXED_CLOCK)


def test_phase8_jobs_have_stable_queue_order_and_complete_run_identity() -> None:
    snapshot = _demo().load(_request(), Event())
    ordinals = tuple(item.summary.stable_queue_ordinal for item in snapshot.jobs)
    assert ordinals == tuple(sorted(ordinals))
    completed = next(item for item in snapshot.jobs if item.summary.state is JobState.COMPLETED)
    assert completed.summary.completed_run_id == "run-phase8-a"
    assert completed.summary.progress == 1.0


@pytest.mark.parametrize(
    ("scenario", "expected"),
    (
        (Phase8Scenario.JOBS_SUCCESS, JobState.RUNNING),
        (Phase8Scenario.JOBS_PAUSE_RESUME, JobState.PAUSED),
        (Phase8Scenario.JOBS_CANCELLATION, JobState.CANCELLED),
        (Phase8Scenario.JOBS_INTERRUPTION, JobState.INTERRUPTED),
        (Phase8Scenario.JOBS_FAILURE, JobState.FAILED),
        (Phase8Scenario.JOBS_CHECKPOINT_INCOMPATIBLE, JobState.INTERRUPTED),
    ),
)
def test_phase8_jobs_scenarios(scenario: Phase8Scenario, expected: JobState) -> None:
    snapshot = _demo().load(_request(scenario=scenario), Event())
    assert snapshot.jobs[0].summary.state is expected


def test_job_commands_are_validated_idempotent_and_auditable() -> None:
    demo = _demo()
    demo.load(_request(), Event())
    pause = JobCommand(
        "command-pause", "correlation-pause", 1, "job-phase8-primary", JobCommandKind.PAUSE
    )
    first = demo.command(pause, Event())
    replay = demo.command(pause, Event())
    assert first.detail.summary.state is JobState.PAUSED
    assert first.detail.lease is not None and first.detail.lease.active_slots == 0
    assert replay.replayed
    assert replay.detail.logs[-1].message.endswith("command-pause")
    resume = JobCommand(
        "command-resume", "correlation-resume", 1, "job-phase8-primary", JobCommandKind.RESUME
    )
    resumed = demo.command(resume, Event()).detail
    assert resumed.summary.state is JobState.RUNNING
    assert resumed.lease is not None and resumed.lease.active_slots == resumed.lease.granted_slots
    cancel = JobCommand(
        "command-cancel", "correlation-cancel", 1, "job-phase8-primary", JobCommandKind.CANCEL
    )
    cancelled = demo.command(cancel, Event())
    assert cancelled.detail.summary.state is JobState.CANCELLED
    refreshed = demo.load(_request(generation=2), Event())
    assert refreshed.jobs[0].summary.state is JobState.CANCELLED
    with pytest.raises(ValueError, match="cancel is valid"):
        demo.command(
            JobCommand(
                "command-cancel-2",
                "correlation-cancel-2",
                1,
                "job-phase8-primary",
                JobCommandKind.CANCEL,
            ),
            Event(),
        )


def test_incompatible_checkpoint_fails_closed_and_retains_previous_valid() -> None:
    demo = _demo()
    snapshot = demo.load(_request(scenario=Phase8Scenario.JOBS_CHECKPOINT_INCOMPATIBLE), Event())
    checkpoint = snapshot.jobs[0].checkpoint
    assert checkpoint.compatibility is CheckpointCompatibility.INCOMPATIBLE
    assert checkpoint.previous_valid_checkpoint_id is not None
    with pytest.raises(ValueError, match="fails closed"):
        demo.command(
            JobCommand(
                "resume-bad",
                "resume-bad-correlation",
                1,
                "job-phase8-primary",
                JobCommandKind.RESUME,
            ),
            Event(),
        )


def test_results_comparison_keeps_validation_and_test_separate_and_is_frozen() -> None:
    demo = _demo()
    snapshot = demo.load(_request(Phase8Workspace.RESULTS, Phase8Scenario.RESULTS_NORMAL), Event())
    query = ComparisonQuery("comparison-1", 1, tuple(item.run_id for item in snapshot.runs[:3]))
    comparison = demo.compare(query, Event())
    assert tuple(item.summary.run_id for item in comparison.runs) == query.run_ids
    partitions = {item.partition for item in comparison.runs[0].metrics}
    assert partitions == {MetricPartition.VALIDATION, MetricPartition.TEST}
    assert comparison.differences
    with pytest.raises(FrozenInstanceError):
        comparison.runs[0].summary.run_id = "changed"  # type: ignore[misc]


def test_empty_comparison_is_valid_and_over_limit_is_rejected() -> None:
    assert _demo().compare(ComparisonQuery("comparison-empty", 1, ()), Event()).runs == ()
    with pytest.raises(ValueError, match="zero to four"):
        ComparisonQuery("comparison-too-large", 1, tuple(f"run-{index}" for index in range(5)))


def test_reducer_rejects_stale_load_and_retains_comparison_during_refresh() -> None:
    demo = _demo()
    first = demo.load(_request(), Event())
    state, _ = reduce_jobs_results(AppState().jobs_results, RequestPhase8(first.request))
    state, _ = reduce_jobs_results(state, Phase8Completed(first))
    state, _ = reduce_jobs_results(state, RequestPhase8(_request(generation=2)))
    unchanged, effects = reduce_jobs_results(state, Phase8Completed(first))
    assert unchanged == state
    assert effects == ()
    assert state.jobs.state is Phase8LoadState.LOADING
    assert state.jobs.stale

    result_request = _request(Phase8Workspace.RESULTS, Phase8Scenario.RESULTS_NORMAL)
    result_snapshot = demo.load(result_request, Event())
    results_state, _ = reduce_jobs_results(AppState().jobs_results, RequestPhase8(result_request))
    results_state, _ = reduce_jobs_results(results_state, Phase8Completed(result_snapshot))
    query = ComparisonQuery("comparison-old", 1, (result_snapshot.runs[0].run_id,))
    comparison = demo.compare(query, Event())
    results_state, _ = reduce_jobs_results(results_state, RequestComparison(query))
    results_state, _ = reduce_jobs_results(results_state, ComparisonCompleted(comparison))
    refresh = ComparisonQuery("comparison-new", 2, (result_snapshot.runs[1].run_id,))
    results_state, _ = reduce_jobs_results(results_state, RequestComparison(refresh))
    assert results_state.results.comparison == comparison
    stale, _ = reduce_jobs_results(results_state, ComparisonCompleted(comparison))
    assert stale == results_state


def test_effects_runtime_executes_phase8_and_bounds_draining() -> None:
    simulator = DeterministicSimulator(SimulatorConfig(208, FIXED_CLOCK))
    config = RuntimeConfig(
        worker_count=1, effect_capacity=2, action_capacity=2, max_actions_per_drain=1
    )
    with EffectsRuntime(simulator, config) as runtime:
        state, effects = reduce(AppState(), RequestPhase8(_request()))
        assert runtime.enqueue(effects[0]).state is EnqueueState.ACCEPTED
        deadline = time.monotonic() + 2
        actions = ()
        while not actions and time.monotonic() < deadline:
            actions = runtime.drain()
            time.sleep(0.001)
        assert len(actions) == 1
        state, _ = reduce(state, actions[0])
        assert state.jobs_results.jobs.state is Phase8LoadState.READY


def test_phase8_workspace_composition_and_responsive_projection() -> None:
    state, _ = reduce(AppState(), SelectWorkspace(Workspace.JOBS))
    jobs_layout = workspace_layout(state)
    assert jobs_layout.root.id == "jobs-root"
    state = AppState(workspace=Workspace.RESULTS)
    results_layout = workspace_layout(state)
    assert results_layout.root.id == "results-root"
    for width, height in ((1280, 720), (1920, 1080)):
        for scale in (100, 150, 200):
            view = project_shell(
                AppState(workspace=Workspace.RESULTS, ui_scale_percent=scale),
                width=width,
                height=height,
            )
            assert view.content_bounds.width == width
            assert all(stack.body_bounds.width >= 0 for stack in view.dock_stacks)


def test_phase8_requests_and_effects_round_trip_strict_replay_codec() -> None:
    request_action = RequestPhase8(_request())
    assert decode_action(encode_action(request_action)) == request_action
    _, effects = reduce(AppState(), request_action)
    assert decode_effect(encode_effect(effects[0])) == effects[0]
    query_action = RequestComparison(ComparisonQuery("comparison-codec", 1, ("run-phase8-a",)))
    assert decode_action(encode_action(query_action)) == query_action


def test_phase8_manifest_owns_exact_sixty_cases() -> None:
    path = (
        Path(__file__).parents[2]
        / "internal"
        / "app"
        / "workstation"
        / "testdata"
        / "phase8-golden"
        / "manifest.json"
    )
    manifest = json.loads(path.read_text(encoding="utf-8"))
    entries = manifest["entries"]
    assert len(entries) == 60
    assert {entry["workspace"] for entry in entries} == {"jobs", "results"}
    assert all(entry["golden"]["channel_tolerance"] == 3 for entry in entries)
    assert all(entry["golden"]["max_different_ratio"] == 0.002 for entry in entries)
