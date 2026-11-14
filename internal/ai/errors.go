package ai

import (
	"errors"
	"net/http"
	"strings"

	openai "github.com/openai/openai-go/v3"
)

// apiDetail 取出错误链里 API 返回的 HTTP 状态码与错误消息；不是 API 错误时返回 0 和空串。
// 消息只取响应体里的 message：完整的 err.Error() 带着请求 URL（含 /responses），不能拿来做关键词判断。
func apiDetail(err error) (status int, message string) {
	var apiErr *openai.Error
	if errors.As(err, &apiErr) {
		return apiErr.StatusCode, strings.ToLower(apiErr.Message)
	}
	return 0, ""
}

// IsRateLimit 判断错误是否为网关限流（429、too many requests、配额用尽）。
// 限流是暂时的：不能当成"端点不支持"去回退，也不该熔断。
func IsRateLimit(err error) bool {
	if err == nil {
		return false
	}
	if status, _ := apiDetail(err); status == http.StatusTooManyRequests {
		return true
	}
	var statusErr *apiStatusError
	if errors.As(err, &statusErr) && (statusErr.Status == http.StatusTooManyRequests || statusErr.Status == 529) {
		return true // 529 是 Anthropic 的过载，和限流一样是暂时的
	}
	msg := strings.ToLower(err.Error())
	for _, marker := range []string{"too many requests", "rate limit", "quota exceeded", "status code 429"} {
		if strings.Contains(msg, marker) {
			return true
		}
	}
	return false
}

// shouldFallbackToChat 判断 Responses 请求的失败是否说明"网关不支持这个端点"，
// 是才值得回退到 Chat Completions 并熔断一段时间。
// 网络错误、鉴权失败、限流、模型参数错误都不是回退的理由——换端点只会再失败一次。
func shouldFallbackToChat(err error) bool {
	if err == nil || IsRateLimit(err) {
		return false
	}
	status, msg := apiDetail(err)
	switch status {
	case http.StatusNotFound, http.StatusMethodNotAllowed, http.StatusNotImplemented:
		return true
	case http.StatusBadRequest:
		// "no tool call" / "previous_response"：网关不保存响应、无法续接工具输出，也属于不支持 Responses 的工具循环。
		// DeepSeek 的原文是 "No tool callfound"（少一个空格），所以只匹配前半句。
		for _, marker := range []string{"responses", "not support", "unsupported", "no tool call", "previous_response"} {
			if strings.Contains(msg, marker) {
				return true
			}
		}
		return false
	}
	return false
}
