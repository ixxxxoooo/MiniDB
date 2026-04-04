import React, { useCallback, useEffect, useMemo, useRef, useState } from "react";
import { ChevronLeft, ChevronRight, Loader2, Sparkles } from "lucide-react";
import { useTabsStore, type QueryResultItem, type Tab } from "@/stores/tabs";
import { DataGrid } from "@/components/table/DataGrid";
import { RowPreview } from "@/components/table/RowPreview";
import { JSONPreviewDialog } from "@/components/table/JSONPreviewDialog";
import { SQLEditor } from "@/components/editor/SQLEditor";
import { RowContextMenu, type ContextMenuPosition } from "@/components/table/ContextMenu";
import { useUIStore } from "@/stores/ui";
import { useTranslation } from "@/i18n";
import { cn, copyToClipboard } from "@/lib/utils";
import { formatJSONForPreview } from "@/lib/json";
import { useConnectionStore } from "@/stores/connection";
import * as QueryService from "../../../wailsjs/go/services/QueryService";
import * as AIService from "../../../wailsjs/go/services/AIService";
import * as ExportService from "../../../wailsjs/go/services/ExportService";
import { extractJSONFromText, isEditableTarget, isGridTarget } from "./tabUtils";

function splitSQLStatements(sql: string): string[] {
  const results: string[] = [];
  let current = "";
  let inString = false;
  let stringChar = "";

  for (let i = 0; i < sql.length; i++) {
    const ch = sql[i];
    if (inString) {
      current += ch;
      if (ch === stringChar && sql[i - 1] !== "\\") {
        inString = false;
      }
    } else if (ch === "'" || ch === '"') {
      current += ch;
      inString = true;
      stringChar = ch;
    } else if (ch === ";") {
      const trimmed = current.trim();
      if (trimmed) results.push(trimmed);
      current = "";
    } else {
      current += ch;
    }
  }

  const trimmed = current.trim();
  if (trimmed) results.push(trimmed);
  if (results.length > 1) return results;

  const nonEmptyLines = sql
    .split(/\r?\n/)
    .map((line) => line.trim())
    .filter(Boolean);
  const sqlLineStart = /^(select|with|insert|update|delete|replace|show|desc|describe|explain|use|create|alter|drop|truncate|call)\b/i;
  if (nonEmptyLines.length > 1) {
    const normalizedLines = nonEmptyLines.map((line) => line.replace(/;+\s*$/, "").trim()).filter(Boolean);
    const allTerminated = nonEmptyLines.every((line) => /;+\s*$/.test(line));
    if (normalizedLines.length > 1 && allTerminated && normalizedLines.every((line) => sqlLineStart.test(line))) {
      return normalizedLines;
    }
  }

  const lines = sql
    .split(/\r?\n/)
    .map((line) => line.trim())
    .filter(Boolean);
  if (lines.length <= 1) return results;

  if (lines.every((line) => sqlLineStart.test(line))) {
    return lines;
  }
  return results;
}

