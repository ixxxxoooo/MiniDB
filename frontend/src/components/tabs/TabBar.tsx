import React, { useState, useEffect, useRef } from "react";
import { X, Table2, Code, FileText, FileCode } from "lucide-react";
import { cn } from "@/lib/utils";
import { useTabsStore, type Tab, type TabType } from "@/stores/tabs";

const TAB_ICONS: Record<TabType, React.ElementType> = {
  table: Table2,
  query: Code,
  ddl: FileCode,
  doc: FileText,
};

interface TabContextMenuState {
  x: number;
  y: number;
  tabId: string;
}

export function TabBar() {
  const { tabs, activeTabId, setActiveTab, removeTab, closeOtherTabs, closeAllTabs } = useTabsStore();
  const [contextMenu, setContextMenu] = useState<TabContextMenuState | null>(null);
  const menuRef = useRef<HTMLDivElement>(null);

  // 点击外部关闭右键菜单
  useEffect(() => {
    if (!contextMenu) return;
    const handler = (e: MouseEvent) => {
      if (menuRef.current && !menuRef.current.contains(e.target as Node)) {
        setContextMenu(null);
      }
    };
    document.addEventListener("mousedown", handler);
    return () => document.removeEventListener("mousedown", handler);
  }, [contextMenu]);

  if (tabs.length === 0) return null;

  const handleContextMenu = (e: React.MouseEvent, tabId: string) => {
    e.preventDefault();
    setContextMenu({ x: e.clientX, y: e.clientY, tabId });
  };

  const contextTab = contextMenu ? tabs.find((t) => t.id === contextMenu.tabId) : null;
  const closableTabs = tabs.filter((t) => t.closable);
  const otherClosableTabs = tabs.filter((t) => t.id !== contextMenu?.tabId && t.closable);
  // 右侧可关闭的 tab
  const rightClosableTabs = contextMenu
    ? tabs.filter((t, i) => {
        const ctxIdx = tabs.findIndex((tt) => tt.id === contextMenu.tabId);
        return i > ctxIdx && t.closable;
      })
    : [];

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
            onMouseDown={(e) => {
              if (e.button === 1 && tab.closable) {
                e.preventDefault();
                removeTab(tab.id);
              }
            }}
            onContextMenu={(e) => handleContextMenu(e, tab.id)}
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

      {/* Tab 右键菜单 */}
      {contextMenu && (
        <div
          ref={menuRef}
          className={cn(
            "fixed z-[100] min-w-[180px] py-1 rounded-lg shadow-lg border",
            "bg-[var(--surface-elevated)] border-[var(--border-color)]"
          )}
          style={{ left: contextMenu.x, top: contextMenu.y }}
        >
          {contextTab?.closable && (
            <button
              className="w-full px-3 py-1.5 text-xs text-left hover:bg-[var(--sidebar-hover)] text-[var(--fg)]"
              onClick={() => { removeTab(contextMenu.tabId); setContextMenu(null); }}
            >
              关闭
            </button>
          )}
          <button
            className="w-full px-3 py-1.5 text-xs text-left hover:bg-[var(--sidebar-hover)] text-[var(--fg)] disabled:opacity-40"
            disabled={otherClosableTabs.length === 0}
            onClick={() => { closeOtherTabs(contextMenu.tabId); setContextMenu(null); }}
          >
            关闭其他
          </button>
          <button
            className="w-full px-3 py-1.5 text-xs text-left hover:bg-[var(--sidebar-hover)] text-[var(--fg)] disabled:opacity-40"
            disabled={rightClosableTabs.length === 0}
            onClick={() => {
              rightClosableTabs.forEach((t) => removeTab(t.id));
              setContextMenu(null);
            }}
          >
            关闭右侧
          </button>
          <div className="h-px bg-[var(--border-subtle)] my-1" />
          <button
            className="w-full px-3 py-1.5 text-xs text-left hover:bg-[var(--sidebar-hover)] text-[var(--fg)] disabled:opacity-40"
            disabled={closableTabs.length === 0}
            onClick={() => { closeAllTabs(); setContextMenu(null); }}
          >
            关闭所有
          </button>
        </div>
      )}
    </div>
  );
}
