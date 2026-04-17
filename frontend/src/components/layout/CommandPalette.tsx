import React, { useState, useEffect, useMemo, useRef, useCallback } from "react";
import {
  Search,
  Database,
  Table2,
  Settings,
  Moon,
  Sun,
} from "lucide-react";
import { cn } from "@/lib/utils";
import { useConnectionStore } from "@/stores/connection";
import { useTabsStore } from "@/stores/tabs";
import { useThemeStore } from "@/stores/theme";
import { useTranslation } from "@/i18n";

interface CommandPaletteProps {
  open: boolean;
  onClose: () => void;
  onOpenSettings: () => void;
  onNewConnection: () => void;
}

interface CommandItem {
  id: string;
  title: string;
  subtitle?: string;
  icon: React.ElementType;
  action: () => void;
  category: string;
}

const MAX_RESULTS = 50;
const NON_ASCII_RE = /[^\x20-\x7E]/g;

function normalizeQuickSearchQuery(value: string): string {
  return value.replace(NON_ASCII_RE, "");
}

export function CommandPalette({
  open,
  onClose,
  onOpenSettings,
  onNewConnection,
}: CommandPaletteProps) {
  const [query, setQuery] = useState("");
  const [selectedIndex, setSelectedIndex] = useState(0);
  const inputRef = useRef<HTMLInputElement>(null);
  const listRef = useRef<HTMLDivElement>(null);
  const [selectionMode, setSelectionMode] = useState<"keyboard" | "mouse">("keyboard");
  const lastPointerPosRef = useRef<{ x: number; y: number } | null>(null);
  const { t } = useTranslation();

  const closeFn = useRef(onClose);
  const openSettingsFn = useRef(onOpenSettings);
  const newConnectionFn = useRef(onNewConnection);
  closeFn.current = onClose;
  openSettingsFn.current = onOpenSettings;
  newConnectionFn.current = onNewConnection;

  const { connections, connectionStates, tables, workspaces, activeWorkspaceId } =
    useConnectionStore();
  const addTab = useTabsStore((s) => s.addTab);
  const { resolved, setTheme } = useThemeStore();

  const commands = useMemo<CommandItem[]>(() => {
    const items: CommandItem[] = [];

    items.push(
      {
        id: "new-connection",
        title: t("command.newConnection"),
        icon: Database,
        action: () => { closeFn.current(); setTimeout(() => newConnectionFn.current(), 50); },
        category: t("command.categoryActions"),
      },
      {
        id: "settings",
        title: t("command.openSettings"),
        icon: Settings,
        action: () => { closeFn.current(); setTimeout(() => openSettingsFn.current(), 50); },
        category: t("command.categoryActions"),
      },
      {
        id: "toggle-theme",
        title: resolved === "dark" ? t("toolbar.switchToLight") : t("toolbar.switchToDark"),
        icon: resolved === "dark" ? Sun : Moon,
        action: () => { setTheme(resolved === "dark" ? "light" : "dark"); closeFn.current(); },
        category: t("command.categoryActions"),
      }
    );

    const activeWorkspace = workspaces.find((workspace) => workspace.id === activeWorkspaceId);
    if (activeWorkspace) {
      const conn = connections.find((item) => item.id === activeWorkspace.connectionId);
      const state = connectionStates[activeWorkspace.connectionId];
      const tableList = tables[`${activeWorkspace.connectionId}:${activeWorkspace.database}`] || [];

      if (conn && state?.status === "connected") {
        for (const tbl of tableList) {
          items.push({
            id: `table:${activeWorkspace.connectionId}:${activeWorkspace.database}:${tbl.name}`,
            title: tbl.name,
            subtitle: `${conn.name} / ${activeWorkspace.database}`,
            icon: Table2,
            action: () => {
              addTab({
                type: "table",
                title: tbl.name,
                connectionId: activeWorkspace.connectionId,
                database: activeWorkspace.database,
                table: tbl.name,
                closable: true,
              });
              closeFn.current();
            },
            category: t("command.categoryTables"),
          });
        }
      }
    }

    return items;
  }, [connections, connectionStates, tables, workspaces, activeWorkspaceId, resolved, addTab, setTheme, t]);

  const filtered = useMemo(() => {
    let results = commands;
    if (query.trim()) {
      const q = query.toLowerCase();
      results = commands.filter(
        (c) =>
          c.title.toLowerCase().includes(q) ||
          c.subtitle?.toLowerCase().includes(q)
      );
    }
    return results.slice(0, MAX_RESULTS);
  }, [commands, query]);

  useEffect(() => {
    setSelectedIndex(0);
    setSelectionMode("keyboard");
  }, [query]);

  // 选中项变化时自动滚动到可视区域
  useEffect(() => {
    if (!listRef.current) return;
    const item = listRef.current.children[selectedIndex] as HTMLElement | undefined;
    item?.scrollIntoView({ block: "nearest" });
  }, [selectedIndex]);

  useEffect(() => {
    if (!open) {
      setQuery("");
      setSelectedIndex(0);
      setSelectionMode("keyboard");
      lastPointerPosRef.current = null;
    } else {
      setTimeout(() => inputRef.current?.focus(), 50);
    }
  }, [open]);

  const handleKeyDown = useCallback((e: React.KeyboardEvent) => {
    if (e.key === "ArrowDown") {
      e.preventDefault();
      setSelectionMode("keyboard");
      setSelectedIndex((i) => Math.min(i + 1, filtered.length - 1));
    } else if (e.key === "ArrowUp") {
      e.preventDefault();
      setSelectionMode("keyboard");
      setSelectedIndex((i) => Math.max(i - 1, 0));
    } else if (e.key === "Enter" && filtered[selectedIndex]) {
      filtered[selectedIndex].action();
    } else if (e.key === "Escape") {
      closeFn.current();
    }
  }, [filtered, selectedIndex]);

  const handleQueryChange = useCallback((next: string) => {
    setQuery(normalizeQuickSearchQuery(next));
  }, []);

  const handleListMouseMove = useCallback((e: React.MouseEvent<HTMLDivElement>) => {
    const nextPos = { x: e.clientX, y: e.clientY };
    const prev = lastPointerPosRef.current;
    lastPointerPosRef.current = nextPos;
    if (!prev) {
      setSelectionMode("mouse");
      return;
    }
    if (Math.abs(nextPos.x - prev.x) > 1 || Math.abs(nextPos.y - prev.y) > 1) {
      setSelectionMode("mouse");
    }
  }, []);

  if (!open) return null;

  return (
    <>
      <div
        className="fixed inset-0 z-50 bg-black/20 backdrop-blur-sm"
        onClick={onClose}
      />
      <div
        className={cn(
          "fixed z-50 top-[20%] left-1/2 -translate-x-1/2",
          "w-[400px] max-h-[360px] rounded-[var(--radius-panel)] shadow-lg border overflow-hidden",
          "bg-[var(--surface)] border-[var(--border-color)]"
        )}
      >
        {/* 搜索栏 - 与 DatabaseSwitcher 风格统一 */}
        <div className="flex items-center gap-2 px-3 py-2.5 border-b border-[var(--border-color)]">
          <Search className="h-3.5 w-3.5 text-[var(--fg-muted)]" />
          <input
            ref={inputRef}
            className="flex-1 bg-transparent text-sm text-[var(--fg)] placeholder:text-[var(--fg-muted)] focus:outline-none"
            placeholder={t("command.searchPlaceholder")}
            value={query}
            onChange={(e) => handleQueryChange(e.target.value)}
            onKeyDown={handleKeyDown}
            autoFocus
            inputMode="text"
            lang="en"
            autoCapitalize="none"
            autoCorrect="off"
            spellCheck={false}
          />
        </div>

        <div
          ref={listRef}
          className="overflow-y-auto max-h-[290px] py-0.5"
          onMouseMove={handleListMouseMove}
        >
          {filtered.length === 0 && (
            <div className="px-3 py-4 text-center text-sm text-[var(--fg-muted)]">
              {t("common.noResults")}
            </div>
          )}
          {filtered.map((item, idx) => {
            const Icon = item.icon;
            const isSelected = idx === selectedIndex;
            return (
              <button
                key={item.id}
                className={cn(
                  "w-full flex items-center gap-2.5 px-3 py-1.5 text-left transition-colors",
                  isSelected
                    ? "bg-[var(--accent)] text-white"
                    : "text-[var(--fg)] hover:bg-[var(--sidebar-hover)]"
                )}
                onClick={item.action}
                onMouseEnter={() => {
                  if (selectionMode !== "mouse") return;
                  setSelectedIndex(idx);
                }}
              >
                <Icon className={cn(
                  "h-3.5 w-3.5 flex-shrink-0",
                  isSelected ? "text-white" : "text-[var(--fg-secondary)]"
                )} />
                <div className="flex-1 min-w-0">
                  <div className="text-sm truncate">{item.title}</div>
                  {item.subtitle && (
                    <div className={cn(
                      "text-2xs truncate",
                      isSelected ? "text-white/90" : "text-[var(--fg-muted)]"
                    )}>
                      {item.subtitle}
                    </div>
                  )}
                </div>
                <span className={cn(
                  "text-2xs flex-shrink-0",
                  isSelected ? "text-white/95" : "text-[var(--fg-muted)]"
                )}>
                  {item.category}
                </span>
              </button>
            );
          })}
          {commands.length > MAX_RESULTS && !query.trim() && (
            <div className="px-3 py-1.5 text-center text-2xs text-[var(--fg-muted)]">
              {t("command.narrowSearch")}（{commands.length}）
            </div>
          )}
        </div>
      </div>
    </>
  );
}
