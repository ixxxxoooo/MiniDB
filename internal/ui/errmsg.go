package ui

import (
	"errors"
	"fmt"

	"github.com/egoist/mygo/i18n"

	"minidb/internal/model"
	"minidb/locales"
)

// errMessage 把后端错误写成当前语言的文字：带错误码且语言包里有对应词条时用词条，
// 否则原样显示（驱动或系统给的报错本来就不是我们能翻译的）。
func errMessage(err error) string { return errMessageIn(i18n.Locale(), err) }

// errMessageIn 同 errMessage，但指定语言；测试用它切换语言。
func errMessageIn(locale string, err error) string {
	if err == nil {
		return ""
	}
	var e *model.Error
	if !errors.As(err, &e) {
		return err.Error()
	}
	key := errKey(e.Code)
	if _, _, ok := locales.Catalog.Message(locale, key); !ok {
		return err.Error()
	}
	args := make(map[string]any, len(e.Args))
	for i, a := range e.Args {
		args[fmt.Sprintf("a%d", i)] = a
	}
	return locales.Catalog.Format(locale, key, args)
}

// errKey 返回错误码对应的词条键："err" + 首字母大写的错误码。
func errKey(code string) string {
	if code == "" {
		return "err"
	}
	r := []rune(code)
	if r[0] >= 'a' && r[0] <= 'z' {
		r[0] -= 'a' - 'A'
	}
	return "err" + string(r)
}
