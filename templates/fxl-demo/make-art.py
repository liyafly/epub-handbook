#!/usr/bin/env python3
"""Regenerate the self-made fixed-layout demo images (stdlib only).

Writes OEBPS/Images/*.png. Every row of each image is identical (vertical
bands, horizontal gradients), so the files stay a few KB. Run it only when
the art changes; build.sh does not call it.
"""

from pathlib import Path
import struct
import zlib

W, H = 768, 1024
OUT = Path(__file__).resolve().parent / "OEBPS" / "Images"


def png(path: Path, width: int, row: bytes, height: int) -> None:
    raw = (b"\x00" + row) * height
    def chunk(tag: bytes, data: bytes) -> bytes:
        return struct.pack(">I", len(data)) + tag + data + struct.pack(">I", zlib.crc32(tag + data) & 0xFFFFFFFF)
    data = b"\x89PNG\r\n\x1a\n" + chunk(b"IHDR", struct.pack(">IIBBBBB", width, height, 8, 2, 0, 0, 0))
    data += chunk(b"IDAT", zlib.compress(raw, 9)) + chunk(b"IEND", b"")
    path.write_bytes(data)


def gradient(width: int, a: tuple, b: tuple) -> bytes:
    out = bytearray()
    for x in range(width):
        t = x / (width - 1)
        out += bytes(round(a[i] + (b[i] - a[i]) * t) for i in range(3))
    return bytes(out)


OUT.mkdir(parents=True, exist_ok=True)
png(OUT / "cover.png", W, gradient(W, (40, 60, 90), (90, 120, 150)), H)
png(OUT / "scene.png", W, gradient(W, (235, 225, 200), (200, 215, 225)), H)
spread = gradient(2 * W, (30, 90, 80), (220, 200, 140))
png(OUT / "spread-left.png", W, spread[: 3 * W], H)
png(OUT / "spread-right.png", W, spread[3 * W :], H)
print("wrote", ", ".join(sorted(p.name for p in OUT.glob("*.png"))))
