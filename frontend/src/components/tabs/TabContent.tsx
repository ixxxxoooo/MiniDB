import React, { useState, useCallback, useEffect, useRef } from "react";
import { createPortal } from "react-dom";
import { useTabsStore, type Tab } from "@/stores/tabs";
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
import { cn, copyToClipboard, rowToInsertSQL } from "@/lib/utils";
import { Database, RefreshCw, Download, ChevronLeft, ChevronRight } from "lucide-react";
import { Button } from "@/components/ui/button";
import type { ColumnMeta } from "@/types/database";
import * as QueryService from "../../../wailsjs/go/services/QueryService";
import * as DatabaseService from "../../../wailsjs/go/services/DatabaseService";
import * as DocService from "../../../wailsjs/go/services/DocService";

export function TabContent() {
  const { tabs, activeTabId } = useTabsStore();
  const activeTab = tabs.find((t) => t.id === activeTabId);

  if (!activeTab) {
    return <EmptyState />;
  }

  switch (activeTab.type) {
    case "table":
      return <TableView key={activeTab.id} tab={activeTab} />;
    case "query":
      return <QueryView key={activeTab.id} tab={activeTab} />;
    case "ddl":
      return <DDLView key={activeTab.id} tab={activeTab} />;
    case "doc":
      return <DocView key={activeTab.id} tab={activeTab} />;
    default:
      return <EmptyState />;
  }
}

function EmptyState() {
  return (
    <div className="flex-1 flex flex-col items-center justify-center text-[var(--fg-muted)]">
      <Database className="h-16 w-16 mb-4 opacity-20" />
      <p className="text-lg font-medium mb-1 text-[var(--fg-secondary)]">TablePlus AI</p>
      <p className="text-sm">选择左侧连接或表开始使用</p>
      <div className="mt-6 flex gap-4 text-xs text-[var(--fg-muted)]">
        <div className="flex items-center gap-1">
          <kbd className="px-1.5 py-0.5 rounded border border-[var(--border-color)] bg-[var(--surface-secondary)]">⌘K</kbd>
          <span>快速搜索</span>
        </div>
        <div className="flex items-center gap-1">
          <kbd className="px-1.5 py-0.5 rounded border border-[var(--border-color)] bg-[var(--surface-secondary)]">⌘N</kbd>
          <span>新建连接</span>
        </div>
        <div className="flex items-center gap-1">
          <kbd className="px-1.5 py-0.5 rounded border border-[var(--border-color)] bg-[var(--surface-secondary)]">⌘T</kbd>
          <span>新查询</span>
        </div>
      </div>
    </div>
  );
}

// =========== 表视图（参考 TablePlus：Data/Structure/Info 底部切换） ===========
type TableSubView = "data" | "structure" | "info";

