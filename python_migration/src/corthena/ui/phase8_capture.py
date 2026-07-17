"""Subprocess entry point for isolated Phase 8 Raylib captures."""

from __future__ import annotations

import argparse
from pathlib import Path

from corthena.ui.jobs_results.models import Phase8Scenario, Phase8Workspace
from corthena.ui.lifecycle import LaunchConfig, launch


def main() -> int:
    parser = argparse.ArgumentParser()
    parser.add_argument("--output", required=True, type=Path)
    parser.add_argument("--width", required=True, type=int)
    parser.add_argument("--height", required=True, type=int)
    parser.add_argument("--scale", required=True, type=int)
    parser.add_argument("--workspace", required=True, choices=("jobs", "results"))
    parser.add_argument("--scenario", required=True, choices=tuple(Phase8Scenario))
    arguments = parser.parse_args()
    launch(
        LaunchConfig(
            hidden=True,
            max_frames=30,
            capture_path=arguments.output,
            width=arguments.width,
            height=arguments.height,
            ui_scale_percent=arguments.scale,
            phase8_workspace=Phase8Workspace(arguments.workspace),
            phase8_scenario=Phase8Scenario(arguments.scenario),
        )
    )
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
