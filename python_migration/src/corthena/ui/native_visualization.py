"""Locked-UI-thread Raylib primitives for immutable Phase 5b visualization views."""

# pyright: reportArgumentType=false, reportAttributeAccessIssue=false, reportUnknownArgumentType=false
# pyright: reportUnknownMemberType=false, reportUnknownVariableType=false

from __future__ import annotations

from corthena.ui.phase5b import MarkerShape, StyleRole, VisualizationView

_COLORS: dict[StyleRole, tuple[int, int, int, int]] = {
    StyleRole.PRIMARY: (60, 200, 200, 255),
    StyleRole.SECONDARY: (155, 124, 246, 255),
    StyleRole.POSITIVE: (76, 195, 138, 255),
    StyleRole.NEGATIVE: (239, 107, 115, 255),
    StyleRole.WARNING: (216, 180, 90, 255),
    StyleRole.MUTED: (126, 136, 150, 255),
    StyleRole.TRAIN: (60, 200, 200, 30),
    StyleRole.VALIDATION: (155, 124, 246, 30),
    StyleRole.TEST: (216, 180, 90, 30),
}


def draw_visualization(view: VisualizationView, inter_font: object, mono_font: object) -> None:
    """Draw prepared primitives and only the fixture's visible table cells."""
    import pyray as rl

    scale = view.scale
    rl.begin_drawing()
    try:
        rl.clear_background(rl.Color(11, 13, 16, 255))
        rl.draw_rectangle(
            0,
            0,
            round(view.viewport.max_x),
            round(44 * scale),
            rl.Color(17, 21, 26, 255),
        )
        rl.draw_text_ex(
            inter_font,
            "Corthena  /  Generic Visualization",
            rl.Vector2(16 * scale, 11 * scale),
            14 * scale,
            0,
            rl.Color(214, 220, 229, 255),
        )
        chart = view.chart_bounds
        rl.draw_rectangle_rec(_rect(rl, chart), rl.Color(11, 13, 16, 255))
        rl.begin_scissor_mode(
            round(chart.min_x), round(chart.min_y), round(chart.width), round(chart.height)
        )
        try:
            for layer in view.frame.layers:
                for item in layer.rects:
                    color = _color(rl, item.style)
                    if layer.kind.value == "heatmap":
                        color = rl.Color(
                            int(155 * (1 - item.value) + 60 * item.value),
                            int(124 * (1 - item.value) + 200 * item.value),
                            int(246 * (1 - item.value) + 200 * item.value),
                            210,
                        )
                    rl.draw_rectangle_rec(_rect(rl, item.bounds), color)
                for polygon in layer.polygons:
                    points = tuple(rl.Vector2(point.x, point.y) for point in polygon.points)
                    rl.draw_triangle_fan(points, len(points), _alpha(rl, polygon.style, 72))
                for segment in layer.segments:
                    rl.draw_line_ex(
                        rl.Vector2(segment.start.x, segment.start.y),
                        rl.Vector2(segment.end.x, segment.end.y),
                        1.25 * scale,
                        _color(rl, segment.style),
                    )
                for marker in layer.markers:
                    _draw_marker(rl, marker, scale)
                for label in layer.labels:
                    rl.draw_text_ex(
                        mono_font,
                        label.text,
                        rl.Vector2(label.position.x, label.position.y),
                        12 * scale,
                        0,
                        _color(rl, label.style),
                    )
            if view.selection is not None:
                rl.draw_rectangle_rec(_rect(rl, view.selection), rl.Color(60, 200, 200, 28))
                rl.draw_rectangle_lines_ex(
                    _rect(rl, view.selection), 1, rl.Color(60, 200, 200, 255)
                )
            if view.crosshair is not None:
                rl.draw_line_ex(
                    rl.Vector2(chart.min_x, view.crosshair.y),
                    rl.Vector2(chart.max_x, view.crosshair.y),
                    1,
                    rl.Color(126, 136, 150, 255),
                )
                rl.draw_line_ex(
                    rl.Vector2(view.crosshair.x, chart.min_y),
                    rl.Vector2(view.crosshair.x, chart.max_y),
                    1,
                    rl.Color(126, 136, 150, 255),
                )
                _draw_tooltip(rl, view, mono_font)
        finally:
            rl.end_scissor_mode()
        rl.draw_rectangle_lines_ex(_rect(rl, chart), 1, rl.Color(37, 43, 51, 255))
        _draw_table(rl, view, inter_font, mono_font)
    finally:
        rl.end_drawing()


