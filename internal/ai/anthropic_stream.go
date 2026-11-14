package ai

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// apiStatusError 非 OpenAI 通道返回的 HTTP 错误；IsRateLimit 靠它识别 429 / 529。
type apiStatusError struct {
	Status  int
	Message string
}

func (e *apiStatusError) Error() string {
	return fmt.Sprintf("Anthropic 请求失败 (HTTP %d): %s", e.Status, e.Message)
}

// 重试参数：变量是为了让测试把等待缩短。
var (
	anthropicRetryBase = time.Second
	anthropicRetries   = 3
	anthropicHTTP      = &http.Client{}
)

// anthropicRound 一轮流式响应的产出。
type anthropicRound struct {
	text       string
	calls      []FunctionToolCall
	stopReason string
}

// streamAnthropic 发起一次流式 Messages 请求并读完整个响应：文字与推理增量实时转给回调，
// 工具调用在流正常结束（message_stop）后才返回，半截的流不会触发工具执行。
func streamAnthropic(ctx context.Context, cfg *Config, body map[string]any, cb ToolStreamCallbacks) (anthropicRound, error) {
	payload, err := json.Marshal(body)
	if err != nil {
		return anthropicRound{}, err
	}
	ctx, cancel := context.WithTimeout(ctx, 10*time.Minute)
	defer cancel()
	resp, err := openAnthropic(ctx, cfg, payload)
	if err != nil {
		return anthropicRound{}, err
	}
	defer resp.Body.Close()
	return readAnthropicStream(resp.Body, cb)
}

// openAnthropic 建立连接；限流、服务端错误和网络错误按指数退避重试（响应还没开始，重试是安全的）。
func openAnthropic(ctx context.Context, cfg *Config, payload []byte) (*http.Response, error) {
	url := strings.TrimRight(strings.TrimSpace(cfg.BaseURL), "/")
	if url == "" {
		url = anthropicDefaultBase
	}
	url += "/messages"
	for attempt := 0; ; attempt++ {
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(payload))
		if err != nil {
			return nil, err
		}
		req.Header.Set("content-type", "application/json")
		req.Header.Set("anthropic-version", anthropicVersion)
		if key := strings.TrimSpace(cfg.APIKey); key != "" {
			req.Header.Set("x-api-key", key)
		}
		for k, v := range cfg.Headers { // 用户的请求头最后设置，可以覆盖默认值（例如 anthropic-beta）
			if strings.TrimSpace(k) != "" {
				req.Header.Set(k, v)
			}
		}
		resp, err := anthropicHTTP.Do(req)
		var wait time.Duration
		switch {
		case err != nil:
			if ctx.Err() != nil || attempt >= anthropicRetries {
				return nil, fmt.Errorf("AI 请求失败: %w", err)
			}
		case resp.StatusCode/100 == 2:
			return resp, nil
		default:
			apiErr := readAnthropicError(resp)
			if !retryableStatus(resp.StatusCode) || attempt >= anthropicRetries {
				return nil, apiErr
			}
			if secs, e := strconv.Atoi(resp.Header.Get("Retry-After")); e == nil && secs > 0 {
				wait = min(time.Duration(secs)*time.Second, 30*time.Second)
			}
		}
		if wait == 0 {
			wait = anthropicRetryBase << attempt
		}
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(wait):
		}
	}
}

func retryableStatus(code int) bool {
	return code == http.StatusTooManyRequests || code >= 500 // 529 是 Anthropic 的"过载"
}

// readAnthropicError 读出错误响应体里的 message，并关闭响应。
func readAnthropicError(resp *http.Response) error {
	defer resp.Body.Close()
	raw, _ := io.ReadAll(io.LimitReader(resp.Body, 16<<10))
	var parsed struct {
		Error struct{ Message string } `json:"error"`
	}
	msg := strings.TrimSpace(string(raw))
	if json.Unmarshal(raw, &parsed) == nil && parsed.Error.Message != "" {
		msg = parsed.Error.Message
	}
	return &apiStatusError{Status: resp.StatusCode, Message: msg}
}

