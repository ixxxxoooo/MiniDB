import React, { useState, useEffect } from "react";
import { Input } from "@/components/ui/input";
import { Button } from "@/components/ui/button";
import { Loader2, Check, AlertCircle, Eye, EyeOff, TestTube2, Save } from "lucide-react";
import { cn } from "@/lib/utils";
import { useTranslation } from "@/i18n";
import * as SettingsService from "../../../wailsjs/go/services/SettingsService";

interface AIConfig {
  baseURL: string;
  apiKey: string;
  model: string;
  systemPrompt: string;
}

function getDefaultSystemPrompt(locale: string) {
  if (locale === "en-US") {
    return "Please answer in English. Keep responses concise, practical, and SQL-first for database tasks.";
  }
  return "请使用简体中文回答。对于数据库问题优先给出可执行 SQL，并简要说明关键风险与注意事项。";
}

const DEFAULT_AI_CONFIG: AIConfig = {
  baseURL: "https://api.openai.com/v1",
  apiKey: "",
  model: "gpt-4o",
  systemPrompt: getDefaultSystemPrompt("zh-CN"),
};

export function AISettings() {
  const { t, locale } = useTranslation();
  const [config, setConfig] = useState<AIConfig>({
    ...DEFAULT_AI_CONFIG,
    systemPrompt: getDefaultSystemPrompt(locale),
  });
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
          systemPrompt: cfg.systemPrompt || getDefaultSystemPrompt(locale),
        });
      })
      .catch(() => {});
  }, [locale]);

  const updateField = (field: keyof AIConfig, value: string) => {
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
      setTestResult(e?.message || t("aiSettings.testFailed"));
    }
  };

  const getSaveLabel = () => {
    switch (saveStatus) {
      case "saving": return t("aiSettings.saving");
      case "saved": return t("aiSettings.saved");
      case "error": return t("aiSettings.saveFailed");
      default: return t("common.save");
    }
  };

  return (
    <div className="space-y-[var(--size-gap)]">
      <div>
        <h3 className="text-[length:var(--size-font-xs)] font-semibold mb-0.5">{t("aiSettings.title")}</h3>
        <p className="text-[length:var(--size-font-2xs)] text-[var(--fg-secondary)]">
          {t("aiSettings.description")}
        </p>
      </div>

      <div>
        <label className="text-[length:var(--size-font-2xs)] font-medium text-[var(--fg-secondary)] mb-1 block">{t("aiSettings.baseURL")}</label>
        <Input className="h-[var(--size-input)] text-[length:var(--size-font-xs)]" value={config.baseURL} onChange={(e) => updateField("baseURL", e.target.value)} placeholder={t("aiSettings.baseURLPlaceholder")} />
      </div>

      <div>
        <label className="text-[length:var(--size-font-2xs)] font-medium text-[var(--fg-secondary)] mb-1 block">{t("aiSettings.apiKey")}</label>
        <div className="relative">
          <Input
            className="h-[var(--size-input)] text-[length:var(--size-font-xs)] pr-8"
            type={showKey ? "text" : "password"}
            value={config.apiKey}
            onChange={(e) => updateField("apiKey", e.target.value)}
            placeholder={t("aiSettings.apiKeyPlaceholder")}
          />
          <button
            className="absolute right-2 top-1/2 -translate-y-1/2 text-[var(--fg-muted)] hover:text-[var(--fg)]"
            onClick={() => setShowKey(!showKey)}
          >
            {showKey ? <EyeOff className="h-3 w-3" /> : <Eye className="h-3 w-3" />}
          </button>
        </div>
      </div>

      <div>
        <label className="text-[length:var(--size-font-2xs)] font-medium text-[var(--fg-secondary)] mb-1 block">{t("aiSettings.model")}</label>
        <Input className="h-[var(--size-input)] text-[length:var(--size-font-xs)]" value={config.model} onChange={(e) => updateField("model", e.target.value)} placeholder={t("aiSettings.modelPlaceholder")} />
      </div>

      <div>
        <label className="text-[length:var(--size-font-2xs)] font-medium text-[var(--fg-secondary)] mb-1 block">{t("aiSettings.systemPrompt")}</label>
        <textarea
          className="w-full min-h-[calc(var(--size-input)*3)] rounded-[var(--radius-input)] border border-[var(--border-color)] bg-[var(--surface)] px-2 py-1.5 text-[length:var(--size-font-xs)] text-[var(--fg)] placeholder:text-[var(--fg-muted)] focus:outline-none focus:ring-1 focus:ring-[var(--accent)] resize-y"
          value={config.systemPrompt}
          onChange={(e) => updateField("systemPrompt", e.target.value)}
          placeholder={t("aiSettings.systemPromptPlaceholder")}
        />
        <p className="mt-1 text-[length:var(--size-font-2xs)] text-[var(--fg-muted)]">
          {t("aiSettings.systemPromptHint")}
        </p>
      </div>

      {/* 测试结果 */}
      {testStatus !== "idle" && (
        <div className={cn(
          "flex items-start gap-1.5 px-2 py-1.5 rounded-[var(--radius-btn)] text-[length:var(--size-font-2xs)]",
          { "bg-blue-50 dark:bg-blue-900/20 text-blue-600 dark:text-blue-400": testStatus === "testing",
            "bg-green-50 dark:bg-green-900/20 text-green-600 dark:text-green-400": testStatus === "success",
            "bg-red-50 dark:bg-red-900/20 text-red-600 dark:text-red-400": testStatus === "error" }
        )}>
          {testStatus === "testing" && <Loader2 className="h-3 w-3 animate-spin mt-0.5" />}
          {testStatus === "success" && <Check className="h-3 w-3 mt-0.5" />}
          {testStatus === "error" && <AlertCircle className="h-3 w-3 mt-0.5" />}
          <span className="flex-1 break-all">
            {testStatus === "testing" && t("aiSettings.testingConnection")}
            {testStatus === "success" && (testResult || t("aiSettings.testSuccess"))}
            {testStatus === "error" && (testResult || t("aiSettings.testFailed"))}
          </span>
        </div>
      )}

      {/* 按钮区 */}
      <div className="flex items-center gap-2 pt-0.5">
        <Button variant="outline" size="sm" className="h-[var(--size-btn)] text-[length:var(--size-font-xs)]" onClick={handleTest} disabled={!config.baseURL || testStatus === "testing"}>
          <TestTube2 className="h-3 w-3 mr-1" />
          {testStatus === "testing" ? t("aiSettings.testingConnection") : t("aiSettings.testConnection")}
        </Button>
        <div className="flex-1" />
        <Button size="sm" className="h-[var(--size-btn)] text-[length:var(--size-font-xs)]" onClick={handleSave}>
          {saveStatus === "saving" ? <Loader2 className="h-3 w-3 mr-1 animate-spin" /> : <Save className="h-3 w-3 mr-1" />}
          {getSaveLabel()}
        </Button>
      </div>
    </div>
  );
}
