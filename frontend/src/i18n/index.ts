import { create } from "zustand";
import { persist } from "zustand/middleware";
import { zhCN } from "./zh-CN";
import { enUS } from "./en-US";
import type { Locale, TranslationKey, TranslationKeys } from "./types";
import { migratePersistedKey } from "@/stores/persistMigration";

const locales: Record<Locale, TranslationKeys> = {
  "zh-CN": zhCN,
  "en-US": enUS,
};

export const LOCALE_LABELS: Record<Locale, string> = {
  "zh-CN": "简体中文",
  "en-US": "English",
};

const I18N_STORAGE_KEY = "minidb-i18n";
migratePersistedKey(I18N_STORAGE_KEY, "tableplus-ai-i18n");

interface I18nStore {
  locale: Locale;
  setLocale: (locale: Locale) => void;
}

function detectSystemLocale(): Locale {
  if (typeof navigator === "undefined") {
    return "zh-CN";
  }
  const lang = String(navigator.language || "").toLowerCase();
  if (lang.startsWith("en")) {
    return "en-US";
  }
  return "zh-CN";
}

export const useI18nStore = create<I18nStore>()(
  persist(
    (set) => ({
      // 默认跟随系统语言，用户手动切换后由 persist 保持
      locale: detectSystemLocale(),
      setLocale: (locale) => set({ locale }),
    }),
    { name: I18N_STORAGE_KEY }
  )
);

/**
 * 翻译函数：根据 key 获取当前语言的翻译文本
 * 支持 {param} 占位符替换
 */
export function t(key: TranslationKey, params?: Record<string, string | number>): string {
  const locale = useI18nStore.getState().locale;
  let text = locales[locale]?.[key] || locales["zh-CN"][key] || key;
  if (params) {
    for (const [k, v] of Object.entries(params)) {
      text = text.replace(`{${k}}`, String(v));
    }
  }
  return text;
}

/**
 * React Hook：在组件中使用，确保语言切换时自动触发重渲染
 */
export function useTranslation() {
  const locale = useI18nStore((s) => s.locale);
  const translations = locales[locale] || locales["zh-CN"];

  const translate = (key: TranslationKey, params?: Record<string, string | number>): string => {
    let text = translations[key] || locales["zh-CN"][key] || key;
    if (params) {
      for (const [k, v] of Object.entries(params)) {
        text = text.replace(`{${k}}`, String(v));
      }
    }
    return text;
  };

  return { t: translate, locale };
}

export type { Locale, TranslationKey } from "./types";
