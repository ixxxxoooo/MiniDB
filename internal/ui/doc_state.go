package ui

import (
	"strings"
	"time"

	"minidb/internal/app"
	"minidb/locales"
)

// @author ixxxxoooo
//
// 表「文档」页的状态：库里保存的 Markdown 文档（只取第一段当一句说明）、AI 补全出来的草稿，以及它们拼成的页面内容。

// docState 是文档页的状态。
type docState struct {
	loaded     bool
	loading    bool
	err        string
	text       string            // 库里保存的 Markdown 文档全文
	draftDesc  string            // AI 草拟的表概述
	drafts     map[string]string // AI 草拟的列说明：列名 → 说明
	generating bool
	shownAt    time.Time
}

// load 在后台读取保存的文档。参数：t 是所属的表标签。
func (d *docState) load(t *tableTab) {
	d.loading = true
	w := t.w
	db := w.db
	var text string
	var err error
	w.sh.up.bg(func() { text, err = w.sh.app.Docs.GetTableDoc(w.cfg.ID, db, t.table) }, func() {
		d.loaded, d.loading = true, false
		if err != nil {
			d.err = errMessage(err)
			return
		}
		d.text = text
	})
}

// tableComment 是这张表在库里的备注（表标题旁边的小标签），没有则为空串。
func (t *tableTab) tableComment() string {
	for _, tb := range t.w.tables {
		if tb.Name == t.table {
			return strings.TrimSpace(tb.Comment)
		}
	}
	return ""
}

// docDesc 是标题下面的一句说明：先用保存的文档第一段，没有就用 AI 草稿。返回值：文字，以及它是不是 AI 草拟的。
func (t *tableTab) docDesc() (string, bool) {
	if s := firstParagraph(t.doc.text); s != "" {
		return s, false
	}
	return t.doc.draftDesc, t.doc.draftDesc != ""
}

// docRows 是字段表的行：说明取库里的备注，没有备注就用 AI 草稿（带 ai 标记）。
func (t *tableTab) docRows() []docRow {
	rows := make([]docRow, 0, len(t.st.info))
	for _, c := range t.st.info {
		r := docRow{name: c.Name, typ: c.Type, comment: strings.TrimSpace(c.Comment)}
		if d := t.doc.drafts[c.Name]; r.comment == "" && d != "" {
			r.comment, r.ai = d, true
		}
		rows = append(rows, r)
	}
	return rows
}

// docMissing 返回还缺说明的列名（既没有备注也没有草稿），以及是否还缺一句表概述。
func (t *tableTab) docMissing() (cols []string, needDesc bool) {
	for _, r := range t.docRows() {
		if r.comment == "" {
			cols = append(cols, r.name)
		}
	}
	desc, _ := t.docDesc()
	return cols, desc == ""
}

// fillDoc 让 AI 补全缺失的说明（只是草稿，不写进数据库）。没有要补的就提示一下。
func (t *tableTab) fillDoc() {
	d := &t.doc
	if d.generating || !t.st.loaded {
		return
	}
	need, needDesc := t.docMissing()
	w := t.w
	if len(need) == 0 && !needDesc {
		w.sh.toast(locales.DocNothingToFill())
		return
	}
	d.generating = true
	db := w.db
	var draft *app.DocDraft
	var err error
	w.sh.up.bg(func() {
		draft, err = w.sh.app.AI.GenerateDocDraft(w.cfg.ID, db, t.table, need, needDesc)
	}, func() {
		d.generating = false
		if err != nil {
			w.sh.toastErr(errMessage(err))
			return
		}
		if d.drafts == nil {
			d.drafts = map[string]string{}
		}
		for k, v := range draft.Columns {
			d.drafts[k] = v
		}
		if needDesc {
			d.draftDesc = draft.Description
		}
	})
}

// copyDocMarkdown 把文档页内容复制成 Markdown（复制完由 copyText 提示）。
func (t *tableTab) copyDocMarkdown() {
	desc, _ := t.docDesc()
	t.w.sh.copyText(docMarkdown(t.table, t.tableComment(), desc, t.docRows()))
}
