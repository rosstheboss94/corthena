"""Subprocess entry point for isolated Phase 6 Research Raylib captures."""

from __future__ import annotations

import argparse
from pathlib import Path

from corthena.ui.lifecycle import LaunchConfig, launch
from corthena.ui.research.models import ResearchScenario


def main() -> int:
    parser = argparse.ArgumentParser()
    parser.add_argument("--output", required=True, type=Path)
    parser.add_argument("--width", required=True, type=int)
    parser.add_argument("--height", required=True, type=int)
    parser.add_argument("--scale", required=True, type=int)
    parser.add_argument(
        "--scenario",
        required=True,
        choices=(
            "normal",
            "linked_selection",
            "loading",
            "failure",
            "degraded",
            "recovered",
        ),
    )
    arguments = parser.parse_args()
    linked_selection = arguments.scenario == "linked_selection"
    scenario = ResearchScenario.NORMAL if linked_selection else ResearchScenario(arguments.scenario)
    launch(
        LaunchConfig(
            hidden=True,
            max_frames=30,
            capture_path=arguments.output,
            width=arguments.width,
            height=arguments.height,
            ui_scale_percent=arguments.scale,
            research_scenario=scenario,
            research_linked_selection=linked_selection,
        )
    )
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