export function QueryView({ tab }: { tab: Tab }) {
  const updateTab = useTabsStore((s) => s.updateTab);
  const { t, locale } = useTranslation();
  const queryDriver = useConnectionStore((s) =>
    s.connections.find((c) => c.id === tab.connectionId)?.type
  );
  const queryServerVersion = useConnectionStore((s) =>
    s.connectionStates[tab.connectionId || ""]?.serverVersion || ""
  );
  const queryDialect = useMemo(() => {
    if (queryDriver === "postgres") return "postgres";
    if (queryDriver === "sqlite") return "sqlite";
    if (queryDriver === "tidb") return "tidb";
    if (queryDriver === "starrocks") return "starrocks";
    return "mysql";
  }, [queryDriver]);

  const resultTabs = tab.queryResults || [];
  const activeResultIdx = tab.queryActiveIdx || 0;
  const [loading, setLoading] = useState(false);
  const { pageSize, showDataRowNumbers } = useUIStore();
  const [resultPage, setResultPage] = useState(1);
  const [jumpPageInput, setJumpPageInput] = useState("1");
  const [editorHeight, setEditorHeight] = useState(250);
  const resizingEditor = useRef(false);
  const resizeRafRef = useRef<number | null>(null);
  const pendingEditorHeightRef = useRef<number | null>(null);
  const [selectedRowIndex, setSelectedRowIndex] = useState<number | null>(null);
  const [contextMenu, setContextMenu] = useState<ContextMenuPosition | null>(null);
  const [clickedColumn, setClickedColumn] = useState<string | null>(null);
  const [contextRowIndex, setContextRowIndex] = useState<number | null>(null);
  const [jsonPreviewContent, setJsonPreviewContent] = useState<string | null>(null);
  const [aiFixing, setAIFixing] = useState(false);
  const [aiFixError, setAIFixError] = useState("");
  const { previewVisible, setPreviewVisible } = useUIStore();
  const gridContainerRef = useRef<HTMLDivElement>(null);

  const setResultTabs = useCallback((results: QueryResultItem[]) => {
    updateTab(tab.id, { queryResults: results });
  }, [tab.id, updateTab]);

  const setActiveResultIdx = useCallback((idx: number) => {
    updateTab(tab.id, { queryActiveIdx: idx });
  }, [tab.id, updateTab]);

  const handleSQLChange = useCallback((sql: string) => {
    updateTab(tab.id, { sql });
  }, [tab.id, updateTab]);

  useEffect(() => {
    const onRunSQL = (e: any) => {
      const { tabId, sql } = e.detail;
      if (tabId === tab.id && sql) {
        handleSQLChange(sql);
        handleExecute(sql);
      }
    };
    window.addEventListener("tableplus-ai:run-sql", onRunSQL);
    return () => window.removeEventListener("tableplus-ai:run-sql", onRunSQL);
  }, [tab.id, handleSQLChange]);

  const handleExecute = async (sql: string, page = 1) => {
    if (!tab.connectionId || !tab.database) return;
    const statements = splitSQLStatements(sql);
    if (statements.length === 0) return;
    setLoading(true);
    const results: QueryResultItem[] = [];
    for (const stmt of statements) {
      try {
        const stmtPage = statements.length > 1 ? 1 : page;
        const result = await QueryService.ExecuteSQLPaged(tab.connectionId, tab.database, stmt, stmtPage, pageSize);
        results.push({
          columns: result.columns || [],
          rows: result.rows || [],
          total: result.total || 0,
          duration: result.duration || 0,
          error: result.error || undefined,
          sql: stmt,
          autoLimited: (result as any).autoLimited || false,
        });
      } catch (e: any) {
        results.push({
          columns: [],
          rows: [],
          total: 0,
          duration: 0,
          error: e?.message || t("query.executionFailed"),
          sql: stmt,
        });
      }
    }
    setResultTabs(results);
    setActiveResultIdx(0);
    setResultPage(statements.length > 1 ? 1 : page);
    setSelectedRowIndex(null);
    setLoading(false);
  };

  const handleExecuteAll = async (sql: string) => {
    await handleExecute(sql, 1);
  };

  const activeResult = resultTabs[activeResultIdx];

  const handleResultPageChange = useCallback(async (newPage: number) => {
    if (!activeResult?.autoLimited || !activeResult.sql || !tab.connectionId || !tab.database) {
      setResultPage(newPage);
      return;
    }
    setLoading(true);
    try {
      const result = await QueryService.ExecuteSQLPaged(tab.connectionId, tab.database, activeResult.sql, newPage, pageSize);
      const nextTabs = [...resultTabs];
      nextTabs[activeResultIdx] = {
        columns: result.columns || [],
        rows: result.rows || [],
        total: result.total || 0,
        duration: result.duration || 0,
        error: result.error || undefined,
        sql: activeResult.sql,
        autoLimited: (result as any).autoLimited || false,
      };
      setResultTabs(nextTabs);
      setResultPage(newPage);
      setSelectedRowIndex(null);
    } catch (e: any) {
      useUIStore.getState().addToast("error", `${t("query.pageFailed")}: ${e?.message || e}`);
    } finally {
      setLoading(false);
    }
  }, [activeResult, activeResultIdx, pageSize, resultTabs, setResultTabs, t, tab.connectionId, tab.database]);

  const handleAnalyzeResultError = useCallback(async () => {
    if (!activeResult?.error || !activeResult.sql || !tab.connectionId || !tab.database) return;
    setAIFixError("");
    setAIFixing(true);
    try {
      const versionText = queryServerVersion
        ? (locale === "en-US" ? `\nCurrent database version: ${queryServerVersion}` : `\n当前数据库版本: ${queryServerVersion}`)
        : (locale === "en-US" ? "\nCurrent database version: unknown" : "\n当前数据库版本: 未知");

      const prompt = locale === "en-US"
        ? `You are a senior SQL error analyzer and fixer.\nCurrent SQL dialect: ${queryDialect}${versionText}\nPlease analyze the failing SQL and the error, then return STRICT JSON only:\n{\n  \"sql\": \"fixed SQL\",\n  \"analysis\": \"brief root cause and fix strategy\"\n}\nSQL:\n\`\`\`sql\n${activeResult.sql}\n\`\`\`\nError:\n${activeResult.error}`
        : `你是一个资深 SQL 报错分析与修复助手。\n当前数据库方言: ${queryDialect}${versionText}\n请根据失败 SQL 与报错信息进行分析，并严格只返回 JSON：\n{\n  \"sql\": \"修复后的 SQL\",\n  \"analysis\": \"根因和修复思路（简短）\"\n}\nSQL:\n\`\`\`sql\n${activeResult.sql}\n\`\`\`\n错误信息:\n${activeResult.error}`;

      const resp = await AIService.ChatAI(tab.connectionId, tab.database, [{ role: "user", content: prompt }] as any);
      const parsed = extractJSONFromText(String(resp?.content || ""));
      const fixedSQL = String(parsed?.sql || "").trim();
      if (!fixedSQL) {
        throw new Error(locale === "en-US" ? "AI did not return valid fixed SQL." : "AI 未返回有效修复 SQL。");
      }
      handleSQLChange(fixedSQL);
      await handleExecute(fixedSQL);
    } catch (e: any) {
      setAIFixError(e?.message || (locale === "en-US" ? "AI fix failed." : "AI 修复失败。"));
    } finally {
      setAIFixing(false);
    }
  }, [activeResult, tab.connectionId, tab.database, queryServerVersion, locale, queryDialect, handleSQLChange]);

  const isServerPaged = !!activeResult?.autoLimited;
  const totalPages = activeResult
    ? (isServerPaged
      ? Math.max(1, Math.ceil(activeResult.total / pageSize))
      : Math.max(1, Math.ceil(activeResult.rows.length / pageSize)))
    : 1;
  const pagedRows = activeResult
    ? (isServerPaged
      ? activeResult.rows
      : activeResult.rows.slice((resultPage - 1) * pageSize, resultPage * pageSize))
    : [];
  const selectedRow = selectedRowIndex !== null ? pagedRows[selectedRowIndex] : null;
  const contextRow = contextRowIndex !== null ? pagedRows[contextRowIndex] : null;
  const contextCellValue = useMemo(() => {
    if (!contextRow || !clickedColumn) return null;
    return contextRow[clickedColumn];
  }, [contextRow, clickedColumn]);
  const formattedContextJSON = useMemo(
    () => formatJSONForPreview(contextCellValue),
    [contextCellValue]
  );

  useEffect(() => {
    setJumpPageInput(String(resultPage));
  }, [resultPage]);

  useEffect(() => {
    const handler = (e: KeyboardEvent) => {
      if ((e.key === "ArrowDown" || e.key === "ArrowUp") && selectedRowIndex !== null && pagedRows.length > 0) {
        const target = e.target as HTMLElement;
        if (!isEditableTarget(target) && isGridTarget(target)) {
          e.preventDefault();
          const next = e.key === "ArrowDown"
            ? Math.min(selectedRowIndex + 1, pagedRows.length - 1)
            : Math.max(selectedRowIndex - 1, 0);
          setSelectedRowIndex(next);
          requestAnimationFrame(() => {
            const container = gridContainerRef.current;
            if (!container) return;
            const tr = container.querySelector(`tbody tr:nth-child(${next + 1})`) as HTMLElement;
            tr?.scrollIntoView({ block: "nearest" });
          });
          return;
        }
      }
      if (e.code === "Space" && selectedRow) {
        const target = e.target as HTMLElement;
        if (!isEditableTarget(target) && isGridTarget(target)) {
          e.preventDefault();
          setPreviewVisible(!previewVisible);
        }
      }
    };
    window.addEventListener("keydown", handler);
    return () => window.removeEventListener("keydown", handler);
  }, [pagedRows.length, selectedRow, selectedRowIndex, previewVisible, setPreviewVisible]);

  const handleJumpToPage = useCallback(() => {
    const parsed = Number.parseInt(jumpPageInput, 10);
    if (!Number.isFinite(parsed)) {
      setJumpPageInput(String(resultPage));
      return;
    }
    const target = Math.max(1, Math.min(totalPages, parsed));
    if (target !== resultPage) {
      if (isServerPaged) {
        void handleResultPageChange(target);
      } else {
        setResultPage(target);
        setSelectedRowIndex(null);
      }
    }
    setJumpPageInput(String(target));
  }, [jumpPageInput, resultPage, totalPages, isServerPaged, handleResultPageChange]);

  const handleContextMenu = useCallback((e: React.MouseEvent, rowIndex: number, columnName?: string) => {
    e.preventDefault();
    setContextRowIndex(rowIndex);
    setClickedColumn(columnName || null);
    setContextMenu({ x: e.clientX, y: e.clientY });
  }, []);

  const handleExportQueryResult = useCallback(async (format: "csv" | "json" | "sql" = "csv") => {
    if (!activeResult?.sql || !tab.connectionId || !tab.database) return;
    try {
      const taskId = await ExportService.ExportSQLResultStream(tab.connectionId, tab.database, activeResult.sql, format);
      if (!taskId) return;
    } catch (e: any) {
      useUIStore.getState().addToast("error", `导出失败: ${e?.message || e}`);
    }
  }, [activeResult, tab.connectionId, tab.database]);

  const handleEditorResizeStart = useCallback((e: React.MouseEvent) => {
    e.preventDefault();
    resizingEditor.current = true;
    const startY = e.clientY;
    const startH = editorHeight;
    const onMove = (ev: MouseEvent) => {
      if (!resizingEditor.current) return;
      const newH = Math.max(80, Math.min(800, startH + ev.clientY - startY));
      pendingEditorHeightRef.current = newH;
      if (resizeRafRef.current !== null) return;
      resizeRafRef.current = requestAnimationFrame(() => {
        resizeRafRef.current = null;
        if (pendingEditorHeightRef.current !== null) {
          setEditorHeight(pendingEditorHeightRef.current);
        }
      });
    };
    const onUp = () => {
      resizingEditor.current = false;
      if (resizeRafRef.current !== null) {
        cancelAnimationFrame(resizeRafRef.current);
        resizeRafRef.current = null;
      }
      if (pendingEditorHeightRef.current !== null) {
        setEditorHeight(pendingEditorHeightRef.current);
        pendingEditorHeightRef.current = null;
      }
      document.removeEventListener("mousemove", onMove);
      document.removeEventListener("mouseup", onUp);
    };
    document.addEventListener("mousemove", onMove);
    document.addEventListener("mouseup", onUp);
  }, [editorHeight]);

  return (
    <div className="flex flex-col h-full relative">
      <div style={{ height: editorHeight, minHeight: 80 }} className="flex-shrink-0">
        <SQLEditor
          initialSQL={tab.sql}
          onExecute={handleExecute}
          onExecuteAll={handleExecuteAll}
          onSQLChange={handleSQLChange}
          loading={loading}
          connectionId={tab.connectionId}
          database={tab.database}
          dialect={queryDialect}
          serverVersion={queryServerVersion}
        />
      </div>
      <div className="h-1 flex-shrink-0 cursor-row-resize group relative border-b border-[var(--border-color)] hover:bg-[var(--accent)]/20 transition-colors" onMouseDown={handleEditorResizeStart}>
        <div className="absolute inset-x-0 top-1/2 -translate-y-1/2 h-[3px] opacity-0 group-hover:opacity-100 bg-[var(--accent)]/30 transition-opacity rounded-full" />
      </div>
      {resultTabs.length > 1 && (
        <div className="flex items-center h-7 border-b border-[var(--border-color)] bg-[var(--surface-secondary)] overflow-x-auto">
          {resultTabs.map((r, idx) => (
            <button
              key={idx}
              className={cn(
                "px-3 h-full text-xs border-r border-[var(--border-subtle)] transition-colors whitespace-nowrap",
                idx === activeResultIdx ? "bg-[var(--surface)] text-[var(--fg)] font-medium" : "text-[var(--fg)] opacity-80 hover:opacity-100 hover:bg-[var(--tab-hover-bg)]"
              )}
              onClick={() => { setActiveResultIdx(idx); setResultPage(1); setSelectedRowIndex(null); }}
            >
              结果 {idx + 1}{r.error ? " ❌" : ` (${r.rows.length}行, ${r.duration}ms)`}
            </button>
          ))}
        </div>
      )}
      {activeResult?.error && (
        <div className="px-4 py-2 text-sm text-[var(--danger)] bg-red-50 dark:bg-red-900/10 border-b border-[var(--border-color)]">
          <div className="whitespace-pre-wrap break-words">{activeResult.error}</div>
          <div className="mt-1.5 flex items-center gap-2">
            <button className="h-6 px-2 rounded-[var(--radius-btn)] border border-[var(--danger)]/30 bg-white/70 dark:bg-red-900/20 text-[var(--danger)] hover:bg-white dark:hover:bg-red-900/30 transition-colors text-xs inline-flex items-center gap-1 disabled:opacity-60" onClick={handleAnalyzeResultError} disabled={aiFixing} type="button">
              {aiFixing ? <Loader2 className="h-3 w-3 animate-spin" /> : <Sparkles className="h-3 w-3" />}
              <span>{locale === "en-US" ? "AI Analyze & Fix" : "AI 分析并修复"}</span>
            </button>
            {aiFixError ? <span className="text-2xs text-[var(--danger)]/90">{aiFixError}</span> : null}
          </div>
        </div>
      )}
      <div className="relative flex flex-1 overflow-hidden" style={{ paddingBottom: "20px" }}>
        {activeResult && activeResult.columns.length > 0 ? (
          <>
            <div ref={gridContainerRef} className="flex-1 flex overflow-hidden">
              <DataGrid
                columns={activeResult.columns}
                data={pagedRows}
                selectedRowIndex={selectedRowIndex}
                onSelectRow={setSelectedRowIndex}
                onContextMenu={handleContextMenu}
                showRowNumbers={showDataRowNumbers}
                rowNumberOffset={(resultPage - 1) * pageSize}
                database={tab.database || ""}
                tableName={`__query_${tab.id}`}
              />
            </div>
            {previewVisible && selectedRow && selectedRowIndex !== null && (
              <RowPreview
                row={selectedRow}
                columns={activeResult.columns}
                tableName={tab.table || t("toolbar.sqlQuery")}
                rowKey={`${activeResultIdx}:${resultPage}:${selectedRowIndex}`}
                onClose={() => setPreviewVisible(false)}
              />
            )}
          </>
        ) : activeResult && !activeResult.error && activeResult.total > 0 ? (
          <div className="absolute inset-0 pb-5 flex items-center justify-center text-sm text-[var(--success)] select-none pointer-events-none">
            <div className="px-3 py-1.5 rounded-[var(--radius-btn)] bg-[var(--success)]/10 border border-[var(--success)]/20">
              操作成功，影响 {activeResult.total} 行 ({activeResult.duration}ms)
            </div>
          </div>
        ) : !activeResult ? (
          <div className="absolute inset-0 flex items-center justify-center text-sm text-[var(--fg-muted)] select-none pointer-events-none">
            <div className="max-w-md text-center px-4">
              <p>{t("query.showResultHint")}</p>
              <p className="text-2xs mt-2">{t("editor.shortcutsHint")}</p>
            </div>
          </div>
        ) : null}
      </div>
      {activeResult && (
        <div className="absolute bottom-0 left-0 right-0 z-20 h-5 flex items-center px-2 text-2xs text-[var(--fg-muted)] border-t border-[var(--border-color)] bg-[var(--surface-secondary)]">
          <span className="text-2xs">{isServerPaged ? `${activeResult.rows.length} 行 / 共 ${activeResult.total.toLocaleString()} 行` : `${activeResult.rows.length} 行`}</span>
          <span className="mx-1">·</span>
          <span className="text-2xs">{activeResult.duration}ms</span>
          <div className="flex-1" />
          {totalPages > 1 && (
            <div className="flex items-center gap-px ml-1">
              <button className="h-4 w-4 flex items-center justify-center rounded-[var(--radius-btn)] hover:bg-[var(--sidebar-hover)] disabled:opacity-30" disabled={resultPage <= 1 || loading} onClick={() => {
                const prev = resultPage - 1;
                if (isServerPaged) handleResultPageChange(prev);
                else { setResultPage(prev); setSelectedRowIndex(null); }
              }}>
                <ChevronLeft className="h-2.5 w-2.5" />
              </button>
              <span className="text-[var(--fg-secondary)] text-2xs min-w-[32px] text-center">{resultPage}/{totalPages}</span>
              <input
                value={jumpPageInput}
                onChange={(e) => setJumpPageInput(e.target.value.replace(/[^\d]/g, ""))}
                onKeyDown={(e) => {
                  if (e.key === "Enter") {
                    e.preventDefault();
                    handleJumpToPage();
                  }
                }}
                onBlur={handleJumpToPage}
                className="h-4 w-9 rounded-[var(--radius-sm)] border border-[var(--border-color)] bg-[var(--surface)] px-1 text-2xs text-center text-[var(--fg)] focus:outline-none focus:border-[var(--accent)]"
                title={t("common.gotoPage")}
                aria-label={t("common.gotoPage")}
              />
              <button className="h-4 w-4 flex items-center justify-center rounded-[var(--radius-btn)] hover:bg-[var(--sidebar-hover)] disabled:opacity-30" disabled={resultPage >= totalPages || loading} onClick={() => {
                const next = resultPage + 1;
                if (isServerPaged) handleResultPageChange(next);
                else { setResultPage(next); setSelectedRowIndex(null); }
              }}>
                <ChevronRight className="h-2.5 w-2.5" />
              </button>
            </div>
          )}
        </div>
      )}

      <RowContextMenu
        position={contextMenu}
        onClose={() => setContextMenu(null)}
        onCopyCell={() => {
          if (contextRow && clickedColumn) {
            copyToClipboard(String(contextRow[clickedColumn] ?? ""));
          }
          setContextMenu(null);
        }}
        onCopyRow={() => {
          if (contextRow) copyToClipboard(JSON.stringify(contextRow, null, 2));
          setContextMenu(null);
        }}
        onFormatJSON={() => {
          if (formattedContextJSON) {
            setJsonPreviewContent(formattedContextJSON);
          }
          setContextMenu(null);
        }}
        onCopyAsInsert={async () => {
          const targetRow = contextRow || selectedRow;
          if (targetRow && tab.table) {
            try {
              const sql = await QueryService.GenerateInsertSQL(tab.table, targetRow as Record<string, unknown>);
              await copyToClipboard(sql);
            } catch (e) {
              console.error("[QueryView] 生成 INSERT SQL 失败:", e);
            }
          }
          setContextMenu(null);
        }}
        showCopyAsInsert={false}
        onDeleteRow={() => { setContextMenu(null); }}
        onRefresh={() => {
          if (activeResult) handleExecute(activeResult.sql);
          setContextMenu(null);
        }}
        onPreview={() => {
          if (contextRow || selectedRow) {
            setPreviewVisible(true);
          }
          setContextMenu(null);
        }}
        onDownloadPage={() => { handleExportQueryResult("csv"); setContextMenu(null); }}
        showFormatJSON={!!formattedContextJSON}
      />

      <JSONPreviewDialog
        open={!!jsonPreviewContent}
        formattedJSON={jsonPreviewContent || ""}
        onClose={() => setJsonPreviewContent(null)}
      />
    </div>
  );
}
