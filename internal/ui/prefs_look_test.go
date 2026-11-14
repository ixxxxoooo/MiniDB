package ui

import (
	"encoding/json"
	"testing"
)

// @author ixxxxoooo

// 外观偏好要真正作用到全局的绘制开关上，并且重启后（JSON 往返）还在。
func TestLookPrefsTakeEffect(t *testing.T) {
	e := newEnv(t)
	defer func() { // 不污染别的测试
		e.sh.prefs = uiPrefs{}
		e.sh.applyRuntimePrefs()
	}()
	pf := &e.sh.prefs
	pf.Scheme, pf.Comfy, pf.NumLeft, pf.PlainNull, pf.Zebra = "paper", true, true, true, true
	e.sh.applyRuntimePrefs()
	if activeScheme != schemePaper || gridRowH != sizeRowComfy || numRightAlign || nullStyled || !showZebra {
		t.Fatalf("外观偏好没有生效: %v %v %v %v %v", activeScheme, gridRowH, numRightAlign, nullStyled, showZebra)
	}
	raw, err := json.Marshal(*pf)
	if err != nil {
		t.Fatal(err)
	}
	var back uiPrefs
	if err := json.Unmarshal(raw, &back); err != nil || back.Scheme != "paper" || !back.Comfy || !back.NumLeft || !back.PlainNull || !back.Zebra {
		t.Fatalf("JSON 往返丢了字段: %s %v", raw, err)
	}
	*pf = uiPrefs{}
	e.sh.applyRuntimePrefs()
	if activeScheme != schemeGraphite || gridRowH != sizeRow || !numRightAlign || !nullStyled || showZebra {
		t.Fatal("默认值应回到石墨、紧凑、右对齐、NULL 斜体、细行线")
	}
}

// 设置页的外观页要有设计稿里的新选项，点击后写进偏好。
func TestAppearanceSettingsLookOptions(t *testing.T) {
	e := newEnv(t)
	defer func() {
		e.sh.prefs = uiPrefs{}
		e.sh.applyRuntimePrefs()
	}()
	e.sh.openSettings(setAppearance)
	e.tt.Frame()
	for _, s := range []string{"配色", "石墨", "纸墨", "极光", "界面密度", "紧凑", "标准", "表格行样式", "细行线", "斑马纹", "数字右对齐", "NULL 显示为灰色斜体"} {
		e.has(s)
	}
	e.shot("p7-settings-look")
	e.tt.Click("纸墨")
	if e.sh.prefs.Scheme != "paper" || activeScheme != schemePaper {
		t.Fatalf("点纸墨应切换配色: %q", e.sh.prefs.Scheme)
	}
	e.tt.Click("标准")
	if !e.sh.prefs.Comfy || gridRowH != sizeRowComfy {
		t.Fatal("点标准应把行高调成 30")
	}
	e.tt.Click("斑马纹")
	if !e.sh.prefs.Zebra || !showZebra {
		t.Fatal("点斑马纹应打开斑马纹")
	}
}
