import React, { useState, useCallback, useEffect, useMemo, useRef } from "react";
import { useTabsStore, type Tab } from "@/stores/tabs";
import { DataGrid } from "@/components/table/DataGrid";
import { DataGridToolbar, type FilterCondition } from "@/components/table/DataGridToolbar";
import { RowPreview } from "@/components/table/RowPreview";
import { JSONPreviewDialog } from "@/components/table/JSONPreviewDialog";
import { DDLViewer } from "@/components/table/DDLViewer";
import { MarkdownEditor } from "@/components/editor/MarkdownEditor";
import { RowContextMenu, type ContextMenuPosition } from "@/components/table/ContextMenu";
import { useUIStore } from "@/stores/ui";
import { useTranslation } from "@/i18n";
import { cn, copyToClipboard } from "@/lib/utils";
import { formatJSONForPreview } from "@/lib/json";
import { RefreshCw, ChevronLeft, ChevronRight, Plus, Trash2 } from "lucide-react";
import { useConnectionStore } from "@/stores/connection";
import type { ColumnMeta, ColumnInfo, QueryResult } from "@/types/database";
import * as QueryService from "../../../wailsjs/go/services/QueryService";
import * as DocService from "../../../wailsjs/go/services/DocService";
import * as ExportService from "../../../wailsjs/go/services/ExportService";
import { StructureView } from "./StructureView";
import { TipBtn } from "./TipBtn";
import type { TableSubView } from "./tabTypes";
import { isGridTarget, isEditableTarget } from "./tabUtils";
import { useTableViewResources } from "./useTableViewResources";
import { useTableViewKeyboardShortcuts } from "./useTableViewKeyboardShortcuts";
import { useTableDataEditor } from "./useTableDataEditor";
import { reportTabError } from "./tabFeedback";

