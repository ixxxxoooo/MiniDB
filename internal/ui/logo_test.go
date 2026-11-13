package ui

import "testing"

// 应用图标已嵌入界面，连接列表等处用它做品牌标识（设 MINIDB_SHOTS 才会落盘截图）。
func TestLogoEmbedded(t *testing.T) {
	if w, h := logoBitmap.Size(); w != 256 || h != 256 {
		t.Fatalf("logo 尺寸 %dx%d，应为 256x256", w, h)
	}
	e := newEnv(t)
	e.sh.newConnectionDialog()
	for range 3 {
		e.tt.Frame()
	}
	e.shot("logo-connlist")
}
