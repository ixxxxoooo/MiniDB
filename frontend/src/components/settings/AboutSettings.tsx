import React from "react";
import { Info, Package, User } from "lucide-react";
import { useTranslation } from "@/i18n";

const APP_NAME = "TablePlus AI";
const APP_VERSION = "0.1.0";
const AUTHOR_NAME = "Jason";

export function AboutSettings() {
  const { t } = useTranslation();

  const infoItems = [
    { label: t("about.appName"), value: APP_NAME, icon: Info },
    { label: t("about.version"), value: APP_VERSION, icon: Package },
    { label: t("about.author"), value: AUTHOR_NAME, icon: User },
  ];

  return (
    <div className="space-y-[var(--size-gap)]">
      <div>
        <h3 className="text-[length:var(--size-font-xs)] font-semibold mb-0.5">{t("about.title")}</h3>
        <p className="text-[length:var(--size-font-2xs)] text-[var(--fg-secondary)]">{t("about.description")}</p>
      </div>

      <div className="rounded-[var(--radius-panel)] border border-[var(--border-color)] bg-[var(--surface-secondary)]/50 p-[var(--size-padding)]">
        <div className="text-[length:var(--size-font-sm)] font-semibold text-[var(--fg)]">{APP_NAME}</div>
        <div className="mt-1 text-[length:var(--size-font-xs)] text-[var(--fg-secondary)] leading-6">
          {t("about.summary")}
        </div>
      </div>

      <div className="rounded-[var(--radius-panel)] border border-[var(--border-color)] bg-[var(--surface-secondary)]/50 overflow-hidden">
        <div className="divide-y divide-[var(--border-color)]/70">
          {infoItems.map(({ label, value, icon: Icon }) => (
            <div
              key={label}
              className="flex items-center gap-[var(--size-gap)] px-[var(--size-padding)] py-[var(--size-padding-sm)]"
            >
              <div className="h-7 w-7 rounded-[var(--radius-btn)] border border-[var(--border-color)] bg-[var(--surface)] flex items-center justify-center flex-shrink-0">
                <Icon className="h-3.5 w-3.5 text-[var(--fg-secondary)]" />
              </div>
              <div className="min-w-[92px] text-[length:var(--size-font-2xs)] text-[var(--fg-muted)] flex-shrink-0">
                {label}
              </div>
              <div className="text-[length:var(--size-font-xs)] text-[var(--fg)] break-all">
                {value}
              </div>
            </div>
          ))}
        </div>
      </div>
    </div>
  );
}
