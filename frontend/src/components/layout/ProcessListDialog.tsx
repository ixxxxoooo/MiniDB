import React, { useCallback, useEffect, useMemo, useState } from "react";
import { Activity, Loader2, RefreshCw, X } from "lucide-react";
import { Button } from "@/components/ui/button";
import { useTranslation } from "@/i18n";
import { cn } from "@/lib/utils";
import * as DatabaseService from "@/lib/wails/services/DatabaseService";
import { requestConfirm } from "@/stores/confirm";
import { useConnectionStore } from "@/stores/connection";
import { useUIStore } from "@/stores/ui";

interface ProcessRow {
  id: string;
  user: string;
  host: string;
  database: string;
  command: string;
  timeSec: number;
  state: string;
  info: string;
}

function formatDuration(sec: number): string {
  if (sec < 60) return `${sec}s`;
  if (sec < 3600) return `${Math.floor(sec / 60)}m ${sec % 60}s`;
  return `${Math.floor(sec / 3600)}h ${Math.floor((sec % 3600) / 60)}m`;
}

/** 服务端会话/进程列表：查看正在运行的 SQL，可中断查询或断开会话。 */
export function ProcessListDialog({ connectionId, onClose }: { connectionId: string; onClose: () => void }) {
  const { t } = useTranslation();
  const connName = useConnectionStore((s) => s.connections.find((c) => c.id === connectionId)?.name || "");
  const readOnly = useConnectionStore((s) => !!s.connections.find((c) => c.id === connectionId)?.readOnly);
  const [rows, setRows] = useState<ProcessRow[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState("");
  const [autoRefresh, setAutoRefresh] = useState(true);
  const [onlyActive, setOnlyActive] = useState(false);
  const [expanded, setExpanded] = useState<string | null>(null);

  const load = useCallback(async () => {
    try {
      const list = await DatabaseService.GetProcessList(connectionId);
      setRows((list || []) as ProcessRow[]);
      setError("");
    } catch (e: any) {
      setError(e?.message || String(e));
    } finally {
      setLoading(false);
    }
  }, [connectionId]);

  useEffect(() => {
    void load();
  }, [load]);

  useEffect(() => {
    if (!autoRefresh) return;
    const timer = window.setInterval(() => void load(), 5000);
    return () => window.clearInterval(timer);
  }, [autoRefresh, load]);

  useEffect(() => {
    const onKey = (e: KeyboardEvent) => {
      if (e.key === "Escape") onClose();
    };
    window.addEventListener("keydown", onKey);
    return () => window.removeEventListener("keydown", onKey);
  }, [onClose]);

  const visible = useMemo(() => {
    const list = onlyActive ? rows.filter((r) => r.info.trim() !== "" && r.command.toLowerCase() !== "sleep") : rows;
    return [...list].sort((a, b) => b.timeSec - a.timeSec);
  }, [rows, onlyActive]);

  const kill = async (row: ProcessRow, queryOnly: boolean) => {
    const ok = await requestConfirm({
      title: t(queryOnly ? "process.cancelQuery" : "process.killSession"),
      message: `${t("process.confirmKill", { id: row.id, user: row.user })}\n\n${row.info.slice(0, 300)}`,
      confirmText: t(queryOnly ? "process.cancelQuery" : "process.killSession"),
      danger: true,
    });
    if (!ok) return;
    try {
      await DatabaseService.KillProcess(connectionId, row.id, queryOnly);
      useUIStore.getState().addToast("success", t("process.killed", { id: row.id }));
      await load();
    } catch (e: any) {
      useUIStore.getState().addToast("error", `${t("process.killFailed")}: ${e?.message || e}`, 6000);
    }
  };

  return (
    <>
      <div className="fixed inset-0 z-50 bg-black/40 backdrop-blur-sm" onClick={onClose} />
      <div
        className={cn(
          "fixed z-50 top-1/2 left-1/2 -translate-x-1/2 -translate-y-1/2 w-[980px] max-w-[96vw] h-[600px] max-h-[90vh]",
          "rounded-[var(--radius-panel)] shadow-lg border overflow-hidden flex flex-col",
          "bg-[var(--surface)] border-[var(--border-color)]",
        )}
      >
        <div className="flex items-center justify-between px-4 py-2.5 border-b border-[var(--border-color)] bg-[var(--surface-secondary)]">
          <div className="flex items-center gap-2 min-w-0">
            <Activity className="h-4 w-4 text-[var(--fg-secondary)]" />
            <span className="text-sm font-medium">{t("process.title")}</span>
            <span className="text-2xs text-[var(--fg-muted)] truncate">{connName}</span>
            <span className="text-2xs text-[var(--fg-muted)]">· {t("process.count", { n: visible.length })}</span>
          </div>
          <div className="flex items-center gap-3">
            <label className="flex items-center gap-1 text-2xs text-[var(--fg-secondary)] cursor-pointer select-none">
              <input type="checkbox" checked={onlyActive} onChange={(e) => setOnlyActive(e.target.checked)} />
              {t("process.onlyActive")}
            </label>
            <label className="flex items-center gap-1 text-2xs text-[var(--fg-secondary)] cursor-pointer select-none">
              <input type="checkbox" checked={autoRefresh} onChange={(e) => setAutoRefresh(e.target.checked)} />
              {t("process.autoRefresh")}
            </label>
            <Button variant="ghost" size="icon" className="h-7 w-7" onClick={() => void load()} aria-label="refresh">
              <RefreshCw className="h-3.5 w-3.5" />
            </Button>
            <Button variant="ghost" size="icon" className="h-7 w-7" onClick={onClose} aria-label="close">
              <X className="h-3.5 w-3.5" />
            </Button>
          </div>
        </div>

        {error && <div className="px-4 py-2 text-xs text-[var(--danger)] border-b border-[var(--border-color)] whitespace-pre-wrap">{error}</div>}

        <div className="flex-1 overflow-auto">
          {loading ? (
            <div className="flex items-center justify-center h-full text-[var(--fg-muted)] text-xs gap-2">
              <Loader2 className="h-4 w-4 animate-spin" />
              {t("logViewer.loading")}
            </div>
          ) : (
            <table className="w-full text-xs border-collapse">
              <thead className="sticky top-0 bg-[var(--surface-secondary)] text-[var(--fg-secondary)]">
                <tr>
                  {["ID", t("process.user"), t("process.host"), t("process.db"), t("process.command"), t("process.time"), t("process.state"), "SQL", ""].map((h, i) => (
                    <th key={i} className="px-2 py-1.5 text-left font-medium border-b border-[var(--border-color)] whitespace-nowrap">{h}</th>
                  ))}
                </tr>
              </thead>
              <tbody>
                {visible.map((r) => (
                  <tr
                    key={r.id}
                    className="border-b border-[var(--border-subtle)] hover:bg-[var(--row-hover)] align-top cursor-default"
                    onClick={() => setExpanded((cur) => (cur === r.id ? null : r.id))}
                  >
                    <td className="px-2 py-1 font-mono whitespace-nowrap">{r.id}</td>
                    <td className="px-2 py-1 whitespace-nowrap">{r.user}</td>
                    <td className="px-2 py-1 whitespace-nowrap text-[var(--fg-secondary)]">{r.host}</td>
                    <td className="px-2 py-1 whitespace-nowrap">{r.database}</td>
                    <td className="px-2 py-1 whitespace-nowrap">{r.command}</td>
                    <td className={cn("px-2 py-1 whitespace-nowrap font-mono", r.timeSec >= 60 && r.info && "text-[var(--warning)]")}>{formatDuration(r.timeSec)}</td>
                    <td className="px-2 py-1 whitespace-nowrap text-[var(--fg-secondary)]">{r.state}</td>
                    <td className="px-2 py-1 font-mono max-w-[360px]">
                      <div className={cn(expanded === r.id ? "whitespace-pre-wrap break-all" : "truncate")}>{r.info}</div>
                    </td>
                    <td className="px-2 py-1 whitespace-nowrap text-right" onClick={(e) => e.stopPropagation()}>
                      {!readOnly && (
                        <>
                          <button className="px-1.5 text-[var(--accent)] hover:underline" onClick={() => void kill(r, true)}>
                            {t("process.cancelQuery")}
                          </button>
                          <button className="px-1.5 text-[var(--danger)] hover:underline" onClick={() => void kill(r, false)}>
                            {t("process.killSession")}
                          </button>
                        </>
                      )}
                    </td>
                  </tr>
                ))}
                {visible.length === 0 && !error && (
                  <tr>
                    <td colSpan={9} className="px-4 py-8 text-center text-[var(--fg-muted)]">{t("process.empty")}</td>
                  </tr>
                )}
              </tbody>
            </table>
          )}
        </div>
      </div>
    </>
  );
}
