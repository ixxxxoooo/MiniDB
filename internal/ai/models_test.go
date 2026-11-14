package ai

import (
	"context"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"
)

// OpenAI 兼容：Bearer 鉴权、自定义请求头、结果去重并排序。
func TestListModelsOpenAICompat(t *testing.T) {
	var gotAuth, gotCustom, gotPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth, gotCustom, gotPath = r.Header.Get("Authorization"), r.Header.Get("X-Title"), r.URL.Path
		_, _ = w.Write([]byte(`{"object":"list","data":[{"id":"gpt-4o"},{"id":"a-model"},{"id":"gpt-4o"},{"id":" "}]}`))
	}))
	defer srv.Close()
	got, err := ListModels(context.Background(), &Config{BaseURL: srv.URL + "/v1/", APIKey: "sk-x", Headers: map[string]string{"X-Title": "MiniDB"}})
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got, []string{"a-model", "gpt-4o"}) {
		t.Fatalf("结果不对: %v", got)
	}
	if gotAuth != "Bearer sk-x" || gotCustom != "MiniDB" || gotPath != "/v1/models" {
		t.Fatalf("请求不对: auth=%q custom=%q path=%q", gotAuth, gotCustom, gotPath)
	}
}

// Anthropic：x-api-key + 版本头，并跟着 has_more 翻页。
func TestListModelsAnthropicPaging(t *testing.T) {
	var afters []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("x-api-key") != "sk-ant" || r.Header.Get("anthropic-version") != anthropicVersion || r.Header.Get("Authorization") != "" {
			t.Errorf("鉴权头不对: %v", r.Header)
		}
		afters = append(afters, r.URL.Query().Get("after_id"))
		if r.URL.Query().Get("after_id") == "" {
			_, _ = w.Write([]byte(`{"data":[{"id":"claude-b"}],"has_more":true,"last_id":"claude-b"}`))
			return
		}
		_, _ = w.Write([]byte(`{"data":[{"id":"claude-a"}],"has_more":false,"last_id":"claude-a"}`))
	}))
	defer srv.Close()
	got, err := ListModels(context.Background(), &Config{Provider: ProviderAnthropic, BaseURL: srv.URL + "/v1", APIKey: "sk-ant"})
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got, []string{"claude-a", "claude-b"}) || !reflect.DeepEqual(afters, []string{"", "claude-b"}) {
		t.Fatalf("分页结果不对: %v afters=%v", got, afters)
	}
}

func TestListModelsErrors(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(401)
		_, _ = w.Write([]byte(`{"error":{"message":"invalid api key"}}`))
	}))
	defer srv.Close()
	if _, err := ListModels(context.Background(), &Config{BaseURL: srv.URL}); err == nil || !strings.Contains(err.Error(), "401") || !strings.Contains(err.Error(), "invalid api key") {
		t.Fatalf("应带上状态码与服务商的错误消息: %v", err)
	}
	if _, err := ListModels(context.Background(), &Config{}); err == nil {
		t.Fatal("地址为空应报错")
	}
}

func TestParseModelListShapes(t *testing.T) {
	cases := map[string][]string{
		`[{"id":"x"},{"id":"y"}]`:                       {"x", "y"},                // 直接返回数组
		`{"models":[{"name":"models/gemini-2.5-pro"}]}`: {"models/gemini-2.5-pro"}, // 名字放在 name 里
		`{"data":[]}`: nil,
	}
	for raw, want := range cases {
		got, _, _, err := parseModelList([]byte(raw))
		if err != nil || !reflect.DeepEqual(got, want) {
			t.Errorf("%s => %v %v, want %v", raw, got, err, want)
		}
	}
	if _, _, _, err := parseModelList([]byte(`<html>`)); err == nil {
		t.Error("非 JSON 应报错")
	}
}
