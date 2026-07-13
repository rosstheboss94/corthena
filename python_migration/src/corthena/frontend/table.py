"""Typed deterministic Phase 5 table operations and two-axis virtualization."""

from __future__ import annotations

import math
from dataclasses import dataclass, replace
from datetime import UTC, datetime
from enum import StrEnum
from functools import cmp_to_key


class TableError(ValueError):
    """Raised for invalid typed table input."""


class CellKind(StrEnum):
    """Closed cell value kinds."""

    STRING = "string"
    INTEGER = "integer"
    FLOAT = "float"
    BOOLEAN = "boolean"
    TIME = "time"


CellValue = str | int | float | bool | datetime | None


@dataclass(frozen=True, slots=True)
class Column:
    """A validated table column definition."""

    id: str
    title: str
    kind: CellKind
    width: float
    minimum_width: float = 48.0
    pinned: bool = False
    sortable: bool = True

    def __post_init__(self) -> None:
        if (
            not self.id
            or not self.title
            or not math.isfinite(self.width)
            or self.width < self.minimum_width
        ):
            raise TableError("invalid column definition")
        if not math.isfinite(self.minimum_width) or self.minimum_width <= 0:
            raise TableError("minimum width must be finite and positive")


@dataclass(frozen=True, slots=True)
class Cell:
    """A nullable value whose runtime type agrees with its column kind."""

    kind: CellKind
    value: CellValue

    def __post_init__(self) -> None:
        if self.value is None:
            return
        valid = (
            (self.kind is CellKind.STRING and type(self.value) is str)
            or (self.kind is CellKind.INTEGER and type(self.value) is int)
            or (
                self.kind is CellKind.FLOAT
                and type(self.value) is float
                and math.isfinite(self.value)
            )
            or (self.kind is CellKind.BOOLEAN and type(self.value) is bool)
            or (
                self.kind is CellKind.TIME
                and isinstance(self.value, datetime)
                and self.value.tzinfo is not None
            )
        )
        if not valid:
            raise TableError("cell value does not match its kind")


@dataclass(frozen=True, slots=True)
class Row:
    """One stable table row."""

    id: str
    source_index: int
    cells: tuple[Cell, ...]

    def __post_init__(self) -> None:
        if not self.id or self.source_index < 0:
            raise TableError("row identity is invalid")


@dataclass(frozen=True, slots=True)
class Dataset:
    """An immutable typed table dataset."""

    columns: tuple[Column, ...]
    rows: tuple[Row, ...]

    def __post_init__(self) -> None:
        if len({column.id for column in self.columns}) != len(self.columns):
            raise TableError("column IDs must be unique")
        if len({row.id for row in self.rows}) != len(self.rows):
            raise TableError("row IDs must be unique")
        if len({row.source_index for row in self.rows}) != len(self.rows):
            raise TableError("source indexes must be unique")
        for row in self.rows:
            if len(row.cells) != len(self.columns):
                raise TableError("row cell count must match columns")
            if any(
                cell.kind is not column.kind
                for cell, column in zip(row.cells, self.columns, strict=True)
            ):
                raise TableError("cell kind must match its column")


class SortDirection(StrEnum):
    """Typed sort direction."""

    ASCENDING = "ascending"
    DESCENDING = "descending"


class NullOrder(StrEnum):
    """Explicit null placement independent of direction."""

    FIRST = "first"
    LAST = "last"


@dataclass(frozen=True, slots=True)
class SortSpec:
    """One stable typed sort key."""

    column_id: str
    direction: SortDirection = SortDirection.ASCENDING
    nulls: NullOrder = NullOrder.LAST


def sort_dataset(dataset: Dataset, specs: tuple[SortSpec, ...]) -> Dataset:
    """Sort typed values, explicit nulls, later keys, then source index."""
    indexes = {_column_index(dataset, spec.column_id): spec for spec in specs}
    if any(not dataset.columns[index].sortable for index in indexes):
        raise TableError("column is not sortable")

    def compare(left: Row, right: Row) -> int:
        for index, spec in indexes.items():
            result = _compare_cells(left.cells[index], right.cells[index], spec.nulls)
            if (
                result
                and left.cells[index].value is not None
                and right.cells[index].value is not None
            ):
                return -result if spec.direction is SortDirection.DESCENDING else result
            if result:
                return result
        return (left.source_index > right.source_index) - (left.source_index < right.source_index)

    return Dataset(dataset.columns, tuple(sorted(dataset.rows, key=cmp_to_key(compare))))


