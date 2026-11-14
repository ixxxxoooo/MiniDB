package ui

import (
	"encoding/json"
	"fmt"
	"regexp"
	"sort"
	"strings"

	"github.com/egoist/mygo/ui"

	"minidb/locales"
)

// JSON 树：Quick Look 和行预览栏共用。对象、数组可折叠；右键一个节点可以复制它的路径或值。

// jsonTree 是一棵树的折叠状态，路径是 key。
type jsonTree struct {
	expanded map[string]bool // 深层节点默认折叠，用户点开的记在这里
	collapse map[string]bool // 浅层节点用户点合上的
}

// parseJSONValue 把文本解析成 JSON 值（数字保留原样，不转成浮点）。
func parseJSONValue(text string) (any, bool) {
	var v any
	dec := json.NewDecoder(strings.NewReader(strings.TrimSpace(text)))
	dec.UseNumber()
	if dec.Decode(&v) != nil || dec.More() {
		return nil, false
	}
	return v, true
}

var plainKey = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)

// jsonPathKey 给路径追加一个对象键：普通标识符用 .key，其他用 ["key"]。
func jsonPathKey(path, key string) string {
	if plainKey.MatchString(key) {
		return path + "." + key
	}
	q, _ := json.Marshal(key)
	return path + "[" + string(q) + "]"
}

// jsonValueText 是复制「值」时放进剪贴板的文字：容器是缩进的 JSON，字符串去掉引号，其他照原样。
func jsonValueText(v any) string {
	switch x := v.(type) {
	case string:
		return x
	case json.Number:
		return x.String()
	case nil:
		return "null"
	case map[string]any, []any:
		b, _ := json.MarshalIndent(x, "", "  ")
		return string(b)
	}
	return fmt.Sprint(v)
}

// node 画树里的一个值；copy 把文字放进剪贴板（从菜单回调里调用）。
func (jt *jsonTree) node(c *ui.Context, p tokens, copy func(string), path, key string, v any, depth int) {
	menu := func(row ui.Element) {
		row.ContextMenu(func(m *ui.Menu) {
			if m.Item(locales.QlCopyPath()).Chosen() {
				defer copy(path)
			}
			if m.Item(locales.QlCopyValue()).Chosen() {
				defer copy(jsonValueText(v))
			}
		})
	}
	label := func() {
		if key != "" {
			ui.Text(c, key).Font(mono).FontSize(font2XS).TextColor(p.iconView).SingleLine()
			ui.Text(c, ":").Font(mono).FontSize(font2XS).TextColor(p.fgMuted)
		}
	}
	scalar := func(text string, col ui.Color) {
		row := ui.Row(c.Key(path)).Gap(4).AlignItems(ui.Center).Padding(1, 0, 1, float32(depth)*14+16).FillWidth()
		row.Children(func() {
			label()
			ui.Text(c, text).Font(mono).FontSize(font2XS).TextColor(col).Selectable()
		})
		menu(row)
	}
	fold := func(open, close string, n int, children func()) {
		folded := jt.collapse[path] || (depth >= 2 && !jt.expanded[path])
		ic := icChevronD
		if folded {
			ic = icChevronR
		}
		row := ui.Row(c.Key(path)).Gap(4).AlignItems(ui.Center).Padding(1, 0, 1, float32(depth)*14).FillWidth()
		row.Children(func() {
			ui.Icon(c, ic).FontSize(11).TextColor(p.fgMuted).Width(12)
			label()
			ui.Text(c, open+" "+fmt.Sprint(n)+" "+close).Font(mono).FontSize(font2XS - 1).TextColor(p.fgMuted)
		})
		menu(row)
		if row.Clicked() {
			if jt.collapse == nil {
				jt.collapse, jt.expanded = map[string]bool{}, map[string]bool{}
			}
			if folded {
				jt.collapse[path] = false
				jt.expanded[path] = true
			} else {
				jt.collapse[path] = true
				delete(jt.expanded, path)
			}
		}
		if !folded {
			children()
		}
	}
	switch x := v.(type) {
	case map[string]any:
		keys := make([]string, 0, len(x))
		for k := range x {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		fold("{", "keys }", len(keys), func() {
			for _, k := range keys {
				jt.node(c, p, copy, jsonPathKey(path, k), k, x[k], depth+1)
			}
		})
	case []any:
		fold("[", "items ]", len(x), func() {
			for i, it := range x {
				jt.node(c, p, copy, fmt.Sprintf("%s[%d]", path, i), fmt.Sprint(i), it, depth+1)
			}
		})
	case string:
		scalar(`"`+x+`"`, p.fg)
	case json.Number:
		scalar(x.String(), p.valNum)
	case bool:
		scalar(fmt.Sprint(x), p.iconView)
	case nil:
		scalar("null", p.fgMuted)
	default:
		scalar(fmt.Sprint(x), p.fg)
	}
}
