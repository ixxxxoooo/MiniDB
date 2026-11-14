package ui

import (
	"fmt"
	"sort"
	"strings"
	"unicode"

	"github.com/egoist/mygo/ui"

	"minidb/internal/app"
	"minidb/internal/model"
	"minidb/locales"
)

// @author ixxxxoooo
//
// 对话里的 @ 引用：输入框里敲 @ 就弹出表名列表（↑↓ 选、↵ / Tab 引用、Esc 关闭），
// 引用的表以 chip 的样子列在输入框上方，发出去之后在消息里是同样的 chip；
// @表名 直接写在文字里，所以会话保存、恢复都不用额外存什么，后端读同一段文字就知道用户点了哪些表。

// mentionMenuMax 弹出的列表最多几行。
const mentionMenuMax = 8

// mentionMenu 是 @ 弹出列表的状态。它由上一帧的文字和光标算出，这一帧画出来。
type mentionMenu struct {
	open       bool
	start, end int // 光标所在的那个 @片段的 rune 区间，含 @；引用时整段被替换
	query      string
	items      []model.TableInfo
	sel        int

	accept string // 按键或点击选定的表名：下一帧在输入框上替换进去
	remove string // 点了某个 chip 的 ✕：下一帧把输入框里对它的引用删掉
	// 按 Esc 关掉列表之后，同一个片段不再自动弹出，直到光标换了地方或片段变了
	dismissed bool
	dismissAt int
	focus     bool // 点击列表项之后把焦点还给输入框
}

// mentionAt 找出光标所在的 @片段：光标紧跟在 @名字 里（或 @ 后面）。返回区间（含 @）和已输入的部分。
func mentionAt(text []rune, caret int) (start, end int, query string, ok bool) {
	if caret < 1 || caret > len(text) {
		return 0, 0, "", false
	}
	i := caret
	for i > 0 && mentionRune(text[i-1]) {
		i--
	}
	if i == 0 || text[i-1] != '@' {
		return 0, 0, "", false
	}
	start = i - 1
	if start > 0 && (text[start-1] == '@' || text[start-1] == '.' || asciiWord(text[start-1])) {
		return 0, 0, "", false // 邮箱里的 @
	}
	end = caret
	for end < len(text) && mentionRune(text[end]) {
		end++
	}
	return start, end, string(text[start+1 : caret]), true
}

// mentionRune 是 @ 后面名字里的字符，与后端 FindMentions 一致（点号只在名字中间算，这里输入过程中放宽到算）。
func mentionRune(r rune) bool {
	return r == '_' || r == '$' || r == '.' || unicode.IsLetter(r) || unicode.IsDigit(r)
}

func asciiWord(r rune) bool {
	return r == '_' || r >= '0' && r <= '9' || r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z'
}

// matchTables 按输入的部分挑表：表名前缀匹配的在前，其次表名里包含，最后是注释里包含；不分大小写。
func matchTables(tables []model.TableInfo, query string, limit int) []model.TableInfo {
	q := strings.ToLower(query)
	type scored struct {
		t     model.TableInfo
		score int
	}
	var hits []scored
	for _, t := range tables {
		name := strings.ToLower(t.Name)
		switch {
		case q == "":
			hits = append(hits, scored{t, 0})
		case strings.HasPrefix(name, q):
			hits = append(hits, scored{t, 0})
		case strings.Contains(name, q):
			hits = append(hits, scored{t, 1})
		case strings.Contains(strings.ToLower(t.Comment), q):
			hits = append(hits, scored{t, 2})
		}
	}
	sort.SliceStable(hits, func(i, j int) bool { return hits[i].score < hits[j].score })
	out := make([]model.TableInfo, 0, min(len(hits), limit))
	for _, h := range hits {
		if len(out) == limit {
			break
		}
		out = append(out, h.t)
	}
	return out
}

// update 按输入框当前的文字和光标刷新列表；有变化返回 true（要再画一帧）。
func (m *mentionMenu) update(tables []model.TableInfo, text string, selStart, selEnd int) bool {
	before := *m
	m.open = false
	if selStart == selEnd {
		if start, end, query, ok := mentionAt([]rune(text), selStart); ok {
			if m.dismissed && m.dismissAt == start {
				m.start, m.end, m.query = start, end, query
				return before.open != m.open
			}
			m.dismissed = false
			items := matchTables(tables, query, mentionMenuMax)
			m.start, m.end, m.query, m.items = start, end, query, items
			m.open = len(items) > 0
		} else {
			m.dismissed = false
		}
	}
	if !m.open {
		m.items = nil
	}
	m.sel = min(max(m.sel, 0), max(len(m.items)-1, 0))
	if before.query != m.query || before.start != m.start {
		m.sel = 0
	}
	return before.open != m.open || before.query != m.query || before.start != m.start || before.sel != m.sel
}

// key 处理列表打开时的按键，返回是否吃掉了这个键。
func (m *mentionMenu) key(k ui.TextKey) bool {
	if !m.open || len(m.items) == 0 || k.Mods != 0 {
		return false
	}
	switch k.Key {
	case ui.KeyDown:
		m.sel = (m.sel + 1) % len(m.items)
	case ui.KeyUp:
		m.sel = (m.sel + len(m.items) - 1) % len(m.items)
	case ui.KeyEnter, ui.KeyTab:
		m.accept = m.items[m.sel].Name
	default:
		return false
	}
	return true
}

