package app

import (
	"testing"
)

func TestAIPresetsWellFormed(t *testing.T) {
	seen := map[string]bool{}
	for _, p := range AIPresets {
		if seen[p.ID] {
			t.Errorf("预设标识重复: %s", p.ID)
		}
		seen[p.ID] = true
		if p.ID == AIPresetCustom {
			continue
		}
		if err := ValidateAIBaseURL(p.BaseURL); err != nil {
			t.Errorf("预设 %s 的地址不满足隐私策略: %v", p.ID, err)
		}
		if p.DefaultModel == "" {
			t.Errorf("预设 %s 缺少默认模型", p.ID)
		}
	}
	if !seen[AIPresetCustom] {
		t.Error("缺少自定义预设")
	}
}

func TestAIPresetOf(t *testing.T) {
	cases := []struct {
		cfg  AIConfig
		want string
	}{
		{AIConfig{PresetID: AIPresetDeepSeek}, AIPresetDeepSeek},
		{AIConfig{BaseURL: "https://api.deepseek.com/v1/"}, AIPresetDeepSeek}, // 旧配置：按地址识别
		{AIConfig{BaseURL: "https://API.openai.com/v1"}, AIPresetOpenAI},
		{AIConfig{BaseURL: "https://example.com/v1"}, AIPresetCustom},
		{AIConfig{PresetID: "不存在", BaseURL: "http://127.0.0.1:11434/v1"}, AIPresetOllama},
	}
	for _, c := range cases {
		if got := AIPresetOf(c.cfg).ID; got != c.want {
			t.Errorf("AIPresetOf(%+v) = %s, 期望 %s", c.cfg, got, c.want)
		}
	}
}

func TestApplyAIPreset(t *testing.T) {
	cfg := AIConfig{BaseURL: "https://api.openai.com/v1", Model: "gpt-4o", Headers: map[string]string{"X-Mine": "1"}}
	ApplyAIPreset(&cfg, AIPresetOpenRouter)
	if cfg.PresetID != AIPresetOpenRouter || cfg.BaseURL != "https://openrouter.ai/api/v1" || cfg.Model != "openai/gpt-4o-mini" {
		t.Fatalf("套用 OpenRouter 失败: %+v", cfg)
	}
	if cfg.Headers["X-Title"] != "MiniDB" || cfg.Headers["X-Mine"] != "1" {
		t.Fatalf("请求头应含预设头并保留用户自己的: %v", cfg.Headers)
	}
	// 换到别家：OpenRouter 的头要消失，用户自己的头保留
	ApplyAIPreset(&cfg, AIPresetDeepSeek)
	if _, ok := cfg.Headers["X-Title"]; ok || cfg.Headers["X-Mine"] != "1" {
		t.Fatalf("切换预设后请求头不对: %v", cfg.Headers)
	}
	// 用户改过的预设头值视为自己的，不再清除
	ApplyAIPreset(&cfg, AIPresetOpenRouter)
	cfg.Headers["X-Title"] = "我的应用"
	ApplyAIPreset(&cfg, AIPresetOpenAI)
	if cfg.Headers["X-Title"] != "我的应用" {
		t.Fatalf("用户改过的请求头不该被清掉: %v", cfg.Headers)
	}
	// 切到自定义保留已有内容
	before := cfg
	ApplyAIPreset(&cfg, AIPresetCustom)
	if cfg.PresetID != AIPresetCustom || cfg.BaseURL != before.BaseURL || cfg.Model != before.Model {
		t.Fatalf("自定义不该改地址和模型: %+v", cfg)
	}
	// 不认识的预设不动配置
	same := cfg
	ApplyAIPreset(&cfg, "xxx")
	if cfg.PresetID != same.PresetID {
		t.Fatal("未知预设不该生效")
	}
	// 纯净切换后没有请求头时置空，避免存空 map
	clean := AIConfig{}
	ApplyAIPreset(&clean, AIPresetOllama)
	if clean.Headers != nil {
		t.Fatalf("没有请求头时应为 nil: %v", clean.Headers)
	}
}

func TestAIConfigured(t *testing.T) {
	cases := []struct {
		name string
		cfg  *AIConfig
		want bool
	}{
		{"空", nil, false},
		{"OpenAI 没填密钥", &AIConfig{BaseURL: "https://api.openai.com/v1", Model: "gpt-4o"}, false},
		{"OpenAI 填了密钥", &AIConfig{BaseURL: "https://api.openai.com/v1", Model: "gpt-4o", APIKey: " sk-x "}, true},
		{"Ollama 预设免密钥", &AIConfig{PresetID: AIPresetOllama, BaseURL: "http://127.0.0.1:11434/v1", Model: "qwen"}, true},
		{"自定义的本机地址免密钥", &AIConfig{PresetID: AIPresetCustom, BaseURL: "http://localhost:8000/v1", Model: "m"}, true},
		{"自定义的内网地址免密钥", &AIConfig{BaseURL: "http://192.168.1.5:8000/v1", Model: "m"}, true},
		{"缺模型", &AIConfig{BaseURL: "http://127.0.0.1:11434/v1", APIKey: "k"}, false},
		{"缺地址", &AIConfig{Model: "m", APIKey: "k"}, false},
		{"公网地址仍需密钥", &AIConfig{PresetID: AIPresetCustom, BaseURL: "https://example.com/v1", Model: "m"}, false},
	}
	for _, c := range cases {
		if got := AIConfigured(c.cfg); got != c.want {
			t.Errorf("%s: AIConfigured = %v, 期望 %v", c.name, got, c.want)
		}
	}
}
