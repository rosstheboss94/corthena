"""Typed native UI boundary and Raylib implementation."""

from corthena.ui.native.models import CapturedFrame, FrameMetrics, WindowSize
from corthena.ui.native.protocol import NativeUIProtocol
from corthena.ui.native.raylib import RaylibUIAdapter, UiThreadViolationError

# Compatibility alias for the pre-contract module surface.
NativeUI = NativeUIProtocol

__all__ = [
    "CapturedFrame",
    "FrameMetrics",
    "NativeUI",
    "NativeUIProtocol",
    "RaylibUIAdapter",
    "UiThreadViolationError",
    "WindowSize",
]
