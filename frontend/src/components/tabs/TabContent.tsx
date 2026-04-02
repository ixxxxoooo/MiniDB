import React, { useState, useCallback, useEffect, useRef, useMemo } from "react";
import { createPortal } from "react-dom";
import { useTabsStore, type Tab, type QueryResultItem } from "@/stores/tabs";
import { DataGrid } from "@/components/table/DataGrid";
import { DataGridToolbar, type FilterCondition } from "@/components/table/DataGridToolbar";
import { RowPreview } from "@/components/table/RowPreview";
import { DDLViewer } from "@/components/table/DDLViewer";
import { SQLEditor } from "@/components/editor/SQLEditor";
import { MarkdownEditor } from "@/components/editor/MarkdownEditor";
import {
  RowContextMenu,
  type ContextMenuPosition,
} from "@/components/table/ContextMenu";
import { Tooltip, TooltipTrigger, TooltipContent, TooltipProvider } from "@/components/ui/tooltip";
import { useUIStore } from "@/stores/ui";
import { useTranslation } from "@/i18n";
import { cn, copyToClipboard, rowToInsertSQL } from "@/lib/utils";
import { Database, RefreshCw, Download, ChevronLeft, ChevronRight, Plus, Trash2, Key, Hash, Undo2, ChevronDown, Sparkles, Loader2, Info, Check, X } from "lucide-react";
import { useConnectionStore } from "@/stores/connection";
import type { DatabaseDriver } from "@/types/connection";
import type { ColumnMeta, ColumnInfo } from "@/types/database";
import * as QueryService from "../../../wailsjs/go/services/QueryService";
import * as DatabaseService from "../../../wailsjs/go/services/DatabaseService";
import * as AIService from "../../../wailsjs/go/services/AIService";
import * as DocService from "../../../wailsjs/go/services/DocService";
import * as ExportService from "../../../wailsjs/go/services/ExportService";

// 带快捷键提示的按钮包装组件
function TipBtn({ tip, shortcut, children, ...rest }: {
  tip: string;
  shortcut?: string;
  children: React.ReactNode;
} & React.ButtonHTMLAttributes<HTMLButtonElement>) {
  return (
    <Tooltip delayDuration={400}>
      <TooltipTrigger asChild>
        <button {...rest}>{children}</button>
      </TooltipTrigger>
      <TooltipContent side="top" className="flex items-center gap-1.5">
        <span>{tip}</span>
        {shortcut && (
          <kbd className="ml-1 px-1 py-0.5 rounded border border-[var(--border-color)] bg-[var(--surface-secondary)] text-2xs font-mono text-[var(--fg-secondary)]">
            {shortcut}
          </kbd>
        )}
      </TooltipContent>
    </Tooltip>
  );
}

function extractJSONFromText(text: string): any | null {
  const direct = text.trim();
  try {
    return JSON.parse(direct);
  } catch {}
  const codeMatch = direct.match(/```json\s*([\s\S]*?)```/i);
  if (codeMatch) {
    try {
      return JSON.parse(codeMatch[1].trim());
    } catch {}
  }
  const objectMatch = direct.match(/\{[\s\S]*\}/);
  if (objectMatch) {
    try {
      return JSON.parse(objectMatch[0]);
    } catch {}
  }
  return null;
}

function isEditableTarget(target: EventTarget | null): boolean {
  if (!(target instanceof HTMLElement)) return false;
  const tagName = target.tagName;
  if (tagName === "INPUT" || tagName === "TEXTAREA" || tagName === "SELECT" || tagName === "BUTTON") {
    return true;
  }
  if (target.isContentEditable) {
    return true;
  }
  return Boolean(target.closest("input, textarea, select, button, [contenteditable='true'], .monaco-editor, .monaco-inputbox, [role='textbox']"));
}

function isGridTarget(target: EventTarget | null): boolean {
  return target instanceof HTMLElement && Boolean(target.closest("[role='grid']"));
}

export function TabContent() {
  const { tabs, activeTabId } = useTabsStore();
  const activeTab = tabs.find((t) => t.id === activeTabId);

  if (!activeTab) {
    return <EmptyState />;
  }

  // 使用 display:none 隐藏非活跃Tab而非卸载，保留查询页面的状态
  return (
    <TooltipProvider>
      {tabs.map((tab) => (
        <div
          key={tab.id}
          className="h-full"
          style={{ display: tab.id === activeTabId ? "flex" : "none", flexDirection: "column" }}
        >
          {tab.type === "table" && <TableView tab={tab} />}
          {tab.type === "query" && <QueryView tab={tab} />}
          {tab.type === "ddl" && <DDLView tab={tab} />}
          {tab.type === "doc" && <DocView tab={tab} />}
        </div>
      ))}
    </TooltipProvider>
  );
}

function EmptyState() {
  const { t } = useTranslation();
  return (
    <div className="h-full w-full flex items-center justify-center">
      <div className="flex flex-col items-center text-[var(--fg-muted)]">
        <Database className="h-12 w-12 mb-3 opacity-20" />
        <p className="text-base font-medium mb-0.5 text-[var(--fg-secondary)]">{t("empty.title")}</p>
        <p className="text-xs text-[var(--fg-muted)]">{t("empty.subtitle")}</p>
        <div className="mt-4 flex gap-4 text-2xs text-[var(--fg-muted)]">
          <div className="flex items-center gap-1">
            <kbd className="px-1 py-0.5 rounded border border-[var(--border-color)] bg-[var(--surface-secondary)] text-2xs">⌘P</kbd>
            <span>{t("empty.quickSearch")}</span>
          </div>
          <div className="flex items-center gap-1">
            <kbd className="px-1 py-0.5 rounded border border-[var(--border-color)] bg-[var(--surface-secondary)] text-2xs">⌘N</kbd>
            <span>{t("empty.newConnection")}</span>
          </div>
          <div className="flex items-center gap-1">
            <kbd className="px-1 py-0.5 rounded border border-[var(--border-color)] bg-[var(--surface-secondary)] text-2xs">⌘T</kbd>
            <span>{t("empty.newQuery")}</span>
          </div>
        </div>
      </div>
    </div>
  );
}

// =========== 表视图 ===========
type TableSubView = "data" | "structure" | "info" | "doc";

