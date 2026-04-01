import React, { useState, useEffect, useRef, useMemo } from "react";
import { Search, Star, Trash2, Clock, X } from "lucide-react";
import { cn } from "@/lib/utils";
import { useSQLHistoryStore, type SQLHistoryItem } from "@/stores/sqlHistory";
import { useTranslation } from "@/i18n";

interface SQLHistoryPanelProps {
  open: boolean;
  onClose: () => void;
  /** 选择一条SQL时回调，用于粘贴到编辑器 */
  onSelect: (sql: string) => void;
  /** 当前视图模式 */
  mode: "history" | "favorites";
}

export function SQLHistoryPanel({ open, onClose, onSelect, mode }: SQLHistoryPanelProps) {
  const [query, setQuery] = useState("");
  const [selectedIndex, setSelectedIndex] = useState(0);
  const inputRef = useRef<HTMLInputElement>(null);
  const panelRef = useRef<HTMLDivElement>(null);
  const listRef = useRef<HTMLDivElement>(null);
  const { t } = useTranslation();
  const { history, toggleFavorite, removeHistory, clearHistory } = useSQLHistoryStore();

  // 根据模式筛选
  const baseList = useMemo(() => {
    if (mode === "favorites") {
      return history.filter((h) => h.favorite);
    }
    return history;
  }, [history, mode]);

  // 搜索过滤
  const filtered = useMemo(() => {
    if (!query.trim()) return baseList;
    const q = query.toLowerCase();
    return baseList.filter(
      (h) => h.sql.toLowerCase().includes(q) || h.database.toLowerCase().includes(q)
    );
  }, [baseList, query]);

  useEffect(() => {
    if (open) {
      setQuery("");
      setSelectedIndex(0);
      setTimeout(() => inputRef.current?.focus(), 50);
    }
  }, [open]);

  useEffect(() => {
    setSelectedIndex(0);
  }, [query]);

  // 选中项滚动到可视区
  useEffect(() => {
    if (!listRef.current) return;
    const item = listRef.current.children[selectedIndex] as HTMLElement | undefined;
    item?.scrollIntoView({ block: "nearest" });
  }, [selectedIndex]);

  // 点击外部关闭
  useEffect(() => {
    if (!open) return;
    const handler = (e: MouseEvent) => {
      if (panelRef.current && !panelRef.current.contains(e.target as Node)) {
        onClose();
      }
    };
    document.addEventListener("mousedown", handler);
    return () => document.removeEventListener("mousedown", handler);
  }, [open, onClose]);

  const handleKeyDown = (e: React.KeyboardEvent) => {
    if (e.key === "ArrowDown") {
      e.preventDefault();
      setSelectedIndex((i) => Math.min(i + 1, filtered.length - 1));
    } else if (e.key === "ArrowUp") {
      e.preventDefault();
      setSelectedIndex((i) => Math.max(i - 1, 0));
    } else if (e.key === "Enter" && filtered[selectedIndex]) {
      onSelect(filtered[selectedIndex].sql);
      onClose();
    } else if (e.key === "Escape") {
      onClose();
    }
  };

  // 格式化时间
  const formatTime = (iso: string) => {
    const d = new Date(iso);
    const now = new Date();
    const diff = now.getTime() - d.getTime();
    const mins = Math.floor(diff / 60000);
    if (mins < 1) return "刚刚";
    if (mins < 60) return `${mins}分钟前`;
    const hours = Math.floor(mins / 60);
    if (hours < 24) return `${hours}小时前`;
    const days = Math.floor(hours / 24);
    if (days < 7) return `${days}天前`;
    return d.toLocaleDateString();
  };

  // SQL 预览截断
  const truncateSQL = (sql: string, max = 120) => {
    const oneLine = sql.replace(/\s+/g, " ").trim();
    return oneLine.length > max ? oneLine.slice(0, max) + "..." : oneLine;
  };

  if (!open) return null;

  return (
    <>
      <div className="fixed inset-0 z-[60] bg-black/20 backdrop-blur-sm" onClick={onClose} />
      <div
        ref={panelRef}
        className={cn(
          "fixed z-[60] top-[15%] left-1/2 -translate-x-1/2",
          "w-[480px] max-h-[420px] rounded-[var(--radius-panel)] shadow-lg border overflow-hidden",
          "bg-[var(--surface)] border-[var(--border-color)]"
        )}
      >
        {/* 搜索栏 */}
        <div className="flex items-center gap-2 px-3 py-2.5 border-b border-[var(--border-color)]">
          <Search className="h-3.5 w-3.5 text-[var(--fg-muted)]" />
          <input
            ref={inputRef}
            className="flex-1 bg-transparent text-sm text-[var(--fg)] placeholder:text-[var(--fg-muted)] focus:outline-none"
            placeholder={mode === "favorites" ? t("editor.searchFavorites") : t("editor.searchHistory")}
            value={query}
            onChange={(e) => setQuery(e.target.value)}
            onKeyDown={handleKeyDown}
            autoFocus
          />
          {mode === "history" && history.length > 0 && (
            <button
              className="text-2xs text-[var(--fg-muted)] hover:text-[var(--danger)] transition-colors"
              onClick={clearHistory}
              title={t("editor.clearHistory")}
            >
              {t("common.clear")}
            </button>
          )}
        </div>

        {/* 列表 */}
        <div ref={listRef} className="overflow-y-auto max-h-[350px] py-0.5">
          {filtered.length === 0 && (
            <div className="px-4 py-8 text-center text-sm text-[var(--fg-muted)]">
              {mode === "favorites" ? t("editor.noFavorites") : t("editor.noHistory")}
            </div>
          )}
          {filtered.map((item, idx) => {
            const isSelected = idx === selectedIndex;
            return (
              <button
                key={item.id}
                className={cn(
                  "w-full flex items-start gap-2.5 px-3 py-2 text-left transition-colors group",
                  isSelected
                    ? "bg-[var(--accent)] text-white"
                    : "text-[var(--fg)] hover:bg-[var(--sidebar-hover)]"
                )}
                onClick={() => { onSelect(item.sql); onClose(); }}
                onMouseEnter={() => setSelectedIndex(idx)}
              >
                {/* SQL 预览 */}
                <div className="flex-1 min-w-0">
                  <div className={cn(
                    "text-xs font-mono truncate",
                    isSelected ? "text-white" : "text-[var(--fg)]"
                  )}>
                    {truncateSQL(item.sql)}
                  </div>
                  <div className={cn(
                    "text-2xs mt-0.5 flex items-center gap-1.5",
                    isSelected ? "text-white/60" : "text-[var(--fg-muted)]"
                  )}>
                    <Clock className="h-2.5 w-2.5" />
                    <span>{formatTime(item.executedAt)}</span>
                    {item.database && (
                      <>
                        <span>·</span>
                        <span>{item.database}</span>
                      </>
                    )}
                  </div>
                </div>
                {/* 操作按钮 */}
                <div className="flex items-center gap-0.5 flex-shrink-0 mt-0.5">
                  <button
                    className={cn(
                      "h-4 w-4 flex items-center justify-center rounded transition-colors",
                      isSelected ? "hover:bg-white/20" : "hover:bg-[var(--sidebar-hover)]"
                    )}
                    onClick={(e) => {
                      e.stopPropagation();
                      toggleFavorite(item.id);
                    }}
                    title={item.favorite ? t("editor.unfavorite") : t("editor.favorite")}
                  >
                    <Star className={cn(
                      "h-2.5 w-2.5",
                      item.favorite
                        ? (isSelected ? "text-yellow-300 fill-yellow-300" : "text-yellow-500 fill-yellow-500")
                        : (isSelected ? "text-white/50" : "text-[var(--fg-muted)]")
                    )} />
                  </button>
                  <button
                    className={cn(
                      "h-4 w-4 flex items-center justify-center rounded transition-colors opacity-0 group-hover:opacity-100",
                      isSelected ? "hover:bg-white/20" : "hover:bg-[var(--sidebar-hover)]"
                    )}
                    onClick={(e) => {
                      e.stopPropagation();
                      removeHistory(item.id);
                    }}
                    title={t("common.delete")}
                  >
                    <Trash2 className={cn(
                      "h-2.5 w-2.5",
                      isSelected ? "text-white/50" : "text-[var(--fg-muted)]"
                    )} />
                  </button>
                </div>
              </button>
            );
          })}
        </div>
      </div>
    </>
  );
}