def _draw_marker(rl: object, marker: object, scale: float) -> None:
    color = _color(rl, marker.style)
    size = marker.size * scale
    center = rl.Vector2(marker.center.x, marker.center.y)
    if marker.shape is MarkerShape.CIRCLE:
        rl.draw_circle_v(center, size, color)
    elif marker.shape is MarkerShape.TRIANGLE_UP:
        rl.draw_triangle(
            rl.Vector2(center.x, center.y - size),
            rl.Vector2(center.x - size, center.y + size),
            rl.Vector2(center.x + size, center.y + size),
            color,
        )
    else:
        rl.draw_triangle(
            rl.Vector2(center.x, center.y + size),
            rl.Vector2(center.x + size, center.y - size),
            rl.Vector2(center.x - size, center.y - size),
            color,
        )


def _draw_tooltip(rl: object, view: VisualizationView, mono_font: object) -> None:
    if view.crosshair is None or not view.tooltip:
        return
    width, height = 124 * view.scale, (12 + 20 * len(view.tooltip)) * view.scale
    x = min(view.crosshair.x + 10 * view.scale, view.chart_bounds.max_x - width)
    y = min(view.crosshair.y + 10 * view.scale, view.chart_bounds.max_y - height)
    bounds = rl.Rectangle(x, y, width, height)
    rl.draw_rectangle_rec(bounds, rl.Color(23, 28, 34, 245))
    rl.draw_rectangle_lines_ex(bounds, 1, rl.Color(37, 43, 51, 255))
    for index, value in enumerate(view.tooltip):
        payload = (
            "--"
            if value.missing
            else f"{value.number:.4f}"
            if value.number is not None
            else value.text
            if value.text is not None
            else str(value.unix_nanoseconds)
        )
        rl.draw_text_ex(
            mono_font,
            f"{value.label}: {payload}",
            rl.Vector2(x + 6 * view.scale, y + (6 + index * 20) * view.scale),
            12 * view.scale,
            0,
            rl.Color(214, 220, 229, 255),
        )


def _draw_table(rl: object, view: VisualizationView, inter_font: object, mono_font: object) -> None:
    bounds, scale = view.table_bounds, view.scale
    row_height = 24 * scale
    column_width = bounds.width / len(view.headers)
    rl.begin_scissor_mode(
        round(bounds.min_x), round(bounds.min_y), round(bounds.width), round(bounds.height)
    )
    try:
        for column, title in enumerate(view.headers):
            x = bounds.min_x + column * column_width
            cell = rl.Rectangle(x, bounds.min_y, column_width, row_height)
            rl.draw_rectangle_rec(cell, rl.Color(23, 28, 34, 255))
            rl.draw_rectangle_lines_ex(cell, 1, rl.Color(37, 43, 51, 255))
            rl.draw_text_ex(
                inter_font,
                title,
                rl.Vector2(x + 6 * scale, bounds.min_y + 5 * scale),
                12 * scale,
                0,
                rl.Color(214, 220, 229, 255),
            )
        for row_index, row in enumerate(view.rows):
            y = bounds.min_y + (row_index + 1) * row_height
            for column, value in enumerate(row):
                x = bounds.min_x + column * column_width
                cell = rl.Rectangle(x, y, column_width, row_height)
                if row_index == 0:
                    rl.draw_rectangle_rec(cell, rl.Color(23, 28, 34, 255))
                rl.draw_rectangle_lines_ex(cell, 1, rl.Color(37, 43, 51, 160))
                rl.draw_text_ex(
                    mono_font,
                    value,
                    rl.Vector2(x + 6 * scale, y + 5 * scale),
                    12 * scale,
                    0,
                    rl.Color(214, 220, 229, 255),
                )
    finally:
        rl.end_scissor_mode()


def _rect(rl: object, bounds: object) -> object:
    return rl.Rectangle(
        bounds.min_x, bounds.min_y, bounds.max_x - bounds.min_x, bounds.max_y - bounds.min_y
    )


def _color(rl: object, style: StyleRole) -> object:
    return rl.Color(*_COLORS[style])


def _alpha(rl: object, style: StyleRole, alpha: int) -> object:
    red, green, blue, _ = _COLORS[style]
    return rl.Color(red, green, blue, alpha)


__all__ = ["draw_visualization"]
