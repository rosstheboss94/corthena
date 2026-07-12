"""Typed adapter containing all Phase 1 Raylib and Raygui values."""

# pyright: reportAttributeAccessIssue=false, reportUnknownArgumentType=false
# pyright: reportUnknownMemberType=false

from __future__ import annotations

import threading
from dataclasses import dataclass
from typing import TYPE_CHECKING, Protocol

if TYPE_CHECKING:
    from pyray import Font, Texture

from corthena.frontend.assets import FrontendAssets


class UiThreadViolationError(RuntimeError):
    """Raised before a native call when the UI OS thread does not own it."""


class NativeFrontend(Protocol):
    """Native-free lifecycle surface consumed by frontend startup."""

    @property
    def owner_thread_id(self) -> int: ...

    def initialize(self, assets: FrontendAssets, *, hidden: bool) -> None: ...

    def should_close(self) -> bool: ...

    def render_frame(self) -> None: ...

    def close(self) -> None: ...


@dataclass(frozen=True, slots=True)
class WindowSize:
    """Native-independent initial window dimensions."""

    width: int = 1280
    height: int = 720


class RaylibFrontendAdapter:
    """Own Raylib resources on the constructing Windows OS thread."""

    def __init__(self, size: WindowSize | None = None) -> None:
        self._owner_thread_id = threading.get_native_id()
        self._size = WindowSize() if size is None else size
        self._window_open = False
        self._inter_font: Font | None = None
        self._mono_font: Font | None = None
        self._atlas: Texture | None = None

    @property
    def owner_thread_id(self) -> int:
        """Return the locked Windows OS thread identifier."""
        return self._owner_thread_id

    def _assert_owner(self) -> None:
        if threading.get_native_id() != self._owner_thread_id:
            raise UiThreadViolationError("Raylib call attempted outside the locked UI OS thread")

    def initialize(self, assets: FrontendAssets, *, hidden: bool) -> None:
        """Initialize a window and load validated assets on the owner thread."""
        self._assert_owner()
        import pyray as rl

        if self._window_open:
            raise RuntimeError("frontend adapter is already initialized")
        if hidden:
            rl.set_config_flags(rl.FLAG_WINDOW_HIDDEN)
        rl.init_window(self._size.width, self._size.height, "Corthena")
        self._window_open = True
        self._inter_font = rl.load_font_ex(str(assets.inter_font), 20, None, 0)
        self._mono_font = rl.load_font_ex(str(assets.mono_font), 16, None, 0)
        self._atlas = rl.load_texture(str(assets.icon_atlas))
        if (
            self._inter_font.texture.id == 0
            or self._mono_font.texture.id == 0
            or self._atlas.id == 0
        ):
            raise RuntimeError("a bundled frontend asset failed to load")
        rl.set_target_fps(60)

    def should_close(self) -> bool:
        """Poll the window-close flag on the owner thread."""
        self._assert_owner()
        import pyray as rl

        return bool(rl.window_should_close())

    def render_frame(self) -> None:
        """Render one empty Phase 1 frame and a Raygui control."""
        self._assert_owner()
        import pyray as rl

        if not self._window_open:
            raise RuntimeError("frontend adapter is not initialized")
        rl.begin_drawing()
        try:
            rl.clear_background(rl.Color(15, 23, 42, 255))
            rl.gui_label(rl.Rectangle(20, 20, 240, 28), "Corthena")
        finally:
            rl.end_drawing()

    def close(self) -> None:
        """Release owned native resources exactly once on the owner thread."""
        self._assert_owner()
        if not self._window_open:
            return
        import pyray as rl

        first_error: BaseException | None = None
        if self._atlas is not None:
            try:
                rl.unload_texture(self._atlas)
            except BaseException as exc:
                first_error = exc
            self._atlas = None
        if self._mono_font is not None:
            try:
                rl.unload_font(self._mono_font)
            except BaseException as exc:
                if first_error is None:
                    first_error = exc
            self._mono_font = None
        if self._inter_font is not None:
            try:
                rl.unload_font(self._inter_font)
            except BaseException as exc:
                if first_error is None:
                    first_error = exc
            self._inter_font = None
        try:
            rl.close_window()
        except BaseException as exc:
            if first_error is None:
                first_error = exc
        finally:
            self._window_open = False
        if first_error is not None:
            raise first_error


__all__ = [
    "NativeFrontend",
    "RaylibFrontendAdapter",
    "UiThreadViolationError",
    "WindowSize",
]
