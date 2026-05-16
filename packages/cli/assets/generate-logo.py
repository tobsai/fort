#!/usr/bin/env python3
"""
Generate the Fort CLI banner PNG.

Run after editing this file to refresh fort-logo.png:
    python3 packages/cli/assets/generate-logo.py
"""
from PIL import Image, ImageDraw, ImageFont
from pathlib import Path

OUT = Path(__file__).parent / "fort-logo.png"

# 2x retina; iTerm/Kitty will scale down to ~7 rows when rendered with width cells
W, H = 760, 260
PURPLE = (138, 100, 255, 255)
PURPLE_DEEP = (96, 70, 200, 255)
CYAN = (88, 220, 232, 255)
TAGLINE = (140, 140, 160, 255)

img = Image.new("RGBA", (W, H), (0, 0, 0, 0))
d = ImageDraw.Draw(img)

# Geometry — tighter, more balanced
crown_w = 220
crown_left = (W - crown_w) // 2
crown_right = crown_left + crown_w
crown_top = 16
base_top = 60
base_bottom = 100

# Solid fortress base
d.rectangle([crown_left, base_top, crown_right, base_bottom], fill=PURPLE)

# 4 merlons on top
slot = crown_w // 7
for i in range(4):
    x0 = crown_left + i * 2 * slot
    x1 = x0 + slot
    d.rectangle([x0, crown_top, x1, base_top], fill=PURPLE)

# Cyan accent line under the crown
d.rectangle([crown_left, base_bottom + 6, crown_right, base_bottom + 10], fill=CYAN)

# Wordmark — bold purple, large, centered
try:
    font_path = "/System/Library/Fonts/Supplemental/Futura.ttc"
    wordmark = ImageFont.truetype(font_path, 92, index=2)  # bold (0=medium, 1=italic, 2=bold)
    tag_font = ImageFont.truetype(font_path, 22, index=0)
except Exception:
    wordmark = ImageFont.load_default()
    tag_font = ImageFont.load_default()

word = "FORT"
bb = d.textbbox((0, 0), word, font=wordmark)
tw, th = bb[2] - bb[0], bb[3] - bb[1]
wx = (W - tw) // 2 - bb[0]
wy = base_bottom + 24 - bb[1]
d.text((wx, wy), word, font=wordmark, fill=PURPLE_DEEP)

# Tagline — leave clear vertical space below the wordmark
tag = "A self-improving AI agent platform"
tb = d.textbbox((0, 0), tag, font=tag_font)
ttw = tb[2] - tb[0]
ttx = (W - ttw) // 2 - tb[0]
tty = wy + th + 40 - tb[1]
d.text((ttx, tty), tag, font=tag_font, fill=TAGLINE)

img.save(OUT, "PNG", optimize=True)
print(f"Wrote {OUT} ({OUT.stat().st_size} bytes, {W}x{H})")
