package app

import (
	"net/netip"
	"net/url"
	"strings"

	"minidb/internal/model"
)

// ValidateAIBaseURL 校验 AI 服务地址。API Key 和库里的数据都会发往这个地址，
// 所以必须是 https；只有本机回环和内网地址（本地部署的模型、公司内网网关）允许用 http。
func ValidateAIBaseURL(raw string) error {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || u.Host == "" || u.Hostname() == "" {
		return model.Errf(model.CodeAIBaseURLInvalid, "AI 服务地址格式不正确: %s", raw)
	}
	if u.User != nil {
		return model.Errf(model.CodeAIBaseURLInvalid, "AI 服务地址里不能带用户名密码，请填到 API Key 或请求头里")
	}
	switch strings.ToLower(u.Scheme) {
	case "https":
		return nil
	case "http":
		if isLocalHost(u.Hostname()) {
			return nil
		}
		return model.Errf(model.CodeAIBaseURLInsecure, "AI 服务地址必须使用 https：http 会明文传输 API Key 和库里的数据（仅本机与内网地址允许 http）")
	}
	return model.Errf(model.CodeAIBaseURLInvalid, "AI 服务地址只支持 http(s): %s", raw)
}

// isLocalHost 判断主机是不是本机或内网：localhost、*.local、回环、私有网段、链路本地地址。
func isLocalHost(host string) bool {
	h := strings.ToLower(strings.TrimSuffix(host, "."))
	if h == "localhost" || strings.HasSuffix(h, ".localhost") || strings.HasSuffix(h, ".local") {
		return true
	}
	ip, err := netip.ParseAddr(strings.Trim(h, "[]"))
	if err != nil {
		return false
	}
	return ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast()
}
