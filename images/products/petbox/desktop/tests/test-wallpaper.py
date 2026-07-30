#!/usr/bin/env python3

import unittest
import xml.etree.ElementTree as ElementTree
from pathlib import Path

import petwallpaper

PRODUCT_DIRECTORY = Path(__file__).resolve().parents[1]
WALLPAPER_DIRECTORY = (
    PRODUCT_DIRECTORY
    / "overlay/usr/share/backgrounds/petbox"
)


class WallpaperTests(unittest.TestCase):
    def test_periods_follow_local_clock(self):
        expected = {
            0: "night",
            4: "night",
            5: "dawn",
            7: "dawn",
            8: "day",
            16: "day",
            17: "dusk",
            19: "dusk",
            20: "night",
            23: "night",
        }
        for hour, period in expected.items():
            with self.subTest(hour=hour):
                self.assertEqual(petwallpaper.period_for_hour(hour), period)

    def test_invalid_hours_are_rejected(self):
        for hour in (-1, 24, "noon"):
            with self.subTest(hour=hour):
                with self.assertRaises(ValueError):
                    petwallpaper.period_for_hour(hour)

    def test_override_is_available_for_previewing_each_period(self):
        self.assertEqual(
            petwallpaper.current_period(override="dusk"),
            "dusk",
        )
        with self.assertRaises(ValueError):
            petwallpaper.current_period(override="lunchtime")

    def test_every_period_has_a_distinct_valid_svg(self):
        contents = []
        for period in petwallpaper.PERIODS:
            path = WALLPAPER_DIRECTORY / f"{period}.svg"
            ElementTree.parse(path)
            contents.append(path.read_text())
        self.assertEqual(len(set(contents)), len(petwallpaper.PERIODS))


if __name__ == "__main__":
    unittest.main()
