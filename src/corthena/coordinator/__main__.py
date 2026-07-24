"""Named Corthena loopback coordinator command."""

from __future__ import annotations

import argparse
from collections.abc import Sequence

import uvicorn

from corthena.compatibility.runtime import probe_runtime
from corthena.coordinator.api import create_data_app
from corthena.coordinator.runtime import CoordinatorRuntime


def main(argv: Sequence[str] | None = None) -> int:
    """Validate the runtime and serve only on IPv4 loopback."""
    parser = argparse.ArgumentParser(prog="corthena-coordinator")
    parser.add_argument("--port", type=int, default=8765)
    args = parser.parse_args(argv)
    if not 1 <= args.port <= 65535:
        parser.error("port must be between 1 and 65535")
    probe_runtime("coordinator")
    runtime = CoordinatorRuntime()
    runtime.start()
    try:
        uvicorn.run(create_data_app(runtime.service), host="127.0.0.1", port=args.port)
    finally:
        runtime.close()
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
