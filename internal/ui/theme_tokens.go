package ui

import (
	"github.com/egoist/mygo/ui"
)

// @author ixxxxoooo
//
// 界面令牌的结构与取值入口。取值的来源是 docs/UI_TOKENS.md（设计稿「MiniDB 界面重设计」的令牌），
// 色值写在 theme_palettes.go，尺寸写在 theme_metrics.go；业务代码只引用这里的字段和常量，
// 不写死颜色值、字号和控件尺寸（archtest 的 TestUINoHardcodedStyle 守着）。

// scheme 是配色方案：只改变表面、文字、线条和强调色，状态色、语法色、图表色两套主题共用。
type scheme int

const (
	schemeGraphite scheme = iota // 石墨：冷灰 + 靛蓝（默认）
	schemePaper                  // 纸墨：暖灰 + 陶土橙
	schemeAurora                 // 极光：深海蓝黑 + 青绿
)

// schemeNames 是配色方案在设置与持久化里的名字，顺序与 scheme 常量一致。
var schemeNames = [...]string{"graphite", "paper", "aurora"}

// schemeByName 按名字取配色方案；认不出时回到默认的石墨。
func schemeByName(name string) scheme {
	for i, n := range schemeNames {
		if n == name {
			return scheme(i)
		}
	}
	return schemeGraphite
}

// activeScheme 是当前生效的配色方案。只在界面线程里读写（Shell.applyLook 每帧写入），
// 所以不需要加锁；baseTokens 靠它在没有 Shell 的地方取到同一套颜色。
var activeScheme = schemeGraphite

// tokens 是一套完整的界面令牌（某个配色 × 某个明暗）。字段名沿用旧版，新增的在注释里标「新」。
type tokens struct {
	// 强调色固定，不再跟随连接颜色；连接色只出现在连接头像和生产环境的顶部色带上。
	accent, accentHover ui.Color
	accentText          ui.Color // 新：强调色的文字/图标色（--acc-t），在浅底上比 accent 更深、深色下更亮
	accentFg            ui.Color // 新：强调色实心底上的文字（--acc-fg）
	accentSoft          ui.Color // 新：强调色淡底（--acc-bg），聚焦的输入框、建议卡
	accentSoft2         ui.Color // 新：强调色中等淡底（--acc-bg2），选中行、悬浮光环

	// 底色层级：surface（内容区）→ surfaceSecondary（条栏、侧栏）→ surfaceTertiary（轨道、悬浮底）→ surfaceElev（浮层）。
	surface, surfaceSecondary, surfaceTertiary, surfaceElev ui.Color
	fg, fgSecondary, fgMuted, fgFaint                       ui.Color // fgFaint 是最淡的一档（--fg-4）：占位符、行号、表头里的类型
	// 线条分两档：border 区域分隔与控件描边，borderSubtle 卡片内部细线。
	border, borderSubtle ui.Color
	// stripe 是斑马纹底色，只在设置里选了「斑马纹」时才用。
	stripe ui.Color
	// segTrack / segThumb 是分段开关的轨道与滑块。
	segTrack, segThumb ui.Color
	hover              ui.Color // 悬停底：半透明的一层，叠在任何底色上都成立（设计稿 --hover）

	success, warning, danger, info              ui.Color
	successBg, warningBg, dangerBg, infoBg      ui.Color // 新：状态色淡底（芯片、提示条）
	rowNew, rowDelete, cellEdit                 ui.Color
	toolbar                                     ui.Color // 标题栏、活动栏、侧栏、状态栏的底色（= surfaceSecondary）
	iconTable, iconView, iconQuery, iconRoutine ui.Color
	valNum, valTime                             ui.Color

	// 新：SQL 语法高亮色。
	sxKeyword, sxFunc, sxString, sxNumber, sxComment, sxOp ui.Color
	// 新：图表系列色 c1~c5。
	chart [5]ui.Color
	// 新：浮层的阴影色与遮罩色；阴影形状见 theme_metrics.go 的 shadow* 常量。
	shadow, scrim ui.Color
}

// shade 把颜色向白（amt>0）或向黑（amt<0）混合 |amt|，用来推导悬停色，保证 accentHover 不用单独登记。
func shade(c ui.Color, amt float32) ui.Color {
	to, k := float32(255), amt
	if amt < 0 {
		to, k = 0, -amt
	}
	mix := func(v uint8) uint8 { return uint8(float32(v) + (to-float32(v))*k + 0.5) }
	return ui.Color{R: mix(c.R), G: mix(c.G), B: mix(c.B), A: c.A}
}

// tokensFor 取某个配色在某个明暗下的完整令牌。
func tokensFor(pal scheme, dark bool) tokens {
	p := sharedTokens(dark)
	s := schemeSurface(pal, dark)
	p.surface, p.surfaceSecondary, p.surfaceTertiary, p.surfaceElev = s.bg, s.bg2, s.bg3, s.elev
	p.segThumb, p.segTrack = s.segOn, s.bg3
	p.fg, p.fgSecondary, p.fgMuted, p.fgFaint = s.fg, s.fg2, s.fg3, s.fg4
	p.border, p.borderSubtle, p.stripe = s.line, s.line2, s.stripe
	p.accent, p.accentText, p.accentFg = s.acc, s.accT, s.accFg
	p.accentSoft, p.accentSoft2 = s.accBg, s.accBg2
	p.hover, p.toolbar = s.hov, s.bg2
	if dark {
		p.accentHover = shade(s.acc, 0.10)
	} else {
		p.accentHover = shade(s.acc, -0.08)
	}
	return p
}

// 向后兼容：旧代码和测试里直接引用的两套令牌，都是默认的石墨配色。
var (
	lightTokens = tokensFor(schemeGraphite, false)
	darkTokens  = tokensFor(schemeGraphite, true)
)