class FilterOperator(StrEnum):
    """Closed typed filter operations."""

    EQUAL = "equal"
    NOT_EQUAL = "not_equal"
    LESS = "less"
    LESS_EQUAL = "less_equal"
    GREATER = "greater"
    GREATER_EQUAL = "greater_equal"
    CONTAINS = "contains"
    IS_NULL = "is_null"
    IS_NOT_NULL = "is_not_null"


@dataclass(frozen=True, slots=True)
class FilterSpec:
    """One conjunctive typed filter."""

    column_id: str
    operator: FilterOperator
    value: Cell | None = None


def filter_dataset(dataset: Dataset, specs: tuple[FilterSpec, ...]) -> Dataset:
    """Apply deterministic conjunctive filters while retaining stable IDs."""
    indexed = tuple((_column_index(dataset, spec.column_id), spec) for spec in specs)
    for index, spec in indexed:
        if spec.operator not in (FilterOperator.IS_NULL, FilterOperator.IS_NOT_NULL) and (
            spec.value is None
            or spec.value.value is None
            or spec.value.kind is not dataset.columns[index].kind
        ):
            raise TableError("filter value must match its column")
        if (
            spec.operator is FilterOperator.CONTAINS
            and dataset.columns[index].kind is not CellKind.STRING
        ):
            raise TableError("contains requires a string column")
    return Dataset(
        dataset.columns,
        tuple(
            row
            for row in dataset.rows
            if all(_matches(row.cells[index], spec) for index, spec in indexed)
        ),
    )


@dataclass(frozen=True, slots=True)
class TableWindow:
    """Final virtual row and column window with bounded overscan."""

    row_start: int
    row_end: int
    scrolling_columns: tuple[int, ...]
    pinned_columns: tuple[int, ...]
    cell_work: int


