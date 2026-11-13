package ui

import (
	_ "embed"

	"github.com/egoist/mygo/ui"
)

// logoPNG 是应用图标（由 scripts/gen_icon.py 生成，和 resources/icon.png 同一个设计）。
//
//go:embed logo.png
var logoPNG []byte

var logoBitmap = func() *ui.Bitmap {
	b, err := ui.DecodeBitmap(logoPNG)
	if err != nil {
		panic("logo.png: " + err.Error())
	}
	return b
}()

// logo 画一枚 size×size 的应用图标。
func logo(c *ui.Context, size float32) ui.Element {
	return ui.Image(c, logoBitmap).Size(size, size).Shrink(0)
}
