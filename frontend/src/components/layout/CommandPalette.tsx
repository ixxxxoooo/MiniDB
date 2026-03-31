import React, { useState, useEffect, useMemo } from "react";
import {
  Search,
  Database,
  Table2,
  Code,
  FileText,
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

export function CommandPalette({
  open,
  onClose,
  onOpenSettings,
  onNewConnection,
}: CommandPaletteProps) {
  const [query, setQuery] = useState("");
  const [selectedIndex, setSelectedIndex] = useState(0);
  const { connections, connectionStates, databases, tables } =
    useConnectionStore();
  const { addTab } = useTabsStore();
  const { resolved, setTheme } = useThemeStore();

  // 构建命令列表
  const commands = useMemo<CommandItem[]>(() => {
    const items: CommandItem[] = [];

    // 系统命令
    items.push(
      {
        id: "new-connection",
        title: "新建连接",
        icon: Database,
        action: () => {
          onNewConnection();
          onClose();
        },
        category: "操作",
      },
      {
        id: "settings",
        title: "打开设置",
        icon: Settings,
        action: () => {
          onOpenSettings();
          onClose();
        },
        category: "操作",
      },
      {
        id: "toggle-theme",
        title: resolved === "dark" ? "切换到浅色主题" : "切换到深色主题",
        icon: resolved === "dark" ? Sun : Moon,
        action: () => {
          setTheme(resolved === "dark" ? "light" : "dark");
          onClose();
        },
        category: "操作",
      }
    );

    // 数据库和表
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
              onClose();
            },
            category: "表",
          });
        }
      }
    }

    return items;
  }, [
    connections,
    connectionStates,
    databases,
    tables,
    resolved,
    addTab,
    onClose,
    onNewConnection,
    onOpenSettings,
    setTheme,
  ]);

  // 过滤
  const filtered = useMemo(() => {
    if (!query.trim()) return commands;
    const q = query.toLowerCase();
    return commands.filter(
      (c) =>
        c.title.toLowerCase().includes(q) ||
        c.subtitle?.toLowerCase().includes(q)
    );
  }, [commands, query]);

  // 键盘导航
  useEffect(() => {
    setSelectedIndex(0);
  }, [query]);

  useEffect(() => {
    if (!open) {
      setQuery("");
      setSelectedIndex(0);
    }
  }, [open]);

  const handleKeyDown = (e: React.KeyboardEvent) => {
    if (e.key === "ArrowDown") {
      e.preventDefault();
      setSelectedIndex((i) => Math.min(i + 1, filtered.length - 1));
    } else if (e.key === "ArrowUp") {
      e.preventDefault();
      setSelectedIndex((i) => Math.max(i - 1, 0));
    } else if (e.key === "Enter" && filtered[selectedIndex]) {
      filtered[selectedIndex].action();
    } else if (e.key === "Escape") {
      onClose();
    }
  };

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
        {/* 搜索框 */}
        <div className="flex items-center gap-2 px-4 py-3 border-b border-[var(--border-color)]">
          <Search className="h-4 w-4 text-[var(--fg-muted)]" />
          <input
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

        {/* 结果列表 */}
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
        </div>
      </div>
    </>
  );
}
