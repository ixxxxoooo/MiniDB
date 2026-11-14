package ui

import (
	"testing"
	"time"

	"minidb/internal/app"
	"minidb/internal/model"
)

func TestSettingsPagesRender(t *testing.T) {
	e := newEnv(t)
	want := map[int][]string{
		setGeneral:    {"恢复默认设置"},
		setData:       {"斑马纹", "显示行号"},
		setEditor:     {"运行"},
		setCSV:        {"分隔符"},
		setConn:       {"连接保活", "查询超时"},
		setAppearance: {"编辑器字号", "浅色"},
		setSecurity:   {"新连接的安全模式"},
	}
	for tab, texts := range want {
		e.sh.openSettings(tab)
		e.tt.Frame()
		for _, s := range texts {
			e.has(s)
		}
	}
}

func TestSettingsPrefsTakeEffect(t *testing.T) {
	e := newEnv(t)
	pf := &e.sh.prefs
	pf.NoZebra, pf.QueryTimeout, pf.KeepaliveSec = true, 60, 300
	e.sh.applyRuntimePrefs()
	if showZebra {
		t.Fatal("斑马纹应关闭")
	}
	if got := e.sh.app.Connections.KeepaliveInterval(); got != 5*time.Minute {
		t.Fatalf("保活间隔 %v", got)
	}
	pf.KeepaliveSec = -1
	e.sh.applyRuntimePrefs()
	if e.sh.app.Connections.KeepaliveInterval() != 0 {
		t.Fatal("保活应关闭")
	}

	// 新建连接预选设置里的安全模式
	pf.DefaultSafe = string(model.SafeModeAlert2)
	e.sh.newConnection()
	if got := safeModeFromLabel(e.sh.dlg.safeSel); got != model.SafeModeAlert2 || !e.sh.dlg.safeSet {
		t.Fatalf("新连接应预选提醒 2，实际 %v", got)
	}

	// 恢复默认
	e.sh.set.theme = 2
	e.sh.resetPrefs()
	if e.sh.prefs.QueryTimeout != 0 || !showZebra || e.sh.set.theme != 0 || e.sh.app.Connections.KeepaliveInterval() != 30*time.Second {
		t.Fatalf("恢复默认没有生效: %+v", e.sh.prefs)
	}
}

func TestDurationLabel(t *testing.T) {
	cases := []struct {
		sec  int
		zero string
		want string
	}{{-1, "z", "关闭"}, {0, "不限", "不限"}, {30, "", "30 秒"}, {300, "", "5 分钟"}, {90, "", "90 秒"}}
	for _, c := range cases {
		if got := durationLabel(c.sec, c.zero); got != c.want {
			t.Errorf("durationLabel(%d)=%q want %q", c.sec, got, c.want)
		}
	}
}

func TestAIFormRoundTrip(t *testing.T) {
	var f aiForm
	f.load(app.AIConfig{MaxTokens: 2048, Temperature: 0.3, Headers: map[string]string{"B": "2", "A": "1"}})
	if len(f.headers) != 2 || f.headers[0].key != "A" || f.maxTok != "2048" || f.temp != "0.3" {
		t.Fatalf("载入不对: %+v", f)
	}
	f.headers = append(f.headers, headerRow{key: "  "}) // 空名字的行要被丢弃
	var cfg app.AIConfig
	if !f.apply(&cfg) || cfg.MaxTokens != 2048 || cfg.Temperature != 0.3 || len(cfg.Headers) != 2 || cfg.Headers["B"] != "2" {
		t.Fatalf("写回不对: %+v", cfg)
	}
	// 清空输入框表示用模型默认值
	f.maxTok, f.temp = "", " "
	if !f.apply(&cfg) || cfg.MaxTokens != 0 || cfg.Temperature != 0 {
		t.Fatalf("留空应为默认: %+v", cfg)
	}
	for _, bad := range [][2]string{{"abc", ""}, {"-1", ""}, {"", "3"}, {"", "x"}} {
		f.maxTok, f.temp = bad[0], bad[1]
		if f.apply(&cfg) {
			t.Errorf("%v 应判为非法", bad)
		}
	}
}

func TestAISettingsPresetSwitch(t *testing.T) {
	e := newEnv(t)
	e.sh.openSettings(setAI)
	e.tt.Frame()
	e.has("服务商")
	e.has("自定义请求头")

	d := &e.sh.set
	d.aiForm.maxTok = "512"
	e.sh.choosePreset(app.AIPresetOpenRouter)
	e.tt.Frame()
	e.shot("62-settings-ai")
	if d.cfg.BaseURL != "https://openrouter.ai/api/v1" || len(d.aiForm.headers) != 2 || d.aiForm.maxTok != "512" {
		t.Fatalf("切换到 OpenRouter 后状态不对: %+v %+v", d.cfg, d.aiForm)
	}
	e.tt.Frame()
	e.has("删除") // 请求头行已渲染出来（输入框内容不在文本列表里）
	e.sh.choosePreset(app.AIPresetOllama)
	if len(d.aiForm.headers) != 0 || !app.AIKeyOptional(d.cfg) {
		t.Fatalf("切换到 Ollama 后状态不对: %+v %+v", d.cfg, d.aiForm)
	}
	e.tt.Frame()
	e.has("本地服务可以不填 API Key")
}
