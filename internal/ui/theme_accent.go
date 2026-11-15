package ui

import (
	"github.com/egoist/mygo/ui"
)

// @author ixxxxoooo
//
// 强调色选项（设计稿 A「外观」页的色板）：靛蓝（各配色自己的默认强调色）、蓝、青、玫红、琥珀、单色。
// 选中后只替换令牌里的强调色那几项（实心色、文字色、实心上的文字、两档淡底），
// 底色和文字色不动。色值取自设计稿 tokens.css 的 [data-accent=…] 与深色下单独调过的亮度版本。

// accentNames 是强调色的存盘名，顺序就是设置页色板的顺序；空串（靛蓝）表示用配色自己的默认强调色。
var accentNames = []string{"", "blue", "teal", "rose", "amber", "mono"}

// activeAccent 是当前生效的强调色存盘名，写入方式同 activeScheme：只在界面线程里读写。
var activeAccent = ""

// accentSwatch 是一种强调色在浅 / 深色下的色值。fg 是实心强调色上的文字。
type accentSwatch struct {
	light, dark     ui.Color
	lightFg, darkFg ui.Color
}

// accentSwatches 按存盘名登记各强调色；单色没有固定色值，跟随文字色，见 withAccent。
var accentSwatches = map[string]accentSwatch{
	"blue":  {ui.Hex("#2f7cf6"), ui.Hex("#4d94ff"), ui.Hex("#ffffff"), ui.Hex("#ffffff")},
	"teal":  {ui.Hex("#0fa89a"), ui.Hex("#2fd0be"), ui.Hex("#ffffff"), ui.Hex("#04201c")},
	"rose":  {ui.Hex("#e5457a"), ui.Hex("#ff6b9b"), ui.Hex("#ffffff"), ui.Hex("#2a0612")},
	"amber": {ui.Hex("#e08a0a"), ui.Hex("#f2a52a"), ui.Hex("#1b1303"), ui.Hex("#1b1303")},
}

// withAccent 返回把强调色换成 name 的表面色集合；name 为空或不认识时原样返回。
func withAccent(s surfaceSet, name string, dark bool) surfaceSet {
	var acc, fg ui.Color
	if name == "mono" {
		acc, fg = s.fg, s.bg
	} else if sw, ok := accentSwatches[name]; ok {
		acc, fg = sw.light, sw.lightFg
		if dark {
			acc, fg = sw.dark, sw.darkFg
		}
	} else {
		return s
	}
	s.acc, s.accFg = acc, fg
	if dark {
		s.accT = shade(acc, 0.25)
	} else {
		s.accT = shade(acc, -0.2)
	}
	soft, soft2 := float32(0.12), float32(0.20)
	if name == "mono" { // 单色的淡底要更淡，不然整个界面发灰
		soft, soft2, s.accT = 0.08, 0.14, s.fg
	}
	s.accBg, s.accBg2 = acc.Alpha(soft), acc.Alpha(soft2)
	return s
}

// accentColor 返回色板里显示的颜色：默认（空串）取当前配色的强调色，其余取该强调色的色值。
func accentColor(name string, pal scheme, dark bool) ui.Color {
	return withAccent(schemeSurface(pal, dark), name, dark).acc
}
