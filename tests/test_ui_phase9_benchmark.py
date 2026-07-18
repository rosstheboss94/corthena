from collections.abc import Callable
from datetime import UTC, datetime
from threading import Event
from typing import Protocol

from corthena.ui.models_inference.deterministic import ModelsInferenceDemo
from corthena.ui.models_inference.models import InferenceMode, InferenceQuery


class Benchmark(Protocol):
    def __call__[**Parameters, Result](
        self,
        target: Callable[Parameters, Result],
        *args: Parameters.args,
        **kwargs: Parameters.kwargs,
    ) -> Result: ...


def test_phase9_scoring_benchmark(benchmark: Benchmark) -> None:
    query = InferenceQuery(
        "phase9-benchmark",
        1,
        "champion",
        "dataset-us-equities",
        "dataset-fingerprint-phase9",
        InferenceMode.LATEST,
    )
    benchmark(ModelsInferenceDemo(309, datetime(2026, 7, 10, 12, tzinfo=UTC)).score, query, Event())
