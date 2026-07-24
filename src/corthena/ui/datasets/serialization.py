"""Strict codecs for the closed built-in feature-step union."""

from __future__ import annotations

from typing import TypeIs

from corthena.ui.datasets.models import (
    CrossSectionalMethod,
    CrossSectionalStep,
    FeatureStep,
    LaggedReturnStep,
    PriceVolumeRatioStep,
    RollingRangeStep,
    RollingStatistic,
    RollingStatisticStep,
    RollingVolatilityStep,
)

JsonScalar = str | int | float | bool | None
JsonValue = JsonScalar | list["JsonValue"] | dict[str, "JsonValue"]


def encode_feature_step(step: FeatureStep) -> dict[str, JsonValue]:
    """Encode one closed-union step without executable or free-form content."""
    if isinstance(step, LaggedReturnStep):
        return {
            "kind": step.kind,
            "output_name": step.output_name,
            "input_column": step.input_column,
            "periods": step.periods,
            "log_return": step.log_return,
        }
    if isinstance(step, RollingStatisticStep):
        return {
            "kind": step.kind,
            "output_name": step.output_name,
            "input_column": step.input_column,
            "window": step.window,
            "statistic": step.statistic.value,
        }
    if isinstance(step, PriceVolumeRatioStep):
        return {
            "kind": step.kind,
            "output_name": step.output_name,
            "price_column": step.price_column,
            "volume_column": step.volume_column,
        }
    if isinstance(step, RollingVolatilityStep):
        return {
            "kind": step.kind,
            "output_name": step.output_name,
            "input_column": step.input_column,
            "window": step.window,
        }
    if isinstance(step, RollingRangeStep):
        return {
            "kind": step.kind,
            "output_name": step.output_name,
            "high_column": step.high_column,
            "low_column": step.low_column,
            "window": step.window,
        }
    return {
        "kind": step.kind,
        "output_name": step.output_name,
        "input_column": step.input_column,
        "method": step.method.value,
        "timestamp_column": step.timestamp_column,
        "symbol_column": step.symbol_column,
    }


def decode_feature_step(value: object) -> FeatureStep:
    """Decode one step and reject unknown kinds or partially migrated fields."""
    if not _is_object(value):
        raise ValueError("feature step must be an object")
    kind = _string(value, "kind")
    if kind == "lagged_return":
        _fields(value, {"kind", "output_name", "input_column", "periods", "log_return"})
        return LaggedReturnStep(
            _string(value, "output_name"),
            _string(value, "input_column"),
            _integer(value, "periods"),
            _boolean(value, "log_return"),
        )
    if kind == "rolling_statistic":
        _fields(value, {"kind", "output_name", "input_column", "window", "statistic"})
        return RollingStatisticStep(
            _string(value, "output_name"),
            _string(value, "input_column"),
            _integer(value, "window"),
            RollingStatistic(_string(value, "statistic")),
        )
    if kind == "price_volume_ratio":
        _fields(value, {"kind", "output_name", "price_column", "volume_column"})
        return PriceVolumeRatioStep(
            _string(value, "output_name"),
            _string(value, "price_column"),
            _string(value, "volume_column"),
        )
    if kind == "rolling_volatility":
        _fields(value, {"kind", "output_name", "input_column", "window"})
        return RollingVolatilityStep(
            _string(value, "output_name"),
            _string(value, "input_column"),
            _integer(value, "window"),
        )
    if kind == "rolling_range":
        _fields(value, {"kind", "output_name", "high_column", "low_column", "window"})
        return RollingRangeStep(
            _string(value, "output_name"),
            _string(value, "high_column"),
            _string(value, "low_column"),
            _integer(value, "window"),
        )
    if kind == "cross_sectional":
        _fields(
            value,
            {
                "kind",
                "output_name",
                "input_column",
                "method",
                "timestamp_column",
                "symbol_column",
            },
        )
        return CrossSectionalStep(
            _string(value, "output_name"),
            _string(value, "input_column"),
            CrossSectionalMethod(_string(value, "method")),
            _string(value, "timestamp_column"),
            _string(value, "symbol_column"),
        )
    raise ValueError(f"unknown feature step kind {kind!r}")


def _is_object(value: object) -> TypeIs[dict[str, object]]:
    return _is_dict(value) and all(isinstance(key, str) for key in value)


def _is_dict(value: object) -> TypeIs[dict[object, object]]:
    return isinstance(value, dict)


def _fields(record: dict[str, object], expected: set[str]) -> None:
    if set(record) != expected:
        raise ValueError("feature step fields are incompatible with its kind")


def _string(record: dict[str, object], field: str) -> str:
    value = record.get(field)
    if not isinstance(value, str):
        raise ValueError(f"{field} must be a string")
    return value


def _integer(record: dict[str, object], field: str) -> int:
    value = record.get(field)
    if not isinstance(value, int) or isinstance(value, bool):
        raise ValueError(f"{field} must be an integer")
    return value


def _boolean(record: dict[str, object], field: str) -> bool:
    value = record.get(field)
    if not isinstance(value, bool):
        raise ValueError(f"{field} must be a boolean")
    return value
