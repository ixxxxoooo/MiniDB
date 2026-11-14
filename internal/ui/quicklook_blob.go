package ui

import (
	"os"

	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/editor"

	"minidb/internal/app"
	"minidb/internal/model"
	"minidb/locales"
)

// Quick Look 里的二进制模式：十六进制视图（偏移 / HEX / ASCII）与 HEX 文本编辑，可从文件载入。

// blobHugeText 超过这个字节数的值不进文本编辑器（HEX 文本太长会卡），只给十六进制视图，改值靠「载入文件」。
const blobHugeText = 256 << 10

// blobState 是 Quick Look 在二进制模式下的状态。
type blobState struct {
	on      bool   // 当前值是二进制
	dump    bool   // 显示十六进制视图（否则是 HEX 文本编辑器）
	huge    bool   // 值太大，不提供 HEX 文本编辑
	orig    []byte // 打开时的字节
	pending []byte // 从文件载入的字节，非空时以它为准
	file    string // 载入的文件名
	err     string // HEX 文本不合法时的提示
	key     string // dumpText 的缓存键
	cache   string // dumpText 的缓存
}

// blobOf 判断单元格是不是二进制并取出字节：只认二进制类型的列；NULL 与数字不算。
func blobOf(col model.ColumnMeta, v any) ([]byte, bool) {
	if v == nil || !isBlobType(col.Type) {
		return nil, false
	}
	if b, ok := blobBytes(v); ok {
		return b, true
	}
	if s, ok := v.(string); ok { // 存在二进制列里的普通文本
		return []byte(s), true
	}
	return nil, false
}

// colMeta 按列名取列信息。
func (g *resultGrid) colMeta(name string) model.ColumnMeta {
	for _, c := range g.cols {
		if c.Name == name {
			return c
		}
	}
	return model.ColumnMeta{Name: name}
}

// openBlob 把 Quick Look 切到二进制模式。
func (q *quickLook) openBlob(b []byte, ro bool) {
	q.blob = blobState{on: true, orig: b, huge: len(b) > blobHugeText, dump: len(b) > blobHugeText}
	text := ""
	if !q.blob.huge {
		text = formatHexText(b)
	}
	q.shown, q.orig, q.isJSON, q.tree, q.info = text, text, false, false, valueInfo{}
	buf := editor.NewBuffer(text)
	q.ed = &editor.State{Buffer: buf, SoftWrap: true, LineNumbers: editor.LineNumbersOff, NoFolding: true, ReadOnly: ro || q.blob.huge}
	noBracketMatch(q.ed)
}

// bytes 是现在的值：载入的文件，或把 HEX 文本解析出来的字节。HEX 文本不合法时返回 false。
func (q *quickLook) bytes() ([]byte, bool) {
	if q.blob.pending != nil {
		return q.blob.pending, true
	}
	if q.blob.huge {
		return q.blob.orig, true
	}
	b, err := parseHexText(q.text())
	return b, err == nil
}

// blobDirty 报告二进制值有没有改过。
func (q *quickLook) blobDirty() bool {
	return q.blob.pending != nil || (!q.blob.huge && q.text() != q.shown)
}

// dumpText 是十六进制视图的文字，按内容缓存，不用每帧重算。
func (q *quickLook) dumpText() string {
	key := q.text()
	if q.blob.pending != nil || q.blob.huge {
		key = "\x00" + q.blob.file + itoa(len(q.blob.pending))
	}
	if q.blob.key == key && q.blob.cache != "" {
		return q.blob.cache
	}
	b, ok := q.bytes()
	q.blob.key = key
	if !ok {
		q.blob.cache = locales.QlHexInvalid()
		return q.blob.cache
	}
	q.blob.cache = hexDump(b, blobDumpMax)
	return q.blob.cache
}

// qlBlobHeader 画二进制模式的工具：视图切换与「载入文件」。
func (t *tableTab) qlBlobHeader(c *ui.Context, q *quickLook, ro bool) {
	if !q.blob.huge {
		mode := 0
		if q.blob.dump {
			mode = 1
		}
		if ui.Segmented(c.Key("ql-blob-mode"), &mode, locales.QlHexText(), locales.QlHexView()).Changed() {
			q.blob.dump = mode == 1
		}
	}
	if actBtn(c.Key("ql-load"), icUpload, locales.QlLoadFile()).Disabled(ro).Clicked() {
		t.qlLoadFile(q)
	}
}

// qlLoadFile 选一个文件载入成新值（不超过 blobMaxFile），点「应用」才写回。
func (t *tableTab) qlLoadFile(q *quickLook) {
	sh := t.w.sh
	path, err := sh.host.OpenFile(app.OpenOptions{Title: locales.QlLoadFile(), Filters: []app.FileFilter{{Name: "*", Exts: []string{"*"}}}})
	if err != nil || path == "" {
		return
	}
	var data []byte
	sh.up.bg(func() {
		if st, e := os.Stat(path); e != nil {
			err = e
		} else if st.Size() > blobMaxFile {
			err = errTooBig
		} else {
			data, err = os.ReadFile(path)
		}
	}, func() {
		switch {
		case err == errTooBig:
			sh.toastWarn(locales.QlFileTooBig(blobMaxFile))
		case err != nil:
			sh.toastErr(errMessage(err))
		default:
			q.blob.pending, q.blob.file, q.blob.dump = data, baseName(path), true
			sh.toastOK(locales.QlBlobLoaded(q.blob.file, len(data)))
		}
	})
}
