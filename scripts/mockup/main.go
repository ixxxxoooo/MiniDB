// mockup 给无窗口的界面截图加上背景：渐变底、圆角、柔和投影和一圈细描边，生成 README 用的配图。
//
// 用法：go run ./scripts/mockup <输入目录> <输出目录>
// 输入目录里每张 readme-<名字>.png 会输出成 <输出目录>/<名字>.png；名字里带 dark 的用深色背景（如 dark-edit）。
// 通常不直接运行，而是用 scripts/readme-shots.sh（它先出原始截图，再调用本程序）。
//
// @author ixxxxoooo
package main

import (
	"fmt"
	"image"
	"image/color"
	"image/png"
	"math"
	"os"
	"path/filepath"
	"strings"
)

// 画布留白：左右各 padX，上方 padTop，下方 padBottom（下方多一点，给投影留空间）。
const (
	padX      = 110
	padTop    = 76
	padBottom = 110
	radius    = 14.0 // 窗口圆角（按 1280 宽的原图算）
)

// theme 是一种背景配色：渐变的三个停靠色，以及窗口描边的颜色。
type theme struct {
	stops  [3]color.RGBA
	stroke color.RGBA
}

var (
	// 浅色：靛蓝 → 紫 → 粉，和应用图标的渐变同一个方向。
	lightTheme = theme{
		stops:  [3]color.RGBA{{0x7a, 0x7d, 0xf5, 255}, {0xa3, 0x8b, 0xf7, 255}, {0xf2, 0xa7, 0xe8, 255}},
		stroke: color.RGBA{255, 255, 255, 90},
	}
	// 深色：同一色相压暗，深色界面截图放上去不刺眼。
	darkTheme = theme{
		stops:  [3]color.RGBA{{0x25, 0x23, 0x6b, 255}, {0x45, 0x1f, 0x7e, 255}, {0x7a, 0x1d, 0x7a, 255}},
		stroke: color.RGBA{255, 255, 255, 46},
	}
)

func main() {
	if len(os.Args) != 3 {
		fmt.Fprintln(os.Stderr, "用法：go run ./scripts/mockup <输入目录> <输出目录>")
		os.Exit(2)
	}
	in, out := os.Args[1], os.Args[2]
	files, _ := filepath.Glob(filepath.Join(in, "readme-*.png"))
	if len(files) == 0 {
		fmt.Fprintln(os.Stderr, "没有找到 readme-*.png：先运行 MINIDB_SHOTS=<目录> go test ./internal/ui/ -run TestReadmeShots")
		os.Exit(1)
	}
	if err := os.MkdirAll(out, 0o755); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	for _, f := range files {
		name := strings.TrimSuffix(strings.TrimPrefix(filepath.Base(f), "readme-"), ".png")
		th := lightTheme
		if strings.Contains(name, "dark") {
			th = darkTheme
		}
		if err := compose(f, filepath.Join(out, name+".png"), th); err != nil {
			fmt.Fprintln(os.Stderr, f+"：", err)
			os.Exit(1)
		}
		fmt.Println("已生成", filepath.Join(out, name+".png"))
	}
}

// compose 把 src 放到带背景的画布中央并写到 dst。
func compose(src, dst string, th theme) error {
	f, err := os.Open(src)
	if err != nil {
		return err
	}
	shot, err := png.Decode(f)
	f.Close()
	if err != nil {
		return err
	}
	sw, sh := shot.Bounds().Dx(), shot.Bounds().Dy()
	w, h := sw+2*padX, sh+padTop+padBottom
	canvas := image.NewRGBA(image.Rect(0, 0, w, h))
	paintBackground(canvas, th)

	win := image.Rect(padX, padTop, padX+sw, padTop+sh)
	// 两层投影：远处一大片柔和的，窗口底边一条贴紧的，合起来才有「浮起来」的感觉。
	drawShadow(canvas, win, 0, 34, 38, 0.38)
	drawShadow(canvas, win, 0, 8, 10, 0.22)
	paintWindow(canvas, shot, win, th.stroke)

	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()
	enc := png.Encoder{CompressionLevel: png.BestCompression}
	return enc.Encode(out, canvas)
}

// paintBackground 画斜向三色渐变，再在左上角叠一团柔光。
func paintBackground(img *image.RGBA, th theme) {
	b := img.Bounds()
	w, h := float64(b.Dx()), float64(b.Dy())
	for y := 0; y < b.Dy(); y++ {
		for x := 0; x < b.Dx(); x++ {
			t := (float64(x)/w*0.8 + float64(y)/h*0.6) / 1.4 // 0（左上）到 1（右下）
			c := gradient(th.stops, t)
			// 左上角的柔光：离光心越近越亮
			d := math.Hypot(float64(x)-w*0.18, float64(y)-h*0.1) / (w * 0.6)
			glow := math.Max(0, 1-d) * 0.22
			img.SetRGBA(x, y, mix(c, color.RGBA{255, 255, 255, 255}, glow))
		}
	}
}

