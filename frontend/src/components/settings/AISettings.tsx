import React, { useState, useEffect } from "react";
import { Input } from "@/components/ui/input";
import { Button } from "@/components/ui/button";
import { Loader2, Check, AlertCircle, Eye, EyeOff, TestTube2, Save } from "lucide-react";
import { cn } from "@/lib/utils";
import * as SettingsService from "../../../wailsjs/go/services/SettingsService";

interface AIConfig {
  baseURL: string;
  apiKey: string;
  model: string;
  maxTokens: number;
  temperature: number;
}

const DEFAULT_AI_CONFIG: AIConfig = {
  baseURL: "https://api.openai.com/v1",
  apiKey: "",
  model: "gpt-4o",
  maxTokens: 4096,
  temperature: 0.3,
};

export function AISettings() {
  const [config, setConfig] = useState<AIConfig>(DEFAULT_AI_CONFIG);
  const [showKey, setShowKey] = useState(false);
  const [saveStatus, setSaveStatus] = useState<"idle" | "saving" | "saved" | "error">("idle");
  const [testStatus, setTestStatus] = useState<"idle" | "testing" | "success" | "error">("idle");
  const [testResult, setTestResult] = useState("");

  useEffect(() => {
    SettingsService.GetAIConfig()
      .then((cfg: any) => {
        if (cfg) setConfig({
          baseURL: cfg.baseURL || DEFAULT_AI_CONFIG.baseURL,
          apiKey: cfg.apiKey || "",
          model: cfg.model || DEFAULT_AI_CONFIG.model,
          maxTokens: cfg.maxTokens || DEFAULT_AI_CONFIG.maxTokens,
          temperature: cfg.temperature ?? DEFAULT_AI_CONFIG.temperature,
        });
      })
      .catch(() => {});
  }, []);

  const updateField = (field: keyof AIConfig, value: string | number) => {
    setConfig((prev) => ({ ...prev, [field]: value }));
  };

  const handleSave = async () => {
    setSaveStatus("saving");
    try {
      await SettingsService.SaveAIConfig(config as any);
      setSaveStatus("saved");
      setTimeout(() => setSaveStatus("idle"), 2000);
    } catch (e) {
      setSaveStatus("error");
      setTimeout(() => setSaveStatus("idle"), 3000);
    }
  };

  const handleTest = async () => {
    setTestStatus("testing");
    setTestResult("");
    try {
      const result = await SettingsService.TestAI(config as any);
      setTestStatus("success");
      setTestResult(String(result));
    } catch (e: any) {
      setTestStatus("error");
      setTestResult(e?.message || "测试失败");
    }
  };

  return (
    <div className="space-y-4">
      <div>
        <h3 className="text-sm font-semibold mb-1">AI 配置</h3>
        <p className="text-xs text-[var(--fg-secondary)]">
          支持 OpenAI 兼容格式的 API（如 OpenAI、Kimi、DeepSeek 等）
        </p>
      </div>

      <div>
        <label className="text-xs font-medium text-[var(--fg-secondary)] mb-1.5 block">API Base URL</label>
        <Input value={config.baseURL} onChange={(e) => updateField("baseURL", e.target.value)} placeholder="https://api.openai.com/v1" />
      </div>

      <div>
        <label className="text-xs font-medium text-[var(--fg-secondary)] mb-1.5 block">API Key</label>
        <div className="relative">
          <Input
            type={showKey ? "text" : "password"}
            value={config.apiKey}
            onChange={(e) => updateField("apiKey", e.target.value)}
            placeholder="sk-..."
            className="pr-10"
          />
          <button
            className="absolute right-2 top-1/2 -translate-y-1/2 text-[var(--fg-muted)] hover:text-[var(--fg)]"
            onClick={() => setShowKey(!showKey)}
          >
            {showKey ? <EyeOff className="h-4 w-4" /> : <Eye className="h-4 w-4" />}
          </button>
        </div>
      </div>

      <div>
        <label className="text-xs font-medium text-[var(--fg-secondary)] mb-1.5 block">模型</label>
        <Input value={config.model} onChange={(e) => updateField("model", e.target.value)} placeholder="gpt-4o / kimi-k2.5" />
      </div>

      <div className="flex gap-3">
        <div className="flex-1">
          <label className="text-xs font-medium text-[var(--fg-secondary)] mb-1.5 block">Max Tokens</label>
          <Input type="number" value={config.maxTokens} onChange={(e) => updateField("maxTokens", Number(e.target.value))} />
        </div>
        <div className="flex-1">
          <label className="text-xs font-medium text-[var(--fg-secondary)] mb-1.5 block">Temperature</label>
          <Input type="number" step="0.1" min="0" max="2" value={config.temperature} onChange={(e) => updateField("temperature", Number(e.target.value))} />
        </div>
      </div>

      {/* 测试结果 */}
      {testStatus !== "idle" && (
        <div className={cn(
          "flex items-start gap-2 px-3 py-2 rounded-lg text-xs",
          { "bg-blue-50 dark:bg-blue-900/20 text-blue-600 dark:text-blue-400": testStatus === "testing",
            "bg-green-50 dark:bg-green-900/20 text-green-600 dark:text-green-400": testStatus === "success",
            "bg-red-50 dark:bg-red-900/20 text-red-600 dark:text-red-400": testStatus === "error" }
        )}>
          {testStatus === "testing" && <Loader2 className="h-3.5 w-3.5 animate-spin mt-0.5" />}
          {testStatus === "success" && <Check className="h-3.5 w-3.5 mt-0.5" />}
          {testStatus === "error" && <AlertCircle className="h-3.5 w-3.5 mt-0.5" />}
          <span className="flex-1 break-all">
            {testStatus === "testing" && "正在测试 AI 连接..."}
            {testStatus === "success" && (testResult || "连接成功！")}
            {testStatus === "error" && (testResult || "连接失败")}
          </span>
        </div>
      )}

      {/* 按钮区 */}
      <div className="flex items-center gap-2 pt-1">
        <Button variant="outline" size="sm" onClick={handleTest} disabled={!config.apiKey || testStatus === "testing"}>
          <TestTube2 className="h-3.5 w-3.5 mr-1.5" />
          {testStatus === "testing" ? "测试中..." : "测试连接"}
        </Button>
        <div className="flex-1" />
        <Button size="sm" onClick={handleSave}>
          {saveStatus === "saving" ? <Loader2 className="h-3.5 w-3.5 mr-1.5 animate-spin" /> : <Save className="h-3.5 w-3.5 mr-1.5" />}
          {saveStatus === "idle" ? "保存" : saveStatus === "saved" ? "已保存 ✓" : saveStatus === "error" ? "保存失败" : "保存中..."}
        </Button>
      </div>
    </div>
  );
}