function TableView({ tab }: { tab: Tab }) {
  const { t } = useTranslation();
  // 获取当前连接的数据库驱动类型
  const driver = useConnectionStore((s) =>
    s.connections.find((c) => c.id === tab.connectionId)?.type
  );
  const [subView, setSubView] = useState<TableSubView>(
    (tab.initialSubView as TableSubView) || "data"
  );

  useEffect(() => {
    if (tab.initialSubView) setSubView(tab.initialSubView as TableSubView);
  }, [tab.initialSubView]);
  const [columns, setColumns] = useState<ColumnMeta[]>([]);
  const [data, setData] = useState<Record<string, unknown>[]>([]);
  const [totalRows, setTotalRows] = useState(0);
  const [queryDuration, setQueryDuration] = useState(0);
  const [selectedRowIndex, setSelectedRowIndex] = useState<number | null>(null);
  const [page, setPage] = useState(1);
  const [loading, setLoading] = useState(false);
  const [contextMenu, setContextMenu] = useState<ContextMenuPosition | null>(null);
  const [clickedColumn, setClickedColumn] = useState<string | null>(null);
  const [activeFilters, setActiveFilters] = useState<FilterCondition[]>([]);
  const [showFilter, setShowFilter] = useState(false);
  const [rawSqlFilter, setRawSqlFilter] = useState("");
  const [editedCells, setEditedCells] = useState<Record<string, unknown>>({});
  // 保存编辑前的原始行数据快照，用于提取正确的主键值
  const [originalData, setOriginalData] = useState<Record<string, unknown>[]>([]);
  // 记录新增行的索引集合（这些行需要 INSERT 而非 UPDATE）
  const [newRowIndexes, setNewRowIndexes] = useState<Set<number>>(new Set());
  // 记录待删除行的索引集合
  const [pendingDeleteIndexes, setPendingDeleteIndexes] = useState<Set<number>>(new Set());
  const { previewVisible, setPreviewVisible, pageSize } = useUIStore();
  const { addTab } = useTabsStore();

  const [structureColumns, setStructureColumns] = useState<ColumnInfo[]>([]);
  const [indexes, setIndexes] = useState<any[]>([]);
  const [ddl, setDDL] = useState("");
  const [docContent, setDocContent] = useState("");
  // Structure 视图的编辑状态和提交回调
  const [structureHasEdits, setStructureHasEdits] = useState(false);
  const structureCommitRef = useRef<(() => Promise<void>) | null>(null);
  const gridContainerRef = useRef<HTMLDivElement>(null);

  const selectedRow = selectedRowIndex !== null ? data[selectedRowIndex] : null;

  const loadData = useCallback(async (p: number, filters: FilterCondition[] = [], rawSql = "") => {
    if (!tab.connectionId || !tab.database || !tab.table) return;
    setLoading(true);
    try {
      let result;
      if (rawSql.trim()) {
        // 将用户输入的条件自动拼接为完整 SQL
        const trimmed = rawSql.trim().toUpperCase();
        const isFullSql = trimmed.startsWith("SELECT") || trimmed.startsWith("INSERT") ||
          trimmed.startsWith("UPDATE") || trimmed.startsWith("DELETE") || trimmed.startsWith("CREATE") ||
          trimmed.startsWith("ALTER") || trimmed.startsWith("DROP") || trimmed.startsWith("SHOW") ||
          trimmed.startsWith("DESCRIBE") || trimmed.startsWith("EXPLAIN");
        const finalSql = isFullSql
          ? rawSql.trim()
          : `SELECT * FROM ${tab.table} WHERE ${rawSql.trim()} LIMIT ${pageSize} OFFSET ${(p - 1) * pageSize}`;
        result = await QueryService.ExecuteSQL(tab.connectionId, tab.database, finalSql);
      } else {
        result = await QueryService.QueryTableData(
          tab.connectionId, tab.database, tab.table, p, pageSize, filters as any, []
        );
      }
      if (result) {
        const rows = result.rows || [];
        setColumns(result.columns || []);
        setData(rows);
        // 深拷贝保存原始数据，用于 commit 时提取正确 PK
        setOriginalData(rows.map((r) => ({ ...r })));
        setTotalRows(result.total || 0);
        setQueryDuration(result.duration || 0);
      }
    } catch (e) {
      console.error("加载表数据失败:", e);
    } finally {
      setLoading(false);
    }
  }, [tab.connectionId, tab.database, tab.table, pageSize]);

  const loadStructure = useCallback(async () => {
    if (!tab.connectionId || !tab.database || !tab.table) return;
    try {
      const [cols, idxs] = await Promise.all([
        DatabaseService.GetColumns(tab.connectionId, tab.database, tab.table),
        DatabaseService.GetIndexes(tab.connectionId, tab.database, tab.table),
      ]);
      // Wails 返回的 database.ColumnInfo 与我们的 ColumnInfo 接口兼容
      setStructureColumns((cols || []) as unknown as ColumnInfo[]);
      setIndexes(idxs || []);
    } catch (e) {
      console.error("加载表结构失败:", e);
    }
  }, [tab.connectionId, tab.database, tab.table]);

  const loadDDL = useCallback(async () => {
    if (!tab.connectionId || !tab.database || !tab.table) return;
    try {
      const d = await DatabaseService.GetDDL(tab.connectionId, tab.database, tab.table);
      setDDL(d || "");
    } catch {
      setDDL("-- 获取 DDL 失败");
    }
  }, [tab.connectionId, tab.database, tab.table]);

  const loadDoc = useCallback(async () => {
    if (!tab.connectionId || !tab.database || !tab.table) return;
    try {
      const doc = await DocService.GetTableDoc(tab.connectionId, tab.database, tab.table);
      setDocContent(doc || "");
    } catch {
      setDocContent("");
    }
  }, [tab.connectionId, tab.database, tab.table]);

  useEffect(() => {
    loadData(page, activeFilters);
    loadStructure();
  }, [loadData, page, loadStructure]);

  useEffect(() => {
    if (subView === "structure") loadStructure();
    if (subView === "info") loadDDL();
    if (subView === "doc") loadDoc();
  }, [subView, loadStructure, loadDDL, loadDoc]);

  const handleContextMenu = useCallback(
    (e: React.MouseEvent, _rowIndex: number, columnName?: string) => {
      e.preventDefault();
      setClickedColumn(columnName || null);
      setContextMenu({ x: e.clientX, y: e.clientY });
    },
    []
  );

  // 事务提交所有编辑修改（⌘S）— 支持新增行、修改行、删除行
  const commitChanges = useCallback(async () => {
    if (!tab.connectionId || !tab.database || !tab.table) return;
    if (Object.keys(editedCells).length === 0 && newRowIndexes.size === 0 && pendingDeleteIndexes.size === 0) return;

    const pkCols = structureColumns.filter((c: any) => c.isPrimary).map((c: any) => c.name);

    try {
      // 1. 处理待删除行
      for (const delIdx of pendingDeleteIndexes) {
        if (newRowIndexes.has(delIdx)) continue;
        if (pkCols.length === 0) { alert("无法删除：未找到主键列"); return; }
        const origRow = originalData[delIdx];
        if (!origRow) continue;
        const pk: Record<string, unknown> = {};
        for (const col of pkCols) pk[col] = origRow[col];
        console.log("[提交变更] 删除行:", JSON.stringify(pk));
        await QueryService.DeleteRow(tab.connectionId, tab.database, tab.table, pk as any);
      }

      // 2. 处理新增行
      for (const newIdx of newRowIndexes) {
        if (pendingDeleteIndexes.has(newIdx)) continue;
        const row = data[newIdx];
        if (!row) continue;
        const rowData: Record<string, any> = {};
        for (const col of columns) {
          if (row[col.name] !== null && row[col.name] !== undefined) {
            rowData[col.name] = row[col.name];
          }
        }
        if (Object.keys(rowData).length === 0) continue;
        console.log("[提交变更] 插入行:", JSON.stringify(rowData));
        await QueryService.InsertRow(tab.connectionId, tab.database, tab.table, rowData);
      }

      // 3. 处理修改行（排除新增行和待删除行）
      if (pkCols.length > 0) {
        const changesByRow: Record<number, Record<string, unknown>> = {};
        for (const [key, val] of Object.entries(editedCells)) {
          const [rowStr, col] = key.split(":");
          const rowIdx = parseInt(rowStr);
          if (newRowIndexes.has(rowIdx) || pendingDeleteIndexes.has(rowIdx)) continue;
          if (!changesByRow[rowIdx]) changesByRow[rowIdx] = {};
          changesByRow[rowIdx][col] = val;
        }

        const updates: { primaryKey: Record<string, unknown>; changes: Record<string, unknown> }[] = [];
        for (const [rowIdxStr, changes] of Object.entries(changesByRow)) {
          const rowIdx = parseInt(rowIdxStr);
          const origRow = originalData[rowIdx];
          if (!origRow) continue;
          const pk: Record<string, unknown> = {};
          for (const col of pkCols) pk[col] = origRow[col];
          updates.push({ primaryKey: pk, changes });
        }

        if (updates.length > 0) {
          console.log("[提交变更] 批量更新:", JSON.stringify(updates));
          await QueryService.BatchUpdateRows(tab.connectionId, tab.database, tab.table, updates as any);
        }
      }

      console.log("[提交变更] 全部提交成功，重新加载数据");
      setEditedCells({});
      setNewRowIndexes(new Set());
      setPendingDeleteIndexes(new Set());
      await loadData(page, activeFilters, rawSqlFilter);
    } catch (e: any) {
      console.error("事务提交失败:", e);
      alert("事务提交失败: " + (e?.message || e));
    }
  }, [editedCells, originalData, structureColumns, tab, page, activeFilters, rawSqlFilter, loadData, newRowIndexes, pendingDeleteIndexes, data, columns]);

  // 快捷键
  useEffect(() => {
    const handler = (e: KeyboardEvent) => {
      if ((e.metaKey || e.ctrlKey) && e.key === "f") {
        e.preventDefault();
        setShowFilter((v) => !v);
      }
      if ((e.metaKey || e.ctrlKey) && e.key === "s") {
        e.preventDefault();
        // Structure 视图使用独立的提交逻辑
        if (subView === "structure" && structureCommitRef.current) {
          structureCommitRef.current();
        } else {
          commitChanges();
        }
      }
      if ((e.metaKey || e.ctrlKey) && e.key === "r") {
        e.preventDefault();
        if (subView === "structure") {
          loadStructure();
        } else {
          setEditedCells({});
          setNewRowIndexes(new Set());
          setPendingDeleteIndexes(new Set());
          loadData(page, activeFilters, rawSqlFilter);
        }
      }
      // ⌃⌘[ 向左循环切换子视图，⌃⌘] 向右循环切换子视图
      if (e.ctrlKey && e.metaKey) {
        const subViews: TableSubView[] = ["data", "structure", "info", "doc"];
        const curIdx = subViews.indexOf(subView);
        if (e.key === "[") {
          e.preventDefault();
          setSubView(subViews[(curIdx - 1 + subViews.length) % subViews.length]);
          return;
        }
        if (e.key === "]") {
          e.preventDefault();
          setSubView(subViews[(curIdx + 1) % subViews.length]);
          return;
        }
      }
      // 上下键切换选中行（仅在非输入框聚焦且处于 data 子视图时生效）
      if ((e.key === "ArrowDown" || e.key === "ArrowUp") && subView === "data") {
        const target = e.target as HTMLElement;
        if (!isEditableTarget(target)) {
          e.preventDefault();
          const cur = selectedRowIndex ?? -1;
          const next = e.key === "ArrowDown"
            ? Math.min(cur + 1, data.length - 1)
            : Math.max(cur - 1, 0);
          setSelectedRowIndex(next);
          // 滚动到选中行
          requestAnimationFrame(() => {
            const container = gridContainerRef.current;
            if (!container) return;
            const tr = container.querySelector(`tbody tr:nth-child(${next + 1})`) as HTMLElement;
            tr?.scrollIntoView({ block: "nearest" });
          });
          return;
        }
      }
      if (e.code === "Space" && selectedRow && subView === "data") {
        const target = e.target as HTMLElement;
        if (!isEditableTarget(target) && isGridTarget(target)) {
          e.preventDefault();
          setPreviewVisible(!previewVisible);
        }
      }
    };
    window.addEventListener("keydown", handler);
    return () => window.removeEventListener("keydown", handler);
  }, [selectedRow, selectedRowIndex, data, previewVisible, setPreviewVisible, commitChanges, loadData, loadStructure, page, activeFilters, rawSqlFilter, subView]);

  // 流式导出整表：先弹窗选路径，再后台分批查询+推送进度
  const handleExportTable = useCallback(async (format: "csv" | "json" | "sql" = "csv") => {
    if (!tab.connectionId || !tab.database || !tab.table) return;
    try {
      const taskId = await ExportService.ExportTableStream(tab.connectionId, tab.database, tab.table, format);
      if (!taskId) return; // 用户取消了路径选择
      console.log(`[流式导出] 任务已启动: taskId=${taskId} format=${format}`);
    } catch (e: any) {
      useUIStore.getState().addToast("error", `导出失败: ${e?.message || e}`);
      console.error(`[流式导出] 启动失败:`, e);
    }
  }, [tab.connectionId, tab.database, tab.table]);

  const handleCellEdit = useCallback((rowIdx: number, column: string, value: unknown) => {
    const key = `${rowIdx}:${column}`;
    setEditedCells((prev) => ({ ...prev, [key]: value }));
    setData((prev) => {
      const next = [...prev];
      next[rowIdx] = { ...next[rowIdx], [column]: value };
      return next;
    });
  }, []);

  // 新增空行到底部（内联编辑）
  const handleAddRow = useCallback(() => {
    const emptyRow: Record<string, unknown> = {};
    for (const col of columns) {
      emptyRow[col.name] = null;
    }
    setData((prev) => {
      const newIdx = prev.length;
      setNewRowIndexes((s) => new Set(s).add(newIdx));
      setSelectedRowIndex(newIdx);
      return [...prev, emptyRow];
    });
  }, [columns]);

  // 删除选中行
  const handleDeleteSelectedRow = useCallback(async () => {
    if (selectedRowIndex === null) return;
    const isNewRow = newRowIndexes.has(selectedRowIndex);
    if (isNewRow) {
      // 新增行直接从前端数据中移除
      setData((prev) => prev.filter((_, i) => i !== selectedRowIndex));
      setNewRowIndexes((prev) => {
        const next = new Set<number>();
        for (const idx of prev) {
          if (idx < selectedRowIndex) next.add(idx);
          else if (idx > selectedRowIndex) next.add(idx - 1);
        }
        return next;
      });
      // 清理该行的编辑记录
      setEditedCells((prev) => {
        const next: Record<string, unknown> = {};
        for (const [key, val] of Object.entries(prev)) {
          const rowIdx = parseInt(key.split(":")[0]);
          const col = key.split(":")[1];
          if (rowIdx < selectedRowIndex) next[key] = val;
          else if (rowIdx > selectedRowIndex) next[`${rowIdx - 1}:${col}`] = val;
        }
        return next;
      });
      setSelectedRowIndex(null);
    } else {
      // 已有行标记待删除
      setPendingDeleteIndexes((prev) => new Set(prev).add(selectedRowIndex));
    }
  }, [selectedRowIndex, newRowIndexes]);

  const totalPages = Math.max(1, Math.ceil(totalRows / pageSize));
  const hasEdits = Object.keys(editedCells).length > 0 || newRowIndexes.size > 0 || pendingDeleteIndexes.size > 0;

  return (
    <div className="flex flex-col h-full relative">
      {showFilter && subView === "data" && (
        <DataGridToolbar
          tableName={tab.table || ""}
          totalRows={totalRows}
          page={page}
          pageSize={pageSize}
          columns={columns}
          onPageChange={setPage}
          onRefresh={() => loadData(page, activeFilters, rawSqlFilter)}
          onOpenQuery={() =>
            addTab({
              type: "query",
              title: `${t("tabs.newQuery")} - ${tab.table}`,
              connectionId: tab.connectionId,
              database: tab.database,
              table: tab.table,
              closable: true,
              sql: `SELECT * FROM ${tab.table} LIMIT ${pageSize};`,
            })
          }
          onExport={() => handleExportTable("csv")}
          onFiltersChange={(filters) => {
            setActiveFilters(filters);
            setPage(1);
            loadData(1, filters);
          }}
          rawSqlFilter={rawSqlFilter}
          onRawSqlChange={(sql) => setRawSqlFilter(sql)}
          onRawSqlExecute={() => {
            if (rawSqlFilter.trim()) loadData(1, [], rawSqlFilter);
          }}
        />
      )}

      {/* 主内容区 */}
      <div className="flex flex-1 overflow-hidden" style={{ paddingBottom: "var(--size-btn)" }}>
        {subView === "data" && (
          <>
            <div ref={gridContainerRef} className="flex-1 flex overflow-hidden">
              <DataGrid
                columns={columns}
                data={data}
                selectedRowIndex={selectedRowIndex}
                onSelectRow={setSelectedRowIndex}
                onContextMenu={handleContextMenu}
                editedCells={editedCells}
                onCellEdit={handleCellEdit}
                database={tab.database || ""}
                tableName={tab.table || ""}
                newRowIndexes={newRowIndexes}
                pendingDeleteIndexes={pendingDeleteIndexes}
              />
            </div>
            {previewVisible && selectedRow && selectedRowIndex !== null && (
              <RowPreview
                row={selectedRow}
                columns={columns}
                tableName={tab.table || ""}
                onClose={() => setPreviewVisible(false)}
                onEdit={(column, value) => {
                  handleCellEdit(selectedRowIndex, column, value);
                }}
              />
            )}
          </>
        )}

        {subView === "structure" && (
          <StructureView
            connectionId={tab.connectionId || ""}
            database={tab.database || ""}
            tableName={tab.table || ""}
            driver={driver}
            columns={structureColumns}
            indexes={indexes}
            onRefresh={loadStructure}
            onHasEditsChange={setStructureHasEdits}
            commitRef={structureCommitRef}
          />
        )}

        {subView === "info" && (
          <DDLViewer ddl={ddl} tableName={tab.table || ""} />
        )}

        {subView === "doc" && (
          <MarkdownEditor
            content={docContent}
            tableName={tab.table || ""}
            onSave={async (md) => {
              if (tab.connectionId && tab.database && tab.table) {
                await DocService.SaveTableDoc(tab.connectionId, tab.database, tab.table, md);
                setDocContent(md);
              }
            }}
          />
        )}
      </div>

      {/* 底部功能栏 — 参考 TablePlus 紧凑底栏 */}
      <div className={cn(
        "absolute bottom-0 left-0 right-0 z-20",
        "h-[var(--size-btn)] flex items-center px-[var(--size-padding-sm)] border-t text-[length:var(--size-font-2xs)] select-none gap-[var(--size-gap-sm)]",
        "bg-[var(--surface-secondary)] border-[var(--border-color)]"
      )}>
        {/* 左侧：子视图切换 — 分段控制器 */}
        <div className="flex items-center flex-shrink-0 bg-[var(--sidebar-hover)] rounded-[var(--radius-btn)] p-0.5 gap-0">
          {(["data", "structure", "info", "doc"] as TableSubView[]).map((v) => (
            <button
              key={v}
              className={cn(
                "px-2.5 py-0.5 rounded-[calc(var(--radius-btn)-2px)] text-[length:var(--size-font-2xs)] transition-all whitespace-nowrap",
                subView === v
                  ? "bg-white dark:bg-[var(--surface)] text-[var(--fg)] font-medium shadow-sm ring-1 ring-black/5 dark:ring-white/10"
                  : "text-[var(--fg-muted)] hover:text-[var(--fg)]"
              )}
              onClick={() => setSubView(v)}
            >
              {v === "data" ? t("contextMenu.viewData") : v === "structure" ? t("contextMenu.viewStructure") : v === "info" ? t("contextMenu.viewDDL") : t("contextMenu.tableDoc")}
            </button>
          ))}
        </div>

        <div className="flex-1 min-w-0" />

        {/* 右侧：操作按钮（Data 视图） */}
        <div className={cn("flex items-center gap-0.5 flex-shrink-0", subView !== "data" && "hidden")}>
          <TipBtn
            tip={t("common.create")}
            className="h-[var(--size-btn-sm)] w-[var(--size-btn-sm)] flex items-center justify-center rounded-[var(--radius-btn)] text-[var(--fg-secondary)] hover:bg-[var(--sidebar-hover)] transition-colors"
            onClick={handleAddRow}
          >
            <Plus className="h-2.5 w-2.5" />
          </TipBtn>
          <TipBtn
            tip={t("contextMenu.deleteRow")}
            shortcut="⌫"
            className={cn(
              "h-[var(--size-btn-sm)] w-[var(--size-btn-sm)] flex items-center justify-center rounded-[var(--radius-btn)] transition-colors",
              selectedRowIndex !== null
                ? "text-[var(--fg-secondary)] hover:bg-[var(--danger)]/10 hover:text-[var(--danger)]"
                : "text-[var(--fg-muted)] opacity-40 cursor-not-allowed"
            )}
            onClick={handleDeleteSelectedRow}
            disabled={selectedRowIndex === null}
          >
            <Trash2 className="h-2.5 w-2.5" />
          </TipBtn>
          <div className="w-px h-3 bg-[var(--border-color)] mx-0.5" />
          <TipBtn
            tip={t("datagrid.addCondition")}
            shortcut="⌘F"
            className={cn(
              "px-1.5 py-0.5 rounded-[var(--radius-btn)] text-[length:var(--size-font-2xs)] transition-colors",
              showFilter
                ? "text-[var(--accent)] bg-[var(--accent)]/10 font-medium"
                : "text-[var(--fg-secondary)] hover:bg-[var(--sidebar-hover)]"
            )}
            onClick={() => setShowFilter((v) => !v)}
          >
            {t("datagrid.addCondition")}
          </TipBtn>
          <TipBtn
            tip={t("toolbar.sqlQuery")}
            className="px-1.5 py-0.5 rounded-[var(--radius-btn)] text-[length:var(--size-font-2xs)] text-[var(--fg-secondary)] hover:bg-[var(--sidebar-hover)] transition-colors font-mono"
            onClick={() =>
              addTab({
                type: "query",
                title: `SQL - ${tab.table}`,
                connectionId: tab.connectionId,
                database: tab.database,
                table: tab.table,
                closable: true,
                sql: `SELECT * FROM ${tab.table} LIMIT ${pageSize};`,
              })
            }
          >
            SQL
          </TipBtn>
          <ExportDropdown onExport={handleExportTable} />
          <TipBtn tip={t("common.refresh")} shortcut="⌘R" className="h-[var(--size-btn-sm)] w-[var(--size-btn-sm)] flex items-center justify-center rounded-[var(--radius-btn)] hover:bg-[var(--sidebar-hover)] transition-colors" onClick={() => { setEditedCells({}); setNewRowIndexes(new Set()); setPendingDeleteIndexes(new Set()); loadData(page, activeFilters, rawSqlFilter); }}>
            <RefreshCw className="h-2.5 w-2.5 text-[var(--fg-secondary)]" />
          </TipBtn>
          {hasEdits && (
            <TipBtn
              tip={t("common.commit")}
              shortcut="⌘S"
              className="px-1.5 py-0.5 rounded-[var(--radius-btn)] text-[length:var(--size-font-2xs)] font-medium text-white bg-[var(--accent)] hover:opacity-90 transition-opacity"
              onClick={commitChanges}
            >
              {t("common.commit")}
            </TipBtn>
          )}
        </div>

        {/* 右侧：操作按钮（Structure 视图） */}
        <div className={cn("flex items-center gap-0.5 flex-shrink-0", subView !== "structure" && "hidden")}>
          <TipBtn tip={t("common.refresh")} shortcut="⌘R" className="h-[var(--size-btn-sm)] w-[var(--size-btn-sm)] flex items-center justify-center rounded-[var(--radius-btn)] hover:bg-[var(--sidebar-hover)] transition-colors" onClick={loadStructure}>
            <RefreshCw className="h-2.5 w-2.5 text-[var(--fg-secondary)]" />
          </TipBtn>
          {structureHasEdits && (
            <TipBtn
              tip={t("common.commit")}
              shortcut="⌘S"
              className="px-1.5 py-0.5 rounded-[var(--radius-btn)] text-[length:var(--size-font-2xs)] font-medium text-white bg-[var(--accent)] hover:opacity-90 transition-opacity"
              onClick={() => structureCommitRef.current?.()}
            >
              {t("common.commit")}
            </TipBtn>
          )}
        </div>

        <div className="w-px h-3 bg-[var(--border-color)] mx-0.5 flex-shrink-0" />

        <span className="text-[var(--fg-muted)] text-2xs flex-shrink-0">{totalRows.toLocaleString()} {t("common.rows")}</span>
        <span className="text-[var(--fg-muted)] flex-shrink-0 mx-0.5">·</span>
        <span className="text-[var(--fg-muted)] text-2xs flex-shrink-0">{queryDuration}ms</span>

        {subView === "data" && (
          <div className="flex items-center gap-px ml-1 flex-shrink-0">
            <button className="h-4 w-4 flex items-center justify-center rounded-[var(--radius-btn)] hover:bg-[var(--sidebar-hover)] disabled:opacity-30" disabled={page <= 1} onClick={() => setPage(page - 1)}>
              <ChevronLeft className="h-2.5 w-2.5 text-[var(--fg-secondary)]" />
            </button>
            <span className="text-[var(--fg-secondary)] text-2xs min-w-[32px] text-center">{page}/{totalPages}</span>
            <button className="h-4 w-4 flex items-center justify-center rounded-[var(--radius-btn)] hover:bg-[var(--sidebar-hover)] disabled:opacity-30" disabled={page >= totalPages} onClick={() => setPage(page + 1)}>
              <ChevronRight className="h-2.5 w-2.5 text-[var(--fg-secondary)]" />
            </button>
          </div>
        )}
      </div>

      {/* 右键菜单 */}
      <RowContextMenu
        position={contextMenu}
        onClose={() => setContextMenu(null)}
        onCopyCell={() => {
          if (selectedRow && clickedColumn) {
            copyToClipboard(String(selectedRow[clickedColumn] ?? ""));
          }
          setContextMenu(null);
        }}
        onCopyRow={() => {
          if (selectedRow) copyToClipboard(JSON.stringify(selectedRow, null, 2));
          setContextMenu(null);
        }}
        onCopyAsInsert={() => {
          if (selectedRow && tab.table) copyToClipboard(rowToInsertSQL(tab.table, selectedRow));
          setContextMenu(null);
        }}
        onDeleteRow={() => {
          if (selectedRowIndex !== null) {
            handleDeleteSelectedRow();
          }
          setContextMenu(null);
        }}
        onRefresh={() => { loadData(page, activeFilters, rawSqlFilter); setContextMenu(null); }}
        onPreview={() => { setPreviewVisible(true); setContextMenu(null); }}
        onDownloadPage={() => { handleExportTable("csv"); setContextMenu(null); }}
      />
    </div>
  );
}

