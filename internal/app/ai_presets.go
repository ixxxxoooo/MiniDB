package app

import (
	"net/url"
	"strings"

	"minidb/internal/ai"
	appversion "minidb/internal/version"
)

// AIPreset 是一家 AI 服务商的预设：选中后自动填好地址、默认模型和必需的请求头。
// 大多数厂商都提供 OpenAI 兼容接口，所以一份预设就是几个字段；真正协议不同的（如 Anthropic）见 Provider。
type AIPreset struct {
	ID             string
	Name           string
	BaseURL        string
	DefaultModel   string
	DefaultHeaders map[string]string
	// AllowEmptyKey 为 true 时不填 API Key 也视为已配置（本地部署的模型不校验密钥）。
	AllowEmptyKey bool
	// Provider 是协议类型（ai.ProviderXxx），空表示 OpenAI 兼容。
	Provider string
}

// AIProviderAnthropic 是 AIConfig.Provider 里 Anthropic 原生协议的取值，供界面使用（界面不直接依赖 ai 包）。
const AIProviderAnthropic = ai.ProviderAnthropic

// 预设的标识。自定义不是一个具体的服务商：地址、模型和请求头都由用户自己填。
const (
	AIPresetOpenAI     = "openai"
	AIPresetDeepSeek   = "deepseek"
	AIPresetOpenRouter = "openrouter"
	AIPresetOllama     = "ollama"
	AIPresetGemini     = "gemini"
	AIPresetAnthropic  = "anthropic"
	AIPresetCustom     = "custom"
)

// AIPresets 按界面上显示的顺序列出全部预设。
var AIPresets = []AIPreset{
	{ID: AIPresetOpenAI, Name: "OpenAI", BaseURL: "https://api.openai.com/v1", DefaultModel: "gpt-4o"},
	{ID: AIPresetDeepSeek, Name: "DeepSeek", BaseURL: "https://api.deepseek.com/v1", DefaultModel: "deepseek-chat"},
	{
		ID: AIPresetOpenRouter, Name: "OpenRouter", BaseURL: "https://openrouter.ai/api/v1", DefaultModel: "openai/gpt-4o-mini",
		// OpenRouter 用这两个头在排行榜上标识应用
		DefaultHeaders: map[string]string{"HTTP-Referer": "https://github.com/" + appversion.Repository, "X-Title": "MiniDB"},
	},
	{ID: AIPresetOllama, Name: "Ollama", BaseURL: "http://127.0.0.1:11434/v1", DefaultModel: "qwen2.5-coder", AllowEmptyKey: true},
	{ID: AIPresetGemini, Name: "Gemini", BaseURL: "https://generativelanguage.googleapis.com/v1beta/openai/", DefaultModel: "gemini-2.5-flash"},
	{ID: AIPresetAnthropic, Name: "Anthropic", BaseURL: "https://api.anthropic.com/v1", DefaultModel: "claude-sonnet-4-5", Provider: ai.ProviderAnthropic},
	{ID: AIPresetCustom, Name: ""}, // 名字由界面本地化为「自定义」
}

// AIPresetByID 按标识取预设。
func AIPresetByID(id string) (AIPreset, bool) {
	for _, p := range AIPresets {
		if p.ID == id {
			return p, true
		}
	}
	return AIPreset{}, false
}

// AIPresetOf 判断配置对应哪个预设：优先看保存的 PresetID，没有（旧配置）就按地址匹配，都不是则为自定义。
func AIPresetOf(cfg AIConfig) AIPreset {
	if p, ok := AIPresetByID(cfg.PresetID); ok {
		return p
	}
	base := normalizeBaseURL(cfg.BaseURL)
	for _, p := range AIPresets {
		if p.BaseURL != "" && normalizeBaseURL(p.BaseURL) == base {
			return p
		}
	}
	p, _ := AIPresetByID(AIPresetCustom)
	return p
}

func normalizeBaseURL(raw string) string {
	return strings.ToLower(strings.TrimRight(strings.TrimSpace(raw), "/"))
}

// ApplyAIPreset 把预设套用到配置上：地址、默认模型、请求头换成预设的。
// 切到「自定义」时保留用户已经填的内容；切到别家时旧预设带来的请求头不会留下，用户自己加的会保留。
func ApplyAIPreset(cfg *AIConfig, id string) {
	next, ok := AIPresetByID(id)
	if !ok {
		return
	}
	prev := AIPresetOf(*cfg)
	cfg.PresetID = next.ID
	if next.ID == AIPresetCustom {
		return
	}
	cfg.BaseURL, cfg.Model, cfg.Provider = next.BaseURL, next.DefaultModel, next.Provider
	cfg.Models = nil // 模型列表是按服务商拉取的，换了服务商就不适用了
	headers := map[string]string{}
	for k, v := range cfg.Headers {
		if pv, own := prev.DefaultHeaders[k]; own && pv == v {
			continue // 旧预设带来的头，随预设一起换掉
		}
		headers[k] = v
	}
	for k, v := range next.DefaultHeaders {
		headers[k] = v
	}
	if len(headers) == 0 {
		headers = nil
	}
	cfg.Headers = headers
}

// AIKeyOptional 报告这份配置能不能不填 API Key：预设允许，或者服务在本机、内网。
func AIKeyOptional(cfg AIConfig) bool {
	if AIPresetOf(cfg).AllowEmptyKey {
		return true
	}
	u, err := url.Parse(strings.TrimSpace(cfg.BaseURL))
	return err == nil && u.Hostname() != "" && isLocalHost(u.Hostname())
}

// AIConfigured 报告 AI 是否已经配置好：有地址、有模型，并且有 API Key（本地服务可以没有）。
func AIConfigured(cfg *AIConfig) bool {
	if cfg == nil || strings.TrimSpace(cfg.BaseURL) == "" || strings.TrimSpace(cfg.Model) == "" {
		return false
	}
	return strings.TrimSpace(cfg.APIKey) != "" || AIKeyOptional(*cfg)
}

// CleanModelList 整理模型列表：去掉空白项和重复项，保持原有顺序。
//
// @author ixxxxoooo
// 参数：models 为用户保存的模型名。
// 返回：整理后的列表；没有内容时返回 nil。
func CleanModelList(models []string) []string {
	var out []string
	seen := map[string]bool{}
	for _, m := range models {
		if m = strings.TrimSpace(m); m != "" && !seen[m] {
			seen[m] = true
			out = append(out, m)
		}
	}
	return out
}
