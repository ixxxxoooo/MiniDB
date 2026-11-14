package app

import "testing"

func TestValidateAIBaseURL(t *testing.T) {
	ok := []string{
		"https://api.openai.com/v1",
		"http://localhost:11434/v1",
		"http://127.0.0.1:8080",
		"http://192.168.1.20:8000/v1",
		"http://10.0.0.5/v1",
	}
	for _, u := range ok {
		if err := ValidateAIBaseURL(u); err != nil {
			t.Errorf("%q 应通过: %v", u, err)
		}
	}
	bad := []string{
		"http://api.example.com/v1",   // 公网明文
		"ftp://example.com",           // 不支持的协议
		"https://user:pw@example.com", // 地址里带凭据
		"not a url",
		"https://",
	}
	for _, u := range bad {
		if err := ValidateAIBaseURL(u); err == nil {
			t.Errorf("%q 应被拒绝", u)
		}
	}
}

// "只共享库结构"的连接：读取行数据的工具根本不注册，模型调用也只会得到"未知工具"。
func TestMetadataOnlyRegistryHasNoDataTools(t *testing.T) {
	h := newAIHarness(t)
	h.env.metadataOnly = true
	reg := h.env.registry()
	for _, name := range []string{"table_sample", "column_values", "table_profile", "sql_readonly_execute"} {
		if _, ok := reg.Get(name); ok {
			t.Errorf("只共享结构时不应提供工具 %s", name)
		}
	}
	for _, name := range []string{"table_list", "table_describe", "sql_explain_plan"} {
		if _, ok := reg.Get(name); !ok {
			t.Errorf("结构类工具 %s 应保留", name)
		}
	}
}
