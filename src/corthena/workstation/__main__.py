"""Named Corthena workstation command."""

from __future__ import annotations

import argparse
from collections.abc import Sequence

from corthena.compatibility.runtime import probe_runtime
from corthena.ui import LaunchConfig, launch


def main(argv: Sequence[str] | None = None) -> int:
    """Validate the runtime and delegate immediately to UI startup."""
    parser = argparse.ArgumentParser(prog="corthena-workstation")
    parser.add_argument("--hidden", action="store_true", help=argparse.SUPPRESS)
    parser.add_argument("--smoke-frames", type=int, help=argparse.SUPPRESS)
    args = parser.parse_args(argv)
    probe_runtime("workstation")
    launch(LaunchConfig(hidden=args.hidden, max_frames=args.smoke_frames))
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