// =========== 导出格式下拉按钮 ===========
function ExportDropdown({ onExport }: { onExport: (format: "csv" | "json" | "sql") => void }) {
  const [open, setOpen] = useState(false);
  const ref = useRef<HTMLDivElement>(null);
  const { t } = useTranslation();

  useEffect(() => {
    if (!open) return;
    const handler = (e: MouseEvent) => {
      if (ref.current && !ref.current.contains(e.target as Node)) setOpen(false);
    };
    document.addEventListener("mousedown", handler);
    return () => document.removeEventListener("mousedown", handler);
  }, [open]);

  return (
    <div className="relative" ref={ref}>
      <TipBtn
        tip={t("logViewer.exportTitle")}
        className="h-[var(--size-btn-sm)] w-[var(--size-btn-sm)] flex items-center justify-center rounded-[var(--radius-btn)] hover:bg-[var(--sidebar-hover)] transition-colors"
        onClick={() => setOpen((v) => !v)}
      >
        <Download className="h-2.5 w-2.5 text-[var(--fg-secondary)]" />
      </TipBtn>
      {open && (
        <div className="absolute right-0 top-full z-[100] mt-0.5 min-w-[120px] py-0.5 rounded-[var(--radius-menu)] shadow-lg border bg-[var(--surface-elevated)] border-[var(--border-color)]">
          {(["csv", "json", "sql"] as const).map((fmt) => (
            <button
              key={fmt}
              className="w-full px-2.5 py-1 text-xs text-left hover:bg-[var(--sidebar-hover)] text-[var(--fg)] flex items-center gap-2"
              onClick={() => { onExport(fmt); setOpen(false); }}
            >
              <Download className="h-3 w-3" /> {fmt.toUpperCase()}
            </button>
          ))}
        </div>
      )}
    </div>
  );
}

// =========== 表结构视图（参考 TablePlus —— 上下分栏 + 内联编辑） ===========

