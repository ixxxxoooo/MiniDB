import React from "react";
import { useThemeStore } from "@/stores/theme";
import { useUIStore, type LayoutMode } from "@/stores/ui";
import { useI18nStore, LOCALE_LABELS, useTranslation } from "@/i18n";
import type { Locale } from "@/i18n";
import { Monitor, Sun, Moon, Globe, LayoutGrid, Minimize2 } from "lucide-react";
import { cn } from "@/lib/utils";

const PAGE_SIZE_OPTIONS = [50, 100, 200, 500, 1000];

export function GeneralSettings() {
  const { theme, setTheme } = useThemeStore();
  const {
    pageSize,
    setPageSize,
    showDataRowNumbers,
    setShowDataRowNumbers,
    layoutMode,
    setLayoutMode,
    showScrollbar,
    setShowScrollbar,
  } = useUIStore();
  const { locale, setLocale } = useI18nStore();
  const { t } = useTranslation();

  const themes = [
    { id: "light" as const, label: t("generalSettings.themeLight"), icon: Sun },
    { id: "dark" as const, label: t("generalSettings.themeDark"), icon: Moon },
    { id: "system" as const, label: t("generalSettings.themeSystem"), icon: Monitor },
  ];

  const layouts: { id: LayoutMode; label: string; desc: string; icon: React.ElementType }[] = [
    {
      id: "compact",
      label: t("generalSettings.layoutCompact"),
      desc: t("generalSettings.layoutCompactDesc"),
      icon: Minimize2,
    },
    {
      id: "default",
      label: t("generalSettings.layoutDefault"),
      desc: t("generalSettings.layoutDefaultDesc"),
      icon: LayoutGrid,
    },
  ];
  const pageSizeOptions = Array.from(new Set([...PAGE_SIZE_OPTIONS, pageSize])).sort((a, b) => a - b);

  return (
    <div className="space-y-[var(--size-gap)]">
      <div>
        <h3 className="text-[length:var(--size-font-xs)] font-semibold mb-0.5">{t("generalSettings.title")}</h3>
        <p className="text-[length:var(--size-font-2xs)] text-[var(--fg-secondary)]">{t("generalSettings.description")}</p>
      </div>

      {/* 语言选择 */}
      <div>
        <label className="text-[length:var(--size-font-2xs)] font-medium text-[var(--fg-secondary)] mb-1 block">
          {t("generalSettings.languageLabel")}
        </label>
        <div className="flex gap-1.5">
          {(Object.keys(LOCALE_LABELS) as Locale[]).map((loc) => (
            <button
              key={loc}
              className={cn(
                "flex-1 flex flex-col items-center gap-1 py-2 rounded-[var(--radius-btn)] border transition-colors",
                locale === loc
                  ? "border-[var(--accent)] bg-[var(--accent)]/5"
                  : "border-[var(--border-color)] hover:border-[var(--fg-muted)]"
              )}
              onClick={() => setLocale(loc)}
            >
              <Globe
                className={cn(
                  "h-3.5 w-3.5",
                  locale === loc ? "text-[var(--accent)]" : "text-[var(--fg-secondary)]"
                )}
              />
              <span
                className={cn(
                  "text-[length:var(--size-font-2xs)]",
                  locale === loc ? "text-[var(--accent)] font-medium" : "text-[var(--fg-secondary)]"
                )}
              >
                {LOCALE_LABELS[loc]}
              </span>
            </button>
          ))}
        </div>
      </div>

      {/* 主题选择 */}
      <div>
        <label className="text-[length:var(--size-font-2xs)] font-medium text-[var(--fg-secondary)] mb-1 block">
          {t("generalSettings.theme")}
        </label>
        <div className="flex gap-1.5">
          {themes.map(({ id, label, icon: Icon }) => (
            <button
              key={id}
              className={cn(
                "flex-1 flex flex-col items-center gap-1 py-2 rounded-[var(--radius-btn)] border transition-colors",
                theme === id
                  ? "border-[var(--accent)] bg-[var(--accent)]/5"
                  : "border-[var(--border-color)] hover:border-[var(--fg-muted)]"
              )}
              onClick={() => setTheme(id)}
            >
              <Icon
                className={cn(
                  "h-3.5 w-3.5",
                  theme === id ? "text-[var(--accent)]" : "text-[var(--fg-secondary)]"
                )}
              />
              <span
                className={cn(
                  "text-[length:var(--size-font-2xs)]",
                  theme === id ? "text-[var(--accent)] font-medium" : "text-[var(--fg-secondary)]"
                )}
              >
                {label}
              </span>
            </button>
          ))}
        </div>
      </div>

      {/* 布局模式 */}
      <div>
        <label className="text-[length:var(--size-font-2xs)] font-medium text-[var(--fg-secondary)] mb-1 block">
          {t("generalSettings.layoutMode")}
        </label>
        <div className="flex gap-1.5">
          {layouts.map(({ id, label, desc, icon: Icon }) => (
            <button
              key={id}
              className={cn(
                "flex-1 flex flex-col items-center gap-0.5 py-2 rounded-[var(--radius-btn)] border transition-colors",
                layoutMode === id
                  ? "border-[var(--accent)] bg-[var(--accent)]/5"
                  : "border-[var(--border-color)] hover:border-[var(--fg-muted)]"
              )}
              onClick={() => setLayoutMode(id)}
            >
              <Icon
                className={cn(
                  "h-3.5 w-3.5",
                  layoutMode === id ? "text-[var(--accent)]" : "text-[var(--fg-secondary)]"
                )}
              />
              <span
                className={cn(
                  "text-[length:var(--size-font-2xs)]",
                  layoutMode === id ? "text-[var(--accent)] font-medium" : "text-[var(--fg-secondary)]"
                )}
              >
                {label}
              </span>
              <span className="text-[length:var(--size-font-2xs)] text-[var(--fg-muted)]">{desc}</span>
            </button>
          ))}
        </div>
      </div>

      {/* 显示滚动条 */}
      <div>
        <label className="text-[length:var(--size-font-2xs)] font-medium text-[var(--fg-secondary)] mb-1 block">
          {t("generalSettings.pageSize")}
        </label>
        <div className="flex flex-wrap gap-1.5">
          {pageSizeOptions.map((size) => (
            <button
              key={size}
              className={cn(
                "px-2.5 py-1 rounded-[var(--radius-btn)] border text-[length:var(--size-font-2xs)] transition-colors",
                pageSize === size
                  ? "border-[var(--accent)] bg-[var(--accent)]/10 text-[var(--accent)] font-medium"
                  : "border-[var(--border-color)] text-[var(--fg-secondary)] hover:text-[var(--fg)] hover:border-[var(--fg-muted)]"
              )}
              onClick={() => setPageSize(size)}
            >
              {size}
            </button>
          ))}
        </div>
      </div>

      {/* 显示滚动条 */}
      <div>
        <label className="flex items-center gap-2 cursor-pointer select-none">
          <input
            type="checkbox"
            checked={showDataRowNumbers}
            onChange={(e) => setShowDataRowNumbers(e.target.checked)}
            className="accent-[var(--accent)] h-3.5 w-3.5 rounded"
          />
          <span className="text-[length:var(--size-font-xs)] font-medium">{t("generalSettings.showDataRowNumbers")}</span>
          <span className="text-[length:var(--size-font-2xs)] text-[var(--fg-muted)]">{t("generalSettings.showDataRowNumbersDesc")}</span>
        </label>
      </div>

      {/* 显示滚动条 */}
      <div>
        <label className="flex items-center gap-2 cursor-pointer select-none">
          <input
            type="checkbox"
            checked={showScrollbar}
            onChange={(e) => setShowScrollbar(e.target.checked)}
            className="accent-[var(--accent)] h-3.5 w-3.5 rounded"
          />
          <span className="text-[length:var(--size-font-xs)] font-medium">{t("generalSettings.showScrollbar")}</span>
          <span className="text-[length:var(--size-font-2xs)] text-[var(--fg-muted)]">{t("generalSettings.showScrollbarDesc")}</span>
        </label>
      </div>
    </div>
  );
}