// dismiss 关掉列表，同一个 @片段不再自动弹出。
func (m *mentionMenu) dismiss() {
	m.open, m.dismissed, m.dismissAt = false, true, m.start
}

// mentionSpans 是文字里每处 @引用 在输入框里的样式：强调色加粗，和 chip 同色。
func mentionRanges(text string, color ui.Color) []ui.TextRange {
	var out []ui.TextRange
	for _, m := range app.FindMentions(text) {
		out = append(out, ui.TextRange{Start: m.Start, End: m.End, Color: color, Weight: 600})
	}
	return out
}

// ---- 界面 ----

// mentionChip 画一个引用的表：圆角胶囊，表图标加名字；onRemove 不为空时末尾带 ✕。
// 找不到这张表时用警告色，提示点名的名字不对。
func mentionChip(c *ui.Context, p tokens, name string, known bool, onRemove func()) {
	col := p.accent
	if !known {
		col = p.warning
	}
	ui.Row(c.Key("chip:"+name)).Gap(4).AlignItems(ui.Center).Height(22).Padding(0, 6, 0, 7).Radius(999).Background(col.Alpha(0.12)).Border(1, col.Alpha(0.32)).Children(func() {
		ui.Icon(c, icTable).FontSize(11).TextColor(col)
		ui.Text(c, name).FontSize(font2XS).FontWeight(500).TextColor(col).SingleLine()
		if onRemove != nil {
			if tipBtn(c.Key("chipx:"+name), icX, locales.AiMentionRemove(name)).Size(16, 16).Clicked() {
				onRemove()
			}
		}
	})
}

// knownTable 报告库里有没有这张表（写成 库名.表名 的取最后一段）。
func (a *assistant) knownTable(name string) bool {
	if len(a.w.tables) == 0 {
		return true // 表列表还没读到：不乱报"没有这张表"
	}
	if i := strings.LastIndexByte(name, '.'); i >= 0 {
		name = name[i+1:]
	}
	for _, t := range a.w.tables {
		if strings.EqualFold(t.Name, name) {
			return true
		}
	}
	return false
}

// mentionChips 是输入框里文字引用的表，一行 chip；没有引用时什么都不画。
func (a *assistant) mentionChips(c *ui.Context, p tokens) {
	names := app.MentionNames(a.input)
	if len(names) == 0 {
		return
	}
	ui.Row(c.Key("ai-chips")).Gap(4).Wrap().FillWidth().Padding(2, 2, 4, 2).Children(func() {
		for _, name := range names {
			name := name
			mentionChip(c, p, name, a.knownTable(name), func() { a.mention.remove = name })
		}
	})
}

// mentionMenuView 是 @ 弹出的列表，画在输入框上方。
func (a *assistant) mentionMenuView(c *ui.Context, p tokens) {
	m := &a.mention
	if !m.open {
		return
	}
	ui.Column(c.Key("ai-mention")).FillWidth().Padding(4).Gap(1).Radius(10).Border(1, p.border).Background(p.surfaceElev).Shrink(0).Children(func() {
		for i, t := range m.items {
			row := ui.Row(c.Key(fmt.Sprint("mi", i))).Gap(8).AlignItems(ui.Center).FillWidth().Height(26).Padding(0, 8).Radius(6)
			switch {
			case i == m.sel:
				row.Background(p.accent.Alpha(0.14))
			case row.Hovered():
				row.Background(p.hover)
			}
			row.Children(func() {
				icon := objectIcon(KindTable)
				if strings.EqualFold(t.Type, "view") {
					icon = objectIcon(KindView)
				}
				ui.Icon(c, icon).FontSize(12).TextColor(p.fgSecondary).Shrink(0)
				ui.Text(c, t.Name).FontSize(font2XS + 1).FontWeight(500).SingleLine().Shrink(0)
				ui.Text(c, t.Comment).FontSize(font2XS).TextColor(p.fgMuted).SingleLine().Grow(1).TruncateMiddle()
			})
			if row.Clicked() {
				m.accept, m.focus = t.Name, true
			}
		}
		ui.Text(c, locales.AiMentionHint()).FontSize(10).TextColor(p.fgMuted).Padding(2, 8, 1, 8).SingleLine()
	})
}

// mentionApply 在输入框上执行上一帧定下的引用 / 移除，返回是否改了文字。
func (a *assistant) mentionApply(in ui.Element) bool {
	m := &a.mention
	changed := false
	if m.accept != "" {
		in.ReplaceText(m.start, m.end, "@"+m.accept+" ")
		m.accept, m.open, changed = "", false, true
		if m.focus {
			in.Focus()
			m.focus = false
		}
	}
	if m.remove != "" {
		ms := app.FindMentions(a.input)
		for i := len(ms) - 1; i >= 0; i-- {
			if strings.EqualFold(ms[i].Name, m.remove) {
				end := ms[i].End
				if rs := []rune(a.input); end < len(rs) && rs[end] == ' ' {
					end++ // 连着后面的一个空格一起删，免得留下双空格
				}
				in.ReplaceText(ms[i].Start, end, "") // 输入框绑定的 a.input 随之更新；从后往前删，前面的偏移不受影响
			}
		}
		m.remove, changed = "", true
	}
	return changed
}
