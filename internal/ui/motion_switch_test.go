package ui

import (
	"image/color"
	"testing"
	"time"

	mui "github.com/egoist/mygo/ui"
)

// @author ixxxxoooo
//
// 第六轮的切换动效：选中指示块要「滑」过去，而不是旧的灭掉、新的一帧亮起。
// 判断方法：点击后的第一帧，新选中项那里还不该是指示块的颜色（它还在旧位置），走完过渡后才是。

// motionOn 打开动效：测试环境默认按「减弱动态」跑，动效用例要自己关掉。
func motionOn(tt *mui.Tester) { tt.SetPreferences(mui.Preferences{TextScale: 1}) }

// settle 让所有过渡走完，再出一帧。
func settle(tt *mui.Tester) {
	tt.Frame()
	tt.Advance(time.Second)
	tt.Frame()
	tt.Advance(time.Second)
	tt.Frame()
}

// pixel 取窗口里 (x, y) 处的像素，坐标是 DIP。
func pixel(tt *mui.Tester, x, y float32) color.RGBA {
	r, g, b, a := tt.Image().At(int(x), int(y)).RGBA() // 测试窗口里 1 DIP = 1 像素
	return color.RGBA{uint8(r >> 8), uint8(g >> 8), uint8(b >> 8), uint8(a >> 8)}
}

// insideLeft 是按钮框内左侧 4 DIP、垂直中线上的像素：落在按钮的左内边距里（文字在中间），那里是指示块（或轨道）的颜色。
func insideLeft(t *testing.T, tt *mui.Tester, label string) color.RGBA {
	t.Helper()
	r, ok := tt.Find(label)
	if !ok {
		t.Fatalf("找不到 %q: %q", label, tt.Texts())
	}
	return pixel(tt, r.X+4, r.Y+r.H/2)
}

// underline 是按钮框最下面 1 DIP、水平中线上的像素：下划线标签的 2px 强调色下划线落在那里。
func underline(t *testing.T, tt *mui.Tester, label string) color.RGBA {
	t.Helper()
	r, ok := tt.Find(label)
	if !ok {
		t.Fatalf("找不到 %q: %q", label, tt.Texts())
	}
	return pixel(tt, r.X+r.W/2, r.Y+r.H-1)
}

// ink 数一数 label 按钮框里「文字笔画」的像素：和框内左侧底色亮度差很大的点。文字被指示块盖住时这个数会掉到 0。
func ink(t *testing.T, tt *mui.Tester, label string) int {
	t.Helper()
	r, ok := tt.Find(label)
	if !ok {
		t.Fatalf("找不到 %q: %q", label, tt.Texts())
	}
	lum := func(c color.RGBA) int { return (int(c.R)*3 + int(c.G)*6 + int(c.B)) / 10 }
	img := tt.Image() // 只取一次：每次取都会复制整张图
	at := func(x, y int) color.RGBA { return img.RGBAAt(x, y) }
	base, n := lum(at(int(r.X+4), int(r.Y+r.H/2))), 0
	for y := int(r.Y); y < int(r.Y+r.H); y++ {
		for x := int(r.X); x < int(r.X+r.W); x++ {
			if d := lum(at(x, y)) - base; d > 90 || d < -90 {
				n++
			}
		}
	}
	return n
}

// slideCase 是一个选中指示块的滑动用例：build 画控件并在点中时改 *sel；thumb 是指示块的颜色。
type slideCase struct {
	name  string
	build func(c *mui.Context, sel *int) (thumb color.RGBA)
	probe func(t *testing.T, tt *mui.Tester, label string) color.RGBA
}

func rgba(c mui.Color) color.RGBA { return color.RGBA{c.R, c.G, c.B, c.A} }

// same 判断两个颜色是否相近（渲染有色彩管理，和令牌差一两级是正常的）。
func same(a, b color.RGBA) bool {
	d := func(x, y uint8) int {
		if x > y {
			return int(x - y)
		}
		return int(y - x)
	}
	return d(a.R, b.R) <= 4 && d(a.G, b.G) <= 4 && d(a.B, b.B) <= 4
}

