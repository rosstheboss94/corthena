from __future__ import annotations

from pathlib import Path

from corthena.ui.golden import compare_pngs, encode_rgba_png
from corthena.ui.lifecycle import LaunchConfig, launch


def test_first_party_rgba_comparator_accepts_identical_canonical_png() -> None:
    baseline = (
        Path(__file__).parent / "goldens" / "phase1to4-golden" / "phase3_application_shell.png"
    )
    result = compare_pngs(baseline, baseline, channel_tolerance=3, max_different_ratio=0.002)
    assert result.passed
    assert result.different_pixels == 0
    assert (result.width, result.height) == (1280, 720)


def test_rgba_encoder_round_trips_without_native_values(tmp_path: Path) -> None:
    capture = tmp_path / "rgba.png"
    encode_rgba_png(capture, 2, 1, bytes((1, 2, 3, 255, 4, 5, 6, 255)))
    result = compare_pngs(capture, capture, channel_tolerance=0, max_different_ratio=0)
    assert result.passed


def test_phase3_python_capture_matches_manifest_owned_go_png(tmp_path: Path) -> None:
    baseline = (
        Path(__file__).parent / "goldens" / "phase1to4-golden" / "phase3_application_shell.png"
    )
    capture = tmp_path / "phase3_application_shell.png"
    evidence = launch(LaunchConfig(hidden=True, max_frames=30, capture_path=capture))
    result = compare_pngs(baseline, capture, channel_tolerance=3, max_different_ratio=0.002)
    assert evidence.frames_rendered == 30
    assert evidence.max_actions_drained <= 4
    assert result.passed, result


def test_phase4_python_capture_matches_manifest_owned_go_png(tmp_path: Path) -> None:
    baseline = Path(__file__).parent / "goldens" / "phase1to4-golden" / "phase4_dockable_data.png"
    capture = tmp_path / "phase4_dockable_data.png"
    evidence = launch(LaunchConfig(hidden=True, max_frames=30, capture_path=capture))
    result = compare_pngs(baseline, capture, channel_tolerance=3, max_different_ratio=0.002)
    assert evidence.frames_rendered == 30
    assert evidence.final_state.ui_scale_percent == 100
    assert result.passed, result
