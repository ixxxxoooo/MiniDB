import React, { useState, useEffect } from "react";
import { Input } from "@/components/ui/input";
import { Button } from "@/components/ui/button";
import { Loader2, Check, AlertCircle, Eye, EyeOff } from "lucide-react";
import type { AIConfig } from "@/types/ai";

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
  const [saveStatus, setSaveStatus] = useState<"idle" | "saving" | "saved">("idle");

  useEffect(() => {
    const stored = localStorage.getItem("tableplus-ai-config");
    if (stored) {
      try {
        setConfig(JSON.parse(stored));
      } catch {}
    }
  }, []);

  const updateField = (field: keyof AIConfig, value: string | number) => {
    setConfig((prev) => ({ ...prev, [field]: value }));
  };

  const handleSave = () => {
    setSaveStatus("saving");
    localStorage.setItem("tableplus-ai-config", JSON.stringify(config));
    setTimeout(() => setSaveStatus("saved"), 300);
    setTimeout(() => setSaveStatus("idle"), 2000);
  };

  return (
    <div className="space-y-5">
      <div>
        <h3 className="text-sm font-semibold mb-1">AI 配置</h3>
        <p className="text-xs text-[var(--fg-secondary)]">
          配置 OpenAI 兼容的 API 服务，支持任何兼容 OpenAI 格式的 API 端点
        </p>
      </div>

      {/* Base URL */}
      <div>
        <label className="text-xs font-medium text-[var(--fg-secondary)] mb-1.5 block">
          API Base URL
        </label>
        <Input
          value={config.baseURL}
          onChange={(e) => updateField("baseURL", e.target.value)}
          placeholder="https://api.openai.com/v1"
        />
        <p className="text-2xs text-[var(--fg-muted)] mt-1">
          支持 OpenAI、Azure、本地部署等兼容端点
        </p>
      </div>

      {/* API Key */}
      <div>
        <label className="text-xs font-medium text-[var(--fg-secondary)] mb-1.5 block">
          API Key
        </label>
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
            {showKey ? (
              <EyeOff className="h-4 w-4" />
            ) : (
              <Eye className="h-4 w-4" />
            )}
          </button>
        </div>
      </div>

      {/* Model */}
      <div>
        <label className="text-xs font-medium text-[var(--fg-secondary)] mb-1.5 block">
          模型
        </label>
        <Input
          value={config.model}
          onChange={(e) => updateField("model", e.target.value)}
          placeholder="gpt-4o"
        />
      </div>

      {/* 参数 */}
      <div className="flex gap-4">
        <div className="flex-1">
          <label className="text-xs font-medium text-[var(--fg-secondary)] mb-1.5 block">
            最大 Token 数
          </label>
          <Input
            type="number"
            value={config.maxTokens}
            onChange={(e) => updateField("maxTokens", Number(e.target.value))}
          />
        </div>
        <div className="flex-1">
          <label className="text-xs font-medium text-[var(--fg-secondary)] mb-1.5 block">
            Temperature
          </label>
          <Input
            type="number"
            step="0.1"
            min="0"
            max="2"
            value={config.temperature}
            onChange={(e) =>
              updateField("temperature", Number(e.target.value))
            }
          />
        </div>
      </div>

      {/* 保存按钮 */}
      <div className="flex justify-end pt-2">
        <Button size="sm" onClick={handleSave}>
          {saveStatus === "saving" && (
            <Loader2 className="h-3 w-3 mr-1.5 animate-spin" />
          )}
          {saveStatus === "saved" && <Check className="h-3 w-3 mr-1.5" />}
          {saveStatus === "idle" ? "保存" : saveStatus === "saved" ? "已保存" : "保存中..."}
        </Button>
      </div>
    </div>
  );
}