func TestSelectionThumbSlides(t *testing.T) {
	labels := []string{"Alpha", "Bravo", "Charlie"}
	cases := []slideCase{
		{"分段开关", func(c *mui.Context, sel *int) color.RGBA {
			p := baseTokens(c)
			segmented(c, sel, labels...)
			return rgba(p.segThumb)
		}, insideLeft},
		{"视图切换", func(c *mui.Context, sel *int) color.RGBA {
			p := baseTokens(c)
			if i := viewSeg(c, p, "vs", labels, *sel); i >= 0 {
				*sel = i
			}
			return rgba(p.segThumb)
		}, insideLeft},
		{"下划线标签", func(c *mui.Context, sel *int) color.RGBA {
			p := baseTokens(c)
			if i := ulTabs(c, p, "ul", labels, nil, *sel); i >= 0 {
				*sel = i
			}
			return rgba(p.accent)
		}, underline},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			sel := 0
			var thumb color.RGBA
			tt := mui.NewTester(func(c *mui.Context) {
				mui.Column(c).Padding(20).Children(func() { thumb = tc.build(c, &sel) })
			}, 400, 120)
			motionOn(tt)
			settle(tt)
			if got := tc.probe(t, tt, "Alpha"); !same(got, thumb) {
				t.Fatalf("前置：选中项 Alpha 处应是指示块色 %v，实际 %v", thumb, got)
			}
			if err := tt.Click("Charlie"); err != nil {
				t.Fatal(err)
			}
			tt.Frame()
			if got := tc.probe(t, tt, "Charlie"); same(got, thumb) {
				t.Fatalf("点击后第一帧 Charlie 处就已经是指示块色，说明是瞬间跳过去的，没有滑动")
			}
			if got := tc.probe(t, tt, "Alpha"); !same(got, thumb) {
				t.Fatalf("点击后第一帧指示块应还停在 Alpha，实际那里是 %v", got)
			}
			if !tt.FramePending() {
				t.Fatal("滑动过程中应还有帧要出")
			}
			before := ink(t, tt, "Bravo")
			tt.Advance(80 * time.Millisecond)
			tt.Frame()
			if got := ink(t, tt, "Bravo"); before > 0 && got < before*3/4 {
				t.Fatalf("指示块滑过 Bravo 时盖住了它的文字：笔画像素 %d → %d", before, got)
			}
			settle(tt)
			if got := tc.probe(t, tt, "Charlie"); !same(got, thumb) {
				t.Fatalf("过渡走完后 Charlie 处应是指示块色 %v，实际 %v", thumb, got)
			}
			if tt.FramePending() {
				t.Fatal("过渡走完后不该再出帧")
			}
		})
	}
}

// railBand 返回活动栏上半部分按钮区里「选中块」的上下边缘：在按钮左侧、图标之外的一条竖线 x=9 上，
// 找和活动栏底色（取中段空白处）不同的像素。找不到返回 (-1, -1)。
func railBand(tt *mui.Tester) (top, bottom int) {
	bg := pixel(tt, 9, 450)
	top, bottom = -1, -1
	for y := 40; y < 300; y++ {
		if !same(pixel(tt, 9, float32(y)), bg) {
			if top < 0 {
				top = y
			}
			bottom = y
		}
	}
	return top, bottom
}

// 活动栏换选中页：选中块从旧按钮滑到新按钮，而不是在原地灭掉、新的位置亮起。
func TestRailHighlightSlides(t *testing.T) {
	e := newShopEnv(t)
	w := openWorkspace(e)
	motionOn(e.tt)
	settle(e.tt)
	oldTop, oldBottom := railBand(e.tt)
	if oldTop < 0 {
		t.Fatal("前置：活动栏应有选中块")
	}
	w.showSidePage(2, false)
	e.tt.Frame()
	e.tt.Frame()
	top, bottom := railBand(e.tt)
	if top != oldTop || bottom != oldBottom {
		t.Fatalf("切换后第一帧选中块应还在旧位置 %d-%d，实际 %d-%d", oldTop, oldBottom, top, bottom)
	}
	if !e.tt.FramePending() {
		t.Fatal("滑动过程中应还有帧要出")
	}
	e.tt.Advance(100 * time.Millisecond)
	e.tt.Frame()
	if mid, _ := railBand(e.tt); mid <= oldTop || mid >= oldTop+2*(sizeRailBtn+3) {
		t.Fatalf("滑到一半时选中块应在新旧位置之间，旧 %d 新 %d，实际 %d", oldTop, oldTop+2*(sizeRailBtn+3), mid)
	}
	settle(e.tt)
	if newTop, _ := railBand(e.tt); newTop != oldTop+2*(sizeRailBtn+3) {
		t.Fatalf("过渡走完后选中块应在第三个按钮 %d，实际 %d", oldTop+2*(sizeRailBtn+3), newTop)
	}
}

