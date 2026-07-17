"""Subprocess entry point for isolated Phase 9 Raylib captures."""

from __future__ import annotations

import argparse
from pathlib import Path

from corthena.ui.lifecycle import LaunchConfig, launch
from corthena.ui.models_inference.models import Phase9Scenario, Phase9Workspace


def main() -> int:
    parser = argparse.ArgumentParser()
    parser.add_argument("--output", required=True, type=Path)
    parser.add_argument("--width", required=True, type=int)
    parser.add_argument("--height", required=True, type=int)
    parser.add_argument("--scale", required=True, type=int)
    parser.add_argument("--workspace", required=True, choices=("models", "inference"))
    parser.add_argument("--scenario", required=True, choices=tuple(Phase9Scenario))
    arguments = parser.parse_args()
    launch(
        LaunchConfig(
            hidden=True,
            max_frames=40,
            capture_path=arguments.output,
            width=arguments.width,
            height=arguments.height,
            ui_scale_percent=arguments.scale,
            phase9_workspace=Phase9Workspace(arguments.workspace),
            phase9_scenario=Phase9Scenario(arguments.scenario),
        )
    )
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
