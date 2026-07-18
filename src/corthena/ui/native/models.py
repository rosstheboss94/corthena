"""Native-free values exchanged with the UI adapter."""

from dataclasses import dataclass


@dataclass(frozen=True, slots=True)
class WindowSize:
    """Initial window dimensions."""

    width: int = 1280
    height: int = 720


@dataclass(frozen=True, slots=True)
class CapturedFrame:
    """Immutable pixels captured on the UI owner thread."""

    width: int
    height: int
    rgba: bytes

    def __post_init__(self) -> None:
        if self.width < 1 or self.height < 1 or len(self.rgba) != self.width * self.height * 4:
            raise ValueError("captured RGBA dimensions and byte length do not agree")


@dataclass(frozen=True, slots=True)
class FrameMetrics:
    """Live viewport inputs sampled once for a render frame."""

    width: int
    height: int
    dpi_scale: float
    fps: int

    def __post_init__(self) -> None:
        if self.width < 1 or self.height < 1:
            raise ValueError("frame dimensions must be positive")
        if self.dpi_scale <= 0:
            raise ValueError("dpi_scale must be positive")
        if self.fps < 0:
            raise ValueError("fps must be non-negative")


__all__ = ["CapturedFrame", "FrameMetrics", "WindowSize"]
