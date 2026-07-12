"""First-party lossless RGBA PNG comparison for manifest-owned captures."""

from __future__ import annotations

import struct
import zlib
from dataclasses import dataclass
from pathlib import Path


@dataclass(frozen=True, slots=True)
class PixelComparison:
    """Deterministic pixel comparison evidence."""

    width: int
    height: int
    different_pixels: int
    different_ratio: float
    passed: bool


def encode_rgba_png(path: Path, width: int, height: int, rgba: bytes) -> None:
    """Encode immutable RGBA pixels without retaining native image values."""
    if width < 1 or height < 1 or len(rgba) != width * height * 4:
        raise ValueError("RGBA dimensions and byte length do not agree")
    scanlines = b"".join(
        b"\x00" + rgba[row * width * 4 : (row + 1) * width * 4] for row in range(height)
    )
    signature = b"\x89PNG\r\n\x1a\n"
    header = struct.pack(">IIBBBBB", width, height, 8, 6, 0, 0, 0)
    path.write_bytes(
        signature
        + _png_chunk(b"IHDR", header)
        + _png_chunk(b"IDAT", zlib.compress(scanlines, level=9))
        + _png_chunk(b"IEND", b"")
    )


def _png_chunk(kind: bytes, payload: bytes) -> bytes:
    return (
        struct.pack(">I", len(payload))
        + kind
        + payload
        + struct.pack(">I", zlib.crc32(kind + payload) & 0xFFFFFFFF)
    )


def compare_pngs(
    expected: Path,
    actual: Path,
    *,
    channel_tolerance: int,
    max_different_ratio: float,
) -> PixelComparison:
    """Compare two non-interlaced 8-bit RGB/RGBA PNGs by RGBA channels."""
    if not 0 <= channel_tolerance <= 255:
        raise ValueError("channel_tolerance must be between 0 and 255")
    if not 0 <= max_different_ratio <= 1:
        raise ValueError("max_different_ratio must be between 0 and 1")
    left_width, left_height, left = _decode_png(expected)
    right_width, right_height, right = _decode_png(actual)
    if (left_width, left_height) != (right_width, right_height):
        return PixelComparison(right_width, right_height, right_width * right_height, 1.0, False)
    different = sum(
        any(
            abs(left[offset + channel] - right[offset + channel]) > channel_tolerance
            for channel in range(4)
        )
        for offset in range(0, len(left), 4)
    )
    total = left_width * left_height
    ratio = different / total
    return PixelComparison(left_width, left_height, different, ratio, ratio <= max_different_ratio)


def _decode_png(path: Path) -> tuple[int, int, bytes]:
    data = path.read_bytes()
    if data[:8] != b"\x89PNG\r\n\x1a\n":
        raise ValueError(f"not a PNG: {path}")
    offset = 8
    width = height = color_type = 0
    compressed = bytearray()
    while offset < len(data):
        length = struct.unpack(">I", data[offset : offset + 4])[0]
        kind = data[offset + 4 : offset + 8]
        payload = data[offset + 8 : offset + 8 + length]
        offset += length + 12
        if kind == b"IHDR":
            width, height, depth, color_type, compression, filtering, interlace = struct.unpack(
                ">IIBBBBB", payload
            )
            if depth != 8 or color_type not in (2, 6) or compression or filtering or interlace:
                raise ValueError("PNG must be non-interlaced 8-bit RGB or RGBA")
        elif kind == b"IDAT":
            compressed.extend(payload)
        elif kind == b"IEND":
            break
    channels = 3 if color_type == 2 else 4
    raw = zlib.decompress(compressed)
    stride = width * channels
    prior = bytearray(stride)
    rgba = bytearray()
    cursor = 0
    for _ in range(height):
        filter_type = raw[cursor]
        cursor += 1
        row = bytearray(raw[cursor : cursor + stride])
        cursor += stride
        _unfilter(row, prior, channels, filter_type)
        for pixel in range(0, stride, channels):
            rgba.extend(row[pixel : pixel + channels])
            if channels == 3:
                rgba.append(255)
        prior = row
    return width, height, bytes(rgba)


def _unfilter(row: bytearray, prior: bytearray, channels: int, filter_type: int) -> None:
    for index in range(len(row)):
        left = row[index - channels] if index >= channels else 0
        above = prior[index]
        upper_left = prior[index - channels] if index >= channels else 0
        match filter_type:
            case 0:
                value = 0
            case 1:
                value = left
            case 2:
                value = above
            case 3:
                value = (left + above) // 2
            case 4:
                value = _paeth(left, above, upper_left)
            case _:
                raise ValueError(f"unsupported PNG filter: {filter_type}")
        row[index] = (row[index] + value) & 255


def _paeth(left: int, above: int, upper_left: int) -> int:
    estimate = left + above - upper_left
    distances = (abs(estimate - left), abs(estimate - above), abs(estimate - upper_left))
    return (left, above, upper_left)[distances.index(min(distances))]


__all__ = ["PixelComparison", "compare_pngs", "encode_rgba_png"]
