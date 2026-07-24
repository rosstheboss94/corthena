"""Subprocess entry point for isolated Milestone 1a Raylib captures."""

from __future__ import annotations

import argparse
from pathlib import Path

from corthena.ui.data_experiments.models import DataIngestionFixture, Phase7Workspace
from corthena.ui.lifecycle import LaunchConfig, launch


def main() -> int:
    parser = argparse.ArgumentParser()
    parser.add_argument("--output", required=True, type=Path)
    parser.add_argument("--width", required=True, type=int)
    parser.add_argument("--height", required=True, type=int)
    parser.add_argument("--scale", required=True, type=int)
    parser.add_argument("--scenario", required=True, choices=tuple(DataIngestionFixture))
    arguments = parser.parse_args()
    launch(
        LaunchConfig(
            hidden=True,
            max_frames=30,
            capture_path=arguments.output,
            width=arguments.width,
            height=arguments.height,
            ui_scale_percent=arguments.scale,
            phase7_workspace=Phase7Workspace.DATA,
            data_ingestion_fixture=DataIngestionFixture(arguments.scenario),
        )
    )
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