// darkness 是窗口里矩形区域（DIP）的「墨量」：每个像素离区域背景色（取右下角那个点）多远的总和。内容淡入时它从小变大。
func darkness(tt *mui.Tester, x0, y0, x1, y1 int) (sum int) {
	img := tt.Image()
	bg := img.RGBAAt(x1-1, y1-1)
	abs := func(v int) int {
		if v < 0 {
			return -v
		}
		return v
	}
	for y := y0; y < y1; y++ {
		for x := x0; x < x1; x++ {
			c := img.RGBAAt(x, y)
			sum += abs(int(c.R)-int(bg.R)) + abs(int(c.G)-int(bg.G)) + abs(int(c.B)-int(bg.B))
		}
	}
	return sum
}

// 表标签里切 数据 / 结构 / DDL：新内容要淡入，而不是一帧换掉。
func TestTableModeSwitchFadesIn(t *testing.T) {
	e := newShopEnv(t)
	w := openWorkspace(e)
	w.openTable("orders")
	motionOn(e.tt)
	settle(e.tt)
	e.click("DDL")
	e.tt.Frame()
	first := darkness(e.tt, 240, 80, 1280, 300)
	if !e.tt.FramePending() {
		t.Fatal("切换后内容应在淡入（还有帧要出）")
	}
	settle(e.tt)
	done := darkness(e.tt, 240, 80, 1280, 300)
	if done == 0 || first*10 > done*7 {
		t.Fatalf("切换后第一帧内容应明显比走完时淡（墨量 %d vs %d）", first, done)
	}
	if e.tt.FramePending() {
		t.Fatal("淡入走完后不该再出帧")
	}
}

// 切标签：新页面淡入 200ms（设计稿「标签页内容切换」），不是 360ms。
func TestTabSwitchPageFadesIn200ms(t *testing.T) {
	e := newShopEnv(t)
	w := openWorkspace(e)
	w.openTable("orders")
	w.openTable("users")
	motionOn(e.tt)
	settle(e.tt)
	w.active = 0
	e.tt.Frame()
	first := darkness(e.tt, 240, 80, 1280, 300)
	e.tt.Advance(time.Second)
	e.tt.Frame()
	done := darkness(e.tt, 240, 80, 1280, 300)
	if done == 0 || first*10 > done*7 {
		t.Fatalf("切标签后第一帧页面应明显比走完时淡（墨量 %d vs %d）", first, done)
	}
	w.active = 1
	e.tt.Frame()
	e.tt.Advance(260 * time.Millisecond)
	e.tt.Frame()
	if e.tt.FramePending() {
		t.Fatal("页面入场应在 200ms 内走完，260ms 后不该还有帧")
	}
}

// 关掉前面的标签后，当前页面还是同一个标签对象，不应该当成新页面重播入场（key 按标签对象而不是下标）。
func TestClosingEarlierTabKeepsCurrentPageStill(t *testing.T) {
	e := newShopEnv(t)
	w := openWorkspace(e)
	w.openTable("orders")
	w.openTable("users")
	motionOn(e.tt)
	settle(e.tt)
	before := darkness(e.tt, 240, 80, 1280, 300)
	w.closeTab(0)
	e.tt.Frame()
	if got := darkness(e.tt, 240, 80, 1280, 300); got < before*9/10 {
		t.Fatalf("关掉前面的标签后当前页面不该重新淡入：墨量 %d → %d", before, got)
	}
}

// fadesIn 做一次切换并断言：切换后第一帧该区域（x0,y0,x1,y1，DIP）的墨量明显低于走完过渡后，
// 过渡期间还有帧要出，走完后不再出帧。region 只能框住切换的内容，不要框住不变的东西。
func fadesIn(t *testing.T, e *env, region [4]int, act func()) {
	t.Helper()
	motionOn(e.tt)
	settle(e.tt)
	act()
	e.tt.Frame()
	first := darkness(e.tt, region[0], region[1], region[2], region[3])
	if !e.tt.FramePending() {
		t.Fatal("切换后内容应在淡入（还有帧要出）")
	}
	settle(e.tt)
	done := darkness(e.tt, region[0], region[1], region[2], region[3])
	if done == 0 || first*10 > done*7 {
		t.Fatalf("切换后第一帧内容应明显比走完时淡（墨量 %d vs %d）", first, done)
	}
	if e.tt.FramePending() {
		t.Fatal("淡入走完后不该再出帧")
	}
}

