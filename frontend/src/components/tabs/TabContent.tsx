import React, { useState, useCallback, useEffect } from "react";
import { useTabsStore, type Tab } from "@/stores/tabs";
import { DataGrid } from "@/components/table/DataGrid";
import { DataGridToolbar } from "@/components/table/DataGridToolbar";
import { RowPreview } from "@/components/table/RowPreview";
import { DDLViewer } from "@/components/table/DDLViewer";
import { SQLEditor } from "@/components/editor/SQLEditor";
import { MarkdownEditor } from "@/components/editor/MarkdownEditor";
import {
  RowContextMenu,
  type ContextMenuPosition,
} from "@/components/table/ContextMenu";
import { useUIStore } from "@/stores/ui";
import { copyToClipboard, rowToInsertSQL } from "@/lib/utils";
import { Database } from "lucide-react";
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
      <Database className="h-16 w-16 mb-4 opacity-30" />
      <p className="text-lg font-medium mb-1">TablePlus AI</p>
      <p className="text-sm">选择左侧连接或表开始使用</p>
      <div className="mt-6 flex gap-4 text-xs text-[var(--fg-muted)]">
        <div className="flex items-center gap-1">
          <kbd className="px-1.5 py-0.5 rounded border border-[var(--border-color)] bg-[var(--surface-secondary)]">
            ⌘K
          </kbd>
          <span>快速搜索</span>
        </div>
        <div className="flex items-center gap-1">
          <kbd className="px-1.5 py-0.5 rounded border border-[var(--border-color)] bg-[var(--surface-secondary)]">
            ⌘T
          </kbd>
          <span>新标签页</span>
        </div>
      </div>
    </div>
  );
}

function TableView({ tab }: { tab: Tab }) {
  const [columns, setColumns] = useState<ColumnMeta[]>([]);
  const [data, setData] = useState<Record<string, unknown>[]>([]);
  const [totalRows, setTotalRows] = useState(0);
  const [selectedRowIndex, setSelectedRowIndex] = useState<number | null>(null);
  const [page, setPage] = useState(1);
  const [loading, setLoading] = useState(false);
  const [contextMenu, setContextMenu] = useState<ContextMenuPosition | null>(null);
  const { previewVisible, setPreviewVisible } = useUIStore();
  const { addTab } = useTabsStore();
  const pageSize = 100;

  const selectedRow =
    selectedRowIndex !== null ? data[selectedRowIndex] : null;

  const loadData = useCallback(async (p: number) => {
    if (!tab.connectionId || !tab.database || !tab.table) return;
    setLoading(true);
    try {
      const result = await QueryService.QueryTableData(
        tab.connectionId, tab.database, tab.table, p, pageSize, [], []
      );
      if (result) {
        setColumns(result.columns || []);
        setData(result.rows || []);
        setTotalRows(result.total || 0);
      }
    } catch (e) {
      console.error("加载表数据失败:", e);
    } finally {
      setLoading(false);
    }
  }, [tab.connectionId, tab.database, tab.table, pageSize]);

  useEffect(() => {
    loadData(page);
  }, [loadData, page]);

  const handleContextMenu = useCallback(
    (e: React.MouseEvent, _rowIndex: number) => {
      e.preventDefault();
      setContextMenu({ x: e.clientX, y: e.clientY });
    },
    []
  );

  // 空格键预览
  useEffect(() => {
    const handler = (e: KeyboardEvent) => {
      if (e.code === "Space" && selectedRow && !e.target) {
        e.preventDefault();
        setPreviewVisible(!previewVisible);
      }
      if (e.code === "Space" && selectedRow) {
        const target = e.target as HTMLElement;
        if (target.tagName !== "INPUT" && target.tagName !== "TEXTAREA") {
          e.preventDefault();
          setPreviewVisible(!previewVisible);
        }
      }
    };
    window.addEventListener("keydown", handler);
    return () => window.removeEventListener("keydown", handler);
  }, [selectedRow, previewVisible, setPreviewVisible]);

  return (
    <div className="flex flex-col h-full">
      <DataGridToolbar
        tableName={tab.table || ""}
        totalRows={totalRows}
        page={page}
        pageSize={pageSize}
        onPageChange={setPage}
        onRefresh={() => loadData(page)}
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
        onExport={() => {}}
        onFilterChange={() => {}}
      />
      <div className="flex flex-1 overflow-hidden">
        <DataGrid
          columns={columns}
          data={data}
          selectedRowIndex={selectedRowIndex}
          onSelectRow={setSelectedRowIndex}
          onContextMenu={handleContextMenu}
        />
        {previewVisible && selectedRow && (
          <RowPreview
            row={selectedRow}
            tableName={tab.table || ""}
            onClose={() => setPreviewVisible(false)}
          />
        )}
      </div>

      <RowContextMenu
        position={contextMenu}
        onClose={() => setContextMenu(null)}
        onCopyCell={() => {
          setContextMenu(null);
        }}
        onCopyRow={() => {
          if (selectedRow) {
            copyToClipboard(JSON.stringify(selectedRow));
          }
          setContextMenu(null);
        }}
        onCopyAsInsert={() => {
          if (selectedRow && tab.table) {
            copyToClipboard(rowToInsertSQL(tab.table, selectedRow));
          }
          setContextMenu(null);
        }}
        onDeleteRow={() => {
          setContextMenu(null);
        }}
        onRefresh={() => {
          loadData(page);
          setContextMenu(null);
        }}
        onPreview={() => {
          setPreviewVisible(true);
          setContextMenu(null);
        }}
      />
    </div>
  );
}

