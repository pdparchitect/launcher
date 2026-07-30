"""Time-of-day selection for the Petbox habitat wallpaper."""

from datetime import datetime
from pathlib import Path

PERIODS = ("dawn", "day", "dusk", "night")
WALLPAPER_DIRECTORY = Path("/usr/share/backgrounds/petbox")


def period_for_hour(hour):
    if isinstance(hour, bool) or not isinstance(hour, int) or not 0 <= hour <= 23:
        raise ValueError(f"hour must be an integer from 0 through 23, got {hour!r}")
    if 5 <= hour < 8:
        return "dawn"
    if 8 <= hour < 17:
        return "day"
    if 17 <= hour < 20:
        return "dusk"
    return "night"


def current_period(now=None, override=None):
    if override:
        if override not in PERIODS:
            raise ValueError(
                f"unknown wallpaper period '{override}'; "
                f"choose from {', '.join(PERIODS)}"
            )
        return override
    return period_for_hour((now or datetime.now()).hour)


def path_for(period, directory=None):
    if period not in PERIODS:
        raise ValueError(f"unknown wallpaper period '{period}'")
    return Path(directory or WALLPAPER_DIRECTORY) / f"{period}.svg"
