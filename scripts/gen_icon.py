#!/usr/bin/env python3
"""生成 MiniDB 的应用图标：Peak（山峰 M）。

设计稿见 docs/design/v2（Logo 页）：靛蓝到紫的对角渐变圆角方块，里面一个白色的 M，
两道山峰像字母，也像数据的起伏；底下两道渐隐的短线是叠起来的「数据页」。
矢量版本在 docs/design/v2/logo/peak-*.svg，这里按同一组坐标栅格化。

用法：python3 scripts/gen_icon.py
产物：
  resources/icon.png        1024x1024，图标主体 824x824 居中，四周留出系统规定的边距，带一层很淡的投影
  internal/ui/logo.png      256x256，界面里（关于页、欢迎页、空状态）使用的同款图标
"""
import math
from PIL import Image, ImageDraw, ImageFilter

S = 4  # 超采样倍数，保证边缘平滑
SIZE = 1024
BODY = 824  # macOS 图标主体的边长
MARGIN = (SIZE - BODY) // 2

START = (91, 93, 240)  # #5b5df0 渐变起点（左上）
END = (139, 92, 246)  # #8b5cf6 渐变终点（右下）
WHITE = (255, 255, 255)

# 以下坐标取自设计稿 SVG（viewBox 64，图标主体占 1..63，共 62 个单位）。
VIEW_ORIGIN, VIEW_SPAN = 1.0, 62.0
PEAK = [(17, 41), (17, 18), (32, 33), (47, 18), (47, 41)]  # M 的折线，线宽 5.2
BARS = [  # (起点, 终点, 线宽, 不透明度)
    ((20, 50), (44, 50), 3.2, 0.60),
    ((26, 55.5), (38, 55.5), 3.2, 0.32),
]


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


def diagonal_gradient(size):
    """左上 → 右下的对角渐变。渐变是平滑的，先在小图上算再放大，省得逐像素循环几千万次。"""
    n = 256
    g = Image.new("RGB", (n, n))
    px = g.load()
    for y in range(n):
        for x in range(n):
            k = (x + y) / (2 * (n - 1))
            px[x, y] = tuple(round(a + (b - a) * k) for a, b in zip(START, END))
    return g.resize((size, size), Image.BICUBIC)


def top_sheen(size):
    """顶部半幅的白色高光（38% → 0），让方块有一点立体感。"""
    half = size // 2
    ramp = Image.linear_gradient("L").rotate(180).resize((size, half))  # 255 → 0，自上而下
    m = Image.new("L", (size, size), 0)
    m.paste(ramp.point(lambda v: int(255 * 0.38 * (v / 255) ** 2)), (0, 0))  # 平方衰减，避免中线处的明暗断层
    return m


def stroke(layer, pts, width, alpha, scale):
    """在 layer 上画一条圆头圆角的白色折线。每段画成一个矩形、顶点补圆，接缝才干净；
    透明度单独走一层蒙版再合成，重叠处不会叠深。"""
    mask = Image.new("L", layer.size, 0)
    d = ImageDraw.Draw(mask)
    p = [((x - VIEW_ORIGIN) * scale, (y - VIEW_ORIGIN) * scale) for x, y in pts]
    r = width * scale / 2
    for (x0, y0), (x1, y1) in zip(p, p[1:]):
        dx, dy = x1 - x0, y1 - y0
        n = math.hypot(dx, dy)
        ox, oy = -dy / n * r, dx / n * r
        d.polygon([(x0 + ox, y0 + oy), (x1 + ox, y1 + oy), (x1 - ox, y1 - oy), (x0 - ox, y0 - oy)], fill=255)
    for x, y in p:
        d.ellipse([x - r, y - r, x + r, y + r], fill=255)
    white = Image.new("RGBA", layer.size, WHITE + (0,))
    white.putalpha(mask.point(lambda v: int(v * alpha)))
    layer.alpha_composite(white)


def render_body():
    """图标主体（824x824，带透明圆角）。"""
    big = BODY * S
    scale = big / VIEW_SPAN
    img = diagonal_gradient(big).convert("RGBA")
    sheen = Image.new("RGBA", (big, big), WHITE + (255,))
    sheen.putalpha(top_sheen(big))
    img.alpha_composite(sheen)
    glyph = Image.new("RGBA", (big, big), (0, 0, 0, 0))
    stroke(glyph, PEAK, 5.2, 1.0, scale)
    for a, b, w, op in BARS:
        stroke(glyph, [a, b], w, op, scale)
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
