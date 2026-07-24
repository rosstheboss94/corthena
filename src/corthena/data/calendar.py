"""Typed XNYS session adapter backed by exchange_calendars."""

# pyright: reportMissingTypeStubs=false, reportUnknownMemberType=false

from __future__ import annotations

from datetime import UTC, date, datetime, timedelta

import exchange_calendars

from corthena.contracts.data import Interval
from corthena.data.types import CanonicalBar, SessionWindow

_DURATION = {
    Interval.MINUTE_1: timedelta(minutes=1),
    Interval.MINUTE_5: timedelta(minutes=5),
    Interval.MINUTE_15: timedelta(minutes=15),
    Interval.HOUR_1: timedelta(hours=1),
    Interval.DAY_1: timedelta(days=1),
}


class XnysCalendar:
    """US-equity sessions including holidays, DST, and early closes."""

    def __init__(self) -> None:
        self._calendar = exchange_calendars.get_calendar("XNYS")

    @property
    def version(self) -> str:
        return f"exchange_calendars:{exchange_calendars.__version__}:XNYS"

    def session(self, value: date) -> SessionWindow | None:
        label = value.isoformat()
        if not bool(self._calendar.is_session(label)):
            return None
        opened = self._calendar.session_open(label).to_pydatetime().astimezone(UTC)
        closed = self._calendar.session_close(label).to_pydatetime().astimezone(UTC)
        label_at = datetime(value.year, value.month, value.day, tzinfo=UTC)
        return SessionWindow(label_at, opened, closed)

    def is_regular_bar(self, bar: CanonicalBar, interval: Interval) -> bool:
        window = self.session(bar.timestamp.astimezone(UTC).date())
        if window is None:
            return False
        if interval is Interval.DAY_1:
            return True
        return (
            window.open_at <= bar.timestamp
            and bar.timestamp + _DURATION[interval] <= window.close_at
        )

    def last_completed_bar_start(self, interval: Interval, now: datetime) -> datetime:
        if now.tzinfo is None or now.utcoffset() != timedelta(0):
            raise ValueError("completion clock must be UTC")
        duration = _DURATION[interval]
        if interval is Interval.DAY_1:
            cursor = now.date()
            for _ in range(16):
                window = self.session(cursor)
                if window is not None and window.close_at <= now:
                    return window.session_date
                cursor -= timedelta(days=1)
            raise RuntimeError("no completed XNYS session within bounded lookback")
        epoch = int(now.timestamp())
        seconds = int(duration.total_seconds())
        completed = (epoch // seconds) * seconds - seconds
        return datetime.fromtimestamp(completed, UTC)