// 各处子视图切换都要淡入：侧栏三页、查询结果的数据 / 消息、设置分页、欢迎页卡片 / 列表、连接表单分页。
func TestSubViewSwitchesFadeIn(t *testing.T) {
	t.Run("侧栏换页", func(t *testing.T) {
		e := newShopEnv(t)
		w := openWorkspace(e)
		fadesIn(t, e, [4]int{44, 120, 230, 600}, func() { w.showSidePage(1, false) })
	})
	t.Run("查询结果切到消息", func(t *testing.T) {
		e := newShopEnv(t)
		w := openWorkspace(e)
		q := w.newQuery("SELECT name FROM categories")
		e.tt.Frame()
		q.run(false)
		e.tt.Frame()
		fadesIn(t, e, [4]int{300, 420, 1270, 700}, func() { q.page = pageMessages })
	})
	t.Run("设置分页", func(t *testing.T) {
		e := newShopEnv(t)
		e.sh.openSettings(setGeneral)
		fadesIn(t, e, [4]int{420, 140, 1000, 620}, func() { e.sh.set.tab = setAppearance })
	})
	t.Run("欢迎页卡片列表", func(t *testing.T) {
		e := newShopEnv(t)
		fadesIn(t, e, [4]int{250, 120, 1270, 560}, func() { e.sh.layout.ConnList = !e.sh.layout.ConnList })
	})
	t.Run("连接表单分页", func(t *testing.T) {
		e := newShopEnv(t)
		e.sh.openConnList()
		e.tt.Frame()
		e.clickPlus()
		fadesIn(t, e, [4]int{310, 260, 970, 640}, func() { e.sh.dlg.tab = 1 })
	})
}

// opensFaded 做一次「点击打开」并断言：打开后第一帧整个窗口的墨量（相对打开前）只长了一小部分，
// 走完过渡后才长到位；过渡走完后不再出帧。弹层、模态是新出现的内容，所以整窗墨量会变多。
func opensFaded(t *testing.T, e *env, act func()) {
	t.Helper()
	motionOn(e.tt)
	settle(e.tt)
	base := darkness(e.tt, 0, 0, 1280, 800)
	act()
	e.tt.Frame()
	first := darkness(e.tt, 0, 0, 1280, 800)
	settle(e.tt)
	done := darkness(e.tt, 0, 0, 1280, 800)
	if done-base < 100000 {
		t.Fatalf("前置：打开后画面应有明显新增内容（墨量 %d → %d）", base, done)
	}
	if first-base > (done-base)*6/10 {
		t.Fatalf("点击打开后第一帧内容应还很淡：墨量 %d → 第一帧 %d → 走完 %d", base, first, done)
	}
	if e.tt.FramePending() {
		t.Fatal("过渡走完后不该再出帧")
	}
}

// 弹层、模态点击打开时要淡入（动效规范：菜单 / 锚定浮层进 160ms，模态进 220ms）。
func TestClickOpenedPopupsFadeIn(t *testing.T) {
	t.Run("库切换弹层", func(t *testing.T) {
		e := newShopEnv(t)
		openWorkspace(e)
		opensFaded(t, e, func() {
			if err := e.tt.Click("data.db"); err != nil {
				t.Fatal(err)
			}
		})
	})
	t.Run("设置模态", func(t *testing.T) {
		e := newShopEnv(t)
		openWorkspace(e)
		opensFaded(t, e, func() { e.tt.ClickAt(22, 775) })
		if !e.sh.set.open {
			t.Fatal("点齿轮应打开设置")
		}
	})
}

// 鼠标移到侧栏表行上，悬停底色要渐变过去，而不是一帧跳到位。
func TestRowHoverColorFades(t *testing.T) {
	e := newShopEnv(t)
	openWorkspace(e)
	motionOn(e.tt)
	settle(e.tt)
	r, ok := e.tt.Find("orders")
	if !ok {
		t.Fatalf("找不到 orders: %q", e.tt.Texts())
	}
	x, y := r.X+3, r.Y+r.H/2
	before := pixel(e.tt, x, y)
	e.tt.Move(r.X+r.W/2, y)
	e.tt.Frame()
	first := pixel(e.tt, x, y)
	settle(e.tt)
	done := pixel(e.tt, x, y)
	if done == before {
		t.Fatalf("前置：悬停后底色应变（%v → %v）", before, done)
	}
	if first == done {
		t.Fatalf("悬停底色应渐变：第一帧就已到位 %v（悬停前 %v）", first, before)
	}
}
