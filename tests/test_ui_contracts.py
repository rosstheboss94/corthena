"""Static and compatibility checks for agent-facing UI contracts."""

from datetime import UTC, datetime

from corthena.ui.client import CancellationSignal, UIClient
from corthena.ui.client.protocol import CancellationSignalProtocol, UIClientProtocol
from corthena.ui.native import NativeUI
from corthena.ui.native.protocol import NativeUIProtocol
from corthena.ui.native.raylib import RaylibUIAdapter
from corthena.ui.simulator import DeterministicSimulator, SimulatorConfig


def test_contract_compatibility_aliases_are_preserved() -> None:
    assert CancellationSignal is CancellationSignalProtocol
    assert UIClient is UIClientProtocol
    assert NativeUI is NativeUIProtocol


def test_concrete_adapters_conform_statically() -> None:
    client: UIClientProtocol = DeterministicSimulator(
        SimulatorConfig(seed=1, fixed_clock=datetime(2026, 7, 13, tzinfo=UTC))
    )
    native: NativeUIProtocol = RaylibUIAdapter()

    assert client is not None
    assert native.owner_thread_id > 0