function QueryView({ tab }: { tab: Tab }) {
  const [results, setResults] = useState<Record<string, unknown>[]>([]);
  const [resultColumns, setResultColumns] = useState<ColumnMeta[]>([]);
  const [loading, setLoading] = useState(false);
  const [queryError, setQueryError] = useState<string | null>(null);

  const handleExecute = async (sql: string) => {
    if (!tab.connectionId || !tab.database) return;
    setLoading(true);
    setQueryError(null);
    try {
      const result = await QueryService.ExecuteSQL(
        tab.connectionId, tab.database, sql
      );
      if (result.error) {
        setQueryError(result.error);
      }
      setResultColumns(result.columns || []);
      setResults(result.rows || []);
    } catch (e: any) {
      setQueryError(e?.message || "查询执行失败");
    } finally {
      setLoading(false);
    }
  };

  return (
    <div className="flex flex-col h-full">
      <div className="h-[200px] border-b border-[var(--border-color)]">
        <SQLEditor
          initialSQL={tab.sql}
          onExecute={handleExecute}
          loading={loading}
        />
      </div>
      {queryError && (
        <div className="px-4 py-2 text-sm text-[var(--danger)] bg-red-50 dark:bg-red-900/10 border-b border-[var(--border-color)]">
          {queryError}
        </div>
      )}
      <div className="flex-1 overflow-hidden">
        {resultColumns.length > 0 ? (
          <DataGrid
            columns={resultColumns}
            data={results}
            selectedRowIndex={null}
            onSelectRow={() => {}}
          />
        ) : (
          <div className="flex items-center justify-center h-full text-sm text-[var(--fg-muted)]">
            执行查询后在此显示结果
          </div>
        )}
      </div>
    </div>
  );
}

function DDLView({ tab }: { tab: Tab }) {
  const [ddl, setDDL] = useState("");

  useEffect(() => {
    if (tab.connectionId && tab.database && tab.table) {
      DatabaseService.GetDDL(tab.connectionId, tab.database, tab.table)
        .then(setDDL)
        .catch(() => setDDL("-- 获取 DDL 失败"));
    }
  }, [tab.connectionId, tab.database, tab.table]);

  return <DDLViewer ddl={ddl} tableName={tab.table || ""} />;
}

function DocView({ tab }: { tab: Tab }) {
  const [content, setContent] = useState("");

  useEffect(() => {
    if (tab.connectionId && tab.database && tab.table) {
      DocService.GetTableDoc(tab.connectionId, tab.database, tab.table)
        .then((doc) => setContent(doc || ""))
        .catch(() => {});
    }
  }, [tab.connectionId, tab.database, tab.table]);

  return (
    <MarkdownEditor
      content={content}
      tableName={tab.table || ""}
      onSave={async (md) => {
        if (tab.connectionId && tab.database && tab.table) {
          await DocService.SaveTableDoc(
            tab.connectionId, tab.database, tab.table, md
          );
          setContent(md);
        }
      }}
    />
  );
}
