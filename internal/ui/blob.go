package ui

import (
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"unicode/utf8"

	"minidb/internal/model"
	"minidb/locales"
)

// 二进制（BLOB）单元格：网格里只显示 <BLOB N 字节>，Quick Look 里用十六进制看和改。
// 数据库层把读出来的非 UTF-8 二进制转成 "0x" 开头的十六进制文本（见 database.normalizeScannedValue），
// 这里再按列类型认出它们；写回时转成 []byte，由驱动按二进制参数绑定，不拼进 SQL 文本。

// blobMaxFile 是「载入文件」能接受的最大字节数。
const blobMaxFile = 8 << 20

// blobDumpMax 是十六进制视图最多画多少字节，大文件只画开头。
const blobDumpMax = 64 << 10

// isBlobType 判断列类型是不是二进制：BLOB 系列、BYTEA、BINARY / VARBINARY。
func isBlobType(typ string) bool {
	t := strings.ToLower(typ)
	return strings.Contains(t, "blob") || strings.Contains(t, "bytea") || strings.Contains(t, "binary")
}

// blobBytes 把格子里的值还原成字节：[]byte 本身，或 "0x" 开头的十六进制文本。其他值（普通文本、数字、NULL）返回 false。
func blobBytes(v any) ([]byte, bool) {
	switch x := v.(type) {
	case []byte:
		return x, true
	case string:
		if len(x) >= 2 && x[0] == '0' && (x[1] == 'x' || x[1] == 'X') {
			if b, err := hex.DecodeString(x[2:]); err == nil {
				return b, true
			}
		}
	}
	return nil, false
}

// cellDisplay 是格子里显示的文字：二进制列里的二进制内容显示成 <BLOB N 字节>，其他值同 cellText。
func cellDisplay(col model.ColumnMeta, v any) (text string, null bool) {
	text, null = cellText(v)
	if null || !isBlobType(col.Type) {
		return text, null
	}
	// []byte 是刚编辑或载入的值；string 是数据库层已转成 0x 文本的二进制。
	// 合法的 UTF-8（如存在 BLOB 列里的普通文本）照常显示成文字。
	if b, ok := blobBytes(v); ok && !utf8.Valid(b) {
		return locales.GridBlob(len(b)), false
	}
	return text, false
}

// formatHexText 把字节写成可编辑的十六进制文本：每行 16 字节，字节之间一个空格，大写。
func formatHexText(b []byte) string {
	var sb strings.Builder
	for i, c := range b {
		if i > 0 {
			if i%16 == 0 {
				sb.WriteByte('\n')
			} else {
				sb.WriteByte(' ')
			}
		}
		fmt.Fprintf(&sb, "%02X", c)
	}
	return sb.String()
}

// errHexInvalid 是十六进制文本不合法。
var errHexInvalid = errors.New("invalid hex")

// parseHexText 读十六进制文本：忽略空白、逗号和可选的 0x 前缀（整段开头一个），字节数必须完整。
func parseHexText(s string) ([]byte, error) {
	s = strings.TrimSpace(s)
	s = strings.TrimPrefix(strings.TrimPrefix(s, "0x"), "0X")
	s = strings.Map(func(r rune) rune {
		if r == ' ' || r == '\n' || r == '\r' || r == '\t' || r == ',' {
			return -1
		}
		return r
	}, s)
	b, err := hex.DecodeString(s)
	if err != nil {
		return nil, errHexInvalid
	}
	return b, nil
}

// hexDump 画成「偏移 | 十六进制 | ASCII」三列；超过 limit 字节只画开头，末尾注明省略了多少。
func hexDump(b []byte, limit int) string {
	more := 0
	if limit > 0 && len(b) > limit {
		more, b = len(b)-limit, b[:limit]
	}
	var sb strings.Builder
	for off := 0; off < len(b); off += 16 {
		end := min(off+16, len(b))
		fmt.Fprintf(&sb, "%08X  ", off)
		for i := off; i < off+16; i++ {
			if i < end {
				fmt.Fprintf(&sb, "%02X ", b[i])
			} else {
				sb.WriteString("   ")
			}
			if i == off+7 {
				sb.WriteByte(' ')
			}
		}
		sb.WriteString(" |")
		for _, c := range b[off:end] {
			if c >= 0x20 && c < 0x7f {
				sb.WriteByte(c)
			} else {
				sb.WriteByte('.')
			}
		}
		sb.WriteString("|\n")
	}
	if more > 0 {
		fmt.Fprintf(&sb, "… +%d bytes\n", more)
	}
	return strings.TrimRight(sb.String(), "\n")
}

// errTooBig 是文件超过 blobMaxFile。
var errTooBig = errors.New("file too big")

// baseName 是路径的最后一段。
func baseName(path string) string {
	if i := strings.LastIndexAny(path, `/\`); i >= 0 {
		return path[i+1:]
	}
	return path
}
