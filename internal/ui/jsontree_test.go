package ui

import (
	"encoding/json"
	"testing"
)

func TestParseJSONValue(t *testing.T) {
	v, ok := parseJSONValue(` {"a": 12345678901234567890, "b": [1, "x"]} `)
	if !ok {
		t.Fatal("应当能解析")
	}
	m := v.(map[string]any)
	if n, isNum := m["a"].(json.Number); !isNum || n.String() != "12345678901234567890" {
		t.Errorf("大整数应原样保留: %v", m["a"])
	}
	for _, bad := range []string{"", "{", `{"a":1} x`, "plain"} {
		if _, ok := parseJSONValue(bad); ok {
			t.Errorf("%q 不应当是 JSON", bad)
		}
	}
}

func TestJSONPathKey(t *testing.T) {
	cases := map[string]string{"name": "$.name", "a_1": "$.a_1", "a b": `$["a b"]`, "1x": `$["1x"]`, `q"t`: `$["q\"t"]`, "": `$[""]`}
	for k, want := range cases {
		if got := jsonPathKey("$", k); got != want {
			t.Errorf("%q: got %s want %s", k, got, want)
		}
	}
	if got := jsonPathKey("$.a[0]", "b"); got != "$.a[0].b" {
		t.Errorf("嵌套: %s", got)
	}
}

func TestJSONValueText(t *testing.T) {
	v, _ := parseJSONValue(`{"s":"hi","n":1.50,"o":{"k":[1,2]},"z":null,"t":true}`)
	m := v.(map[string]any)
	for key, want := range map[string]string{"s": "hi", "n": "1.50", "z": "null", "t": "true"} {
		if got := jsonValueText(m[key]); got != want {
			t.Errorf("%s: got %q want %q", key, got, want)
		}
	}
	if got := jsonValueText(m["o"]); got != "{\n  \"k\": [\n    1,\n    2\n  ]\n}" {
		t.Errorf("容器应缩进: %q", got)
	}
}