// 各数据库引擎支持的全部字段类型列表
const DATA_TYPES_MAP: Record<DatabaseDriver, string[]> = {
  mysql: [
    // 数值类型
    "tinyint", "smallint", "mediumint", "int", "bigint",
    "decimal", "numeric", "float", "double", "bit", "boolean",
    // 日期/时间类型
    "date", "datetime", "timestamp", "time", "year",
    // 字符串类型
    "char", "varchar", "tinytext", "text", "mediumtext", "longtext",
    "binary", "varbinary", "tinyblob", "blob", "mediumblob", "longblob",
    "enum", "set",
    // JSON 类型
    "json",
    // 空间类型
    "geometry", "point", "linestring", "polygon",
    "multipoint", "multilinestring", "multipolygon", "geometrycollection",
  ],
  postgres: [
    // 数值类型
    "smallint", "integer", "bigint", "decimal", "numeric",
    "real", "double precision", "smallserial", "serial", "bigserial",
    // 货币类型
    "money",
    // 字符类型
    "character varying", "varchar", "character", "char", "text",
    // 二进制类型
    "bytea",
    // 日期/时间类型
    "date", "time", "time with time zone",
    "timestamp", "timestamp with time zone", "interval",
    // 布尔类型
    "boolean",
    // 枚举类型
    "enum",
    // 位串类型
    "bit", "bit varying",
    // 网络地址类型
    "cidr", "inet", "macaddr", "macaddr8",
    // 几何类型
    "box", "circle", "line", "lseg", "path", "point", "polygon",
    // JSON 类型
    "json", "jsonb",
    // UUID
    "uuid",
    // XML
    "xml",
    // 全文搜索类型
    "tsquery", "tsvector",
    // 范围类型
    "int4range", "int8range", "numrange", "tsrange", "tstzrange", "daterange",
    // 数组（常用）
    "integer[]", "text[]", "boolean[]", "jsonb[]",
  ],
  sqlite: [
    // SQLite 核心存储类 + 常用亲和类型
    "INTEGER", "REAL", "TEXT", "BLOB", "NUMERIC",
    "INT", "TINYINT", "SMALLINT", "MEDIUMINT", "BIGINT",
    "UNSIGNED BIG INT", "INT2", "INT8",
    "CHARACTER(20)", "VARCHAR(255)", "VARYING CHARACTER(255)",
    "NCHAR(55)", "NATIVE CHARACTER(70)", "NVARCHAR(100)", "CLOB",
    "DOUBLE", "DOUBLE PRECISION", "FLOAT",
    "DECIMAL(10,5)", "BOOLEAN", "DATE", "DATETIME",
  ],
  tidb: [
    // 数值类型
    "tinyint", "smallint", "mediumint", "int", "bigint",
    "decimal", "numeric", "float", "double", "bit", "boolean",
    // 日期/时间类型
    "date", "datetime", "timestamp", "time", "year",
    // 字符串类型
    "char", "varchar", "tinytext", "text", "mediumtext", "longtext",
    "binary", "varbinary", "tinyblob", "blob", "mediumblob", "longblob",
    "enum", "set",
    // JSON 类型
    "json",
  ],
  starrocks: [
    // 数值类型
    "BOOLEAN", "TINYINT", "SMALLINT", "INT", "BIGINT", "LARGEINT",
    "FLOAT", "DOUBLE", "DECIMAL",
    // 字符串类型
    "CHAR", "VARCHAR", "STRING", "BINARY", "VARBINARY",
    // 日期/时间类型
    "DATE", "DATETIME",
    // 半结构化类型
    "JSON", "ARRAY", "MAP", "STRUCT",
    // 其他类型
    "BITMAP", "HLL", "PERCENTILE",
  ],
};

// 根据驱动类型获取数据类型列表，默认 mysql
function getDataTypes(driver: DatabaseDriver | undefined): string[] {
  return DATA_TYPES_MAP[driver || "mysql"] || DATA_TYPES_MAP.mysql;
}

// Columns 表格的列定义
interface StructureColDef {
  key: string;
  label: string;
  editable: boolean;
  minWidth: number;
  isTypeSelect?: boolean;
  isCheckbox?: boolean;
}

const STRUCTURE_COL_DEFS: StructureColDef[] = [
  { key: "name", label: "column_name", editable: true, minWidth: 140 },
  { key: "type", label: "data_type", editable: true, minWidth: 120, isTypeSelect: true },
  { key: "characterSet", label: "character_set", editable: false, minWidth: 100 },
  { key: "collation", label: "collation", editable: false, minWidth: 130 },
  { key: "nullable", label: "is_nullable", editable: true, minWidth: 80, isCheckbox: true },
  { key: "defaultValue", label: "column_default", editable: true, minWidth: 110 },
  { key: "extra", label: "extra", editable: false, minWidth: 100 },
  { key: "foreignKey", label: "foreign_key", editable: false, minWidth: 110 },
  { key: "comment", label: "comment", editable: true, minWidth: 140 },
];

// Indexes 表格的列定义
const INDEX_COL_DEFS = [
  { key: "name", label: "index_name", minWidth: 160 },
  { key: "type", label: "index_algorithm", minWidth: 120 },
  { key: "isUnique", label: "is_unique", minWidth: 90 },
  { key: "columns", label: "column_name", minWidth: 220 },
] as const;

// 内联编辑的列行标记
interface EditingStructureCol extends ColumnInfo {
  __status?: "new" | "modified" | "deleted";
  __uid: string;
}

interface EditingIndexRow {
  __uid: string;
  __status?: "new" | "deleted";
  name: string;
  type: string;
  isUnique: boolean;
  columns: string[];
  isPrimary?: boolean;
}

