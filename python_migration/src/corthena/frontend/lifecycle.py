"""Phase 1 workstation startup and deterministic lifecycle."""

from __future__ import annotations

from collections.abc import Callable
from dataclasses import dataclass

from corthena.frontend.assets import AssetLease
from corthena.frontend.native import NativeFrontend, RaylibFrontendAdapter


@dataclass(frozen=True, slots=True)
class LaunchConfig:
    """Validated bounds for one workstation launch."""

    hidden: bool = False
    max_frames: int | None = None

    def __post_init__(self) -> None:
        if self.max_frames is not None and self.max_frames < 1:
            raise ValueError("max_frames must be at least one")


@dataclass(frozen=True, slots=True)
class LaunchEvidence:
    """Native-free evidence from a completed workstation lifecycle."""

    owner_thread_id: int
    frames_rendered: int
    asset_sha256: tuple[str, str, str]


def launch(
    config: LaunchConfig | None = None,
    *,
    adapter_factory: Callable[[], NativeFrontend] = RaylibFrontendAdapter,
) -> LaunchEvidence:
    """Validate assets, run the bounded frame loop, and clean up deterministically."""
    if config is None:
        config = LaunchConfig()
    with AssetLease() as assets:
        adapter = adapter_factory()
        primary_error: BaseException | None = None
        frames = 0
        try:
            adapter.initialize(assets, hidden=config.hidden)
            while not adapter.should_close():
                adapter.render_frame()
                frames += 1
                if config.max_frames is not None and frames >= config.max_frames:
                    break
        except BaseException as exc:
            primary_error = exc
            raise
        finally:
            try:
                adapter.close()
            except BaseException as cleanup_error:
                if primary_error is None:
                    raise
                primary_error.add_note(f"cleanup also failed: {cleanup_error!r}")
        return LaunchEvidence(adapter.owner_thread_id, frames, assets.sha256)


__all__ = ["LaunchConfig", "LaunchEvidence", "launch"]
