import React, { useState, useCallback, useEffect, useRef } from "react";
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
import { useUIStore } from "@/stores/ui";
import { useTranslation } from "@/i18n";
import { cn, copyToClipboard, rowToInsertSQL } from "@/lib/utils";
import { Database, RefreshCw, Download, ChevronLeft, ChevronRight } from "lucide-react";
import type { ColumnMeta } from "@/types/database";
import * as QueryService from "../../../wailsjs/go/services/QueryService";
import * as DatabaseService from "../../../wailsjs/go/services/DatabaseService";
import * as DocService from "../../../wailsjs/go/services/DocService";
import * as ExportService from "../../../wailsjs/go/services/ExportService";

export function TabContent() {
  const { tabs, activeTabId } = useTabsStore();
  const activeTab = tabs.find((t) => t.id === activeTabId);

  if (!activeTab) {
    return <EmptyState />;
  }

  // 使用 display:none 隐藏非活跃Tab而非卸载，保留查询页面的状态
  return (
    <>
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
    </>
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
            <kbd className="px-1 py-0.5 rounded border border-[var(--border-color)] bg-[var(--surface-secondary)] text-2xs">⌘K</kbd>
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
  const { previewVisible, setPreviewVisible, pageSize } = useUIStore();
  const { addTab } = useTabsStore();

  const [structureColumns, setStructureColumns] = useState<any[]>([]);
  const [ddl, setDDL] = useState("");
  const [docContent, setDocContent] = useState("");

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
      const cols = await DatabaseService.GetColumns(tab.connectionId, tab.database, tab.table);
      setStructureColumns(cols || []);
    } catch {}
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

  // 事务提交所有编辑修改（⌘S）
  const commitChanges = useCallback(async () => {
    if (!tab.connectionId || !tab.database || !tab.table) return;
    if (Object.keys(editedCells).length === 0) return;

    const pkCols = structureColumns.filter((c: any) => c.isPrimary).map((c: any) => c.name);
    if (pkCols.length === 0) {
      alert("无法提交：未找到主键列");
      return;
    }

    // 按行分组收集变更
    const changesByRow: Record<number, Record<string, unknown>> = {};
    for (const [key, val] of Object.entries(editedCells)) {
      const [rowStr, col] = key.split(":");
      const rowIdx = parseInt(rowStr);
      if (!changesByRow[rowIdx]) changesByRow[rowIdx] = {};
      changesByRow[rowIdx][col] = val;
    }

    // 构建批量更新请求，使用原始数据的主键值（而非编辑后的值）
    const updates: { primaryKey: Record<string, unknown>; changes: Record<string, unknown> }[] = [];
    for (const [rowIdxStr, changes] of Object.entries(changesByRow)) {
      const rowIdx = parseInt(rowIdxStr);
      // 从原始快照中取 PK，避免用户修改了 PK 列后找不到原行
      const origRow = originalData[rowIdx];
      if (!origRow) continue;
      const pk: Record<string, unknown> = {};
      for (const col of pkCols) pk[col] = origRow[col];
      updates.push({ primaryKey: pk, changes });
    }

    try {
      console.log("[提交变更] 开始批量更新:", JSON.stringify(updates));
      await QueryService.BatchUpdateRows(tab.connectionId, tab.database, tab.table, updates as any);
      console.log("[提交变更] 批量更新成功，重新加载数据");
      setEditedCells({});
      // 重新加载数据，确保界面显示最新值
      await loadData(page, activeFilters, rawSqlFilter);
    } catch (e: any) {
      console.error("事务提交失败:", e);
      alert("事务提交失败: " + (e?.message || e));
    }
  }, [editedCells, originalData, structureColumns, tab, page, activeFilters, rawSqlFilter, loadData]);

  // 快捷键
  useEffect(() => {
    const handler = (e: KeyboardEvent) => {
      if ((e.metaKey || e.ctrlKey) && e.key === "f") {
        e.preventDefault();
        setShowFilter((v) => !v);
      }
      if ((e.metaKey || e.ctrlKey) && e.key === "s") {
        e.preventDefault();
        commitChanges();
      }
      if ((e.metaKey || e.ctrlKey) && e.key === "r") {
        e.preventDefault();
        setEditedCells({});
        loadData(page, activeFilters, rawSqlFilter);
      }
      if (e.code === "Space" && selectedRow) {
        const target = e.target as HTMLElement;
        if (target.tagName !== "INPUT" && target.tagName !== "TEXTAREA" && target.tagName !== "SELECT") {
          e.preventDefault();
          setPreviewVisible(!previewVisible);
        }
      }
    };
    window.addEventListener("keydown", handler);
    return () => window.removeEventListener("keydown", handler);
  }, [selectedRow, previewVisible, setPreviewVisible, commitChanges, loadData, page, activeFilters, rawSqlFilter]);

  // 使用后端 ExportService 导出 CSV（Wails WebView 中 blob URL 下载不可用）
  const handleDownloadPage = useCallback(async () => {
    if (data.length === 0 || columns.length === 0) return;
    const colNames = columns.map((c) => c.name);
    // 将 data 转为 Record<string, any>[] 格式
    const rows = data.map((row) => {
      const r: Record<string, any> = {};
      for (const col of colNames) {
        r[col] = row[col] ?? null;
      }
      return r;
    });
    try {
      const filePath = await ExportService.ExportCSV(tab.table || "data", colNames, rows);
      if (filePath) {
        console.log("[导出CSV] 成功:", filePath);
      }
    } catch (e: any) {
      console.error("[导出CSV] 失败:", e);
      alert("导出失败: " + (e?.message || e));
    }
  }, [data, columns, tab.table]);

  const handleCellEdit = useCallback((rowIdx: number, column: string, value: unknown) => {
    const key = `${rowIdx}:${column}`;
    setEditedCells((prev) => ({ ...prev, [key]: value }));
    setData((prev) => {
      const next = [...prev];
      next[rowIdx] = { ...next[rowIdx], [column]: value };
      return next;
    });
  }, []);

  const totalPages = Math.max(1, Math.ceil(totalRows / pageSize));
  const hasEdits = Object.keys(editedCells).length > 0;

  return (
    <div className="flex flex-col h-full">
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
              title: `查询 - ${tab.table}`,
              connectionId: tab.connectionId,
              database: tab.database,
              table: tab.table,
              closable: true,
              sql: `SELECT * FROM ${tab.table} LIMIT ${pageSize};`,
            })
          }
          onExport={handleDownloadPage}
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
      <div className="flex flex-1 overflow-hidden">
        {subView === "data" && (
          <>
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
            />
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
          <div className="flex-1 overflow-auto">
            <table className="w-full border-collapse">
              <thead className="sticky top-0 z-10">
                <tr>
                  <th className="data-grid-header border-r border-b border-[var(--border-color)]">#</th>
                  <th className="data-grid-header border-r border-b border-[var(--border-color)]">column_name</th>
                  <th className="data-grid-header border-r border-b border-[var(--border-color)]">data_type</th>
                  <th className="data-grid-header border-r border-b border-[var(--border-color)]">is_nullable</th>
                  <th className="data-grid-header border-r border-b border-[var(--border-color)]">column_default</th>
                  <th className="data-grid-header border-r border-b border-[var(--border-color)]">is_primary</th>
                  <th className="data-grid-header border-r border-b border-[var(--border-color)]">comment</th>
                </tr>
              </thead>
              <tbody>
                {structureColumns.map((col: any, idx: number) => (
                  <tr key={col.name} className={idx % 2 === 0 ? "bg-[var(--surface)]" : "bg-[var(--row-stripe)]"}>
                    <td className="data-grid-cell text-center text-[var(--fg-muted)] border-r border-[var(--border-subtle)]">{idx + 1}</td>
                    <td className="data-grid-cell font-medium">{col.name}</td>
                    <td className="data-grid-cell text-[var(--fg-secondary)]">{col.type}</td>
                    <td className="data-grid-cell">{col.nullable ? "YES" : "NO"}</td>
                    <td className="data-grid-cell text-[var(--fg-muted)]">{col.defaultValue ?? <span className="italic">NULL</span>}</td>
                    <td className="data-grid-cell">{col.isPrimary ? "✓" : ""}</td>
                    <td className="data-grid-cell text-[var(--fg-secondary)]">{col.comment || ""}</td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
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
        "h-[var(--size-btn)] flex items-center px-[var(--size-padding-sm)] border-t text-[length:var(--size-font-2xs)] select-none gap-[var(--size-gap-sm)]",
        "bg-[var(--surface-secondary)] border-[var(--border-color)]"
      )}>
        {/* 左侧：子视图切换 */}
        <div className="flex items-center gap-px flex-shrink-0">
          {(["data", "structure", "info", "doc"] as TableSubView[]).map((v) => (
            <button
              key={v}
              className={cn(
                "px-2 py-0.5 rounded-[var(--radius-btn)] text-[length:var(--size-font-2xs)] transition-colors whitespace-nowrap",
                subView === v
                  ? "bg-[var(--accent)] text-[var(--accent-fg)] font-medium"
                  : "text-[var(--fg-secondary)] hover:bg-[var(--sidebar-hover)]"
              )}
              onClick={() => setSubView(v)}
            >
              {v === "data" ? "Data" : v === "structure" ? "Structure" : v === "info" ? "DDL" : "Doc"}
            </button>
          ))}
        </div>

        <div className="flex-1 min-w-0" />

        {/* 右侧：操作按钮 */}
        <div className={cn("flex items-center gap-0.5 flex-shrink-0", subView !== "data" && "invisible")}>
          <button
            className={cn(
              "px-1.5 py-0.5 rounded-[var(--radius-btn)] text-[length:var(--size-font-2xs)] transition-colors",
              showFilter
                ? "text-[var(--accent)] bg-[var(--accent)]/10 font-medium"
                : "text-[var(--fg-secondary)] hover:bg-[var(--sidebar-hover)]"
            )}
            onClick={() => setShowFilter((v) => !v)}
            title="筛选 (⌘F)"
          >
            Filters
          </button>
          <button
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
            title="打开 SQL 查询"
          >
            SQL
          </button>
          <button className="h-[var(--size-btn-sm)] w-[var(--size-btn-sm)] flex items-center justify-center rounded-[var(--radius-btn)] hover:bg-[var(--sidebar-hover)] transition-colors" onClick={handleDownloadPage} title="导出 CSV">
            <Download className="h-2.5 w-2.5 text-[var(--fg-secondary)]" />
          </button>
          <button className="h-[var(--size-btn-sm)] w-[var(--size-btn-sm)] flex items-center justify-center rounded-[var(--radius-btn)] hover:bg-[var(--sidebar-hover)] transition-colors" onClick={() => { setEditedCells({}); loadData(page, activeFilters, rawSqlFilter); }} title="刷新">
            <RefreshCw className="h-2.5 w-2.5 text-[var(--fg-secondary)]" />
          </button>
          {hasEdits && (
            <button
              className="px-1.5 py-0.5 rounded-[var(--radius-btn)] text-[length:var(--size-font-2xs)] font-medium text-white bg-[var(--accent)] hover:opacity-90 transition-opacity"
              onClick={commitChanges}
              title={`${t("common.commit")} (⌘S)`}
            >
              {t("common.commit")} ({Object.keys(editedCells).length})
            </button>
          )}
        </div>

        <div className="w-px h-3 bg-[var(--border-color)] mx-0.5 flex-shrink-0" />

        <span className="text-[var(--fg-muted)] text-2xs flex-shrink-0">{totalRows.toLocaleString()} 行</span>
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
        onDeleteRow={async () => {
          if (!selectedRow || !tab.connectionId || !tab.database || !tab.table) {
            setContextMenu(null);
            return;
          }
          const pkCols = structureColumns.filter((c: any) => c.isPrimary).map((c: any) => c.name);
          if (pkCols.length === 0) { alert("无法删除：未找到主键列"); setContextMenu(null); return; }
          const pk: Record<string, unknown> = {};
          for (const col of pkCols) pk[col] = selectedRow[col];
          if (!confirm(`确定删除此行？`)) { setContextMenu(null); return; }
          try {
            await QueryService.DeleteRow(tab.connectionId, tab.database, tab.table, pk as any);
            loadData(page, activeFilters, rawSqlFilter);
          } catch (e: any) {
            alert("删除失败: " + e);
          }
          setContextMenu(null);
        }}
        onRefresh={() => { loadData(page, activeFilters, rawSqlFilter); setContextMenu(null); }}
        onPreview={() => { setPreviewVisible(true); setContextMenu(null); }}
        onDownloadPage={() => { handleDownloadPage(); setContextMenu(null); }}
      />
    </div>
  );
}

// =========== 查询视图 ===========
function QueryView({ tab }: { tab: Tab }) {
  const { updateTab } = useTabsStore();
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

  const handleExecute = async (sql: string) => {
    if (!tab.connectionId || !tab.database) return;
    setLoading(true);
    try {
      const result = await QueryService.ExecuteSQL(tab.connectionId, tab.database, sql);
      setResultTabs([{
        columns: result.columns || [], rows: result.rows || [],
        total: result.total || 0, duration: result.duration || 0,
        error: result.error || undefined, sql,
      }]);
      setActiveResultIdx(0);
      setResultPage(1);
      setSelectedRowIndex(null);
    } catch (e: any) {
      setResultTabs([{
        columns: [], rows: [], total: 0, duration: 0,
        error: e?.message || "查询执行失败", sql,
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
        const result = await QueryService.ExecuteSQL(tab.connectionId, tab.database, stmt);
        results.push({
          columns: result.columns || [], rows: result.rows || [],
          total: result.total || 0, duration: result.duration || 0,
          error: result.error || undefined, sql: stmt,
        });
      } catch (e: any) {
        results.push({ columns: [], rows: [], total: 0, duration: 0, error: e?.message || "执行失败", sql: stmt });
      }
    }
    setResultTabs(results);
    setActiveResultIdx(0);
    setResultPage(1);
    setSelectedRowIndex(null);
    setLoading(false);
  };

  const activeResult = resultTabs[activeResultIdx];

  const totalPages = activeResult ? Math.max(1, Math.ceil(activeResult.rows.length / pageSize)) : 1;
  const pagedRows = activeResult
    ? activeResult.rows.slice((resultPage - 1) * pageSize, resultPage * pageSize)
    : [];
  const selectedRow = selectedRowIndex !== null ? pagedRows[selectedRowIndex] : null;

  const handleContextMenu = useCallback(
    (e: React.MouseEvent, _rowIndex: number, columnName?: string) => {
      e.preventDefault();
      setClickedColumn(columnName || null);
      setContextMenu({ x: e.clientX, y: e.clientY });
    },
    []
  );

  const handleDownloadPage = useCallback(async () => {
    if (!activeResult || pagedRows.length === 0 || activeResult.columns.length === 0) return;
    const colNames = activeResult.columns.map((c) => c.name);
    const rows = pagedRows.map((row) => {
      const r: Record<string, any> = {};
      for (const col of colNames) {
        r[col] = row[col] ?? null;
      }
      return r;
    });
    try {
      const filePath = await ExportService.ExportCSV("query_result", colNames, rows);
      if (filePath) {
        console.log("[导出CSV] 成功:", filePath);
      }
    } catch (e: any) {
      console.error("[导出CSV] 失败:", e);
      alert("导出失败: " + (e?.message || e));
    }
  }, [activeResult, pagedRows]);

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
    <div className="flex flex-col h-full">
      <div style={{ height: editorHeight, minHeight: 80 }} className="flex-shrink-0">
        <SQLEditor initialSQL={tab.sql} onExecute={handleExecute} onExecuteAll={handleExecuteAll} onSQLChange={handleSQLChange} loading={loading} />
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
          {activeResult.error}
        </div>
      )}
      <div className="flex-1 overflow-auto">
        {activeResult && activeResult.columns.length > 0 ? (
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
        ) : activeResult && !activeResult.error && activeResult.total > 0 ? (
          <div className="flex items-center justify-center h-full text-sm text-[var(--success)]">
            操作成功，影响 {activeResult.total} 行 ({activeResult.duration}ms)
          </div>
        ) : !activeResult ? (
          <div className="flex items-center justify-center h-full text-sm text-[var(--fg-muted)]">
            <div className="text-center">
              <p>执行查询后在此显示结果</p>
              <p className="text-2xs mt-2">⌘↵ 执行当前语句 · ⌘⇧↵ 执行所有 · ⌘⇧F 格式化</p>
            </div>
          </div>
        ) : null}
      </div>
      {activeResult && (
        <div className="h-5 flex items-center px-2 text-2xs text-[var(--fg-muted)] border-t border-[var(--border-color)] bg-[var(--surface-secondary)]">
          <span className="text-2xs">{activeResult.rows.length} 行</span>
          <span className="mx-1">·</span>
          <span className="text-2xs">{activeResult.duration}ms</span>

          <div className="flex-1" />

          <button className="h-4 w-4 flex items-center justify-center rounded-[var(--radius-btn)] hover:bg-[var(--sidebar-hover)] transition-colors" onClick={handleDownloadPage} title="导出 CSV">
            <Download className="h-2.5 w-2.5" />
          </button>

          {activeResult.rows.length > pageSize && (
            <div className="flex items-center gap-px ml-1">
              <button className="h-4 w-4 flex items-center justify-center rounded-[var(--radius-btn)] hover:bg-[var(--sidebar-hover)] disabled:opacity-30" disabled={resultPage <= 1} onClick={() => { setResultPage(resultPage - 1); setSelectedRowIndex(null); }}>
                <ChevronLeft className="h-2.5 w-2.5" />
              </button>
              <span className="text-[var(--fg-secondary)] text-2xs min-w-[32px] text-center">{resultPage}/{totalPages}</span>
              <button className="h-4 w-4 flex items-center justify-center rounded-[var(--radius-btn)] hover:bg-[var(--sidebar-hover)] disabled:opacity-30" disabled={resultPage >= totalPages} onClick={() => { setResultPage(resultPage + 1); setSelectedRowIndex(null); }}>
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
        onDeleteRow={() => { setContextMenu(null); }}
        onRefresh={() => {
          if (activeResult) handleExecute(activeResult.sql);
          setContextMenu(null);
        }}
        onPreview={() => { setContextMenu(null); }}
        onDownloadPage={() => { handleDownloadPage(); setContextMenu(null); }}
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
