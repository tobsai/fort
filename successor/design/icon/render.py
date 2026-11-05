#!/usr/bin/env python3
"""Render Fort's app icon from the one master SVG into every place that uses it.

    python3 -m pip install cairosvg pillow
    python3 successor/design/icon/render.py

Writes the iPhone icon (default, dark and tinted), the Mac icon at every size in
macOS's rounded-square shape with its drop shadow, the web icon, and preview.png.
Edit fort-icon.svg, then run this again; never hand-edit the PNGs.
"""
import io
import re
from pathlib import Path

import cairosvg
from PIL import Image, ImageChops, ImageDraw, ImageFilter, ImageOps

HERE = Path(__file__).resolve().parent
REPO = HERE.parents[2]
MASTER = HERE / "fort-icon.svg"
ICONSET = REPO / "successor/apple/Apps/Assets.xcassets/AppIcon.appiconset"
WEB_ICON = REPO / "successor/web/icon.svg"
PREVIEW = HERE / "preview.png"

SVG = MASTER.read_text()
NO_BG = re.sub(r'<g id="bg">.*?</g>', "", SVG, flags=re.S)
NO_HALFTONE = re.sub(r'<g id="halftone".*?</g>', "", SVG, flags=re.S)


def render(svg: str, size: int) -> Image.Image:
    png = cairosvg.svg2png(bytestring=svg.encode(), output_width=size, output_height=size)
    return Image.open(io.BytesIO(png)).convert("RGBA")


def squircle_mask(size: int, n: float = 5.0) -> Image.Image:
    """macOS app shape: a superellipse, close to Apple's continuous-corner rounded square."""
    scale = 4
    big = size * scale
    mask = Image.new("L", (big, big), 0)
    r = big / 2
    pts = []
    for i in range(720):
        import math
        t = 2 * math.pi * i / 720
        c, s = math.cos(t), math.sin(t)
        x = r + r * (abs(c) ** (2 / n)) * (1 if c >= 0 else -1)
        y = r + r * (abs(s) ** (2 / n)) * (1 if s >= 0 else -1)
        pts.append((x, y))
    ImageDraw.Draw(mask).polygon(pts, fill=255)
    return mask.resize((size, size), Image.LANCZOS)


def mac_icon() -> Image.Image:
    # Apple's macOS grid: 1024 canvas, 824 shape centred, soft shadow below.
    canvas, shape, offset = 1024, 824, 100
    art = render(SVG, shape)
    art.putalpha(squircle_mask(shape))
    out = Image.new("RGBA", (canvas, canvas), (0, 0, 0, 0))
    shadow_alpha = Image.new("L", (canvas, canvas), 0)
    shadow_alpha.paste(squircle_mask(shape), (offset, offset + 12))
    shadow_alpha = shadow_alpha.filter(ImageFilter.GaussianBlur(14)).point(lambda a: int(a * 0.45))
    shadow = Image.new("RGBA", (canvas, canvas), (0, 0, 0, 255))
    shadow.putalpha(shadow_alpha)
    out.alpha_composite(shadow)
    out.alpha_composite(art, (offset, offset))
    return out


def main() -> None:
    ICONSET.mkdir(parents=True, exist_ok=True)
    for old in ICONSET.glob("*.png"):
        old.unlink()

    ios = render(SVG, 1024).convert("RGB")  # App Store icons must be opaque
    ios.save(ICONSET / "icon-ios-1024.png", optimize=True)

    dark = render(NO_BG, 1024)  # iOS draws its own dark backdrop behind this
    dark.save(ICONSET / "icon-ios-1024-dark.png", optimize=True)

    black = Image.new("RGBA", (1024, 1024), (0, 0, 0, 255))
    black.alpha_composite(render(NO_BG, 1024))
    tinted = ImageOps.grayscale(black.convert("RGB"))
    tinted.convert("RGB").save(ICONSET / "icon-ios-1024-tinted.png", optimize=True)

    mac = mac_icon()
    for px in (16, 32, 64, 128, 256, 512, 1024):
        mac.resize((px, px), Image.LANCZOS).save(ICONSET / f"icon-mac-{px}.png", optimize=True)

    # Web: same art, no halftone (it's noise at favicon size), rounded like the old icon.
    web = NO_HALFTONE.replace(
        '<rect width="1024" height="1024" fill="url(#sky)"/>',
        '<rect width="1024" height="1024" rx="224" fill="url(#sky)"/>',
    )
    web = re.sub(r"<!--.*?-->", "", web, flags=re.S)
    web = web.replace('<defs>', '<defs><clipPath id="round"><rect width="1024" height="1024" rx="224"/></clipPath>', 1)
    web = web.replace("</defs>", '</defs>\n  <g clip-path="url(#round)">', 1).replace("</svg>", "  </g>\n</svg>")
    web = re.sub(r"\n\s*\n", "\n", web)
    WEB_ICON.write_text(web)

    make_preview(ios, dark, tinted, mac)


def make_preview(ios, dark, tinted, mac) -> None:
    tile, pad = 300, 40
    w = pad + 5 * (tile + pad)
    h = tile + 2 * pad + 60
    sheet = Image.new("RGB", (w, h), (232, 236, 241))
    mask = Image.new("L", (tile, tile), 0)
    ImageDraw.Draw(mask).rounded_rectangle((0, 0, tile - 1, tile - 1), radius=int(tile * 0.2237), fill=255)

    def ios_tile(img, backdrop):
        bg = Image.new("RGBA", (tile, tile), backdrop)
        bg.alpha_composite(img.convert("RGBA").resize((tile, tile), Image.LANCZOS))
        return bg

    tiles = [
        ios_tile(ios, (0, 0, 0, 255)),
        ios_tile(dark, (28, 28, 30, 255)),
        ImageChops.multiply(ios_tile(tinted, (0, 0, 0, 255)).convert("RGB"), Image.new("RGB", (tile, tile), (120, 200, 255))).convert("RGBA"),
    ]
    x = pad
    for t in tiles:
        sheet.paste(t.convert("RGB"), (x, pad), mask)
        x += tile + pad
    sheet.paste(mac.resize((tile, tile), Image.LANCZOS), (x, pad), mac.resize((tile, tile), Image.LANCZOS))
    x += tile + pad
    # small sizes, where most people actually see it: dock-small, Spotlight, menu, Finder list
    sx = x
    for px in (128, 64, 32, 16):
        small = mac.resize((px, px), Image.LANCZOS)
        sheet.paste(small, (sx, pad + tile - px), small)
        sx += px + 10
    try:
        from PIL import ImageFont
        font = ImageFont.load_default(size=22)
    except Exception:
        font = None
    d = ImageDraw.Draw(sheet)
    for i, label in enumerate(["iPhone", "iPhone dark", "iPhone tinted", "Mac", "Mac small sizes"]):
        d.text((pad + i * (tile + pad), pad + tile + 30), label, fill=(14, 18, 24), font=font)
    sheet.save(PREVIEW, optimize=True)


if __name__ == "__main__":
    main()
