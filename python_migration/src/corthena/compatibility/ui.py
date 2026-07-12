"""Narrow Raylib/Raygui adapter used by the Windows compatibility gate."""

# pyright: reportAttributeAccessIssue=false, reportUnknownArgumentType=false
# pyright: reportUnknownMemberType=false

from __future__ import annotations

import threading
from dataclasses import dataclass
from pathlib import Path

from corthena.compatibility.assets import stage_assets


@dataclass(frozen=True, slots=True)
class UiEvidence:
    """Native UI evidence containing no Raylib values."""

    owner_thread: int
    asset_sha256: tuple[str, str, str]
    capture: Path


def capture_hidden_frame(capture: Path) -> UiEvidence:
    """Exercise assets, Raygui, capture, and cleanup on one locked OS thread."""
    import pyray as rl

    owner_thread = threading.get_ident()
    inter, mono, atlas_asset = stage_assets()
    rl.set_config_flags(rl.FLAG_WINDOW_HIDDEN)
    rl.init_window(320, 180, "Corthena Phase 0")
    font = None
    mono_font = None
    atlas = None
    try:
        if threading.get_ident() != owner_thread:
            raise RuntimeError("Raylib ownership moved off its initializing thread")
        font = rl.load_font_ex(str(inter.path), 20, None, 0)
        mono_font = rl.load_font_ex(str(mono.path), 16, None, 0)
        atlas = rl.load_texture(str(atlas_asset.path))
        if font.texture.id == 0 or mono_font.texture.id == 0 or atlas.id == 0:
            raise RuntimeError("a bundled native asset failed to load")
        rl.begin_drawing()
        rl.clear_background(rl.Color(15, 23, 42, 255))
        rl.draw_text_ex(font, "Corthena", rl.Vector2(16, 16), 20, 1, rl.RAYWHITE)
        rl.draw_texture_ex(atlas, rl.Vector2(16, 48), 0, 0.25, rl.WHITE)
        rl.gui_button(rl.Rectangle(150, 120, 120, 32), "Compatibility")
        rl.end_drawing()
        image = rl.load_image_from_screen()
        try:
            if not rl.export_image(image, str(capture)):
                raise RuntimeError("hidden frame export failed")
        finally:
            rl.unload_image(image)
    finally:
        if atlas is not None:
            rl.unload_texture(atlas)
        if mono_font is not None:
            rl.unload_font(mono_font)
        if font is not None:
            rl.unload_font(font)
        rl.close_window()
    if not capture.is_file() or capture.stat().st_size == 0:
        raise RuntimeError("hidden frame was not captured")
    return UiEvidence(owner_thread, (inter.sha256, mono.sha256, atlas_asset.sha256), capture)
