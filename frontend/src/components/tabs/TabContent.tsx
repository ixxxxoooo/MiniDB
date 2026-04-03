import React from "react";
import { TooltipProvider } from "@/components/ui/tooltip";
import { useTabsStore } from "@/stores/tabs";
import { useTranslation } from "@/i18n";
import { Database } from "lucide-react";
import { TableView } from "./TableView";
import { QueryView } from "./QueryView";
import { DDLView } from "./DDLView";
import { DocView } from "./DocView";

export function TabContent() {
  const { tabs, activeTabId } = useTabsStore();
  if (!activeTabId) {
    return <EmptyState />;
  }

  return (
    <TooltipProvider>
      <div className="h-full flex flex-col">
        {tabs.map((tab) => (
          <div
            key={tab.id}
            className={tab.id === activeTabId ? "h-full flex flex-col min-h-0" : "hidden"}
          >
            {tab.type === "table" && <TableView tab={tab} />}
            {tab.type === "query" && <QueryView tab={tab} />}
            {tab.type === "ddl" && <DDLView tab={tab} />}
            {tab.type === "doc" && <DocView tab={tab} />}
          </div>
        ))}
      </div>
    </TooltipProvider>
  );
}

function EmptyState() {
  const { t } = useTranslation();

  return (
    <div className="h-full w-full flex items-center justify-center">
      <div className="flex flex-col items-center text-[var(--fg-muted)]">
        <Database className="h-12 w-12 mb-3 opacity-20" />
        <p className="text-base font-medium mb-0.5 text-[var(--fg-secondary)]">{t("empty.title")}</p>
        <p className="text-xs text-[var(--fg-muted)]">{t("empty.subtitle")}</p>
        <div className="mt-4 flex gap-4 text-2xs text-[var(--fg-muted)]">
          <div className="flex items-center gap-1">
            <kbd className="px-1 py-0.5 rounded border border-[var(--border-color)] bg-[var(--surface-secondary)] text-2xs">⌘P</kbd>
            <span>{t("empty.quickSearch")}</span>
          </div>
          <div className="flex items-center gap-1">
            <kbd className="px-1 py-0.5 rounded border border-[var(--border-color)] bg-[var(--surface-secondary)] text-2xs">⌘N</kbd>
            <span>{t("empty.newConnection")}</span>
          </div>
          <div className="flex items-center gap-1">
            <kbd className="px-1 py-0.5 rounded border border-[var(--border-color)] bg-[var(--surface-secondary)] text-2xs">⌘T</kbd>
            <span>{t("empty.newQuery")}</span>
          </div>
        </div>
      </div>
    </div>
  );
}
