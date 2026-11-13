package dataimport

import (
	"fmt"
	"strings"

	"golang.org/x/text/encoding"
	"golang.org/x/text/encoding/charmap"
	"golang.org/x/text/encoding/japanese"
	"golang.org/x/text/encoding/korean"
	"golang.org/x/text/encoding/simplifiedchinese"
	"golang.org/x/text/encoding/traditionalchinese"
	"golang.org/x/text/encoding/unicode"
	"golang.org/x/text/transform"
)

// Encodings 是导入时可选的文字编码，第一项是默认的 UTF-8。
var Encodings = []string{
	"UTF-8", "GBK", "GB18030", "Big5", "Shift_JIS", "EUC-JP", "EUC-KR",
	"UTF-16LE", "UTF-16BE", "Windows-1252", "ISO-8859-1",
}

// decoderFor 返回把 name 编码的字节转成 UTF-8 的转换器；UTF-8（或空）返回 nil。
func decoderFor(name string) (transform.Transformer, error) {
	var e encoding.Encoding
	switch strings.ToUpper(strings.TrimSpace(name)) {
	case "", "UTF-8", "UTF8":
		return nil, nil
	case "GBK":
		e = simplifiedchinese.GBK
	case "GB18030":
		e = simplifiedchinese.GB18030
	case "BIG5":
		e = traditionalchinese.Big5
	case "SHIFT_JIS", "SJIS":
		e = japanese.ShiftJIS
	case "EUC-JP":
		e = japanese.EUCJP
	case "EUC-KR":
		e = korean.EUCKR
	case "UTF-16LE":
		e = unicode.UTF16(unicode.LittleEndian, unicode.UseBOM)
	case "UTF-16BE":
		e = unicode.UTF16(unicode.BigEndian, unicode.UseBOM)
	case "WINDOWS-1252":
		e = charmap.Windows1252
	case "ISO-8859-1", "LATIN1":
		e = charmap.ISO8859_1
	default:
		return nil, fmt.Errorf("不支持的编码: %s", name)
	}
	return e.NewDecoder(), nil
}