// gradient 在三个停靠色之间插值，t 取 0~1。
func gradient(s [3]color.RGBA, t float64) color.RGBA {
	if t < 0.5 {
		return mix(s[0], s[1], t*2)
	}
	return mix(s[1], s[2], (t-0.5)*2)
}

// mix 把 a 向 b 混合 k（0~1）。
func mix(a, b color.RGBA, k float64) color.RGBA {
	l := func(x, y uint8) uint8 { return uint8(math.Round(float64(x) + (float64(y)-float64(x))*k)) }
	return color.RGBA{l(a.R, b.R), l(a.G, b.G), l(a.B, b.B), 255}
}

// roundedCoverage 返回点 (px,py) 对圆角矩形 r 的覆盖率（0~1，边缘 1px 抗锯齿）和到边缘的有向距离（内部为负）。
func roundedCoverage(px, py float64, r image.Rectangle, rad float64) (cov, dist float64) {
	cx, cy := float64(r.Min.X+r.Max.X)/2, float64(r.Min.Y+r.Max.Y)/2
	hx, hy := float64(r.Dx())/2-rad, float64(r.Dy())/2-rad
	qx, qy := math.Abs(px-cx)-hx, math.Abs(py-cy)-hy
	dist = math.Hypot(math.Max(qx, 0), math.Max(qy, 0)) + math.Min(math.Max(qx, qy), 0) - rad
	return math.Min(1, math.Max(0, 0.5-dist)), dist
}

// paintWindow 把截图按圆角裁出来放到 win 位置，并在边缘内侧描一圈细线。
func paintWindow(canvas *image.RGBA, shot image.Image, win image.Rectangle, stroke color.RGBA) {
	sb := shot.Bounds()
	for y := win.Min.Y; y < win.Max.Y; y++ {
		for x := win.Min.X; x < win.Max.X; x++ {
			cov, dist := roundedCoverage(float64(x)+0.5, float64(y)+0.5, win, radius)
			if cov == 0 {
				continue
			}
			r, g, b, _ := shot.At(sb.Min.X+x-win.Min.X, sb.Min.Y+y-win.Min.Y).RGBA()
			px := color.RGBA{uint8(r >> 8), uint8(g >> 8), uint8(b >> 8), 255}
			if dist > -1.2 { // 最外一圈：描边
				px = mix(px, color.RGBA{stroke.R, stroke.G, stroke.B, 255}, float64(stroke.A)/255)
			}
			canvas.SetRGBA(x, y, mix(canvas.RGBAAt(x, y), px, cov))
		}
	}
}

// drawShadow 在 win 周围画一层投影：把窗口形状偏移 (dx,dy)、模糊 blur 像素、按 alpha 压暗背景。
func drawShadow(canvas *image.RGBA, win image.Rectangle, dx, dy int, blur int, alpha float64) {
	b := canvas.Bounds()
	w, h := b.Dx(), b.Dy()
	mask := make([]float32, w*h)
	shifted := win.Add(image.Pt(dx, dy))
	for y := shifted.Min.Y - 2; y < shifted.Max.Y+2; y++ {
		for x := shifted.Min.X - 2; x < shifted.Max.X+2; x++ {
			if x < 0 || y < 0 || x >= w || y >= h {
				continue
			}
			cov, _ := roundedCoverage(float64(x)+0.5, float64(y)+0.5, shifted, radius)
			mask[y*w+x] = float32(cov)
		}
	}
	for i := 0; i < 3; i++ { // 三次盒式模糊逼近高斯模糊
		boxBlur(mask, w, h, blur/2, true)
		boxBlur(mask, w, h, blur/2, false)
	}
	black := color.RGBA{0, 0, 0, 255}
	for i, m := range mask {
		if m <= 0.002 {
			continue
		}
		x, y := i%w, i/w
		canvas.SetRGBA(x, y, mix(canvas.RGBAAt(x, y), black, float64(m)*alpha))
	}
}

// boxBlur 对 mask 做一次半径 r 的盒式模糊，horizontal 为 true 时沿横向，否则沿纵向。
func boxBlur(mask []float32, w, h, r int, horizontal bool) {
	if r < 1 {
		return
	}
	n, m := h, w // 外层循环 n 行（或列），内层长度 m
	at := func(line, i int) int { return line*w + i }
	if !horizontal {
		n, m = w, h
		at = func(line, i int) int { return i*w + line }
	}
	buf := make([]float32, m)
	inv := 1 / float32(2*r+1)
	for line := 0; line < n; line++ {
		var sum float32
		for i := -r; i <= r; i++ {
			sum += mask[at(line, clamp(i, 0, m-1))]
		}
		for i := 0; i < m; i++ {
			buf[i] = sum * inv
			sum += mask[at(line, clamp(i+r+1, 0, m-1))] - mask[at(line, clamp(i-r, 0, m-1))]
		}
		for i := 0; i < m; i++ {
			mask[at(line, i)] = buf[i]
		}
	}
}

func clamp(v, lo, hi int) int { return max(lo, min(hi, v)) }
