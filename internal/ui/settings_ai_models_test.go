package ui

import (
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"

	"minidb/internal/app"
)

func TestModelRowsAndToggle(t *testing.T) {
	saved := []string{"b", "z"}
	fetched := []string{"a", "b", "c", "Z-Pro"}
	if got := modelRows(saved, fetched, ""); !reflect.DeepEqual(got, []string{"b", "z", "a", "c", "Z-Pro"}) {
		t.Fatalf("已保存的应排在前面且不重复: %v", got)
	}
	if got := modelRows(saved, fetched, " z"); !reflect.DeepEqual(got, []string{"z", "Z-Pro"}) {
		t.Fatalf("筛选应忽略大小写: %v", got)
	}
	cfg := app.AIConfig{Models: []string{"b"}}
	toggleModel(&cfg, "c", true)
	toggleModel(&cfg, "c", true) // 重复勾选不应出现两份
	toggleModel(&cfg, "b", false)
	toggleModel(&cfg, "nope", false)
	if !reflect.DeepEqual(cfg.Models, []string{"c"}) {
		t.Fatalf("勾选/取消不对: %v", cfg.Models)
	}
}

// 获取模型 → 勾选 → 保存：整条流程走真实的设置页，服务商用本机假服务。
func TestAISettingsFetchAndPickModels(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"data":[{"id":"m-alpha"},{"id":"m-beta"}]}`))
	}))
	defer srv.Close()
	e := newEnv(t)
	e.sh.openSettings(setAI)
	e.tt.Frame()
	d := &e.sh.set
	d.cfg.BaseURL, d.cfg.PresetID = srv.URL, app.AIPresetCustom

	e.sh.fetchModels()
	for i := 0; i < 200 && d.aiForm.models.fetching; i++ {
		sleepMs(10)
		e.tt.Frame()
	}
	e.tt.Frame()
	if !d.aiForm.models.open || len(d.aiForm.models.fetched) != 2 {
		t.Fatalf("应展开勾选面板: %+v", d.aiForm.models)
	}
	e.has("m-alpha")
	e.has("m-beta")
	e.shot("63-settings-ai-models")

	toggleModel(&d.cfg, "m-beta", true)
	cfg, ok := e.sh.currentAIConfig()
	if !ok {
		t.Fatal("配置应合法")
	}
	e.sh.saveAIConfig(cfg)
	var saved *app.AIConfig
	for i := 0; i < 200; i++ {
		sleepMs(10)
		e.tt.Frame()
		if saved, _ = e.sh.app.Settings.GetAIConfig(); saved != nil && len(saved.Models) > 0 {
			break
		}
	}
	if saved == nil || !reflect.DeepEqual(saved.Models, []string{"m-beta"}) {
		t.Fatalf("勾选的模型应随配置保存: %+v", saved)
	}
	// 换服务商后，之前的模型列表不再适用
	e.sh.choosePreset(app.AIPresetOpenAI)
	if len(d.cfg.Models) != 0 || d.aiForm.models.open {
		t.Fatalf("切换服务商应清空模型列表: %+v", d.cfg.Models)
	}
}

// 拉取失败时在输入框下面显示原因，不展开面板。
func TestAISettingsFetchModelsError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(401)
		_, _ = w.Write([]byte(`{"error":{"message":"bad key"}}`))
	}))
	defer srv.Close()
	e := newEnv(t)
	e.sh.openSettings(setAI)
	e.tt.Frame()
	d := &e.sh.set
	d.cfg.BaseURL = srv.URL
	e.sh.fetchModels()
	for i := 0; i < 200 && d.aiForm.models.fetching; i++ {
		sleepMs(10)
		e.tt.Frame()
	}
	if d.aiForm.models.open || d.aiForm.models.err == "" {
		t.Fatalf("应显示错误而不是面板: %+v", d.aiForm.models)
	}
	e.has("bad key")
}

// 已保存的模型出现在输入框的下拉里，点一项就填入；手写的内容不会被改回去。
func TestAISettingsModelDropdown(t *testing.T) {
	e := newEnv(t)
	e.sh.openSettings(setAI)
	e.tt.Frame()
	d := &e.sh.set
	d.cfg.PresetID = app.AIPresetCustom // 自定义服务多一行协议选择，输入框的位置固定下来
	d.cfg.Models = []string{"gpt-4o-mini", "o3"}
	e.tt.Frame()
	e.tt.ClickAt(600, 548)
	e.tt.Frame()
	e.has("gpt-4o-mini")
	e.shot("64-settings-ai-model-dropdown")
	if err := e.tt.Click("o3"); err != nil {
		t.Fatal(err)
	}
	e.tt.Frame()
	if d.cfg.Model != "o3" {
		t.Fatalf("点选后应填入: %q", d.cfg.Model)
	}
	d.cfg.Model = "my-own-model" // 手写
	e.tt.Frame()
	e.tt.Frame()
	if d.cfg.Model != "my-own-model" {
		t.Fatalf("手写的模型名不应被改动: %q", d.cfg.Model)
	}
}