function StructureView({
  connectionId,
  database: dbName,
  tableName,
  driver,
  columns,
  indexes,
  onRefresh,
  onHasEditsChange,
  commitRef,
}: {
  connectionId: string;
  database: string;
  tableName: string;
  driver?: DatabaseDriver;
  columns: ColumnInfo[];
  indexes: any[];
  onRefresh: () => void;
  onHasEditsChange: (hasEdits: boolean) => void;
  commitRef: React.MutableRefObject<(() => Promise<void>) | null>;
}) {
  const { t } = useTranslation();
  // 工作副本（可增删改，与 columns prop 形成 diff）
  const [workingCols, setWorkingCols] = useState<EditingStructureCol[]>([]);
  // 原始快照（用于 diff 生成 DDL）
  const [originalCols, setOriginalCols] = useState<EditingStructureCol[]>([]);

  // 当前正在内联编辑的单元格
  const [editingCell, setEditingCell] = useState<{ uid: string; key: string } | null>(null);
  const [editValue, setEditValue] = useState("");
  const editInputRef = useRef<HTMLInputElement | HTMLSelectElement>(null);
  // 当前选中的行
  const [selectedUid, setSelectedUid] = useState<string | null>(null);

  // 上下面板拖拽分割
  const [topHeight, setTopHeight] = useState<number>(0);
  const containerRef = useRef<HTMLDivElement>(null);
  const resizingRef = useRef(false);

  // 索引区内联新增状态
  const [workingIndexes, setWorkingIndexes] = useState<EditingIndexRow[]>([]);
  const [selectedIndexUid, setSelectedIndexUid] = useState<string | null>(null);
  const [editingIndexCell, setEditingIndexCell] = useState<{ uid: string; key: "name" | "columns" } | null>(null);
  const [indexEditValue, setIndexEditValue] = useState("");
  const indexInputRef = useRef<HTMLInputElement>(null);

  // 数据类型下拉框的搜索与显示状态
  const [typeDropdownOpen, setTypeDropdownOpen] = useState(false);
  const [typeFilter, setTypeFilter] = useState("");
  // 下拉列表中键盘高亮的索引（-1 表示无高亮）
  const [typeHighlightIdx, setTypeHighlightIdx] = useState(-1);
  // 下拉列表 fixed 定位坐标（portal 到 body 时使用）
  const [typeDropdownPos, setTypeDropdownPos] = useState<{ top: number; left: number; width: number }>({ top: 0, left: 0, width: 180 });
  const typeInputRef = useRef<HTMLInputElement>(null);
  const typeDropdownRef = useRef<HTMLDivElement>(null);

  // 将 prop columns 同步为工作副本
  useEffect(() => {
    const mapped: EditingStructureCol[] = columns.map((c, i) => ({
      ...c,
      __uid: `orig_${i}_${c.name}`,
    }));
    setWorkingCols(mapped);
    setOriginalCols(mapped.map((c) => ({ ...c })));
    setEditingCell(null);
    setSelectedUid(null);
  }, [columns]);

  useEffect(() => {
    const mappedIndexes: EditingIndexRow[] = (indexes || []).map((idx: any, i: number) => ({
      __uid: `idx_${i}_${idx.name}`,
      name: idx.name || "",
      type: idx.type || "BTREE",
      isUnique: !!idx.isUnique,
      columns: Array.isArray(idx.columns) ? idx.columns : [],
      isPrimary: !!idx.isPrimary,
    }));
    setWorkingIndexes(mappedIndexes);
    setSelectedIndexUid(null);
    setEditingIndexCell(null);
    setIndexEditValue("");
  }, [indexes]);

  useEffect(() => {
    if (editingIndexCell && indexInputRef.current) {
      indexInputRef.current.focus();
      indexInputRef.current.select();
    }
  }, [editingIndexCell]);

  // 初始分割高度：60% 给 Columns
  useEffect(() => {
    if (containerRef.current && topHeight === 0) {
      setTopHeight(Math.floor(containerRef.current.clientHeight * 0.6));
    }
  }, [topHeight]);

  // 计算是否有变更
  const hasEdits = useMemo(() => {
    if (workingCols.length !== originalCols.length) return true;
    for (let i = 0; i < workingCols.length; i++) {
      const w = workingCols[i];
      if (w.__status === "new" || w.__status === "deleted") return true;
      const o = originalCols.find((c) => c.__uid === w.__uid);
      if (!o) return true;
      if (w.name !== o.name || w.type !== o.type || w.nullable !== o.nullable ||
        (w.defaultValue ?? "") !== (o.defaultValue ?? "") || w.comment !== o.comment) return true;
    }
    return false;
  }, [workingCols, originalCols]);

  // 通知父组件编辑状态
  useEffect(() => {
    onHasEditsChange(hasEdits);
  }, [hasEdits, onHasEditsChange]);

  // 拼接列定义的 DDL 片段（NULL/NOT NULL + DEFAULT + COMMENT）
  const buildColumnClause = useCallback((col: EditingStructureCol, prefix: string) => {
    let ddl = `${prefix} ${col.type}`;
    if (!col.nullable) {
      ddl += " NOT NULL";
      // NOT NULL 列不生成 DEFAULT NULL
      if (col.defaultValue && col.defaultValue !== "NULL") {
        ddl += ` DEFAULT ${col.defaultValue}`;
      }
    } else {
      ddl += " NULL";
      if (col.defaultValue && col.defaultValue !== "NULL") {
        ddl += ` DEFAULT ${col.defaultValue}`;
      } else {
        ddl += " DEFAULT NULL";
      }
    }
    if (col.comment) ddl += ` COMMENT '${col.comment.replace(/'/g, "\\'")}'`;
    return ddl;
  }, []);

  // 提交变更：根据 diff 生成 ALTER TABLE DDL 批量执行
  const commitStructureChanges = useCallback(async () => {
    const sqlParts: string[] = [];

    for (const w of workingCols) {
      if (w.__status === "new") {
        if (!w.name.trim() || !w.type.trim()) continue;
        sqlParts.push(buildColumnClause(w, `ADD COLUMN \`${w.name}\``));
      } else if (w.__status === "deleted") {
        const orig = originalCols.find((c) => c.__uid === w.__uid);
        if (orig) sqlParts.push(`DROP COLUMN \`${orig.name}\``);
      } else {
        const orig = originalCols.find((c) => c.__uid === w.__uid);
        if (!orig) continue;
        const changed = w.name !== orig.name || w.type !== orig.type || w.nullable !== orig.nullable ||
          (w.defaultValue ?? "") !== (orig.defaultValue ?? "") || w.comment !== orig.comment;
        if (!changed) continue;
        sqlParts.push(buildColumnClause(w, `CHANGE COLUMN \`${orig.name}\` \`${w.name}\``));
      }
    }

    for (const idx of workingIndexes) {
      if (idx.__status === "new") {
        const indexName = idx.name.trim();
        const indexColumns = idx.columns.map((col) => col.trim()).filter(Boolean);
        if (!indexName || indexColumns.length === 0) continue;
        const cols = indexColumns.map((c) => `\`${c}\``).join(", ");
        const uniqueStr = idx.isUnique ? "UNIQUE " : "";
        sqlParts.push(`ADD ${uniqueStr}INDEX \`${indexName}\` (${cols})`);
      } else if (idx.__status === "deleted") {
        if (!idx.isPrimary && idx.name) {
          sqlParts.push(`DROP INDEX \`${idx.name}\``);
        }
      }
    }

    // 检查被删除但已从 workingCols 中被标记的列
    for (const o of originalCols) {
      const inWorking = workingCols.find((w) => w.__uid === o.__uid);
      if (!inWorking) {
        sqlParts.push(`DROP COLUMN \`${o.name}\``);
      }
    }

    if (sqlParts.length === 0) return;

    const fullSQL = `ALTER TABLE \`${tableName}\` ${sqlParts.join(", ")}`;
    try {
      console.log("[Structure] 批量提交 DDL:", fullSQL);
      await DatabaseService.ExecuteRawSQL(connectionId, dbName, fullSQL);
      console.log("[Structure] 提交成功，刷新结构");
      onRefresh();
    } catch (e: any) {
      console.error("[Structure] DDL 执行失败:", e);
      alert(`${t("structure.commitFailed")}: ` + (e?.message || e));
    }
  }, [workingCols, originalCols, workingIndexes, tableName, connectionId, dbName, onRefresh, buildColumnClause, t]);

  // 注册到父组件的 ref
  useEffect(() => {
    commitRef.current = commitStructureChanges;
  }, [commitStructureChanges, commitRef]);

  // 添加新行
  const handleAddColumn = useCallback(() => {
    const uid = `new_${Date.now()}_${Math.random().toString(36).slice(2, 7)}`;
    const newRow: EditingStructureCol = {
      name: "", type: "varchar(255)", nullable: true, defaultValue: null,
      isPrimary: false, isAutoIncrement: false, comment: "", maxLength: null,
      characterSet: "", collation: "", extra: "", foreignKey: "",
      __status: "new", __uid: uid,
    };
    setWorkingCols((prev) => [...prev, newRow]);
    // 自动聚焦到新行的 name 列
    requestAnimationFrame(() => {
      setEditingCell({ uid, key: "name" });
      setEditValue("");
      setSelectedUid(uid);
    });
  }, []);

  // 删除选中行（标记为 deleted）
  const handleDeleteSelected = useCallback(() => {
    if (!selectedUid) return;
    setWorkingCols((prev) => prev.map((c) => {
      if (c.__uid !== selectedUid) return c;
      if (c.__status === "new") return { ...c, __status: "deleted" as const };
      return { ...c, __status: "deleted" as const };
    }).filter((c) => !(c.__status === "deleted" && c.__uid.startsWith("new_"))));
    setSelectedUid(null);
  }, [selectedUid]);

  // 撤销所有修改
  const handleRevertAll = useCallback(() => {
    setWorkingCols(originalCols.map((c) => ({ ...c })));
    setEditingCell(null);
    setSelectedUid(null);
  }, [originalCols]);

  // 根据输入框所在 td 单元格的位置计算下拉列表的 fixed 坐标
  const updateDropdownPos = useCallback(() => {
    if (!typeInputRef.current) return;
    // 向上找到 td 单元格，用 td 的宽度保持下拉与单元格对齐
    const td = typeInputRef.current.closest("td");
    const rect = td ? td.getBoundingClientRect() : typeInputRef.current.getBoundingClientRect();
    setTypeDropdownPos({
      top: rect.bottom + 1,
      left: rect.left,
      width: rect.width,
    });
  }, []);

  // 双击进入编辑
  const handleCellDoubleClick = useCallback((uid: string, key: string, currentValue: unknown) => {
    const colDef = STRUCTURE_COL_DEFS.find((d) => d.key === key);
    if (!colDef || !colDef.editable) return;
    const row = workingCols.find((c) => c.__uid === uid);
    if (row?.__status === "deleted") return;

    if (colDef.isCheckbox) {
      // checkbox 列直接切换
      setWorkingCols((prev) => prev.map((c) => {
        if (c.__uid !== uid) return c;
        return { ...c, [key]: !c[key as keyof EditingStructureCol] };
      }));
      return;
    }

    setEditingCell({ uid, key });
    const strVal = currentValue === null || currentValue === undefined ? "" : String(currentValue);
    if (colDef.isTypeSelect) {
      // 打开下拉时清空过滤条件，显示完整类型列表
      setTypeFilter("");
      setTypeDropdownOpen(true);
      setTypeHighlightIdx(-1);
      // 延迟计算下拉位置（等待 input 渲染完成）
      requestAnimationFrame(() => updateDropdownPos());
    }
    setEditValue(strVal);
  }, [workingCols, updateDropdownPos]);

  // 提交单元格编辑
  const commitCellEdit = useCallback(() => {
    if (!editingCell) return;
    const { uid, key } = editingCell;
    setWorkingCols((prev) => prev.map((c) => {
      if (c.__uid !== uid) return c;
      const updated = { ...c, [key]: key === "defaultValue" && editValue === "" ? null : editValue };
      if (c.__status !== "new") {
        const orig = originalCols.find((o) => o.__uid === uid);
        if (orig) {
          const changed = updated.name !== orig.name || updated.type !== orig.type ||
            updated.nullable !== orig.nullable || (updated.defaultValue ?? "") !== (orig.defaultValue ?? "") ||
            updated.comment !== orig.comment;
          updated.__status = changed ? "modified" : undefined;
        }
      }
      return updated;
    }));
    setEditingCell(null);
    setTypeDropdownOpen(false);
    setTypeHighlightIdx(-1);
  }, [editingCell, editValue, originalCols]);

  const cancelCellEdit = useCallback(() => {
    setEditingCell(null);
    setTypeDropdownOpen(false);
    setTypeHighlightIdx(-1);
  }, []);

  // 编辑框获取焦点
  useEffect(() => {
    if (editingCell && editInputRef.current) {
      editInputRef.current.focus();
      if (editInputRef.current instanceof HTMLInputElement) {
        editInputRef.current.select();
      }
    }
  }, [editingCell]);

  // 点击下拉框外部关闭
  useEffect(() => {
    if (!typeDropdownOpen) return;
    const handler = (e: MouseEvent) => {
      if (typeDropdownRef.current && !typeDropdownRef.current.contains(e.target as Node) &&
        typeInputRef.current && !typeInputRef.current.contains(e.target as Node)) {
        commitCellEdit();
      }
    };
    document.addEventListener("mousedown", handler);
    return () => document.removeEventListener("mousedown", handler);
  }, [typeDropdownOpen, commitCellEdit]);

  // 键盘高亮项自动滚动到可见区域
  useEffect(() => {
    if (!typeDropdownOpen || typeHighlightIdx < 0 || !typeDropdownRef.current) return;
    const items = typeDropdownRef.current.children;
    if (items[typeHighlightIdx]) {
      (items[typeHighlightIdx] as HTMLElement).scrollIntoView({ block: "nearest" });
    }
  }, [typeHighlightIdx, typeDropdownOpen]);

  // 分割条拖拽逻辑
  const handleSplitDragStart = useCallback((e: React.MouseEvent) => {
    e.preventDefault();
    resizingRef.current = true;
    const startY = e.clientY;
    const startH = topHeight;
    const containerH = containerRef.current?.clientHeight || 600;
    const onMove = (ev: MouseEvent) => {
      if (!resizingRef.current) return;
      const newH = Math.max(100, Math.min(containerH - 100, startH + ev.clientY - startY));
      setTopHeight(newH);
    };
    const onUp = () => {
      resizingRef.current = false;
      document.removeEventListener("mousemove", onMove);
      document.removeEventListener("mouseup", onUp);
    };
    document.addEventListener("mousemove", onMove);
    document.addEventListener("mouseup", onUp);
  }, [topHeight]);

  // 执行原始 SQL
  const execSQL = async (sql: string) => {
    try {
      console.log("[Structure] 执行 SQL:", sql);
      await DatabaseService.ExecuteRawSQL(connectionId, dbName, sql);
      onRefresh();
    } catch (e: any) {
      console.error("[Structure] SQL 执行失败:", e);
      alert(`${t("structure.operationFailed")}: ` + (e?.message || e));
    }
  };

  // 添加索引
  const handleAddIndex = useCallback(() => {
    const newIndexUid = `new_idx_${Date.now()}`;
    setWorkingIndexes((prev) => {
      if (prev.some((item) => item.__status === "new" && !item.name && item.columns.length === 0)) {
        return prev;
      }
      return [
        ...prev,
        {
          __uid: newIndexUid,
          __status: "new",
          name: "",
          type: "BTREE",
          isUnique: false,
          columns: [],
          isPrimary: false,
        },
      ];
    });
    setEditingIndexCell({ uid: newIndexUid, key: "name" });
    setSelectedIndexUid(newIndexUid);
    setIndexEditValue("");
  }, []);

  const startEditIndexCell = useCallback((uid: string, key: "name" | "columns", value: string) => {
    setEditingIndexCell({ uid, key });
    setIndexEditValue(value);
  }, []);

  const cancelEditIndexCell = useCallback(() => {
    setEditingIndexCell(null);
    setIndexEditValue("");
  }, []);

  const commitEditIndexCell = useCallback(() => {
    if (!editingIndexCell) return;
    const nextValue = indexEditValue.trim();
    setWorkingIndexes((prev) => prev.map((item) => {
      if (item.__uid !== editingIndexCell.uid) return item;
      if (editingIndexCell.key === "name") {
        return { ...item, name: nextValue };
      }
      return {
        ...item,
        columns: nextValue
          ? nextValue.split(",").map((part) => part.trim()).filter(Boolean)
          : [],
      };
    }));
    setEditingIndexCell(null);
    setIndexEditValue("");
  }, [editingIndexCell, indexEditValue]);

  const handleToggleInlineIndexUnique = useCallback((uid: string, checked: boolean) => {
    setWorkingIndexes((prev) => prev.map((item) => (
      item.__uid === uid ? { ...item, isUnique: checked } : item
    )));
  }, []);

  const handleCreateInlineIndex = useCallback(async (uid: string) => {
    const target = workingIndexes.find((item) => item.__uid === uid);
    if (!target) return;
    const indexName = target.name.trim();
    const indexColumns = target.columns.map((col) => col.trim()).filter(Boolean);
    if (!indexName || indexColumns.length === 0) {
      alert(t("structure.indexInlineRequired"));
      return;
    }
    const cols = indexColumns.map((c) => `\`${c}\``).join(", ");
    const uniqueStr = target.isUnique ? "UNIQUE " : "";
    await execSQL(`ALTER TABLE \`${tableName}\` ADD ${uniqueStr}INDEX \`${indexName}\` (${cols})`);
  }, [execSQL, tableName, t, workingIndexes]);

  const handleCancelInlineIndex = useCallback((uid: string) => {
    setWorkingIndexes((prev) => prev.filter((item) => item.__uid !== uid));
    setEditingIndexCell((prev) => (prev?.uid === uid ? null : prev));
    setIndexEditValue("");
  }, []);

  // 删除索引
  const handleDropIndex = async (idxName: string) => {
    if (!confirm(t("structure.dropIndexConfirm", { name: idxName }))) return;
    await execSQL(`ALTER TABLE \`${tableName}\` DROP INDEX \`${idxName}\``);
    setSelectedIndexUid(null);
  };

  const handleDeleteSelectedIndex = useCallback(() => {
    if (!selectedIndexUid) return;
    setWorkingIndexes((prev) => prev.map((item) => {
      if (item.__uid !== selectedIndexUid) return item;
      return { ...item, __status: "deleted" as const };
    }).filter((item) => !(item.__status === "deleted" && item.__uid.startsWith("new_idx_"))));
    setSelectedIndexUid(null);
    setEditingIndexCell((prev) => (prev?.uid === selectedIndexUid ? null : prev));
    setIndexEditValue("");
  }, [selectedIndexUid]);

  const inputCls = cn(
    "w-full h-full border-2 border-[var(--accent)] outline-none text-xs px-1 rounded-sm",
    "bg-[var(--surface)] text-[var(--fg)] font-medium",
    "absolute inset-0 z-20"
  );

  // 当前驱动对应的数据类型列表
  const allDataTypes = useMemo(() => getDataTypes(driver), [driver]);

  // 过滤后的类型列表
  const filteredTypes = useMemo(() => {
    if (!typeFilter.trim()) return allDataTypes;
    const lower = typeFilter.toLowerCase();
    return allDataTypes.filter((t) => t.toLowerCase().includes(lower));
  }, [typeFilter, allDataTypes]);

  // 可见行（排除已删除的新增行，已删除的原有行仍显示但标记删除线）
  const visibleCols = useMemo(() => workingCols.filter((c) => !(c.__status === "deleted" && c.__uid.startsWith("new_"))), [workingCols]);
  const visibleIndexes = useMemo(() => workingIndexes.filter((idx) => idx.__status !== "deleted"), [workingIndexes]);

  return (
    <div ref={containerRef} className="flex-1 flex flex-col overflow-hidden">
      {/* ===== 上栏：Columns ===== */}
      <div className="flex items-center h-6 px-2 gap-1 border-b border-[var(--border-color)] bg-[var(--surface-secondary)] flex-shrink-0">
        <span className="text-[length:var(--size-font-xs)] font-medium text-[var(--fg-secondary)]">
          {t("structure.columns")} ({visibleCols.filter((c) => c.__status !== "deleted").length})
        </span>
        <div className="flex-1" />
        <TipBtn
          tip={t("structure.addColumn")}
          className="h-4 w-4 flex items-center justify-center rounded-[var(--radius-btn)] text-[var(--fg-secondary)] hover:bg-[var(--sidebar-hover)] transition-colors"
          onClick={handleAddColumn}
        >
          <Plus className="h-2.5 w-2.5" />
        </TipBtn>
        <TipBtn
          tip={t("structure.deleteSelectedColumn")}
          shortcut="⌫"
          className={cn(
            "h-4 w-4 flex items-center justify-center rounded-[var(--radius-btn)] transition-colors",
            selectedUid ? "text-[var(--fg-secondary)] hover:bg-[var(--danger)]/10 hover:text-[var(--danger)]" : "text-[var(--fg-muted)] opacity-40 cursor-not-allowed"
          )}
          onClick={handleDeleteSelected}
          disabled={!selectedUid}
        >
          <Trash2 className="h-2.5 w-2.5" />
        </TipBtn>
        {hasEdits && (
          <TipBtn
            tip={t("structure.revertAll")}
            className="h-4 w-4 flex items-center justify-center rounded-[var(--radius-btn)] text-[var(--fg-secondary)] hover:bg-[var(--sidebar-hover)] transition-colors"
            onClick={handleRevertAll}
          >
            <Undo2 className="h-2.5 w-2.5" />
          </TipBtn>
        )}
      </div>

      <div className="overflow-auto flex-shrink-0" style={{ height: topHeight > 0 ? topHeight : "60%" }}>
        <table className="w-full border-collapse" style={{ minWidth: "max-content", tableLayout: "fixed" }}>
          <colgroup>
            <col style={{ width: 32 }} />
            {STRUCTURE_COL_DEFS.map((def) => (
              <col key={def.key} style={{ width: def.minWidth }} />
            ))}
          </colgroup>
          <thead className="sticky top-0 z-10">
            <tr>
              <th className="data-grid-header border-b border-[var(--border-color)] shadow-[-1px_0_0_0_var(--border-color)_inset]">#</th>
              {STRUCTURE_COL_DEFS.map((def) => (
                <th
                  key={def.key}
                  className="data-grid-header border-b border-[var(--border-color)] shadow-[-1px_0_0_0_var(--border-color)_inset]"
                >
                  {def.label}
                </th>
              ))}
            </tr>
          </thead>
          <tbody>
            {visibleCols.map((col, idx) => {
              const isDeleted = col.__status === "deleted";
              const isNew = col.__status === "new";
              const isModified = col.__status === "modified";
              const isSelected = selectedUid === col.__uid;
              return (
                <tr
                  key={col.__uid}
                  className={cn(
                    "group transition-colors cursor-default",
                    isSelected
                      ? "bg-[var(--row-selected)] ring-1 ring-inset ring-[var(--accent)]"
                      : idx % 2 === 0 ? "bg-[var(--surface)]" : "bg-[var(--row-stripe)]",
                    !isSelected && !isDeleted && "hover:bg-[var(--row-hover)]",
                    isDeleted && "opacity-40",
                    isNew && "bg-[var(--success)]/5",
                  )}
                  onClick={() => setSelectedUid(col.__uid)}
                >
                  <td className="data-grid-cell text-center text-[var(--fg-muted)] border-r border-[var(--border-subtle)]">
                    <div className="flex items-center justify-center gap-0.5">
                      {col.isPrimary && <Key className="h-2.5 w-2.5 text-[var(--warning)]" />}
                      {!col.isPrimary && <span>{idx + 1}</span>}
                    </div>
                  </td>
                  {STRUCTURE_COL_DEFS.map((def) => {
                    const cellValue = col[def.key as keyof EditingStructureCol];
                    const isEditing = editingCell?.uid === col.__uid && editingCell?.key === def.key;
                    const isCheckbox = !!def.isCheckbox;
                    const isTypeSelect = !!def.isTypeSelect;
                    const isEditableCell = def.editable && !isDeleted;

                    // 判断该单元格是否被修改过
                    const orig = originalCols.find((o) => o.__uid === col.__uid);
                    const cellModified = orig && def.editable &&
                      String(col[def.key as keyof EditingStructureCol] ?? "") !== String(orig[def.key as keyof EditingStructureCol] ?? "");

                    return (
                      <td
                        key={def.key}
                        className={cn(
                          "data-grid-cell overflow-hidden relative",
                          isEditableCell && "cursor-text",
                          isDeleted && "line-through",
                          cellModified && !isNew && "border-l-2 border-l-[var(--warning)] bg-[var(--cell-edit-bg)]/30",
                          isNew && "bg-[var(--success)]/8",
                        )}
                        onClick={(e) => {
                          // data_type 列和 checkbox 列单击即编辑
                          if (isEditableCell && (isTypeSelect || isCheckbox)) {
                            e.stopPropagation();
                            handleCellDoubleClick(col.__uid, def.key, cellValue);
                          }
                        }}
                        onDoubleClick={() => isEditableCell && !isTypeSelect && !isCheckbox && handleCellDoubleClick(col.__uid, def.key, cellValue)}
                      >
                        {isEditing && isTypeSelect ? (
                          <>
                            <div className="absolute inset-[1px] z-10">
                              <div className="relative flex h-full items-center">
                                <input
                                  ref={typeInputRef as React.RefObject<HTMLInputElement>}
                                  className={cn(
                                    "w-full h-full border border-[var(--accent)] outline-none text-[length:var(--size-font-xs)] px-1.5 rounded-[var(--radius-sm)] box-border",
                                    "bg-[var(--surface)] text-[var(--fg)] font-medium pr-5"
                                  )}
                                  value={editValue}
                                  onChange={(e) => {
                                    const val = e.target.value;
                                    setEditValue(val);
                                    // 用户主动输入时才过滤
                                    setTypeFilter(val);
                                    setTypeDropdownOpen(true);
                                    setTypeHighlightIdx(-1);
                                    requestAnimationFrame(() => updateDropdownPos());
                                  }}
                                  onFocus={() => {
                                    // 聚焦时显示全部类型
                                    setTypeFilter("");
                                    setTypeDropdownOpen(true);
                                    setTypeHighlightIdx(-1);
                                    requestAnimationFrame(() => updateDropdownPos());
                                  }}
                                  onKeyDown={(e) => {
                                    if (e.key === "ArrowDown") {
                                      e.preventDefault();
                                      setTypeDropdownOpen(true);
                                      setTypeHighlightIdx((prev) => Math.min(prev + 1, filteredTypes.length - 1));
                                    } else if (e.key === "ArrowUp") {
                                      e.preventDefault();
                                      setTypeHighlightIdx((prev) => Math.max(prev - 1, 0));
                                    } else if (e.key === "Enter") {
                                      e.preventDefault();
                                      if (typeHighlightIdx >= 0 && typeHighlightIdx < filteredTypes.length) {
                                        setEditValue(filteredTypes[typeHighlightIdx]);
                                      }
                                      commitCellEdit();
                                    } else if (e.key === "Escape") {
                                      cancelCellEdit();
                                    } else if (e.key === "Tab") {
                                      e.preventDefault();
                                      if (typeHighlightIdx >= 0 && typeHighlightIdx < filteredTypes.length) {
                                        setEditValue(filteredTypes[typeHighlightIdx]);
                                      }
                                      commitCellEdit();
                                    }
                                  }}
                                  onClick={(e) => e.stopPropagation()}
                                />
                                {/* 下拉箭头按钮 */}
                                <button
                                  className="absolute right-0 top-0 bottom-0 w-5 flex items-center justify-center text-[var(--fg-muted)] hover:text-[var(--fg)] transition-colors"
                                  tabIndex={-1}
                                  onMouseDown={(e) => {
                                    e.preventDefault();
                                    e.stopPropagation();
                                    setTypeDropdownOpen((prev) => {
                                      if (!prev) requestAnimationFrame(() => updateDropdownPos());
                                      return !prev;
                                    });
                                  }}
                                >
                                  <ChevronDown className="h-3 w-3" />
                                </button>
                              </div>
                            </div>
                            {/* 下拉列表通过 portal 渲染到 body，避免被 overflow-hidden 裁切 */}
                            {typeDropdownOpen && createPortal(
                              <div
                                ref={typeDropdownRef}
                                className="fixed z-[9999] max-h-[240px] overflow-auto rounded-[var(--radius-menu)] border border-[var(--border-color)] bg-[var(--surface)] shadow-lg"
                                style={{ top: typeDropdownPos.top, left: typeDropdownPos.left, width: typeDropdownPos.width }}
                              >
                                {filteredTypes.length === 0 ? (
                                  <div className="px-2 py-2 text-xs text-[var(--fg-muted)] text-center">
                                    {t("structure.noMatchingTypes")}
                                  </div>
                                ) : (
                                  filteredTypes.map((t, tIdx) => (
                                    <div
                                      key={t}
                                      className={cn(
                                        "px-2 py-[5px] text-xs cursor-pointer transition-colors",
                                        tIdx === typeHighlightIdx
                                          ? "bg-[var(--accent)] text-white"
                                          : t.toLowerCase() === editValue.toLowerCase()
                                            ? "bg-[var(--accent)]/15 text-[var(--accent)]"
                                            : "hover:bg-[var(--row-hover)] text-[var(--fg)]"
                                      )}
                                      onMouseDown={(e) => {
                                        e.preventDefault();
                                        setEditValue(t);
                                        setWorkingCols((prev) => prev.map((c) => {
                                          if (c.__uid !== col.__uid) return c;
                                          const updated = { ...c, type: t };
                                          if (c.__status !== "new") {
                                            const origC = originalCols.find((o) => o.__uid === col.__uid);
                                            if (origC) {
                                              const changed = updated.name !== origC.name || updated.type !== origC.type ||
                                                updated.nullable !== origC.nullable || (updated.defaultValue ?? "") !== (origC.defaultValue ?? "") ||
                                                updated.comment !== origC.comment;
                                              updated.__status = changed ? "modified" : undefined;
                                            }
                                          }
                                          return updated;
                                        }));
                                        setEditingCell(null);
                                        setTypeDropdownOpen(false);
                                      }}
                                      onMouseEnter={() => setTypeHighlightIdx(tIdx)}
                                    >
                                      {t}
                                    </div>
                                  ))
                                )}
                              </div>,
                              document.body
                            )}
                          </>
                        ) : isEditing && isCheckbox ? null : isEditing ? (
                          <input
                            ref={editInputRef as React.RefObject<HTMLInputElement>}
                            className={cn(
                              "w-full h-full border-2 border-[var(--accent)] outline-none text-xs px-1 rounded-sm",
                              "bg-[var(--surface)] text-[var(--fg)] font-medium",
                              "absolute inset-0 z-20"
                            )}
                            value={editValue}
                            onChange={(e) => setEditValue(e.target.value)}
                            onBlur={commitCellEdit}
                            onKeyDown={(e) => {
                              if (e.key === "Enter") { e.preventDefault(); commitCellEdit(); }
                              if (e.key === "Escape") cancelCellEdit();
                              if (e.key === "Tab") { e.preventDefault(); commitCellEdit(); }
                            }}
                            onClick={(e) => e.stopPropagation()}
                          />
                        ) : isCheckbox ? (
                          <div className="flex items-center justify-center h-full">
                            <span
                              className={cn("cursor-pointer select-none", isEditableCell ? "" : "opacity-50 cursor-not-allowed")}
                              onClick={(e) => {
                                e.stopPropagation();
                                if (isEditableCell) handleCellDoubleClick(col.__uid, def.key, cellValue);
                              }}
                            >
                              {cellValue ? "YES" : "NO"}
                            </span>
                          </div>
                        ) : isTypeSelect ? (
                          <div className="flex items-center h-full group/type">
                            <span className="truncate flex-1">
                              {cellValue === null || cellValue === undefined || cellValue === "" ? (
                                <span className="text-[var(--fg-muted)] italic opacity-50">{t("query.empty")}</span>
                              ) : String(cellValue)}
                            </span>
                            {isEditableCell && (
                              <ChevronDown className="h-3 w-3 text-[var(--fg-muted)] opacity-0 group-hover/type:opacity-100 transition-opacity flex-shrink-0 ml-0.5" />
                            )}
                          </div>
                        ) : (
                          <span className={cn("truncate block", def.key === "name" && "font-medium")}>
                            {cellValue === null || cellValue === undefined || cellValue === "" ? (
                                <span className="text-[var(--fg-muted)] italic opacity-50">{t("query.empty")}</span>
                            ) : String(cellValue)}
                          </span>
                        )}
                      </td>
                    );
                  })}
                </tr>
              );
            })}
            {visibleCols.length === 0 && (
              <tr>
                <td colSpan={STRUCTURE_COL_DEFS.length + 1} className="data-grid-cell text-center text-[var(--fg-muted)] py-4">
                  {t("structure.noColumnsFound")}
                </td>
              </tr>
            )}
          </tbody>
        </table>
      </div>

      {/* 可拖拽分割条 */}
      <div
        className="h-[3px] flex-shrink-0 cursor-row-resize group relative bg-[var(--surface-secondary)]"
        onMouseDown={handleSplitDragStart}
      >
        <div className="absolute inset-x-0 top-1/2 -translate-y-1/2 h-px bg-[var(--border-color)] transition-colors group-hover:bg-[var(--accent)]/50" />
        <div className="absolute left-1/2 top-1/2 -translate-x-1/2 -translate-y-1/2 w-5 h-1 rounded-full opacity-0 group-hover:opacity-100 bg-[var(--accent)]/40 transition-opacity" />
      </div>

      {/* ===== 下栏：Indexes ===== */}
      <div className="flex items-center h-6 px-2 gap-1 border-b border-[var(--border-color)] bg-[var(--surface-secondary)] flex-shrink-0">
        <span className="text-[length:var(--size-font-xs)] font-medium text-[var(--fg-secondary)]">
          Indexes ({visibleIndexes.length})
        </span>
        <div className="flex-1" />
        <TipBtn
          tip={t("structure.addIndex")}
          className="h-4 w-4 flex items-center justify-center rounded-[var(--radius-btn)] text-[var(--fg-secondary)] hover:bg-[var(--sidebar-hover)] transition-colors"
          onClick={handleAddIndex}
        >
          <Plus className="h-2.5 w-2.5" />
        </TipBtn>
        <TipBtn
          tip={t("structure.deleteSelectedColumn")}
          shortcut="⌫"
          className={cn(
            "h-4 w-4 flex items-center justify-center rounded-[var(--radius-btn)] transition-colors",
            selectedIndexUid
              ? "text-[var(--fg-secondary)] hover:bg-[var(--danger)]/10 hover:text-[var(--danger)]"
              : "text-[var(--fg-muted)] opacity-40 cursor-not-allowed"
          )}
          onClick={() => void handleDeleteSelectedIndex()}
          disabled={!selectedIndexUid}
        >
          <Trash2 className="h-2.5 w-2.5" />
        </TipBtn>
      </div>

      <div className="flex-1 overflow-auto">
        <table className="w-full border-collapse" style={{ minWidth: "max-content", tableLayout: "fixed" }}>
          <colgroup>
            {INDEX_COL_DEFS.map((def) => (
              <col key={def.key} style={{ width: def.minWidth }} />
            ))}
            <col style={{ width: 88 }} />
          </colgroup>
          <thead className="sticky top-0 z-10">
            <tr>
              {INDEX_COL_DEFS.map((def) => (
                <th key={def.key} className="data-grid-header border-b border-[var(--border-color)] shadow-[-1px_0_0_0_var(--border-color)_inset]">
                  {def.label}
                </th>
              ))}
              <th className="data-grid-header border-b border-[var(--border-color)] shadow-[-1px_0_0_0_var(--border-color)_inset] w-[88px]"></th>
            </tr>
          </thead>
          <tbody>
            {visibleIndexes.map((idx, i) => {
              const isNew = idx.__status === "new";
              const isEditingName = editingIndexCell?.uid === idx.__uid && editingIndexCell.key === "name";
              const isEditingColumns = editingIndexCell?.uid === idx.__uid && editingIndexCell.key === "columns";
              const columnsText = (idx.columns || []).join(", ");
              return (
                <tr
                  key={idx.__uid}
                  className={cn(
                    "group transition-colors cursor-default",
                    selectedIndexUid === idx.__uid
                      ? "bg-[var(--row-selected)] ring-1 ring-inset ring-[var(--accent)]"
                      : isNew
                        ? "bg-[var(--success)]/6"
                        : i % 2 === 0 ? "bg-[var(--surface)]" : "bg-[var(--row-stripe)]",
                    selectedIndexUid !== idx.__uid && !isNew && "hover:bg-[var(--row-hover)]"
                  )}
                  onClick={() => setSelectedIndexUid(idx.__uid)}
                >
                  <td className="data-grid-cell font-medium relative" title={idx.name || t("structure.indexNamePlaceholder")}>
                    {isNew && isEditingName ? (
                      <input
                        ref={indexInputRef}
                        className={cn(inputCls, "w-full")}
                        value={indexEditValue}
                        onChange={(e) => setIndexEditValue(e.target.value)}
                        onBlur={commitEditIndexCell}
                        onKeyDown={(e) => {
                          if (e.key === "Enter") {
                            e.preventDefault();
                            commitEditIndexCell();
                          } else if (e.key === "Escape") {
                            e.preventDefault();
                            cancelEditIndexCell();
                          }
                        }}
                        placeholder={t("structure.indexNamePlaceholder")}
                      />
                    ) : (
                      <div
                        className={cn("flex items-center gap-1", isNew && "cursor-text")}
                        onDoubleClick={() => isNew && startEditIndexCell(idx.__uid, "name", idx.name)}
                      >
                        {idx.isPrimary && <Key className="h-2.5 w-2.5 text-[var(--warning)] flex-shrink-0" />}
                        {!idx.isPrimary && idx.isUnique && <Hash className="h-2.5 w-2.5 text-[var(--accent)] flex-shrink-0" />}
                        <span className={cn("truncate", !idx.name && "text-[var(--fg-muted)]")}>{idx.name || t("structure.indexNamePlaceholder")}</span>
                      </div>
                    )}
                  </td>
                  <td className="data-grid-cell text-[var(--fg-muted)]">{idx.type || "BTREE"}</td>
                  <td className="data-grid-cell text-center">
                    {isNew ? (
                      <label className="inline-flex items-center justify-center cursor-pointer">
                        <input
                          type="checkbox"
                          checked={idx.isUnique}
                          onChange={(e) => handleToggleInlineIndexUnique(idx.__uid, e.target.checked)}
                        />
                      </label>
                    ) : (
                      idx.isUnique ? "TRUE" : "FALSE"
                    )}
                  </td>
                  <td className="data-grid-cell text-[var(--fg-secondary)] relative" title={columnsText || t("structure.indexColumnsPlaceholder")}>
                    {isNew && isEditingColumns ? (
                      <input
                        ref={indexInputRef}
                        className={cn(inputCls, "w-full")}
                        value={indexEditValue}
                        onChange={(e) => setIndexEditValue(e.target.value)}
                        onBlur={commitEditIndexCell}
                        onKeyDown={(e) => {
                          if (e.key === "Enter") {
                            e.preventDefault();
                            commitEditIndexCell();
                          } else if (e.key === "Escape") {
                            e.preventDefault();
                            cancelEditIndexCell();
                          }
                        }}
                        placeholder={t("structure.indexColumnsPlaceholder")}
                      />
                    ) : (
                      <div
                        className={cn("truncate", isNew && "cursor-text", !columnsText && "text-[var(--fg-muted)]")}
                        onDoubleClick={() => isNew && startEditIndexCell(idx.__uid, "columns", columnsText)}
                      >
                        {columnsText || t("structure.indexColumnsPlaceholder")}
                      </div>
                    )}
                  </td>
                  <td className="data-grid-cell text-center">
                    {isNew ? (
                      <div className="flex items-center justify-center gap-1">
                        <TipBtn
                          tip={t("common.create")}
                          className="h-4 w-4 flex items-center justify-center rounded-[var(--radius-sm)] hover:bg-[var(--success)]/10 text-[var(--fg-secondary)] hover:text-[var(--success)] transition-colors"
                          onClick={() => handleCreateInlineIndex(idx.__uid)}
                        >
                          <Check className="h-2.5 w-2.5" />
                        </TipBtn>
                        <TipBtn
                          tip={t("common.cancel")}
                          className="h-4 w-4 flex items-center justify-center rounded-[var(--radius-sm)] hover:bg-[var(--danger)]/10 text-[var(--fg-secondary)] hover:text-[var(--danger)] transition-colors"
                          onClick={() => handleCancelInlineIndex(idx.__uid)}
                        >
                          <X className="h-2.5 w-2.5" />
                        </TipBtn>
                      </div>
                    ) : null}
                  </td>
                </tr>
              );
            })}
            {workingIndexes.length === 0 && (
              <tr>
                <td colSpan={INDEX_COL_DEFS.length + 1} className="data-grid-cell text-center text-[var(--fg-muted)] py-4">
                  {t("structure.noIndexesFound")}
                </td>
              </tr>
            )}
          </tbody>
        </table>
      </div>

    </div>
  );
}

