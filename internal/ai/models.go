package ai

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strings"
	"time"
)

// modelsTimeout 拉取模型列表的超时；这是设置页里用户在等的一次点击，不能像对话那样放宽到十分钟。
const modelsTimeout = 20 * time.Second

// ListModels 向服务商的 /models 接口拉取可用模型，返回去重、排好序的模型 ID。
// OpenAI 兼容服务用 Bearer 鉴权，Anthropic 原生协议用 x-api-key；用户配置的请求头最后设置，可以覆盖默认值。
//
// @author ixxxxoooo
//
// 参数：cfg 为服务配置（用到 Provider、BaseURL、APIKey、Headers）。
// 返回：模型 ID 列表；网络错误、非 2xx 响应、响应格式不识别时返回错误。
func ListModels(ctx context.Context, cfg *Config) ([]string, error) {
	ctx, cancel := context.WithTimeout(ctx, modelsTimeout)
	defer cancel()
	anthropic := strings.EqualFold(strings.TrimSpace(cfg.Provider), ProviderAnthropic)
	base := strings.TrimRight(strings.TrimSpace(cfg.BaseURL), "/")
	if base == "" && anthropic {
		base = anthropicDefaultBase
	}
	if base == "" {
		return nil, fmt.Errorf("服务地址为空")
	}
	var all []string
	after := ""
	for range 10 { // Anthropic 分页；10 页（每页 1000）足够，也防止服务端异常时死循环
		url := base + "/models"
		if anthropic {
			url += "?limit=1000"
			if after != "" {
				url += "&after_id=" + after
			}
		}
		ids, more, last, err := fetchModelPage(ctx, cfg, url, anthropic)
		if err != nil {
			return nil, err
		}
		all = append(all, ids...)
		if !more || last == "" {
			break
		}
		after = last
	}
	return normalizeModelIDs(all), nil
}

// fetchModelPage 请求一页模型列表；more 与 last 只对 Anthropic 的分页有意义。
func fetchModelPage(ctx context.Context, cfg *Config, url string, anthropic bool) (ids []string, more bool, last string, err error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, false, "", err
	}
	key := strings.TrimSpace(cfg.APIKey)
	if anthropic {
		req.Header.Set("anthropic-version", anthropicVersion)
		if key != "" {
			req.Header.Set("x-api-key", key)
		}
	} else if key != "" {
		req.Header.Set("Authorization", "Bearer "+key)
	}
	for k, v := range cfg.Headers {
		if strings.TrimSpace(k) != "" {
			req.Header.Set(k, v)
		}
	}
	resp, err := (&http.Client{}).Do(req)
	if err != nil {
		return nil, false, "", fmt.Errorf("获取模型列表失败: %w", err)
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(io.LimitReader(resp.Body, 8<<20))
	if resp.StatusCode/100 != 2 {
		return nil, false, "", fmt.Errorf("获取模型列表失败: HTTP %d %s", resp.StatusCode, errorSnippet(raw))
	}
	return parseModelList(raw)
}

// parseModelList 解析 /models 的响应。OpenAI 与 Anthropic 都是 {"data":[{"id":..}]}；
// 少数网关直接返回数组，或把名字放在 name 里（如 Gemini 原生接口的 "models/xxx"），这里一并兼容。
func parseModelList(raw []byte) (ids []string, more bool, last string, err error) {
	type item struct {
		ID   string `json:"id"`
		Name string `json:"name"`
	}
	var wrapped struct {
		Data    []item `json:"data"`
		Models  []item `json:"models"`
		HasMore bool   `json:"has_more"`
		LastID  string `json:"last_id"`
	}
	var items []item
	if jerr := json.Unmarshal(raw, &wrapped); jerr == nil {
		items = append(wrapped.Data, wrapped.Models...)
	} else if jerr := json.Unmarshal(raw, &items); jerr != nil {
		return nil, false, "", fmt.Errorf("无法识别模型列表的响应格式")
	}
	for _, it := range items {
		id := strings.TrimSpace(it.ID)
		if id == "" {
			id = strings.TrimSpace(it.Name)
		}
		if id != "" {
			ids = append(ids, id)
		}
	}
	return ids, wrapped.HasMore, wrapped.LastID, nil
}

// normalizeModelIDs 去重并排序。
func normalizeModelIDs(ids []string) []string {
	seen := make(map[string]bool, len(ids))
	out := make([]string, 0, len(ids))
	for _, id := range ids {
		if !seen[id] {
			seen[id] = true
			out = append(out, id)
		}
	}
	sort.Strings(out)
	return out
}

// errorSnippet 从错误响应体里取一小段给用户看：优先 error.message，否则截取原文。
func errorSnippet(raw []byte) string {
	var parsed struct {
		Error struct {
			Message string `json:"message"`
		} `json:"error"`
	}
	if json.Unmarshal(raw, &parsed) == nil && parsed.Error.Message != "" {
		return parsed.Error.Message
	}
	s := strings.TrimSpace(string(raw))
	if len(s) > 200 {
		s = s[:200]
	}
	return s
}
