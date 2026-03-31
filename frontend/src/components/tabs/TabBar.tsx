import React from "react";
import { X, Table2, Code, FileText, FileCode } from "lucide-react";
import { cn } from "@/lib/utils";
import { useTabsStore, type Tab, type TabType } from "@/stores/tabs";

const TAB_ICONS: Record<TabType, React.ElementType> = {
  table: Table2,
  query: Code,
  ddl: FileCode,
  doc: FileText,
};

export function TabBar() {
  const { tabs, activeTabId, setActiveTab, removeTab } = useTabsStore();

  if (tabs.length === 0) return null;

  return (
    <div
      className={cn(
        "flex items-center h-9 border-b overflow-x-auto",
        "bg-[var(--surface-secondary)] border-[var(--border-color)]"
      )}
    >
      {tabs.map((tab) => {
        const Icon = TAB_ICONS[tab.type];
        const isActive = tab.id === activeTabId;

        return (
          <div
            key={tab.id}
            className={cn(
              "flex items-center gap-1.5 px-3 h-full text-xs cursor-pointer select-none",
              "border-r border-[var(--border-subtle)] transition-colors group min-w-0",
              isActive
                ? "bg-[var(--surface)] text-[var(--fg)] border-b-2 border-b-[var(--accent)]"
                : "text-[var(--fg-secondary)] hover:bg-[var(--tab-hover-bg)] hover:text-[var(--fg)]"
            )}
            onClick={() => setActiveTab(tab.id)}
          >
            <Icon className="h-3 w-3 flex-shrink-0" />
            <span className="truncate max-w-[120px]">{tab.title}</span>
            {tab.dirty && (
              <span className="w-1.5 h-1.5 rounded-full bg-[var(--accent)] flex-shrink-0" />
            )}
            {tab.closable && (
              <button
                className={cn(
                  "h-4 w-4 flex items-center justify-center rounded-sm flex-shrink-0",
                  "opacity-0 group-hover:opacity-100 hover:bg-[var(--surface-elevated)] transition-opacity"
                )}
                onClick={(e) => {
                  e.stopPropagation();
                  removeTab(tab.id);
                }}
              >
                <X className="h-2.5 w-2.5" />
              </button>
            )}
          </div>
        );
      })}
    </div>
  );
}
