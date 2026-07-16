from __future__ import annotations

from collections.abc import Callable
from typing import Protocol

from corthena.ui.table import CellKind, Column, compute_window
from corthena.ui.visualization import Sample, aggregate_continuous


class Benchmark(Protocol):
    """Typed surface used from pytest-benchmark's fixture."""

    def __call__[**Parameters, Result](
        self,
        target: Callable[Parameters, Result],
        *args: Parameters.args,
        **kwargs: Parameters.kwargs,
    ) -> Result: ...


def test_continuous_lod_benchmark(benchmark: Benchmark) -> None:
    """Record representative 250k-source/1920-pixel LOD timing and work."""
    samples = tuple(Sample(index, index / 10, float(index % 101)) for index in range(250_000))
    result = benchmark(aggregate_continuous, samples, 0.0, 25_000.0, 1920)
    output, stats = result
    assert len(output) <= 1920 * 4
    assert stats.output_values <= 1920 * 4


def test_ten_million_row_window_benchmark(benchmark: Benchmark) -> None:
    """Prove virtual-window work is independent of a ten-million-row count."""
    columns = tuple(
        Column(f"c{index}", f"Column {index}", CellKind.FLOAT, 100, pinned=index == 0)
        for index in range(40)
    )
    result = benchmark(
        compute_window,
        columns,
        10_000_000,
        1920,
        900,
        1200,
        100_000_000,
        20,
        row_overscan=2,
        column_overscan=1,
    )
    assert result.cell_work <= 49 * 23
