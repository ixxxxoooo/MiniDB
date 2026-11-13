#!/usr/bin/env python3
"""生成 MiniDB 的应用图标（扁平化、符合 macOS Big Sur 之后的图标规范）。

用法：python3 scripts/gen_icon.py
产物：
  resources/icon.png        1024x1024，图标主体 824x824 居中，四周留出系统规定的边距，带一层很淡的投影
  internal/ui/logo.png      256x256，界面里（关于页、空状态）使用的同款图标
"""
import math
from PIL import Image, ImageChops, ImageDraw, ImageFilter

S = 4  # 超采样倍数，保证边缘平滑
SIZE = 1024
BODY = 824  # macOS 图标主体的边长
MARGIN = (SIZE - BODY) // 2

TOP_COLOR = (62, 142, 255)  # 渐变顶部
BOTTOM_COLOR = (16, 98, 232)  # 渐变底部
PAPER = (255, 255, 255)


def squircle_mask(size, n=5.0):
    """超椭圆（比圆角矩形更接近 macOS 的连续曲率）蒙版。"""
    big = size * S
    pts = []
    r = big / 2
    steps = 720
    for i in range(steps):
        t = 2 * math.pi * i / steps
        c, s = math.cos(t), math.sin(t)
        x = r + r * math.copysign(abs(c) ** (2 / n), c)
        y = r + r * math.copysign(abs(s) ** (2 / n), s)
        pts.append((x, y))
    m = Image.new("L", (big, big), 0)
    ImageDraw.Draw(m).polygon(pts, fill=255)
    return m


def gradient(size):
    g = Image.new("RGB", (size, size))
    d = ImageDraw.Draw(g)
    for y in range(size):
        k = y / (size - 1)
        d.line([(0, y), (size, y)], fill=tuple(round(a + (b - a) * k) for a, b in zip(TOP_COLOR, BOTTOM_COLOR)))
    return g


def disc(draw, cx, cy, rx, ry, h, top_fill, side_fill, edge):
    """一片圆盘：椭圆顶面 + 侧面；edge 是缝宽：先把比实体大一圈的区域擦成透明，
    再画实体，盘与盘之间就留出一条透出背景渐变的细缝。"""
    grow, clear = edge, (0, 0, 0, 0)
    draw.ellipse([cx - rx - grow, cy + h - ry - grow, cx + rx + grow, cy + h + ry + grow], fill=clear)
    draw.rectangle([cx - rx - grow, cy - grow, cx + rx + grow, cy + h], fill=clear)
    draw.ellipse([cx - rx - grow, cy - ry - grow, cx + rx + grow, cy + ry + grow], fill=clear)
    draw.ellipse([cx - rx, cy + h - ry, cx + rx, cy + h + ry], fill=side_fill)
    draw.rectangle([cx - rx, cy, cx + rx, cy + h], fill=side_fill)
    draw.ellipse([cx - rx, cy - ry, cx + rx, cy + ry], fill=top_fill)


def render_body():
    """图标主体（824x824，带透明圆角）。"""
    big = BODY * S
    base = gradient(big)
    glyph = Image.new("RGBA", (big, big), (0, 0, 0, 0))
    d = ImageDraw.Draw(glyph)

    k = S  # 以 1024 画布的坐标设计，1 个单位 = S 个超采样像素
    rx, ry, h = 240 * k, 68 * k, 92 * k
    cx = big / 2
    gap = 14 * k
    # 从下往上画三片：越靠上越白，扁平的"层次"全靠颜色深浅
    tops = [(214, 232, 255), (236, 244, 255), PAPER]
    sides = [(188, 214, 252), (214, 230, 255), (240, 246, 255)]
    ys = [594, 466, 338]  # 各片顶面中心 y（1024 画布坐标，已相对主体居中）
    off = -MARGIN * k  # 1024 画布坐标 → 主体坐标
    for top, side, y in zip(tops, sides, ys):
        yy = y * k + off
        disc(d, cx, yy, rx, ry, h, top, side, gap)

    img = base.convert("RGBA")
    img.alpha_composite(glyph)
    img.putalpha(squircle_mask(BODY))
    return img.resize((BODY, BODY), Image.LANCZOS)


def main():
    body = render_body()
    canvas = Image.new("RGBA", (SIZE, SIZE), (0, 0, 0, 0))
    # 系统规定的淡投影：向下偏 10px，模糊 18px，25% 黑
    shadow = Image.new("RGBA", (SIZE, SIZE), (0, 0, 0, 0))
    alpha = Image.new("L", (SIZE, SIZE), 0)
    alpha.paste(body.getchannel("A").point(lambda v: int(v * 0.25)), (MARGIN, MARGIN + 10))
    shadow.putalpha(alpha.filter(ImageFilter.GaussianBlur(18)))
    canvas.alpha_composite(shadow)
    canvas.alpha_composite(body, (MARGIN, MARGIN))
    canvas.save("resources/icon.png")
    # 界面里用的小图：不带多余边距，直接用主体
    body.resize((256, 256), Image.LANCZOS).save("internal/ui/logo.png")
    print("ok")


if __name__ == "__main__":
    main()