function TableView({ tab }: { tab: Tab }) {
  const [subView, setSubView] = useState<TableSubView>("data");
  const [columns, setColumns] = useState<ColumnMeta[]>([]);
  const [data, setData] = useState<Record<string, unknown>[]>([]);
  const [totalRows, setTotalRows] = useState(0);
  const [queryDuration, setQueryDuration] = useState(0);
  const [selectedRowIndex, setSelectedRowIndex] = useState<number | null>(null);
  const [page, setPage] = useState(1);
  const [loading, setLoading] = useState(false);
  const [contextMenu, setContextMenu] = useState<ContextMenuPosition | null>(null);
  const [activeFilters, setActiveFilters] = useState<FilterCondition[]>([]);
  const [showFilter, setShowFilter] = useState(false);
  const [rawSqlFilter, setRawSqlFilter] = useState("");
  const [editedCells, setEditedCells] = useState<Record<string, unknown>>({});
  const { previewVisible, setPreviewVisible } = useUIStore();
  const { addTab } = useTabsStore();
  const pageSize = 100;

  // 表结构数据
  const [structureColumns, setStructureColumns] = useState<any[]>([]);
  const [ddl, setDDL] = useState("");

  const selectedRow = selectedRowIndex !== null ? data[selectedRowIndex] : null;

  const loadData = useCallback(async (p: number, filters: FilterCondition[] = [], rawSql = "") => {
    if (!tab.connectionId || !tab.database || !tab.table) return;
    setLoading(true);
    try {
      let result;
      if (rawSql.trim()) {
        // Raw SQL 筛选模式
        result = await QueryService.ExecuteSQL(tab.connectionId, tab.database, rawSql);
      } else {
        result = await QueryService.QueryTableData(
          tab.connectionId, tab.database, tab.table, p, pageSize, filters as any, []
        );
      }
      if (result) {
        setColumns(result.columns || []);
        setData(result.rows || []);
        setTotalRows(result.total || 0);
        setQueryDuration(result.duration || 0);
      }
    } catch (e) {
      console.error("加载表数据失败:", e);
    } finally {
      setLoading(false);
    }
  }, [tab.connectionId, tab.database, tab.table, pageSize]);

  // 加载表结构
  const loadStructure = useCallback(async () => {
    if (!tab.connectionId || !tab.database || !tab.table) return;
    try {
      const cols = await DatabaseService.GetColumns(tab.connectionId, tab.database, tab.table);
      setStructureColumns(cols || []);
    } catch {}
  }, [tab.connectionId, tab.database, tab.table]);

  // 加载 DDL
  const loadDDL = useCallback(async () => {
    if (!tab.connectionId || !tab.database || !tab.table) return;
    try {
      const d = await DatabaseService.GetDDL(tab.connectionId, tab.database, tab.table);
      setDDL(d || "");
    } catch {
      setDDL("-- 获取 DDL 失败");
    }
  }, [tab.connectionId, tab.database, tab.table]);

  useEffect(() => {
    loadData(page, activeFilters);
    loadStructure();
  }, [loadData, page, loadStructure]);

  useEffect(() => {
    if (subView === "structure") loadStructure();
    if (subView === "info") loadDDL();
  }, [subView, loadStructure, loadDDL]);

  const handleContextMenu = useCallback(
    (e: React.MouseEvent, _rowIndex: number) => {
      e.preventDefault();
      setContextMenu({ x: e.clientX, y: e.clientY });
    },
    []
  );

  // 提交所有编辑修改（⌘S）
  const commitChanges = useCallback(async () => {
    if (!tab.connectionId || !tab.database || !tab.table) return;
    if (Object.keys(editedCells).length === 0) return;

    // 按行分组收集变更
    const changesByRow: Record<number, Record<string, unknown>> = {};
    for (const [key, val] of Object.entries(editedCells)) {
      const [rowStr, col] = key.split(":");
      const rowIdx = parseInt(rowStr);
      if (!changesByRow[rowIdx]) changesByRow[rowIdx] = {};
      changesByRow[rowIdx][col] = val;
    }

    // 查找主键列
    const pkCols = structureColumns.filter((c: any) => c.isPrimary).map((c: any) => c.name);

    for (const [rowIdxStr, changes] of Object.entries(changesByRow)) {
      const rowIdx = parseInt(rowIdxStr);
      const row = data[rowIdx];
      if (!row) continue;

      if (pkCols.length > 0) {
        const pk: Record<string, unknown> = {};
        for (const col of pkCols) pk[col] = row[col];
        try {
          await QueryService.UpdateRow(tab.connectionId, tab.database, tab.table, pk as any, changes as any);
        } catch (e) {
          console.error("更新行失败:", e);
          alert("更新失败: " + e);
          return;
        }
      }
    }

    setEditedCells({});
    loadData(page, activeFilters, rawSqlFilter);
  }, [editedCells, data, structureColumns, tab, page, activeFilters, rawSqlFilter, loadData]);

  // ⌘F 开关筛选框 + 空格预览 + ⌘S 保存
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
  }, [selectedRow, previewVisible, setPreviewVisible, commitChanges]);

  // 下载当前页
  const handleDownloadPage = useCallback(() => {
    if (data.length === 0 || columns.length === 0) return;
    const colNames = columns.map((c) => c.name);
    const header = colNames.join(",");
    const rows = data.map((row) =>
      colNames.map((col) => {
        const v = row[col];
        if (v === null || v === undefined) return "";
        const str = String(v);
        if (str.includes(",") || str.includes('"') || str.includes("\n")) return `"${str.replace(/"/g, '""')}"`;
        return str;
      }).join(",")
    );
    const csv = [header, ...rows].join("\n");
    const blob = new Blob([csv], { type: "text/csv;charset=utf-8;" });
    const url = URL.createObjectURL(blob);
    const a = document.createElement("a");
    a.href = url;
    a.download = `${tab.table || "data"}_page${page}.csv`;
    a.click();
    URL.revokeObjectURL(url);
  }, [data, columns, tab.table, page]);

  // 双击编辑单元格
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

  return (
    <div className="flex flex-col h-full">
      {/* ====== 筛选栏（⌘F 切换，参考 TablePlus） ====== */}
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
              sql: `SELECT * FROM ${tab.table} LIMIT 100;`,
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

      {/* ====== 主内容区 ====== */}
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
      </div>

      {/* ====== 底部功能栏（参考 TablePlus） ====== */}
      <div className={cn(
        "h-8 flex items-center px-3 border-t text-xs select-none gap-1",
        "bg-[var(--surface-secondary)] border-[var(--border-color)]"
      )}>
        {/* 左侧：Data/Structure/Info 切换 */}
        <div className="flex items-center gap-0.5">
          {(["data", "structure", "info"] as TableSubView[]).map((v) => (
            <button
              key={v}
              className={cn(
                "px-2.5 py-1 rounded text-xs transition-colors",
                subView === v
                  ? "bg-[var(--accent)] text-[var(--accent-fg)] font-medium"
                  : "text-[var(--fg-secondary)] hover:bg-[var(--sidebar-hover)]"
              )}
              onClick={() => setSubView(v)}
            >
              {v === "data" ? "Data" : v === "structure" ? "Structure" : "Info"}
            </button>
          ))}
        </div>

        <div className="flex-1" />

        {/* 中间：筛选、SQL、导出、刷新 */}
        {subView === "data" && (
          <div className="flex items-center gap-1">
            <button
              className={cn(
                "px-2 py-0.5 rounded text-xs transition-colors border",
                showFilter
                  ? "border-[var(--accent)] text-[var(--accent)] bg-[var(--accent)]/10"
                  : "border-transparent text-[var(--fg-secondary)] hover:bg-[var(--sidebar-hover)]"
              )}
              onClick={() => setShowFilter((v) => !v)}
              title="筛选 (⌘F)"
            >
              ▽ 筛选
            </button>
            <button
              className="px-2 py-0.5 rounded text-xs text-[var(--fg-secondary)] hover:bg-[var(--sidebar-hover)] transition-colors font-mono font-bold"
              onClick={() =>
                addTab({
                  type: "query",
                  title: `SQL - ${tab.table}`,
                  connectionId: tab.connectionId,
                  database: tab.database,
                  table: tab.table,
                  closable: true,
                  sql: `SELECT * FROM ${tab.table} LIMIT 100;`,
                })
              }
              title="打开 SQL 查询"
            >
              SQL
            </button>
            <Button variant="ghost" size="icon" className="h-6 w-6" onClick={handleDownloadPage} title="导出 CSV">
              <Download className="h-3 w-3" />
            </Button>
            <Button variant="ghost" size="icon" className="h-6 w-6" onClick={() => { setEditedCells({}); loadData(page, activeFilters, rawSqlFilter); }} title="刷新">
              <RefreshCw className="h-3 w-3" />
            </Button>
            {Object.keys(editedCells).length > 0 && (
              <button
                className="px-2 py-0.5 rounded text-xs font-medium text-white bg-[var(--accent)] hover:opacity-90 transition-opacity ml-1"
                onClick={commitChanges}
                title="提交修改 (⌘S)"
              >
                保存 ({Object.keys(editedCells).length})
              </button>
            )}
          </div>
        )}

        <div className="w-px h-4 bg-[var(--border-color)] mx-1" />

        {/* 右侧：行数、耗时、分页 */}
        <span className="text-[var(--fg-muted)]">{totalRows.toLocaleString()} 行</span>
        <span className="text-[var(--fg-muted)]">·</span>
        <span className="text-[var(--fg-muted)]">{queryDuration}ms</span>

        {subView === "data" && (
          <div className="flex items-center gap-0.5 ml-2">
            <Button variant="ghost" size="icon" className="h-5 w-5" disabled={page <= 1} onClick={() => setPage(page - 1)}>
              <ChevronLeft className="h-3 w-3" />
            </Button>
            <span className="text-[var(--fg-secondary)] min-w-[40px] text-center">{page}/{totalPages}</span>
            <Button variant="ghost" size="icon" className="h-5 w-5" disabled={page >= totalPages} onClick={() => setPage(page + 1)}>
              <ChevronRight className="h-3 w-3" />
            </Button>
          </div>
        )}
      </div>

      {/* 右键菜单 */}
      <RowContextMenu
        position={contextMenu}
        onClose={() => setContextMenu(null)}
        onCopyCell={() => {
          if (selectedRow) {
            const values = Object.values(selectedRow);
            if (values.length > 0) copyToClipboard(String(values[0] ?? ""));
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
interface QueryResultItem {
  columns: ColumnMeta[];
  rows: Record<string, unknown>[];
  total: number;
  duration: number;
  error?: string;
  sql: string;
}

function QueryView({ tab }: { tab: Tab }) {
  const [resultTabs, setResultTabs] = useState<QueryResultItem[]>([]);
  const [activeResultIdx, setActiveResultIdx] = useState(0);
  const [loading, setLoading] = useState(false);

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
    setLoading(false);
  };

  const activeResult = resultTabs[activeResultIdx];

  return (
    <div className="flex flex-col h-full">
      <div className="h-[250px] min-h-[100px] border-b border-[var(--border-color)]">
        <SQLEditor initialSQL={tab.sql} onExecute={handleExecute} onExecuteAll={handleExecuteAll} loading={loading} />
      </div>
      {resultTabs.length > 1 && (
        <div className="flex items-center h-7 border-b border-[var(--border-color)] bg-[var(--surface-secondary)] overflow-x-auto">
          {resultTabs.map((r, idx) => (
            <button
              key={idx}
              className={cn(
                "px-3 h-full text-xs border-r border-[var(--border-subtle)] transition-colors",
                idx === activeResultIdx ? "bg-[var(--surface)] text-[var(--fg)] font-medium" : "text-[var(--fg-secondary)] hover:bg-[var(--tab-hover-bg)]"
              )}
              onClick={() => setActiveResultIdx(idx)}
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
      <div className="flex-1 overflow-hidden">
        {activeResult && activeResult.columns.length > 0 ? (
          <DataGrid columns={activeResult.columns} data={activeResult.rows} selectedRowIndex={null} onSelectRow={() => {}} />
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
        <div className="h-6 flex items-center px-3 text-2xs text-[var(--fg-muted)] border-t border-[var(--border-color)] bg-[var(--surface-secondary)]">
          <span>{activeResult.rows.length} 行</span>
          <span className="mx-2">·</span>
          <span>{activeResult.duration}ms</span>
        </div>
      )}
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