// =========== 查询视图 ===========
function QueryView({ tab }: { tab: Tab }) {
  const { updateTab } = useTabsStore();
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
  const { pageSize } = useUIStore();
  const [resultPage, setResultPage] = useState(1);
  // SQL 编辑区可拖拽高度
  const [editorHeight, setEditorHeight] = useState(250);
  const resizingEditor = useRef(false);
  const [selectedRowIndex, setSelectedRowIndex] = useState<number | null>(null);
  const [contextMenu, setContextMenu] = useState<ContextMenuPosition | null>(null);
  const [clickedColumn, setClickedColumn] = useState<string | null>(null);
  const [contextRowIndex, setContextRowIndex] = useState<number | null>(null);
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

  // SQL 变化时同步到 store
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

  // 执行单条 SQL（带分页）
  const handleExecute = async (sql: string, page = 1) => {
    if (!tab.connectionId || !tab.database) return;
    setLoading(true);
    try {
      const result = await QueryService.ExecuteSQLPaged(tab.connectionId, tab.database, sql, page, pageSize);
      setResultTabs([{
        columns: result.columns || [], rows: result.rows || [],
        total: result.total || 0, duration: result.duration || 0,
        error: result.error || undefined, sql,
        autoLimited: (result as any).autoLimited || false,
      }]);
      setActiveResultIdx(0);
      setResultPage(page);
      setSelectedRowIndex(null);
    } catch (e: any) {
      setResultTabs([{
        columns: [], rows: [], total: 0, duration: 0,
        error: e?.message || t("query.executionFailed"), sql,
      }]);
      setActiveResultIdx(0);
    } finally {
      setLoading(false);
    }
  };

  const handleExecuteAll = async (sql: string) => {
    if (!tab.connectionId || !tab.database) return;
    const statements = sql.split(";").map((s) => s.trim()).filter(Boolean);
    if (statements.length === 0) return;
    setLoading(true);
    const results: QueryResultItem[] = [];
    for (const stmt of statements) {
      try {
        const result = await QueryService.ExecuteSQLPaged(tab.connectionId, tab.database, stmt, 1, pageSize);
        results.push({
          columns: result.columns || [], rows: result.rows || [],
          total: result.total || 0, duration: result.duration || 0,
          error: result.error || undefined, sql: stmt,
          autoLimited: (result as any).autoLimited || false,
        });
      } catch (e: any) {
        results.push({ columns: [], rows: [], total: 0, duration: 0, error: e?.message || t("common.error"), sql: stmt });
      }
    }
    setResultTabs(results);
    setActiveResultIdx(0);
    setResultPage(1);
    setSelectedRowIndex(null);
    setLoading(false);
  };

  const activeResult = resultTabs[activeResultIdx];

  // 翻页时重新查询后端（对 autoLimited 的结果）
  const handleResultPageChange = useCallback(async (newPage: number) => {
    if (!activeResult?.autoLimited || !activeResult.sql || !tab.connectionId || !tab.database) {
      setResultPage(newPage);
      return;
    }
    setLoading(true);
    try {
      const result = await QueryService.ExecuteSQLPaged(tab.connectionId, tab.database, activeResult.sql, newPage, pageSize);
      setResultTabs([{
        columns: result.columns || [], rows: result.rows || [],
        total: result.total || 0, duration: result.duration || 0,
        error: result.error || undefined, sql: activeResult.sql,
        autoLimited: (result as any).autoLimited || false,
      }]);
      setResultPage(newPage);
      setSelectedRowIndex(null);
    } catch (e: any) {
      useUIStore.getState().addToast("error", `${t("query.pageFailed")}: ${e?.message || e}`);
    } finally {
      setLoading(false);
    }
  }, [activeResult, tab.connectionId, tab.database, pageSize, setResultTabs]);

  const handleAnalyzeResultError = useCallback(async () => {
    if (!activeResult?.error || !activeResult.sql || !tab.connectionId || !tab.database) return;
    setAIFixError("");
    setAIFixing(true);
    try {
      const versionText = queryServerVersion
        ? (locale === "en-US" ? `\nCurrent database version: ${queryServerVersion}` : `\n当前数据库版本: ${queryServerVersion}`)
        : (locale === "en-US" ? "\nCurrent database version: unknown" : "\n当前数据库版本: 未知");

      const prompt = locale === "en-US"
        ? `You are a senior SQL error analyzer and fixer.
Current SQL dialect: ${queryDialect}${versionText}
Please analyze the failing SQL and the error, then return STRICT JSON only:
{
  "sql": "fixed SQL",
  "analysis": "brief root cause and fix strategy"
}
SQL:
\`\`\`sql
${activeResult.sql}
\`\`\`
Error:
${activeResult.error}`
        : `你是一个资深 SQL 报错分析与修复助手。
当前数据库方言: ${queryDialect}${versionText}
请根据失败 SQL 与报错信息进行分析，并严格只返回 JSON：
{
  "sql": "修复后的 SQL",
  "analysis": "根因和修复思路（简短）"
}
SQL:
\`\`\`sql
${activeResult.sql}
\`\`\`
错误信息:
${activeResult.error}`;

      const resp = await AIService.ChatAI(tab.connectionId, tab.database, [{ role: "user", content: prompt }] as any);
      const parsed = extractJSONFromText(String(resp?.content || ""));
      const fixedSQL = String(parsed?.sql || "").trim();
      if (!fixedSQL) {
        throw new Error(locale === "en-US" ? "AI did not return valid fixed SQL." : "AI 未返回有效修复 SQL。");
      }
      // 将修复 SQL 回填到编辑器并自动重试执行，减少手工操作
      handleSQLChange(fixedSQL);
      await handleExecute(fixedSQL);
    } catch (e: any) {
      setAIFixError(e?.message || (locale === "en-US" ? "AI fix failed." : "AI 修复失败。"));
    } finally {
      setAIFixing(false);
    }
  }, [activeResult, tab.connectionId, tab.database, queryServerVersion, locale, queryDialect, handleSQLChange, handleExecute]);

  // autoLimited 模式下 total 是后端返回的总行数，翻页请求后端；否则前端分页
  const isServerPaged = !!activeResult?.autoLimited;
  const totalPages = activeResult
    ? (isServerPaged
      ? Math.max(1, Math.ceil(activeResult.total / pageSize))
      : Math.max(1, Math.ceil(activeResult.rows.length / pageSize)))
    : 1;
  const pagedRows = activeResult
    ? (isServerPaged
      ? activeResult.rows // 后端已经返回了当前页的数据
      : activeResult.rows.slice((resultPage - 1) * pageSize, resultPage * pageSize))
    : [];
  const selectedRow = selectedRowIndex !== null ? pagedRows[selectedRowIndex] : null;
  const contextRow = contextRowIndex !== null ? pagedRows[contextRowIndex] : null;

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

  const handleContextMenu = useCallback(
    (e: React.MouseEvent, rowIndex: number, columnName?: string) => {
      e.preventDefault();
      setContextRowIndex(rowIndex);
      setClickedColumn(columnName || null);
      setContextMenu({ x: e.clientX, y: e.clientY });
    },
    []
  );

  // 流式导出查询结果（弹窗选路径 → 后台分批写入 → 进度条）
  const handleExportQueryResult = useCallback(async (format: "csv" | "json" | "sql" = "csv") => {
    if (!activeResult?.sql || !tab.connectionId || !tab.database) return;
    try {
      const taskId = await ExportService.ExportSQLResultStream(tab.connectionId, tab.database, activeResult.sql, format);
      if (!taskId) return;
      console.log(`[流式导出SQL结果] 任务已启动: taskId=${taskId}`);
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
      setEditorHeight(newH);
    };
    const onUp = () => {
      resizingEditor.current = false;
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
      {/* 可拖拽分割条 */}
      <div
        className="h-1 flex-shrink-0 cursor-row-resize group relative border-b border-[var(--border-color)] hover:bg-[var(--accent)]/20 transition-colors"
        onMouseDown={handleEditorResizeStart}
      >
        <div className="absolute inset-x-0 top-1/2 -translate-y-1/2 h-[3px] opacity-0 group-hover:opacity-100 bg-[var(--accent)]/30 transition-opacity rounded-full" />
      </div>
      {resultTabs.length > 1 && (
        <div className="flex items-center h-7 border-b border-[var(--border-color)] bg-[var(--surface-secondary)] overflow-x-auto">
          {resultTabs.map((r, idx) => (
            <button
              key={idx}
              className={cn(
                "px-3 h-full text-xs border-r border-[var(--border-subtle)] transition-colors whitespace-nowrap",
                idx === activeResultIdx ? "bg-[var(--surface)] text-[var(--fg)] font-medium" : "text-[var(--fg-secondary)] hover:bg-[var(--tab-hover-bg)]"
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
            <button
              className="h-6 px-2 rounded-[var(--radius-btn)] border border-[var(--danger)]/30 bg-white/70 dark:bg-red-900/20 text-[var(--danger)] hover:bg-white dark:hover:bg-red-900/30 transition-colors text-xs inline-flex items-center gap-1 disabled:opacity-60"
              onClick={handleAnalyzeResultError}
              disabled={aiFixing}
              type="button"
            >
              {aiFixing ? <Loader2 className="h-3 w-3 animate-spin" /> : <Sparkles className="h-3 w-3" />}
              <span>{locale === "en-US" ? "AI Analyze & Fix" : "AI 分析并修复"}</span>
            </button>
            {aiFixError ? <span className="text-2xs text-[var(--danger)]/90">{aiFixError}</span> : null}
          </div>
        </div>
      )}
      {/* 自动分页提示已移除 */}
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
                showRowNumbers
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
                onClose={() => setPreviewVisible(false)}
              />
            )}
          </>
        ) : activeResult && !activeResult.error && activeResult.total > 0 ? (
          <div className="flex items-center justify-center h-full text-sm text-[var(--success)]">
            操作成功，影响 {activeResult.total} 行 ({activeResult.duration}ms)
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
          <span className="text-2xs">
            {isServerPaged
              ? `${activeResult.rows.length} 行 / 共 ${activeResult.total.toLocaleString()} 行`
              : `${activeResult.rows.length} 行`}
          </span>
          <span className="mx-1">·</span>
          <span className="text-2xs">{activeResult.duration}ms</span>

          <div className="flex-1" />



          {totalPages > 1 && (
            <div className="flex items-center gap-px ml-1">
              <button
                className="h-4 w-4 flex items-center justify-center rounded-[var(--radius-btn)] hover:bg-[var(--sidebar-hover)] disabled:opacity-30"
                disabled={resultPage <= 1 || loading}
                onClick={() => {
                  const prev = resultPage - 1;
                  if (isServerPaged) handleResultPageChange(prev);
                  else { setResultPage(prev); setSelectedRowIndex(null); }
                }}
              >
                <ChevronLeft className="h-2.5 w-2.5" />
              </button>
              <span className="text-[var(--fg-secondary)] text-2xs min-w-[32px] text-center">{resultPage}/{totalPages}</span>
              <button
                className="h-4 w-4 flex items-center justify-center rounded-[var(--radius-btn)] hover:bg-[var(--sidebar-hover)] disabled:opacity-30"
                disabled={resultPage >= totalPages || loading}
                onClick={() => {
                  const next = resultPage + 1;
                  if (isServerPaged) handleResultPageChange(next);
                  else { setResultPage(next); setSelectedRowIndex(null); }
                }}
              >
                <ChevronRight className="h-2.5 w-2.5" />
              </button>
            </div>
          )}
        </div>
      )}

      {/* 右键菜单 */}
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
        onCopyAsInsert={() => {
          const targetRow = contextRow || selectedRow;
          if (targetRow && tab.table) {
            copyToClipboard(rowToInsertSQL(tab.table, targetRow));
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
          if (selectedRow) {
            setPreviewVisible(true);
          }
          setContextMenu(null);
        }}
        onDownloadPage={() => { handleExportQueryResult("csv"); setContextMenu(null); }}
      />
    </div>
  );
}

function DDLView({ tab }: { tab: Tab }) {
  const [ddl, setDDL] = useState("");
  useEffect(() => {
    if (tab.connectionId && tab.database && tab.table) {
      DatabaseService.GetDDL(tab.connectionId, tab.database, tab.table).then(setDDL).catch(() => setDDL("-- 获取 DDL 失败"));
    }
  }, [tab.connectionId, tab.database, tab.table]);
  return <DDLViewer ddl={ddl} tableName={tab.table || ""} />;
}

function DocView({ tab }: { tab: Tab }) {
  const [content, setContent] = useState("");
  useEffect(() => {
    if (tab.connectionId && tab.database && tab.table) {
      DocService.GetTableDoc(tab.connectionId, tab.database, tab.table).then((doc) => setContent(doc || "")).catch(() => {});
    }
  }, [tab.connectionId, tab.database, tab.table]);
  return (
    <MarkdownEditor content={content} tableName={tab.table || ""}
      onSave={async (md) => {
        if (tab.connectionId && tab.database && tab.table) {
          await DocService.SaveTableDoc(tab.connectionId, tab.database, tab.table, md);
          setContent(md);
        }
      }}
    />
  );
}
