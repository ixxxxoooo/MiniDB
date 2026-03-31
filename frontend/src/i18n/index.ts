import { create } from "zustand";
import { persist } from "zustand/middleware";
import { zhCN } from "./zh-CN";
import { enUS } from "./en-US";
import type { Locale, TranslationKey, TranslationKeys } from "./types";

const locales: Record<Locale, TranslationKeys> = {
  "zh-CN": zhCN,
  "en-US": enUS,
};

export const LOCALE_LABELS: Record<Locale, string> = {
  "zh-CN": "简体中文",
  "en-US": "English",
};

interface I18nStore {
  locale: Locale;
  setLocale: (locale: Locale) => void;
}

export const useI18nStore = create<I18nStore>()(
  persist(
    (set) => ({
      locale: "zh-CN",
      setLocale: (locale) => set({ locale }),
    }),
    { name: "tableplus-ai-i18n" }
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
