#!/usr/bin/env python3
"""Make Fort's app icon from the README logo art (assets/fort-logo.png).

    python3 -m pip install pillow
    python3 successor/design/icon/render.py

Crops a square around the keep and the FORT sign, then writes the iPhone icon,
the Mac icon at every size in macOS's rounded-square shape with its shadow,
assets/fort-icon.png and preview.png. Change the crop here and run it again;
never hand-edit the PNGs.
"""
import math
from pathlib import Path

from PIL import Image, ImageDraw, ImageFilter

HERE = Path(__file__).resolve().parent
REPO = HERE.parents[2]
SOURCE = REPO / "assets/fort-logo.png"
ICONSET = REPO / "successor/apple/Apps/Assets.xcassets/AppIcon.appiconset"
README_ICON = REPO / "assets/fort-icon.png"
PREVIEW = HERE / "preview.png"

# The square is the art's full height, centred on the gate.
CENTRE_X = 0.5


def art(size: int) -> Image.Image:
    logo = Image.open(SOURCE).convert("RGB")
    w, h = logo.size
    left = round(w * CENTRE_X - h / 2)
    return logo.crop((left, 0, left + h, h)).resize((size, size), Image.LANCZOS)


def squircle_mask(size: int, n: float = 5.0) -> Image.Image:
    """macOS app shape: a superellipse, close to Apple's continuous-corner rounded square."""
    big = size * 4
    r = big / 2
    pts = []
    for i in range(720):
        t = 2 * math.pi * i / 720
        c, s = math.cos(t), math.sin(t)
        pts.append((r + r * abs(c) ** (2 / n) * math.copysign(1, c), r + r * abs(s) ** (2 / n) * math.copysign(1, s)))
    mask = Image.new("L", (big, big), 0)
    ImageDraw.Draw(mask).polygon(pts, fill=255)
    return mask.resize((size, size), Image.LANCZOS)


def mac_icon() -> Image.Image:
    # Apple's macOS grid: 1024 canvas, 824 shape centred, soft shadow below.
    canvas, shape, offset = 1024, 824, 100
    face = art(shape).convert("RGBA")
    face.putalpha(squircle_mask(shape))
    out = Image.new("RGBA", (canvas, canvas), (0, 0, 0, 0))
    shadow_alpha = Image.new("L", (canvas, canvas), 0)
    shadow_alpha.paste(squircle_mask(shape), (offset, offset + 12))
    shadow_alpha = shadow_alpha.filter(ImageFilter.GaussianBlur(14)).point(lambda a: int(a * 0.45))
    shadow = Image.new("RGBA", (canvas, canvas), (0, 0, 0, 255))
    shadow.putalpha(shadow_alpha)
    out.alpha_composite(shadow)
    out.alpha_composite(face, (offset, offset))
    return out


def main() -> None:
    for old in ICONSET.glob("*.png"):
        old.unlink()
    ios = art(1024)  # opaque RGB, as the App Store requires
    ios.save(ICONSET / "icon-ios-1024.png", optimize=True)
    ios.save(README_ICON, optimize=True)
    mac = mac_icon()
    for px in (16, 32, 64, 128, 256, 512, 1024):
        mac.resize((px, px), Image.LANCZOS).save(ICONSET / f"icon-mac-{px}.png", optimize=True)
    preview(ios, mac)


def preview(ios: Image.Image, mac: Image.Image) -> None:
    tile, pad = 300, 40
    sheet = Image.new("RGB", (pad + 3 * (tile + pad), tile + 2 * pad + 60), (232, 236, 241))
    mask = Image.new("L", (tile, tile), 0)
    ImageDraw.Draw(mask).rounded_rectangle((0, 0, tile - 1, tile - 1), radius=int(tile * 0.2237), fill=255)
    sheet.paste(ios.resize((tile, tile), Image.LANCZOS), (pad, pad), mask)
    m = mac.resize((tile, tile), Image.LANCZOS)
    sheet.paste(m, (2 * pad + tile, pad), m)
    x = 3 * pad + 2 * tile
    for px in (128, 64, 32, 16):  # Dock, Spotlight, Finder list, menu
        small = mac.resize((px, px), Image.LANCZOS)
        sheet.paste(small, (x, pad + tile - px), small)
        x += px + 10
    from PIL import ImageFont
    try:
        font = ImageFont.load_default(size=22)
    except TypeError:
        font = ImageFont.load_default()
    d = ImageDraw.Draw(sheet)
    for i, label in enumerate(["iPhone", "Mac", "Mac small sizes"]):
        d.text((pad + i * (tile + pad), pad + tile + 24), label, fill=(14, 18, 24), font=font)
    sheet.save(PREVIEW, optimize=True)


if __name__ == "__main__":
    main()
