"""Validated, package-owned frontend assets."""

from __future__ import annotations

import hashlib
from contextlib import ExitStack
from dataclasses import dataclass
from importlib.resources import as_file, files
from pathlib import Path
from typing import Protocol


class AssetValidationError(RuntimeError):
    """Raised before native initialization when a required asset is invalid."""


class ReadableResource(Protocol):
    """Small typed surface used to read a package resource."""

    def read_bytes(self) -> bytes: ...


def _read_resource_bytes(resource: ReadableResource) -> bytes:
    return resource.read_bytes()


@dataclass(frozen=True, slots=True)
class FrontendAssets:
    """Validated filesystem paths and stable identities for required assets."""

    inter_font: Path
    mono_font: Path
    icon_atlas: Path
    sha256: tuple[str, str, str]


class AssetLease:
    """Own filesystem leases for validated package resources."""

    def __init__(self) -> None:
        self._stack = ExitStack()

    def __enter__(self) -> FrontendAssets:
        try:
            root = files(__package__)
            required = (
                ("fonts/InterVariable.ttf", b"\x00\x01\x00\x00"),
                ("fonts/JetBrainsMono-Regular.ttf", b"\x00\x01\x00\x00"),
                ("icons/lucide-atlas.png", b"\x89PNG\r\n\x1a\n"),
            )
            notices = (
                "licenses/Inter-OFL-1.1.txt",
                "licenses/JetBrainsMono-OFL-1.1.txt",
                "licenses/Lucide-ISC-MIT.txt",
                "THIRD_PARTY_NOTICES.md",
            )
            paths: list[Path] = []
            digests: list[str] = []
            for relative, signature in required:
                resource = root.joinpath(relative)
                content = _read_resource_bytes(resource)
                if not content.startswith(signature):
                    raise AssetValidationError(f"invalid bundled frontend asset: {relative}")
                paths.append(self._stack.enter_context(as_file(resource)))
                digests.append(hashlib.sha256(content).hexdigest())
            for relative in notices:
                if not root.joinpath(relative).read_text(encoding="utf-8").strip():
                    raise AssetValidationError(f"missing bundled asset notice: {relative}")
            return FrontendAssets(
                paths[0],
                paths[1],
                paths[2],
                tuple(digests),  # type: ignore[arg-type]
            )
        except BaseException:
            self._stack.close()
            raise

    def __exit__(self, exc_type: object, exc: object, traceback: object) -> None:
        self._stack.close()


__all__ = ["AssetLease", "AssetValidationError", "FrontendAssets"]
