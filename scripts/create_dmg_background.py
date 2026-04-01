#!/usr/bin/env python3
"""
生成 DMG 安装窗口背景图
尺寸: 620x400（与 build.sh 中 Finder 窗口匹配）
风格: 简约渐变 + 箭头引导拖放
"""
import sys
from PIL import Image, ImageDraw, ImageFont

WIDTH, HEIGHT = 620, 400
# 渐变色：从浅蓝灰到白
COLOR_TOP = (235, 242, 250)
COLOR_BOTTOM = (248, 250, 253)
ACCENT = (0, 122, 255)  # #007aff

def lerp_color(c1, c2, t):
    return tuple(int(a + (b - a) * t) for a, b in zip(c1, c2))

def main():
    output_path = sys.argv[1] if len(sys.argv) > 1 else "background.png"
    
    img = Image.new("RGB", (WIDTH, HEIGHT), COLOR_TOP)
    draw = ImageDraw.Draw(img)
    
    # 纵向线性渐变背景
    for y in range(HEIGHT):
        color = lerp_color(COLOR_TOP, COLOR_BOTTOM, y / HEIGHT)
        draw.line([(0, y), (WIDTH, y)], fill=color)
    
    # 中间箭头引导：从 app 图标区域指向 Applications
    arrow_y = 195
    arrow_x_start = 210
    arrow_x_end = 410
    
    # 箭头线
    draw.line(
        [(arrow_x_start, arrow_y), (arrow_x_end, arrow_y)],
        fill=(*ACCENT, 180), width=3
    )
    
    # 箭头头部
    arrow_head = [
        (arrow_x_end, arrow_y),
        (arrow_x_end - 12, arrow_y - 8),
        (arrow_x_end - 12, arrow_y + 8),
    ]
    draw.polygon(arrow_head, fill=ACCENT)
    
    # 底部安装提示文字
    try:
        font = ImageFont.truetype("/System/Library/Fonts/PingFang.ttc", 13)
        font_en = ImageFont.truetype("/System/Library/Fonts/Helvetica.ttc", 12)
    except Exception:
        font = ImageFont.load_default()
        font_en = font
    
    hint_cn = "将应用拖入 Applications 文件夹完成安装"
    hint_en = "Drag the app to Applications to install"
    
    # 中文提示
    bbox = draw.textbbox((0, 0), hint_cn, font=font)
    tw = bbox[2] - bbox[0]
    draw.text(((WIDTH - tw) // 2, 330), hint_cn, fill=(100, 110, 120), font=font)
    
    # 英文提示
    bbox = draw.textbbox((0, 0), hint_en, font=font_en)
    tw = bbox[2] - bbox[0]
    draw.text(((WIDTH - tw) // 2, 355), hint_en, fill=(150, 160, 170), font=font_en)
    
    img.save(output_path, "PNG")
    print(f"  DMG 背景图已生成: {output_path}")

if __name__ == "__main__":
    main()
