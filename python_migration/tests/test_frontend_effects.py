from __future__ import annotations

import threading
import time
from dataclasses import dataclass
from datetime import UTC, datetime

import pytest

from corthena.frontend.client import CancellationSignal, RequestCancelledError
from corthena.frontend.effects import (
    EffectsRuntime,
    EnqueueState,
    RuntimeClosedError,
    RuntimeConfig,
)
from corthena.frontend.simulator import DeterministicSimulator, SimulatorConfig
from corthena.frontend.state import CancelRequest, LoadSnapshot, Snapshot, SnapshotCompleted

FIXED_CLOCK = datetime(2026, 7, 12, 14, 30, tzinfo=UTC)


def wait_for_actions(runtime: EffectsRuntime, count: int) -> tuple[object, ...]:
    deadline = time.monotonic() + 2
    actions: list[object] = []
    while len(actions) < count and time.monotonic() < deadline:
        actions.extend(runtime.drain())
        time.sleep(0.001)
    assert len(actions) == count
    return tuple(actions)


def run_replay(worker_count: int, request_ids: tuple[str, ...]) -> dict[str, Snapshot]:
    client = DeterministicSimulator(SimulatorConfig(101, FIXED_CLOCK))
    config = RuntimeConfig(worker_count=worker_count, effect_capacity=8, action_capacity=8)
    with EffectsRuntime(client, config) as runtime:
        for request_id in request_ids:
            assert runtime.enqueue(LoadSnapshot(request_id, 3)).state is EnqueueState.ACCEPTED
        actions = wait_for_actions(runtime, len(request_ids))
    completed = (action for action in actions if isinstance(action, SnapshotCompleted))
    return {action.snapshot.request_id: action.snapshot for action in completed}


def test_replay_is_identical_across_worker_counts_and_completion_orders() -> None:
    one = run_replay(1, ("a", "b", "c"))
    many = run_replay(3, ("c", "a", "b"))
    assert one == many


def test_recorded_phase_2_seeded_startup_scenario() -> None:
    # Canonical migration-baseline Phase 2 inputs from the legacy PNG manifest.
    clock = datetime(2026, 7, 10, 12, tzinfo=UTC)
    client = DeterministicSimulator(SimulatorConfig(42, clock))
    snapshots: list[Snapshot] = []
    for workers in (1, 3):
        with EffectsRuntime(client, RuntimeConfig(worker_count=workers)) as runtime:
            runtime.enqueue(LoadSnapshot("phase2-startup", 0))
            action = wait_for_actions(runtime, 1)[0]
            assert isinstance(action, SnapshotCompleted)
            snapshots.append(action.snapshot)
    assert snapshots[0] == snapshots[1]
    assert snapshots[0].seed == 42
    assert snapshots[0].as_of == clock


@dataclass
class BlockingClient:
    entered: threading.Event
    exited: threading.Event

    def load_snapshot(
        self, request_id: str, generation: int, cancellation: CancellationSignal
    ) -> Snapshot:
        self.entered.set()
        cancellation.wait(2)
        self.exited.set()
        raise RequestCancelledError(request_id)


def test_cancellation_during_work_and_idempotent_shutdown_leave_no_threads() -> None:
    client = BlockingClient(threading.Event(), threading.Event())
    runtime = EffectsRuntime(client, RuntimeConfig(shutdown_timeout_seconds=1))
    names = runtime.worker_names
    runtime.enqueue(LoadSnapshot("request", 0))
    assert client.entered.wait(1)
    assert runtime.enqueue(CancelRequest("request")).state is EnqueueState.CANCELLED
    assert client.exited.wait(1)
    runtime.close()
    runtime.close()
    assert not any(thread.name in names for thread in threading.enumerate())
    with pytest.raises(RuntimeClosedError):
        runtime.enqueue(LoadSnapshot("late", 0))


@dataclass
class GateClient:
    gate: threading.Event

    def load_snapshot(
        self, request_id: str, generation: int, cancellation: CancellationSignal
    ) -> Snapshot:
        while not self.gate.wait(0.001):
            if cancellation.is_set():
                raise RequestCancelledError(request_id)
        return DeterministicSimulator(SimulatorConfig(3, FIXED_CLOCK)).load_snapshot(
            request_id, generation, cancellation
        )


def test_nonblocking_saturation_reports_typed_busy_and_drain_is_bounded() -> None:
    gate = threading.Event()
    runtime = EffectsRuntime(
        GateClient(gate),
        RuntimeConfig(effect_capacity=1, action_capacity=4, max_actions_per_drain=1),
    )
    try:
        assert runtime.enqueue(LoadSnapshot("running", 0)).state is EnqueueState.ACCEPTED
        deadline = time.monotonic() + 1
        result = runtime.enqueue(LoadSnapshot("queued", 0))
        while result.state is EnqueueState.BUSY and time.monotonic() < deadline:
            result = runtime.enqueue(LoadSnapshot("queued", 0))
        assert result.state is EnqueueState.ACCEPTED
        busy = runtime.enqueue(LoadSnapshot("busy", 0))
        assert busy.state is EnqueueState.BUSY
        assert busy.action is not None
        gate.set()
        wait_for_actions(runtime, 2)
        with pytest.raises(ValueError, match="exceeds"):
            runtime.drain(2)
    finally:
        gate.set()
        runtime.close()


def test_duplicate_inflight_identity_does_not_replace_cancellation_owner() -> None:
    gate = threading.Event()
    with EffectsRuntime(GateClient(gate)) as runtime:
        assert runtime.enqueue(LoadSnapshot("same", 0)).state is EnqueueState.ACCEPTED
        duplicate = runtime.enqueue(LoadSnapshot("same", 0))
        assert duplicate.state is EnqueueState.BUSY
        assert duplicate.action is not None
        assert runtime.enqueue(CancelRequest("same")).state is EnqueueState.CANCELLED
        gate.set()
        assert runtime.drain() == ()


def test_simulator_cancellation_before_work() -> None:
    cancellation = threading.Event()
    cancellation.set()
    simulator = DeterministicSimulator(SimulatorConfig(1, FIXED_CLOCK))
    with pytest.raises(RequestCancelledError):
        simulator.load_snapshot("cancelled", 0, cancellation)


def test_cancellation_before_dispatch_and_after_completion_are_safe() -> None:
    gate = threading.Event()
    runtime = EffectsRuntime(GateClient(gate), RuntimeConfig(effect_capacity=2))
    try:
        runtime.enqueue(LoadSnapshot("running", 0))
        runtime.enqueue(LoadSnapshot("queued", 0))
        runtime.enqueue(CancelRequest("queued"))
        gate.set()
        actions = wait_for_actions(runtime, 1)
        assert isinstance(actions[0], SnapshotCompleted)
        assert actions[0].snapshot.request_id == "running"
        assert runtime.enqueue(CancelRequest("running")).state is EnqueueState.CANCELLED
        assert runtime.drain() == ()
    finally:
        gate.set()
        runtime.close()
