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

// 最大显示结果数，避免大量表时渲染卡顿
const MAX_RESULTS = 50;

export function CommandPalette({
  open,
  onClose,
  onOpenSettings,
  onNewConnection,
}: CommandPaletteProps) {
  const [query, setQuery] = useState("");
  const [selectedIndex, setSelectedIndex] = useState(0);
  const inputRef = useRef<HTMLInputElement>(null);

  // 使用 ref 保存回调，避免依赖变化导致 commands 重新计算
  const closeFn = useRef(onClose);
  const openSettingsFn = useRef(onOpenSettings);
  const newConnectionFn = useRef(onNewConnection);
  closeFn.current = onClose;
  openSettingsFn.current = onOpenSettings;
  newConnectionFn.current = onNewConnection;

  const { connections, connectionStates, databases, tables } =
    useConnectionStore();
  const addTab = useTabsStore((s) => s.addTab);
  const { resolved, setTheme } = useThemeStore();

  // 只在 open 变化或数据变化时重建列表，回调通过 ref 引用
  const commands = useMemo<CommandItem[]>(() => {
    const items: CommandItem[] = [];

    items.push(
      {
        id: "new-connection",
        title: "新建连接",
        icon: Database,
        action: () => { newConnectionFn.current(); closeFn.current(); },
        category: "操作",
      },
      {
        id: "settings",
        title: "打开设置",
        icon: Settings,
        action: () => { openSettingsFn.current(); closeFn.current(); },
        category: "操作",
      },
      {
        id: "toggle-theme",
        title: resolved === "dark" ? "切换到浅色主题" : "切换到深色主题",
        icon: resolved === "dark" ? Sun : Moon,
        action: () => { setTheme(resolved === "dark" ? "light" : "dark"); closeFn.current(); },
        category: "操作",
      }
    );

    for (const conn of connections) {
      const state = connectionStates[conn.id];
      if (state?.status !== "connected") continue;

      const dbList = databases[conn.id] || [];
      for (const db of dbList) {
        const tableList = tables[`${conn.id}:${db.name}`] || [];
        for (const t of tableList) {
          items.push({
            id: `table:${conn.id}:${db.name}:${t.name}`,
            title: t.name,
            subtitle: `${conn.name} / ${db.name}`,
            icon: Table2,
            action: () => {
              addTab({
                type: "table",
                title: t.name,
                connectionId: conn.id,
                database: db.name,
                table: t.name,
                closable: true,
              });
              closeFn.current();
            },
            category: "表",
          });
        }
      }
    }

    return items;
  }, [connections, connectionStates, databases, tables, resolved, addTab, setTheme]);

  // 筛选并限制结果数
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
  }, [query]);

  useEffect(() => {
    if (!open) {
      setQuery("");
      setSelectedIndex(0);
    } else {
      // 打开时聚焦
      setTimeout(() => inputRef.current?.focus(), 50);
    }
  }, [open]);

  const handleKeyDown = useCallback((e: React.KeyboardEvent) => {
    if (e.key === "ArrowDown") {
      e.preventDefault();
      setSelectedIndex((i) => Math.min(i + 1, filtered.length - 1));
    } else if (e.key === "ArrowUp") {
      e.preventDefault();
      setSelectedIndex((i) => Math.max(i - 1, 0));
    } else if (e.key === "Enter" && filtered[selectedIndex]) {
      filtered[selectedIndex].action();
    } else if (e.key === "Escape") {
      closeFn.current();
    }
  }, [filtered, selectedIndex]);

  if (!open) return null;

  return (
    <>
      <div
        className="fixed inset-0 z-50 bg-black/30 backdrop-blur-sm"
        onClick={onClose}
      />
      <div
        className={cn(
          "fixed z-50 top-[20%] left-1/2 -translate-x-1/2",
          "w-[520px] max-h-[400px] rounded-xl shadow-lg border overflow-hidden",
          "bg-[var(--surface)] border-[var(--border-color)]"
        )}
      >
        <div className="flex items-center gap-2 px-4 py-3 border-b border-[var(--border-color)]">
          <Search className="h-4 w-4 text-[var(--fg-muted)]" />
          <input
            ref={inputRef}
            className="flex-1 bg-transparent text-sm text-[var(--fg)] placeholder:text-[var(--fg-muted)] focus:outline-none"
            placeholder="搜索表、操作..."
            value={query}
            onChange={(e) => setQuery(e.target.value)}
            onKeyDown={handleKeyDown}
            autoFocus
          />
          <kbd className="px-1.5 py-0.5 rounded text-2xs border border-[var(--border-color)] bg-[var(--surface-secondary)] text-[var(--fg-muted)]">
            ESC
          </kbd>
        </div>

        <div className="overflow-y-auto max-h-[320px] py-1">
          {filtered.length === 0 && (
            <div className="px-4 py-6 text-center text-sm text-[var(--fg-muted)]">
              无匹配结果
            </div>
          )}
          {filtered.map((item, idx) => {
            const Icon = item.icon;
            return (
              <button
                key={item.id}
                className={cn(
                  "w-full flex items-center gap-3 px-4 py-2 text-left transition-colors",
                  idx === selectedIndex
                    ? "bg-[var(--accent)]/10 text-[var(--accent)]"
                    : "text-[var(--fg)] hover:bg-[var(--sidebar-hover)]"
                )}
                onClick={item.action}
                onMouseEnter={() => setSelectedIndex(idx)}
              >
                <Icon className="h-4 w-4 flex-shrink-0 text-[var(--fg-secondary)]" />
                <div className="flex-1 min-w-0">
                  <div className="text-sm truncate">{item.title}</div>
                  {item.subtitle && (
                    <div className="text-2xs text-[var(--fg-muted)] truncate">
                      {item.subtitle}
                    </div>
                  )}
                </div>
                <span className="text-2xs text-[var(--fg-muted)]">
                  {item.category}
                </span>
              </button>
            );
          })}
          {commands.length > MAX_RESULTS && !query.trim() && (
            <div className="px-4 py-2 text-center text-2xs text-[var(--fg-muted)]">
              输入关键字缩小搜索范围（共 {commands.length} 项）
            </div>
          )}
        </div>
      </div>
    </>
  );
}
