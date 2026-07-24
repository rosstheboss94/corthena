"""Named Corthena workstation command."""

from __future__ import annotations

import argparse
from collections.abc import Sequence
from datetime import UTC, datetime

from corthena.compatibility.runtime import probe_runtime
from corthena.ui import LaunchConfig, launch
from corthena.ui.client.coordinator import CoordinatorUIClient
from corthena.ui.simulator import DeterministicSimulator, SimulatorConfig


def main(argv: Sequence[str] | None = None) -> int:
    """Validate the runtime and delegate immediately to UI startup."""
    parser = argparse.ArgumentParser(prog="corthena-workstation")
    parser.add_argument("--hidden", action="store_true", help=argparse.SUPPRESS)
    parser.add_argument("--smoke-frames", type=int, help=argparse.SUPPRESS)
    parser.add_argument("--coordinator-url", default="http://127.0.0.1:8765")
    args = parser.parse_args(argv)
    probe_runtime("workstation")
    fallback = DeterministicSimulator(SimulatorConfig(42, datetime(2026, 7, 10, 12, tzinfo=UTC)))
    client = CoordinatorUIClient(args.coordinator_url, fallback)
    try:
        launch(
            LaunchConfig(hidden=args.hidden, max_frames=args.smoke_frames),
            client=client,
        )
    finally:
        client.close()
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
