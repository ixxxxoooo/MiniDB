import React, { useState, useEffect, useRef, useCallback } from "react";
import { X, Table2, Code, FileText, FileCode, ChevronDown, ChevronLeft, ChevronRight } from "lucide-react";
import { cn } from "@/lib/utils";
import { useTabsStore, type Tab, type TabType } from "@/stores/tabs";
import { useConnectionStore } from "@/stores/connection";
import { useTranslation } from "@/i18n";

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
  const { tabs: allTabs, activeTabId, setActiveTab, removeTab, closeOtherTabs, closeAllTabs } = useTabsStore();
  const { activeWorkspaceId } = useConnectionStore();
  const [contextMenu, setContextMenu] = useState<TabContextMenuState | null>(null);

  const tabs = React.useMemo(() => {
    if (!activeWorkspaceId) return [];
    return allTabs.filter(t => `${t.connectionId}:${t.database}` === activeWorkspaceId);
  }, [allTabs, activeWorkspaceId]);
  const [overflowMenuOpen, setOverflowMenuOpen] = useState(false);
  const [hasOverflow, setHasOverflow] = useState(false);
  const menuRef = useRef<HTMLDivElement>(null);
  const overflowRef = useRef<HTMLDivElement>(null);
  const scrollContainerRef = useRef<HTMLDivElement>(null);
  const { t } = useTranslation();

  // 检测 Tab 是否溢出
  const checkOverflow = useCallback(() => {
    const container = scrollContainerRef.current;
    if (container) {
      setHasOverflow(container.scrollWidth > container.clientWidth);
    }
  }, []);

  const scrollLeft = () => {
    scrollContainerRef.current?.scrollBy({ left: -200, behavior: "smooth" });
  };

  const scrollRight = () => {
    scrollContainerRef.current?.scrollBy({ left: 200, behavior: "smooth" });
  };

  useEffect(() => {
    checkOverflow();
    window.addEventListener("resize", checkOverflow);
    return () => window.removeEventListener("resize", checkOverflow);
  }, [checkOverflow, tabs.length]);

  // 活跃 Tab 变化时滚动到可见区域
  useEffect(() => {
    if (!activeTabId || !scrollContainerRef.current) return;
    const activeEl = scrollContainerRef.current.querySelector(`[data-tab-id="${activeTabId}"]`);
    if (activeEl) {
      activeEl.scrollIntoView({ behavior: "smooth", block: "nearest", inline: "nearest" });
    }
  }, [activeTabId]);

  // 关闭右键菜单
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

  // 关闭溢出菜单
  useEffect(() => {
    if (!overflowMenuOpen) return;
    const handler = (e: MouseEvent) => {
      if (overflowRef.current && !overflowRef.current.contains(e.target as Node)) {
        setOverflowMenuOpen(false);
      }
    };
    document.addEventListener("mousedown", handler);
    return () => document.removeEventListener("mousedown", handler);
  }, [overflowMenuOpen]);

  if (tabs.length === 0) return null;

  const handleContextMenu = (e: React.MouseEvent, tabId: string) => {
    e.preventDefault();
    setContextMenu({ x: e.clientX, y: e.clientY, tabId });
  };

  const contextTab = contextMenu ? tabs.find((tab) => tab.id === contextMenu.tabId) : null;
  const closableTabs = tabs.filter((tab) => tab.closable);
  const otherClosableTabs = tabs.filter((tab) => tab.id !== contextMenu?.tabId && tab.closable);
  const rightClosableTabs = contextMenu
    ? tabs.filter((tab, i) => {
        const ctxIdx = tabs.findIndex((tt) => tt.id === contextMenu.tabId);
        return i > ctxIdx && tab.closable;
      })
    : [];

  return (
    <div
      className={cn(
        "flex items-end h-[var(--size-tab)] border-b flex-shrink-0",
        "bg-[var(--surface-secondary)] border-[var(--border-color)]"
      )}
    >
      {/* 左右滚动按钮 */}
      {hasOverflow && (
        <div className="flex items-center flex-shrink-0 h-[calc(var(--size-tab)-2px)] px-0.5 border-r border-[var(--border-subtle)]">
          <button
            className={cn(
              "h-full px-1 flex items-center justify-center rounded-[var(--radius-btn)]",
              "text-[var(--fg-secondary)] hover:bg-[var(--tab-hover-bg)] hover:text-[var(--fg)] transition-colors"
            )}
            onClick={scrollLeft}
          >
            <ChevronLeft className="h-3 w-3" />
          </button>
          <button
            className={cn(
              "h-full px-1 flex items-center justify-center rounded-[var(--radius-btn)]",
              "text-[var(--fg-secondary)] hover:bg-[var(--tab-hover-bg)] hover:text-[var(--fg)] transition-colors"
            )}
            onClick={scrollRight}
          >
            <ChevronRight className="h-3 w-3" />
          </button>
        </div>
      )}

      {/* 可滚动的 Tab 容器 */}
      <div
        ref={scrollContainerRef}
        className="flex items-end flex-1 min-w-0 overflow-x-hidden"
      >
        {tabs.map((tab) => {
          const Icon = TAB_ICONS[tab.type];
          const isActive = tab.id === activeTabId;

          return (
            <div
              key={tab.id}
              data-tab-id={tab.id}
              className={cn(
                "flex items-center gap-[var(--size-gap-sm)] px-2.5 h-[calc(var(--size-tab)-2px)] text-[length:var(--size-font-2xs)] cursor-pointer select-none",
                "border-r border-[var(--border-subtle)] transition-colors group min-w-0 flex-shrink-0",
                isActive
                  ? "bg-[var(--surface)] text-[var(--fg)] border-b-2 border-b-[var(--accent)]"
                  : "text-[var(--fg-secondary)] hover:bg-[var(--tab-hover-bg)] hover:text-[var(--fg)]"
              )}
              title={tab.title}
              onClick={() => setActiveTab(tab.id)}
              onMouseDown={(e) => {
                if (e.button === 1 && tab.closable) {
                  e.preventDefault();
                  removeTab(tab.id);
                }
              }}
              onContextMenu={(e) => handleContextMenu(e, tab.id)}
            >
              <Icon className="h-2.5 w-2.5 flex-shrink-0" />
              <span className="truncate max-w-[100px]">{tab.title}</span>
              {tab.dirty && (
                <span className="w-1 h-1 rounded-full bg-[var(--accent)] flex-shrink-0" />
              )}
              {tab.closable && (
                <button
                  className={cn(
                    "flex items-center justify-center flex-shrink-0 transition-opacity",
                    "opacity-0 group-hover:opacity-100",
                    "text-[var(--fg-muted)] hover:text-[var(--fg)]"
                  )}
                  onClick={(e) => {
                    e.stopPropagation();
                    removeTab(tab.id);
                  }}
                >
                  <X className="h-3 w-3" />
                </button>
              )}
            </div>
          );
        })}
      </div>

      {/* 溢出时显示"更多"按钮 */}
      {hasOverflow && (
        <div className="relative flex-shrink-0">
          <button
            className={cn(
              "h-[calc(var(--size-tab)-2px)] px-1.5 flex items-center justify-center border-l border-[var(--border-subtle)] rounded-[var(--radius-btn)]",
              "text-[var(--fg-secondary)] hover:bg-[var(--tab-hover-bg)] hover:text-[var(--fg)] transition-colors"
            )}
            onClick={() => setOverflowMenuOpen(!overflowMenuOpen)}
            title={t("tabs.moreTabs")}
          >
            <ChevronDown className="h-3 w-3" />
          </button>

          {overflowMenuOpen && (
            <div
              ref={overflowRef}
              className={cn(
                "absolute right-0 top-full z-[100] mt-0.5 min-w-[180px] max-h-[300px] overflow-y-auto",
                "py-0.5 rounded-[var(--radius-menu)] shadow-lg border",
                "bg-[var(--surface-elevated)] border-[var(--border-color)]"
              )}
            >
              {tabs.map((tab) => {
                const Icon = TAB_ICONS[tab.type];
                const isActive = tab.id === activeTabId;
                return (
                  <button
                    key={tab.id}
                    className={cn(
                      "w-full flex items-center gap-2 px-2.5 py-1 text-[length:var(--size-font-xs)] text-left transition-colors",
                      isActive
                        ? "bg-[var(--accent)]/10 text-[var(--accent)] font-medium"
                        : "text-[var(--fg)] hover:bg-[var(--sidebar-hover)]"
                    )}
                    onClick={() => {
                      setActiveTab(tab.id);
                      setOverflowMenuOpen(false);
                    }}
                  >
                    <Icon className="h-3 w-3 flex-shrink-0" />
                    <span className="flex-1 truncate">{tab.title}</span>
                    {tab.closable && (
                      <button
                        className="h-4 w-4 flex items-center justify-center rounded-[var(--radius-btn)] hover:bg-[var(--surface-elevated)] flex-shrink-0"
                        onClick={(e) => {
                          e.stopPropagation();
                          removeTab(tab.id);
                          if (tabs.length <= 1) setOverflowMenuOpen(false);
                        }}
                      >
                        <X className="h-2.5 w-2.5 text-[var(--fg-muted)]" />
                      </button>
                    )}
                  </button>
                );
              })}
            </div>
          )}
        </div>
      )}

      {/* Tab 右键菜单 */}
      {contextMenu && (
        <div
          ref={menuRef}
          className={cn(
            "fixed z-[100] min-w-[160px] py-0.5 rounded-[var(--radius-menu)] shadow-lg border",
            "bg-[var(--surface-elevated)] border-[var(--border-color)]"
          )}
          style={{ left: contextMenu.x, top: contextMenu.y }}
        >
          {contextTab?.closable && (
            <button
              className="w-full px-2.5 py-1 text-xs text-left hover:bg-[var(--sidebar-hover)] text-[var(--fg)]"
              onClick={() => { removeTab(contextMenu.tabId); setContextMenu(null); }}
            >
              {t("tabs.close")}
            </button>
          )}
          <button
            className="w-full px-2.5 py-1 text-xs text-left hover:bg-[var(--sidebar-hover)] text-[var(--fg)] disabled:opacity-40"
            disabled={otherClosableTabs.length === 0}
            onClick={() => { closeOtherTabs(contextMenu.tabId); setContextMenu(null); }}
          >
            {t("tabs.closeOthers")}
          </button>
          <button
            className="w-full px-2.5 py-1 text-xs text-left hover:bg-[var(--sidebar-hover)] text-[var(--fg)] disabled:opacity-40"
            disabled={rightClosableTabs.length === 0}
            onClick={() => {
              rightClosableTabs.forEach((tab) => removeTab(tab.id));
              setContextMenu(null);
            }}
          >
            {t("tabs.closeRight")}
          </button>
          <div className="h-px bg-[var(--border-subtle)] my-0.5" />
          <button
            className="w-full px-2.5 py-1 text-xs text-left hover:bg-[var(--sidebar-hover)] text-[var(--fg)] disabled:opacity-40"
            disabled={closableTabs.length === 0}
            onClick={() => { closeAllTabs(activeWorkspaceId || undefined); setContextMenu(null); }}
          >
            {t("tabs.closeAll")}
          </button>
        </div>
      )}
    </div>
  );
}
