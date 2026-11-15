package ui

import (
	"image"
	"math"
	"testing"

	mui "github.com/egoist/mygo/ui"
)

// @author ixxxxoooo
//
// 单行输入框的文字要在框里垂直居中（设计稿「输入框」：文字按行高居中，占位符与正文同一条基线）。
// 量法：取框内文字墨迹的「重心」和框的中心比（不比墨迹的上下边：带下伸笔画的字母会让上下边看起来偏）。
// 应用里的单行输入框都走 textField（ctl_input.go）；库的默认内边距在 22 / 26 高的框里会让文字偏下，改回去这里先红。

// inkCenterY 返回 img 里矩形 r 内（缩进避开边框）深色像素按深浅加权的纵向重心；没有文字时 ok 为假。
func inkCenterY(img image.Image, r mui.Rect) (y float64, ok bool) {
	var sum, wsum float64
	for py := int(r.Y) + 3; py < int(r.Y+r.H)-3; py++ {
		for px := int(r.X) + 6; px < int(r.X+r.W)-6; px++ {
			cr, cg, cb, _ := img.At(px, py).RGBA()
			if w := 255 - float64((cr+cg+cb)/3>>8); w > 60 {
				sum += w * float64(py)
				wsum += w
			}
		}
	}
	if wsum == 0 {
		return 0, false
	}
	return sum/wsum + 0.5, true // 像素 py 覆盖 [py, py+1)，取中点
}

func TestSingleLineInputTextIsCentered(t *testing.T) {
	e := newEnv(t)
	heights := []float32{sizeBtnSm, sizeInput, sizeInputLg}
	texts := []string{"HEIOT", "搜索用户"} // 没有下伸笔画的文字，重心就是视觉中心
	rects := make([]mui.Rect, len(heights)*len(texts))
	tt := mui.NewTester(func(c *mui.Context) {
		p := e.sh.applyLook(c)
		mui.Column(c).Fill().Background(p.surface).Padding(spaceXL).Gap(spaceLG).Children(func() {
			for i := range rects {
				v := texts[i%len(texts)]
				rects[i] = textField(c.Key(string(rune('a'+i))), &v).Width(240).Height(heights[i/len(texts)]).Bounds()
			}
		})
	}, 320, 360)
	tt.Frame()
	tt.Frame()
	img := tt.Image()
	for i, r := range rects {
		y, ok := inkCenterY(img, r)
		if !ok {
			t.Fatalf("高 %.0f 的输入框没画出文字", r.H)
		}
		if d := y - float64(r.Y+r.H/2); math.Abs(d) > 0.5 {
			t.Errorf("%q 在高 %.0f 的输入框里偏了 %.2fpx（正数偏下）", texts[i%len(texts)], r.H, d)
		}
	}
}