def compute_window(
    columns: tuple[Column, ...],
    row_count: int,
    viewport_width: float,
    viewport_height: float,
    scroll_x: float,
    scroll_y: float,
    row_height: float,
    *,
    row_overscan: int = 2,
    column_overscan: int = 1,
) -> TableWindow:
    """Compute only visible rows and columns plus fixed bounded overscan."""
    values = (viewport_width, viewport_height, scroll_x, scroll_y, row_height)
    if (
        any(not math.isfinite(value) for value in values)
        or min(viewport_width, viewport_height, row_height) <= 0
    ):
        raise TableError("viewport and row height must be finite and positive")
    if row_count < 0 or scroll_x < 0 or scroll_y < 0 or row_overscan < 0 or column_overscan < 0:
        raise TableError("counts, scroll, and overscan must be non-negative")
    first = max(0, int(scroll_y // row_height) - row_overscan)
    visible_rows = math.ceil(viewport_height / row_height)
    end = min(row_count, first + visible_rows + row_overscan * 2)
    pinned = tuple(index for index, column in enumerate(columns) if column.pinned)
    pinned_width = sum(columns[index].width for index in pinned)
    available = max(0.0, viewport_width - pinned_width)
    scrolling = [index for index, column in enumerate(columns) if not column.pinned]
    positions: list[tuple[int, float, float]] = []
    position = 0.0
    for index in scrolling:
        positions.append((index, position, position + columns[index].width))
        position += columns[index].width
    visible_indexes = [
        offset
        for offset, (_, start, finish) in enumerate(positions)
        if finish >= scroll_x and start <= scroll_x + available
    ]
    if visible_indexes:
        low = max(0, visible_indexes[0] - column_overscan)
        high = min(len(positions), visible_indexes[-1] + column_overscan + 1)
        selected = tuple(index for index, _, _ in positions[low:high])
    else:
        selected = ()
    return TableWindow(first, end, selected, pinned, (end - first) * (len(selected) + len(pinned)))


@dataclass(frozen=True, slots=True)
class Selection:
    """Stable row-ID selection retained through view mutations."""

    ids: tuple[str, ...] = ()
    anchor: str | None = None

    def select(
        self, rows: tuple[Row, ...], row_id: str, *, toggle: bool = False, extend: bool = False
    ) -> Selection:
        positions = {row.id: index for index, row in enumerate(rows)}
        if row_id not in positions:
            raise TableError("selected row is missing")
        if extend and self.anchor in positions:
            start, end = sorted((positions[self.anchor], positions[row_id]))
            return Selection(tuple(row.id for row in rows[start : end + 1]), self.anchor)
        if toggle:
            return Selection(
                tuple(item for item in self.ids if item != row_id)
                if row_id in self.ids
                else (*self.ids, row_id),
                row_id,
            )
        return Selection((row_id,), row_id)

    def prune(self, rows: tuple[Row, ...]) -> Selection:
        present = frozenset(row.id for row in rows)
        return Selection(
            tuple(item for item in self.ids if item in present),
            self.anchor if self.anchor in present else None,
        )


def resize_column(columns: tuple[Column, ...], column_id: str, width: float) -> tuple[Column, ...]:
    """Return immutable columns with one clamped resized header."""
    index = next((i for i, column in enumerate(columns) if column.id == column_id), None)
    if index is None or not math.isfinite(width):
        raise TableError("invalid resize")
    result = list(columns)
    result[index] = replace(result[index], width=max(width, result[index].minimum_width))
    return tuple(result)


def copy_selection(
    dataset: Dataset,
    selection: Selection,
    column_ids: tuple[str, ...],
    *,
    include_header: bool,
    max_bytes: int,
) -> str:
    """Create bounded TSV without returning a partial result on overflow."""
    if max_bytes <= 0:
        raise TableError("copy limit must be positive")
    indexes = tuple(_column_index(dataset, column_id) for column_id in column_ids)
    lines: list[str] = []
    if include_header:
        lines.append("\t".join(_escape(dataset.columns[index].title) for index in indexes))
    selected = frozenset(selection.ids)
    for row in dataset.rows:
        if row.id in selected:
            lines.append("\t".join(_format(row.cells[index]) for index in indexes))
    output = "\n".join(lines) + ("\n" if lines else "")
    if len(output.encode()) > max_bytes:
        raise TableError("copy output exceeds byte limit")
    return output


def _column_index(dataset: Dataset, column_id: str) -> int:
    for index, column in enumerate(dataset.columns):
        if column.id == column_id:
            return index
    raise TableError(f"unknown column: {column_id}")


def _compare_cells(left: Cell, right: Cell, nulls: NullOrder) -> int:
    if left.value is None or right.value is None:
        if left.value is right.value:
            return 0
        return -1 if (left.value is None) is (nulls is NullOrder.FIRST) else 1
    if left.kind is CellKind.STRING:
        left_value, right_value = str(left.value), str(right.value)
        return (left_value > right_value) - (left_value < right_value)
    elif left.kind is CellKind.INTEGER:
        left_value, right_value = int(left.value), int(right.value)  # type: ignore[arg-type]
        return (left_value > right_value) - (left_value < right_value)
    elif left.kind is CellKind.FLOAT:
        left_value, right_value = float(left.value), float(right.value)  # type: ignore[arg-type]
        return (left_value > right_value) - (left_value < right_value)
    elif left.kind is CellKind.BOOLEAN:
        left_value, right_value = bool(left.value), bool(right.value)
        return (left_value > right_value) - (left_value < right_value)
    else:
        assert isinstance(left.value, datetime) and isinstance(right.value, datetime)
        left_value, right_value = left.value.timestamp(), right.value.timestamp()
        return (left_value > right_value) - (left_value < right_value)


def _matches(cell: Cell, spec: FilterSpec) -> bool:
    if spec.operator is FilterOperator.IS_NULL:
        return cell.value is None
    if spec.operator is FilterOperator.IS_NOT_NULL:
        return cell.value is not None
    if cell.value is None or spec.value is None or spec.value.value is None:
        return False
    if spec.operator is FilterOperator.CONTAINS:
        return str(spec.value.value) in str(cell.value)
    comparison = _compare_cells(cell, spec.value, NullOrder.LAST)
    return {
        FilterOperator.EQUAL: comparison == 0,
        FilterOperator.NOT_EQUAL: comparison != 0,
        FilterOperator.LESS: comparison < 0,
        FilterOperator.LESS_EQUAL: comparison <= 0,
        FilterOperator.GREATER: comparison > 0,
        FilterOperator.GREATER_EQUAL: comparison >= 0,
    }[spec.operator]


def _format(cell: Cell) -> str:
    if cell.value is None:
        return ""
    if isinstance(cell.value, datetime):
        return cell.value.astimezone(UTC).isoformat().replace("+00:00", "Z")
    if isinstance(cell.value, bool):
        return str(cell.value).lower()
    return _escape(str(cell.value))


def _escape(value: str) -> str:
    return value.replace("\t", " ").replace("\r", " ").replace("\n", " ")


__all__ = [
    "Cell",
    "CellKind",
    "Column",
    "Dataset",
    "FilterOperator",
    "FilterSpec",
    "NullOrder",
    "Row",
    "Selection",
    "SortDirection",
    "SortSpec",
    "TableError",
    "TableWindow",
    "compute_window",
    "copy_selection",
    "filter_dataset",
    "resize_column",
    "sort_dataset",
]