export function TableView({ tab }: { tab: Tab }) {
  const { t } = useTranslation();
  const initialSubView = (tab.initialSubView as TableSubView) || "data";
  const driver = useConnectionStore((s) =>
    s.connections.find((c) => c.id === tab.connectionId)?.type
  );
  const [subView, setSubView] = useState<TableSubView>(initialSubView);
  const [visitedSubViews, setVisitedSubViews] = useState<Record<TableSubView, boolean>>({
    data: initialSubView === "data",
    structure: initialSubView === "structure",
    info: initialSubView === "info",
    doc: initialSubView === "doc",
  });
  const [columns, setColumns] = useState<ColumnMeta[]>([]);
  const [tableRows, setTableRows] = useState<Record<string, unknown>[]>([]);
  const [totalRows, setTotalRows] = useState(0);
  const [queryDuration, setQueryDuration] = useState(0);
  const [selectedRowIndex, setSelectedRowIndex] = useState<number | null>(null);
  const [page, setPage] = useState(1);
  const [jumpPageInput, setJumpPageInput] = useState("1");
  const [loading, setLoading] = useState(false);
  const [contextMenu, setContextMenu] = useState<ContextMenuPosition | null>(null);
  const [clickedColumn, setClickedColumn] = useState<string | null>(null);
  const [contextRowIndex, setContextRowIndex] = useState<number | null>(null);
  const [jsonPreviewContent, setJsonPreviewContent] = useState<string | null>(null);
  const [activeFilters, setActiveFilters] = useState<FilterCondition[]>([]);
  const [showFilter, setShowFilter] = useState(false);
  const [rawSqlFilter, setRawSqlFilter] = useState("");
  const [originalData, setOriginalData] = useState<Record<string, unknown>[]>([]);
  const { previewVisible, setPreviewVisible, pageSize, showDataRowNumbers } = useUIStore();
  const addTab = useTabsStore((s) => s.addTab);
  const updateTab = useTabsStore((s) => s.updateTab);
  const {
    structureColumns,
    indexes,
    ddl,
    docContent,
    loadStructure,
    loadDDL,
    loadDoc,
    setDocContent,
  } = useTableViewResources({
    connectionId: tab.connectionId,
    database: tab.database,
    table: tab.table,
  });
  const [structureHasEdits, setStructureHasEdits] = useState(false);
  const structureCommitRef = useRef<((source?: "shortcut" | "button") => Promise<void>) | null>(null);
  const structureDeleteRef = useRef<(() => void) | null>(null);
  const structureInsertRef = useRef<(() => void) | null>(null);
  const gridContainerRef = useRef<HTMLDivElement>(null);

  useEffect(() => {
    if (tab.initialSubView && tab.initialSubView !== subView) {
      const nextSubView = tab.initialSubView as TableSubView;
      setSubView(nextSubView);
      setVisitedSubViews((prev) => ({ ...prev, [nextSubView]: true }));
    }
  }, [subView, tab.initialSubView]);

  const switchSubView = useCallback((nextSubView: TableSubView) => {
    if (nextSubView === subView) return;
    setVisitedSubViews((prev) => ({ ...prev, [nextSubView]: true }));
    setSubView(nextSubView);
    if (tab.initialSubView !== nextSubView) {
      updateTab(tab.id, { initialSubView: nextSubView });
    }
  }, [subView, tab.id, tab.initialSubView, updateTab]);

  const loadData = useCallback(async (p: number, filters: FilterCondition[] = [], rawSql = "") => {
    if (!tab.connectionId || !tab.database || !tab.table) return;
    setLoading(true);
    try {
      let result;
      if (rawSql.trim()) {
        result = await QueryService.QueryTableDataWithRawInput(
          tab.connectionId, tab.database, tab.table, p, pageSize, filters as any, [], rawSql.trim()
        );
      } else {
        result = await QueryService.QueryTableData(
          tab.connectionId, tab.database, tab.table, p, pageSize, filters as any, []
        );
      }
      if (result) {
        const rows = result.rows || [];
        setColumns(result.columns || []);
        setTableRows(rows);
        setOriginalData(rows.map((r: Record<string, unknown>) => ({ ...r })));
        setTotalRows(result.total || 0);
        setQueryDuration(result.duration || 0);
      }
    } catch (e) {
      reportTabError({
        logTitle: "[TableView] 加载表数据失败:",
        toastMessage: "加载表数据失败",
        error: e,
      });
    } finally {
      setLoading(false);
    }
  }, [tab.connectionId, tab.database, tab.table, pageSize]);

  const openQueryTabWithDefaultSQL = useCallback(async (title: string) => {
    if (!tab.connectionId || !tab.table) return;
    try {
      const sql = await QueryService.DefaultSelectTableSQL(tab.connectionId, tab.table, pageSize);
      addTab({
        type: "query",
        title,
        connectionId: tab.connectionId,
        database: tab.database,
        table: tab.table,
        closable: true,
        sql,
      });
    } catch (e: any) {
      reportTabError({
        logTitle: "[TableView] 获取默认查询语句失败:",
        toastMessage: "获取默认查询语句失败",
        error: e,
      });
    }
  }, [addTab, pageSize, tab.connectionId, tab.database, tab.table]);

  const {
    data,
    setData,
    editedCells,
    newRowIndexes,
    pendingDeleteIndexes,
    hasEdits,
    resetEditState,
    handleCellEdit,
    handleAddRow,
    handleDeleteSelectedRow,
    commitChanges,
    syncData,
  } = useTableDataEditor({
    connectionId: tab.connectionId,
    database: tab.database,
    table: tab.table,
    columns,
    structureColumns,
    initialData: tableRows,
    originalData,
  });

  const selectedRow = selectedRowIndex !== null ? data[selectedRowIndex] : null;
  const contextRow = contextRowIndex !== null ? data[contextRowIndex] : selectedRow;
  const contextCellValue = useMemo(() => {
    if (!contextRow || !clickedColumn) return null;
    return contextRow[clickedColumn];
  }, [contextRow, clickedColumn]);
  const formattedContextJSON = useMemo(
    () => formatJSONForPreview(contextCellValue),
    [contextCellValue]
  );



  useEffect(() => {
    loadData(page, activeFilters);
  }, [loadData, page, activeFilters]);

  useEffect(() => {
    setJumpPageInput(String(page));
  }, [page]);

  useEffect(() => {
    // Data 视图编辑器需要列结构信息（如 enum/default），提前预加载一次
    void loadStructure();
  }, [loadStructure]);

  useEffect(() => {
    if (subView === "structure") {
      void loadStructure();
      return;
    }
    if (subView === "info") {
      void loadDDL();
      return;
    }
    if (subView === "doc") {
      void loadDoc();
    }
  }, [subView, loadStructure, loadDDL, loadDoc]);

  const reloadDataView = useCallback(() => {
    resetEditState();
    void loadData(page, activeFilters, rawSqlFilter);
  }, [activeFilters, loadData, page, rawSqlFilter, resetEditState]);

  const handleContextMenu = useCallback((e: React.MouseEvent, rowIndex: number, columnName?: string) => {
    e.preventDefault();
    setContextRowIndex(rowIndex);
    setClickedColumn(columnName || null);
    setContextMenu({ x: e.clientX, y: e.clientY });
  }, []);

  const deleteSelectedDataRow = useCallback(() => {
    handleDeleteSelectedRow(selectedRowIndex, setSelectedRowIndex);
  }, [handleDeleteSelectedRow, selectedRowIndex]);

  const insertSelectedDataRow = useCallback(() => {
    handleAddRow(setSelectedRowIndex);
  }, [handleAddRow]);

  const refreshAfterCommit = useCallback(async () => {
    await Promise.all([
      loadData(page, activeFilters, rawSqlFilter),
      loadStructure(true),
      loadDDL(true),
    ]);
  }, [activeFilters, loadDDL, loadData, loadStructure, page, rawSqlFilter]);

  const commitTableChanges = useCallback(async (source: "shortcut" | "button" = "button") => {
    const success = await commitChanges();
    if (success) {
      if (source === "shortcut") {
        useUIStore.getState().addToast("success", t("datagrid.commitSuccess"), 1200, "top-center");
      }
      await refreshAfterCommit();
    }
  }, [commitChanges, refreshAfterCommit, t]);

  useTableViewKeyboardShortcuts({
    tabId: tab.id,
    subView,
    showFilter,
    setSubView: switchSubView,
    setShowFilter,
    structureCommitRef,
    structureDeleteRef,
    structureInsertRef,
    commitChanges: commitTableChanges,
    deleteDataRow: deleteSelectedDataRow,
    insertDataRow: insertSelectedDataRow,
    loadStructure,
    loadDDL,
    loadDoc,
    reloadDataView,
    selectedRow,
    selectedRowIndex,
    setSelectedRowIndex,
    dataLength: data.length,
    previewVisible,
    gridContainerRef,
  });

  const handleExportTable = useCallback(async (format: "csv" | "json" | "sql" = "csv") => {
    if (!tab.connectionId || !tab.database || !tab.table) return;
    try {
      const taskId = await ExportService.ExportTableStream(tab.connectionId, tab.database, tab.table, format);
      if (!taskId) return;
    } catch (e: any) {
      reportTabError({
        logTitle: "[TableView] 流式导出启动失败:",
        toastMessage: "导出失败",
        error: e,
      });
    }
  }, [tab.connectionId, tab.database, tab.table]);


  const totalPages = Math.max(1, Math.ceil(totalRows / pageSize));
  const handleJumpToPage = useCallback(() => {
    const parsed = Number.parseInt(jumpPageInput, 10);
    if (!Number.isFinite(parsed)) {
      setJumpPageInput(String(page));
      return;
    }
    const target = Math.max(1, Math.min(totalPages, parsed));
    if (target !== page) {
      setSelectedRowIndex(null);
      setPage(target);
    }
    setJumpPageInput(String(target));
  }, [jumpPageInput, page, totalPages]);

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
          onOpenQuery={() => void openQueryTabWithDefaultSQL(`${t("tabs.newQuery")} - ${tab.table}`)}
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

      <div className="flex flex-1 overflow-hidden" style={{ paddingBottom: "var(--size-btn)" }}>
        {visitedSubViews.data && (
          <div className={cn("flex-1 min-w-0 overflow-hidden", subView === "data" ? "flex" : "hidden")}>
            <div ref={gridContainerRef} className="flex-1 flex overflow-hidden min-w-0">
              <DataGrid
                columns={columns}
                columnInfos={structureColumns}
                data={data}
                selectedRowIndex={selectedRowIndex}
                onSelectRow={setSelectedRowIndex}
                onContextMenu={handleContextMenu}
                editedCells={editedCells}
                onCellEdit={handleCellEdit}
                onAppendRow={() => handleAddRow(setSelectedRowIndex)}
                showRowNumbers={showDataRowNumbers}
                rowNumberOffset={(page - 1) * pageSize}
                database={tab.database || ""}
                tableName={tab.table || ""}
                newRowIndexes={newRowIndexes}
                pendingDeleteIndexes={pendingDeleteIndexes}
              />
            </div>
            {subView === "data" && previewVisible && selectedRow && selectedRowIndex !== null && (
              <RowPreview
                row={selectedRow}
                columns={columns}
                tableName={tab.table || ""}
                rowKey={`${page}:${selectedRowIndex}`}
                onClose={() => setPreviewVisible(false)}
                onEdit={(column, value) => {
                  handleCellEdit(selectedRowIndex, column, value);
                }}
              />
            )}
          </div>
        )}

        {visitedSubViews.structure && (
          <div className={cn("flex-1 min-w-0 overflow-hidden", subView === "structure" ? "flex" : "hidden")}>
            <StructureView
              connectionId={tab.connectionId || ""}
              database={tab.database || ""}
              tableName={tab.table || ""}
              driver={driver}
              columns={structureColumns}
              indexes={indexes}
              onRefresh={refreshAfterCommit}
              onHasEditsChange={setStructureHasEdits}
              commitRef={structureCommitRef}
              deleteRef={structureDeleteRef}
              insertRef={structureInsertRef}
            />
          </div>
        )}

        {visitedSubViews.info && (
          <div className={cn("flex-1 min-w-0 overflow-hidden", subView === "info" ? "flex" : "hidden")}>
            <DDLViewer ddl={ddl} tableName={tab.table || ""} />
          </div>
        )}

        {visitedSubViews.doc && (
          <div className={cn("flex-1 min-w-0 overflow-hidden", subView === "doc" ? "flex" : "hidden")}>
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
          </div>
        )}
      </div>

      <div className={cn(
        "absolute bottom-0 left-0 right-0 z-20",
        "h-[var(--size-btn)] flex items-center px-[var(--size-padding-sm)] border-t text-[length:var(--size-font-2xs)] select-none gap-[var(--size-gap-sm)]",
        "bg-[var(--surface-secondary)] border-[var(--border-color)]"
      )}>
        <div className="flex items-center flex-shrink-0 bg-[var(--sidebar-hover)] rounded-[var(--radius-btn)] p-0.5 gap-0">
          {(["data", "structure", "info", "doc"] as TableSubView[]).map((v) => (
            <button
              key={v}
              className={cn(
                "px-2.5 py-0.5 rounded-[calc(var(--radius-btn)-2px)] text-[length:var(--size-font-2xs)] transition-all whitespace-nowrap",
                subView === v
                  ? "bg-white dark:bg-[var(--surface)] text-[var(--fg)] font-medium shadow-sm ring-1 ring-black/5 dark:ring-white/10"
                  : "text-[var(--fg-secondary)] hover:text-[var(--fg)]"
              )}
              onClick={() => switchSubView(v)}
            >
              {v === "data" ? t("contextMenu.viewData") : v === "structure" ? t("contextMenu.viewStructure") : v === "info" ? t("contextMenu.viewDDL") : t("contextMenu.tableDoc")}
            </button>
          ))}
        </div>

        <div className="flex-1 min-w-0" />

        <div className={cn("flex items-center gap-0.5 flex-shrink-0", subView !== "data" && "hidden")}>
          <TipBtn tip={t("common.create")} className="h-[var(--size-btn-sm)] w-[var(--size-btn-sm)] flex items-center justify-center rounded-[var(--radius-btn)] text-[var(--fg-secondary)] hover:bg-[var(--sidebar-hover)] transition-colors" onClick={() => handleAddRow(setSelectedRowIndex)}>
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
            onClick={() => handleDeleteSelectedRow(selectedRowIndex, setSelectedRowIndex)}
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
          <TipBtn tip={t("toolbar.sqlQuery")} className="px-1.5 py-0.5 rounded-[var(--radius-btn)] text-[length:var(--size-font-2xs)] text-[var(--fg-secondary)] hover:bg-[var(--sidebar-hover)] transition-colors font-mono" onClick={() => void openQueryTabWithDefaultSQL(`SQL - ${tab.table}`)}>
            SQL
          </TipBtn>
          <TipBtn tip={t("common.refresh")} shortcut="⌘R" className="h-[var(--size-btn-sm)] w-[var(--size-btn-sm)] flex items-center justify-center rounded-[var(--radius-btn)] hover:bg-[var(--sidebar-hover)] transition-colors" onClick={reloadDataView}>
            <RefreshCw className="h-2.5 w-2.5 text-[var(--fg-secondary)]" />
          </TipBtn>
          {hasEdits && (
            <TipBtn tip={t("common.commit")} shortcut="⌘S" className="px-1.5 py-0.5 rounded-[var(--radius-btn)] text-[length:var(--size-font-2xs)] font-medium text-white bg-[var(--accent)] hover:opacity-90 transition-opacity" onClick={() => void commitTableChanges()}>
              {t("common.commit")}
            </TipBtn>
          )}
        </div>

        <div className={cn("flex items-center gap-0.5 flex-shrink-0", subView !== "structure" && "hidden")}>
          <TipBtn tip={t("common.refresh")} shortcut="⌘R" className="h-[var(--size-btn-sm)] w-[var(--size-btn-sm)] flex items-center justify-center rounded-[var(--radius-btn)] hover:bg-[var(--sidebar-hover)] transition-colors" onClick={() => { void loadStructure(true); }}>
            <RefreshCw className="h-2.5 w-2.5 text-[var(--fg-secondary)]" />
          </TipBtn>
          {structureHasEdits && (
            <TipBtn tip={t("common.commit")} shortcut="⌘S" className="px-1.5 py-0.5 rounded-[var(--radius-btn)] text-[length:var(--size-font-2xs)] font-medium text-white bg-[var(--accent)] hover:opacity-90 transition-opacity" onClick={() => structureCommitRef.current?.("button")}>
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
            <button className="h-4 w-4 flex items-center justify-center rounded-[var(--radius-btn)] hover:bg-[var(--sidebar-hover)] disabled:opacity-30" disabled={page >= totalPages} onClick={() => setPage(page + 1)}>
              <ChevronRight className="h-2.5 w-2.5 text-[var(--fg-secondary)]" />
            </button>
          </div>
        )}
      </div>

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
          if (contextRow && tab.table) {
            try {
              const sql = await QueryService.GenerateInsertSQL(tab.table, contextRow as Record<string, unknown>);
              await copyToClipboard(sql);
            } catch (e) {
              console.error("[TableView] 生成 INSERT SQL 失败:", e);
            }
          }
          setContextMenu(null);
        }}
        onDeleteRow={() => {
          const targetIndex = contextRowIndex ?? selectedRowIndex;
          if (targetIndex !== null) {
            handleDeleteSelectedRow(targetIndex, setSelectedRowIndex);
          }
          setContextMenu(null);
        }}
        onRefresh={() => { loadData(page, activeFilters, rawSqlFilter); setContextMenu(null); }}
        onPreview={() => {
          const targetIndex = contextRowIndex ?? selectedRowIndex;
          if (targetIndex !== null) {
            setSelectedRowIndex(targetIndex);
            setPreviewVisible(true);
          }
          setContextMenu(null);
        }}
        onDownloadPage={() => { handleExportTable("csv"); setContextMenu(null); }}
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
