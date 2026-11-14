package ui

import (
	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/editor"
)

// @author ixxxxoooo
//
// SQL 编辑器的配色（设计稿 A 的 .sql）：关键字紫色加粗、函数蓝色、字符串绿色、数字橙色、
// 注释灰色斜体；行号最淡的灰，当前行的行号用正文色，当前行铺强调淡底。字号 12.5、行高 1.7 倍。

// editorTheme 由令牌拼出编辑器的配色；没写的颜色编辑器自己用默认的。
func editorTheme(p tokens) *editor.Theme {
	th := &editor.Theme{
		LineNumber:       p.fgFaint,
		ActiveLineNumber: p.fg,
		ActiveLine:       p.accentSoft,
		Cursor:           p.accent,
	}
	style := func(k editor.TokenKind, col ui.Color, bold, italic bool) {
		th.Tokens[k] = editor.TokenStyle{Color: col, Bold: bold, Italic: italic}
	}
	style(editor.TokenKeyword, p.sxKeyword, true, false)
	style(editor.TokenOperator, p.sxOp, false, false)
	style(editor.TokenType, p.sxKeyword, false, false)
	style(editor.TokenFunction, p.sxFunc, false, false)
	style(editor.TokenNumber, p.sxNumber, false, false)
	style(editor.TokenConstant, p.sxNumber, false, false)
	style(editor.TokenString, p.sxString, false, false)
	style(editor.TokenComment, p.sxComment, false, true)
	return th
}

// applyEditorTheme 给 SQL 编辑器套上设计稿的配色、字号和行高，每帧调用，放在 applyEditorPrefs 之前
// （设置里选了字号的话，后者再覆盖字号）。
func (s *Shell) applyEditorTheme(c *ui.Context, ed *editor.State) {
	ed.Theme = editorTheme(baseTokens(c))
	if ed.Font.Size == 0 {
		ed.Font.Size = fontCtl
	}
	ed.LineHeight = 1.7
}
