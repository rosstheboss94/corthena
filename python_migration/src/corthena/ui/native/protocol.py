"""Agent-facing contract for the thread-affine native UI adapter."""

from typing import Protocol

from corthena.ui.assets import UIAssets
from corthena.ui.native.models import CapturedFrame, FrameMetrics
from corthena.ui.phase5b import ChartAction, VisualizationView
from corthena.ui.shell import ShellView
from corthena.ui.state import UIAction


class NativeUIProtocol(Protocol):
    """Native-free lifecycle surface consumed by UI startup."""

    @property
    def owner_thread_id(self) -> int: ...

    def initialize(self, assets: UIAssets, *, hidden: bool) -> None: ...

    def should_close(self) -> bool: ...

    def frame_metrics(self) -> FrameMetrics: ...

    def render_frame(self) -> None: ...

    def render_shell(self, view: ShellView) -> tuple[UIAction, ...]: ...

    def render_visualization(self, view: VisualizationView) -> tuple[ChartAction, ...]: ...

    def capture_rgba(self) -> CapturedFrame: ...

    def close(self) -> None: ...
