"""Small helpers shared by passes/user/ai modules (Step 2 fill-in)."""

from datetime import datetime

from src.utils.constant import OFF_HOUR_END, OFF_HOUR_START


def is_off_hours(dt: datetime, start: int = OFF_HOUR_START, end: int = OFF_HOUR_END) -> bool:
    """True if naive-UTC datetime falls in the off-hours window [start, 24) U [0, end)."""
    hour = dt.hour
    if start <= end:
        return start <= hour < end
    return hour >= start or hour < end


def hours_between(a: datetime, b: datetime) -> float:
    """Absolute difference between two datetimes in hours."""
    return abs((a - b).total_seconds()) / 3600.0


def safe_limit(n: int | None, default: int = 20, maximum: int = 200) -> int:
    """Clamp a user-supplied limit into [1, maximum]."""
    try:
        value = int(n) if n is not None else default
    except (TypeError, ValueError):
        return default
    return max(1, min(value, maximum))
