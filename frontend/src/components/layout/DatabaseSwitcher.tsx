import React, { useState, useEffect, useRef, useMemo } from "react";
import { Database, Search, Check } from "lucide-react";
import { cn } from "@/lib/utils";
import * as DatabaseServiceAPI from "../../../wailsjs/go/services/DatabaseService";

interface DatabaseSwitcherProps {
  open: boolean;
  onClose: () => void;
  connectionId: string;
  currentDatabase: string;
  onSelect: (dbName: string) => void;
}

interface SimpleDatabaseInfo {
  name: string;
}

export function DatabaseSwitcher({
  open,
  onClose,
  connectionId,
  currentDatabase,
  onSelect,
}: DatabaseSwitcherProps) {
  const [query, setQuery] = useState("");
  const [allDatabases, setAllDatabases] = useState<SimpleDatabaseInfo[]>([]);
  const [loading, setLoading] = useState(false);
  const [selectedIndex, setSelectedIndex] = useState(0);
  const inputRef = useRef<HTMLInputElement>(null);
  const panelRef = useRef<HTMLDivElement>(null);

  useEffect(() => {
    if (!open || !connectionId) return;
    setLoading(true);
    DatabaseServiceAPI.GetAllDatabases(connectionId)
      .then((dbs) => setAllDatabases((dbs || []).map((d: any) => ({ name: d.name || d }))))
      .catch(() => setAllDatabases([]))
      .finally(() => setLoading(false));
  }, [open, connectionId]);

  useEffect(() => {
    if (open) {
      setQuery("");
      setSelectedIndex(0);
      setTimeout(() => inputRef.current?.focus(), 50);
    }
  }, [open]);

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

  const filtered = useMemo(() => {
    if (!query.trim()) return allDatabases;
    const q = query.toLowerCase();
    return allDatabases.filter((db) => db.name.toLowerCase().includes(q));
  }, [allDatabases, query]);

  useEffect(() => { setSelectedIndex(0); }, [query]);

  const handleKeyDown = (e: React.KeyboardEvent) => {
    if (e.key === "ArrowDown") { e.preventDefault(); setSelectedIndex((i) => Math.min(i + 1, filtered.length - 1)); }
    else if (e.key === "ArrowUp") { e.preventDefault(); setSelectedIndex((i) => Math.max(i - 1, 0)); }
    else if (e.key === "Enter" && filtered[selectedIndex]) { onSelect(filtered[selectedIndex].name); onClose(); }
    else if (e.key === "Escape") { onClose(); }
  };

  if (!open) return null;

  return (
    <>
      <div className="fixed inset-0 z-[60] bg-black/20 backdrop-blur-sm" onClick={onClose} />
      <div
        ref={panelRef}
        className={cn(
          "fixed z-[60] top-[20%] left-1/2 -translate-x-1/2",
          "w-[360px] max-h-[350px] rounded-xl shadow-lg border overflow-hidden",
          "bg-[var(--surface)] border-[var(--border-color)]"
        )}
      >
        <div className="flex items-center gap-2 px-3 py-2.5 border-b border-[var(--border-color)]">
          <Search className="h-3.5 w-3.5 text-[var(--fg-muted)]" />
          <input
            ref={inputRef}
            className="flex-1 bg-transparent text-sm text-[var(--fg)] placeholder:text-[var(--fg-muted)] focus:outline-none"
            placeholder="搜索数据库..."
            value={query}
            onChange={(e) => setQuery(e.target.value)}
            onKeyDown={handleKeyDown}
            autoFocus
          />
        </div>
        <div className="overflow-y-auto max-h-[280px] py-0.5">
          {loading && <div className="px-4 py-4 text-center text-sm text-[var(--fg-muted)]">加载中...</div>}
          {!loading && filtered.length === 0 && <div className="px-4 py-4 text-center text-sm text-[var(--fg-muted)]">无匹配数据库</div>}
          {!loading && filtered.map((db, idx) => (
            <button
              key={db.name}
              className={cn(
                "w-full flex items-center gap-2.5 px-3 py-1.5 text-left transition-colors",
                idx === selectedIndex ? "bg-[var(--accent)] text-white" : "text-[var(--fg)] hover:bg-[var(--sidebar-hover)]"
              )}
              onClick={() => { onSelect(db.name); onClose(); }}
              onMouseEnter={() => setSelectedIndex(idx)}
            >
              <Database className={cn("h-3.5 w-3.5 flex-shrink-0", idx === selectedIndex ? "text-white/80" : "text-[var(--fg-secondary)]")} />
              <span className="text-sm flex-1 truncate">{db.name}</span>
              {db.name === currentDatabase && (
                <Check className={cn("h-3.5 w-3.5", idx === selectedIndex ? "text-white" : "text-[var(--success)]")} />
              )}
            </button>
          ))}
        </div>
      </div>
    </>
  );
}
