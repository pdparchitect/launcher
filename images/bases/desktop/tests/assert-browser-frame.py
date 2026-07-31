#!/usr/bin/env python3
"""Assert that the browser smoke-test PPM contains its bright-green page."""

from pathlib import Path
import sys


def read_token(data: bytes, offset: int) -> tuple[bytes, int]:
    while offset < len(data):
        if data[offset : offset + 1] == b"#":
            offset = data.index(b"\n", offset) + 1
        elif data[offset : offset + 1].isspace():
            offset += 1
        else:
            break

    end = offset
    while end < len(data) and not data[end : end + 1].isspace():
        end += 1

    return data[offset:end], end


def main() -> int:
    data = Path(sys.argv[1]).read_bytes()
    offset = 0
    tokens = []
    for _ in range(4):
        token, offset = read_token(data, offset)
        tokens.append(token)

    magic, width_text, height_text, maximum_text = tokens
    if magic != b"P6" or maximum_text != b"255":
        return 1

    width = int(width_text)
    height = int(height_text)
    while offset < len(data) and data[offset : offset + 1].isspace():
        offset += 1
    pixels = data[offset:]
    if len(pixels) != width * height * 3:
        return 1

    green_pixels = sum(
        1
        for index in range(0, len(pixels), 3)
        if pixels[index] < 64
        and pixels[index + 1] > 192
        and pixels[index + 2] < 64
    )
    return 0 if green_pixels > width * height // 2 else 1


if __name__ == "__main__":
    raise SystemExit(main())