// sseEvent 是流里各类事件字段的并集。
type sseEvent struct {
	Type         string `json:"type"`
	Index        int    `json:"index"`
	ContentBlock struct {
		Type, ID, Name string
	} `json:"content_block"`
	Delta struct {
		Type        string `json:"type"`
		Text        string `json:"text"`
		Thinking    string `json:"thinking"`
		PartialJSON string `json:"partial_json"`
		StopReason  string `json:"stop_reason"`
	} `json:"delta"`
	Message struct {
		Usage anthropicUsage `json:"usage"`
	} `json:"message"`
	Usage anthropicUsage `json:"usage"`
	Error struct{ Type, Message string }
}

type anthropicUsage struct {
	Input         int64 `json:"input_tokens"`
	Output        int64 `json:"output_tokens"`
	CacheCreation int64 `json:"cache_creation_input_tokens"`
	CacheRead     int64 `json:"cache_read_input_tokens"`
}

// readAnthropicStream 解析 SSE：只看 data 行，事件类型取自 JSON 里的 type 字段。
func readAnthropicStream(r io.Reader, cb ToolStreamCallbacks) (anthropicRound, error) {
	var res anthropicRound
	var text strings.Builder
	var usage Usage
	calls := map[int]*FunctionToolCall{} // 内容块下标 → 工具调用
	var order []int
	finished := false

	br := bufio.NewReader(r)
	for !finished {
		line, err := br.ReadString('\n')
		if data, ok := strings.CutPrefix(strings.TrimSpace(line), "data:"); ok {
			var ev sseEvent
			if json.Unmarshal([]byte(strings.TrimSpace(data)), &ev) == nil {
				switch ev.Type {
				case "message_start":
					u := ev.Message.Usage
					usage.InputTokens = u.Input + u.CacheCreation + u.CacheRead
				case "content_block_start":
					if ev.ContentBlock.Type == "tool_use" {
						calls[ev.Index] = &FunctionToolCall{ID: ev.ContentBlock.ID, CallID: ev.ContentBlock.ID, Name: ev.ContentBlock.Name}
						order = append(order, ev.Index)
					}
				case "content_block_delta":
					switch ev.Delta.Type {
					case "text_delta":
						text.WriteString(ev.Delta.Text)
						cb.delta(ev.Delta.Text)
					case "thinking_delta":
						cb.thinking(ev.Delta.Thinking)
					case "input_json_delta":
						if c := calls[ev.Index]; c != nil {
							c.Arguments += ev.Delta.PartialJSON
						}
					}
				case "message_delta":
					if ev.Delta.StopReason != "" {
						res.stopReason = ev.Delta.StopReason
					}
					usage.OutputTokens = ev.Usage.Output
				case "message_stop":
					finished = true
				case "error":
					return anthropicRound{}, streamError(ev.Error.Type, ev.Error.Message)
				}
			}
		}
		if err != nil {
			if !finished {
				if errors.Is(err, io.EOF) {
					return anthropicRound{}, fmt.Errorf("AI 流式响应失败: 连接在响应结束前中断")
				}
				return anthropicRound{}, fmt.Errorf("AI 流式响应失败: %w", err)
			}
			break
		}
	}
	for _, idx := range order {
		c := calls[idx]
		if strings.TrimSpace(c.Arguments) == "" {
			c.Arguments = "{}"
		}
		res.calls = append(res.calls, *c)
	}
	usage.TotalTokens = usage.InputTokens + usage.OutputTokens
	cb.usage(usage)
	res.text = text.String()
	return res, nil
}

// streamError 把流中途的错误事件转成带状态码的错误，让限流、过载走统一的识别逻辑。
func streamError(kind, msg string) error {
	status := http.StatusInternalServerError
	switch kind {
	case "rate_limit_error":
		status = http.StatusTooManyRequests
	case "overloaded_error":
		status = 529
	}
	if msg == "" {
		msg = kind
	}
	return &apiStatusError{Status: status, Message: msg}
}
