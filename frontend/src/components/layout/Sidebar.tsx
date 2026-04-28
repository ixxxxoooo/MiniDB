import React, { useState, useRef, useEffect, useMemo, useDeferredValue, useCallback } from "react";
import { createPortal } from "react-dom";
import { useVirtualizer } from "@tanstack/react-virtual";
import {
  Database,
  Table2,
  Copy,
  FileCode,
  Download,
  Trash2,
  Eye,
  ExternalLink,
  Columns,
  Search,
  X,
} from "lucide-react";
import { cn, copyToClipboard } from "@/lib/utils";
import { useConnectionStore } from "@/stores/connection";
import { useUIStore } from "@/stores/ui";
import { useTabsStore } from "@/stores/tabs";
import { useDatabase } from "@/hooks/useDatabase";
import { useTranslation } from "@/i18n";
import { TOOLTIP_DELAY_MS } from "@/components/ui/tooltip";

interface ContextMenuState {
  x: number;
  y: number;
  tableName: string;
}

interface TableHoverTooltipState {
  text: string;
  x: number;
  y: number;
}

const TABLE_HOVER_TOOLTIP_DELAY_MS = TOOLTIP_DELAY_MS;

export function Sidebar({ onNewConnection, onEditConnection }: { onNewConnection: () => void, onEditConnection: (c: any) => void }) {
  const { sidebarWidth, setSidebarWidth } = useUIStore();
  const resizingRef = useRef(false);
  const resizeRafRef = useRef<number | null>(null);
  const pendingSidebarWidthRef = useRef<number | null>(null);
  const workspaces = useConnectionStore((s) => s.workspaces);
  const activeWorkspaceId = useConnectionStore((s) => s.activeWorkspaceId);
  const tables = useConnectionStore((s) => s.tables);
  const connectionStates = useConnectionStore((s) => s.connectionStates);
  const addTab = useTabsStore((s) => s.addTab);
  const selectedTableName = useTabsStore(
    (s) => s.tabs.find((t) => t.id === s.activeTabId)?.table || null
  );
  const { loadTables } = useDatabase();
  const { t } = useTranslation();
  
  const [contextMenu, setContextMenu] = useState<ContextMenuState | null>(null);
  const [searchQuery, setSearchQuery] = useState("");
  const [optimisticSelectedTable, setOptimisticSelectedTable] = useState<string | null>(null);
  const [selectedTableNames, setSelectedTableNames] = useState<Set<string>>(new Set());
  const [tableSelectionAnchor, setTableSelectionAnchor] = useState<number | null>(null);
  const [tableHoverTooltip, setTableHoverTooltip] = useState<TableHoverTooltipState | null>(null);
  const deferredSearchQuery = useDeferredValue(searchQuery);
  const searchInputRef = useRef<HTMLInputElement>(null);
  const menuRef = useRef<HTMLDivElement>(null);
  const tableListRef = useRef<HTMLDivElement>(null);
  const tableHoverTimerRef = useRef<number | null>(null);
  const hoveredTableNameRef = useRef<string | null>(null);
  const requestedTableKeysRef = useRef<Set<string>>(new Set());

  const clearTableHoverTimer = useCallback(() => {
    if (tableHoverTimerRef.current !== null) {
      window.clearTimeout(tableHoverTimerRef.current);
      tableHoverTimerRef.current = null;
    }
  }, []);

  const scheduleTableHoverTooltip = useCallback((tableName: string, x: number, y: number) => {
    clearTableHoverTimer();
    hoveredTableNameRef.current = tableName;
    tableHoverTimerRef.current = window.setTimeout(() => {
      tableHoverTimerRef.current = null;
      if (hoveredTableNameRef.current !== tableName) return;
      setTableHoverTooltip({ text: tableName, x, y });
    }, TABLE_HOVER_TOOLTIP_DELAY_MS);
  }, [clearTableHoverTimer]);

  useEffect(() => {
    if (!contextMenu) return;
    const handler = (e: MouseEvent) => {
      if (menuRef.current && !menuRef.current.contains(e.target as Node)) {
        setContextMenu(null);
      }
    };
    document.addEventListener("mousedown", handler);
    return () => document.removeEventListener("mousedown", handler);
  }, [contextMenu]);

  useEffect(() => {
    if (!optimisticSelectedTable) return;
    if (selectedTableName === optimisticSelectedTable) {
      setOptimisticSelectedTable(null);
    }
  }, [optimisticSelectedTable, selectedTableName]);

  useEffect(() => {
    setOptimisticSelectedTable(null);
    setSelectedTableNames(new Set());
    setTableSelectionAnchor(null);
    clearTableHoverTimer();
    hoveredTableNameRef.current = null;
    setTableHoverTooltip(null);
  }, [activeWorkspaceId, clearTableHoverTimer]);

  useEffect(() => {
    return () => {
      clearTableHoverTimer();
    };
  }, [clearTableHoverTimer]);

  // 根据搜索过滤表
  const filterTables = (tableList: { name: string; type: string }[], query: string) => {
    if (!query.trim()) return tableList;
    const q = query.toLowerCase();
    return tableList.filter((t) => t.name.toLowerCase().includes(q));
  };

  const handleSidebarResizeStart = (e: React.MouseEvent) => {
    e.preventDefault();
    resizingRef.current = true;
    const startX = e.clientX;
    const startW = sidebarWidth;
    const onMove = (ev: MouseEvent) => {
      if (!resizingRef.current) return;
      const newW = Math.max(180, Math.min(500, startW + ev.clientX - startX));
      pendingSidebarWidthRef.current = newW;
      if (resizeRafRef.current !== null) return;
      resizeRafRef.current = requestAnimationFrame(() => {
        resizeRafRef.current = null;
        if (pendingSidebarWidthRef.current !== null) {
          setSidebarWidth(pendingSidebarWidthRef.current);
        }
      });
    };
    const onUp = () => {
      resizingRef.current = false;
      if (resizeRafRef.current !== null) {
        cancelAnimationFrame(resizeRafRef.current);
        resizeRafRef.current = null;
      }
      if (pendingSidebarWidthRef.current !== null) {
        setSidebarWidth(pendingSidebarWidthRef.current);
        pendingSidebarWidthRef.current = null;
      }
      document.removeEventListener("mousemove", onMove);
      document.removeEventListener("mouseup", onUp);
    };
    document.addEventListener("mousemove", onMove);
    document.addEventListener("mouseup", onUp);
  };

  const currentWs = workspaces.find((w) => w.id === activeWorkspaceId);
  const rawTables = currentWs ? tables[`${currentWs.connectionId}:${currentWs.database}`] : undefined;
  const currentConnStatus = currentWs ? connectionStates[currentWs.connectionId]?.status : undefined;
  const isConnecting = currentConnStatus === "connecting";
  const isConnected = currentConnStatus === "connected";
  const displayTables = useMemo(
    () => filterTables(rawTables || [], deferredSearchQuery),
    [rawTables, deferredSearchQuery]
  );
  const isLoadingTables = !!currentWs && (isConnecting || (isConnected && rawTables === undefined));
  const isEmptyTables = !!currentWs && rawTables !== undefined && rawTables.length === 0;

  useEffect(() => {
    if (!currentWs) return;
    const key = `${currentWs.connectionId}:${currentWs.database}`;
    if (!isConnected) return;
    if (rawTables !== undefined) {
      requestedTableKeysRef.current.delete(key);
      return;
    }
    if (requestedTableKeysRef.current.has(key)) return;
    requestedTableKeysRef.current.add(key);
    void loadTables(currentWs.connectionId, currentWs.database);
  }, [currentWs, isConnected, rawTables, loadTables]);
  const rowVirtualizer = useVirtualizer({
    count: displayTables.length,
    getScrollElement: () => tableListRef.current,
    estimateSize: () => 24,
    overscan: 10,
  });

  const handleOpenTable = (tableName: string) => {
    if (!currentWs) return;
    addTab({
      type: "table",
      title: tableName,
      connectionId: currentWs.connectionId,
      database: currentWs.database,
      table: tableName,
      closable: true,
    });
  };

  const selectTableRange = (start: number, end: number) => {
    const min = Math.max(0, Math.min(start, end));
    const max = Math.min(displayTables.length - 1, Math.max(start, end));
    const next = new Set<string>();
    for (let i = min; i <= max; i += 1) {
      const name = displayTables[i]?.name;
      if (name) next.add(name);
    }
    setSelectedTableNames(next);
  };

  const handleContextMenu = (e: React.MouseEvent, tableName: string) => {
    e.preventDefault();
    e.stopPropagation();
    clearTableHoverTimer();
    hoveredTableNameRef.current = null;
    setTableHoverTooltip(null);
    setContextMenu({ x: e.clientX, y: e.clientY, tableName });
  };

  const tableHoverTooltipPosition = useMemo(() => {
    if (!tableHoverTooltip) return null;
    const margin = 10;
    const offset = 14;
    const maxWidth = 420;
    const fallbackHeight = 38;
    const nextLeft = Math.min(
      tableHoverTooltip.x + offset,
      window.innerWidth - maxWidth - margin,
    );
    const nextTop = Math.min(
      tableHoverTooltip.y + offset,
      window.innerHeight - fallbackHeight - margin,
    );
    return {
      left: Math.max(margin, nextLeft),
      top: Math.max(margin, nextTop),
    };
  }, [tableHoverTooltip]);

  return (
    <div
      className={cn(
        "flex flex-col border-r overflow-hidden relative",
        "bg-[var(--sidebar-bg)] border-[var(--sidebar-border)]"
      )}
      style={{ width: sidebarWidth }}
    >
      {/* 右侧拖拽条 */}
      <div
        className="absolute right-0 top-0 h-full w-[5px] cursor-col-resize z-10 group"
        onMouseDown={handleSidebarResizeStart}
      >
        <div className="absolute inset-y-0 right-1/2 translate-x-1/2 w-px bg-transparent group-hover:bg-[var(--accent)]/40 transition-colors duration-200" />
        <div className="absolute top-1/2 right-1/2 translate-x-1/2 -translate-y-1/2 w-1 h-6 rounded-full bg-transparent group-hover:bg-[var(--accent)]/50 transition-all duration-200" />
      </div>

      {/* 搜索框 */}
      {currentWs && (
        <div className="px-[var(--size-padding-sm)] pt-[var(--size-gap-sm)] pb-[var(--size-gap-sm)] flex-shrink-0">
          <div className="relative">
            <Search className="absolute left-1.5 top-1/2 -translate-y-1/2 h-[var(--size-btn-icon-sm)] w-[var(--size-btn-icon-sm)] text-[var(--fg-muted)]" />
            <input
              ref={searchInputRef}
              type="text"
              className={cn(
                "w-full h-[var(--size-input-sm)] pl-6 pr-6 text-[length:var(--size-font-xs)] rounded-[var(--radius-input)] border bg-[var(--surface)] text-[var(--fg)]",
                "border-[var(--border-color)] placeholder:text-[var(--fg-muted)]",
                "focus:outline-none focus:border-[var(--accent)] focus:ring-1 focus:ring-[var(--accent)]/30"
              )}
              placeholder={t("sidebar.searchPlaceholder") || "Filter..."}
              value={searchQuery}
              onChange={(e) => setSearchQuery(e.target.value)}
            />
            {searchQuery && (
              <button
                className="absolute right-1 top-1/2 -translate-y-1/2 h-[var(--size-btn-sm)] w-[var(--size-btn-sm)] flex items-center justify-center rounded-[var(--radius-btn)] hover:bg-[var(--sidebar-hover)]"
                onClick={() => setSearchQuery("")}
              >
                <X className="h-2.5 w-2.5 text-[var(--fg-muted)]" />
              </button>
            )}
          </div>
        </div>
      )}

      {/* 主内容区 */}
      <div className="flex-1 overflow-hidden py-0.5">
        {!currentWs ? (
          <div className="flex-1 flex items-center justify-center h-full">
            <div className="text-center px-3">
              <Database className="h-6 w-6 mx-auto mb-2 text-[var(--fg-muted)]" />
              <p className="text-xs text-[var(--fg-secondary)] font-medium">{t("sidebar.noConnections") || "Not Connected"}</p>
              <p className="text-2xs text-[var(--fg-muted)] mt-1 px-4 leading-relaxed">
                {t("sidebar.connectHint")}
              </p>
            </div>
          </div>
        ) : (
          <div className="h-full flex flex-col">
            <div className="flex items-center h-6 px-2.5 text-[length:var(--size-font-xs)] font-semibold text-[var(--fg-secondary)] uppercase mt-1 mb-0.5">
              <span>{t("sidebar.tables")}</span>
              {rawTables && rawTables.length > 0 && (
                <span className="ml-1.5 text-[length:var(--size-font-2xs)] font-normal text-[var(--fg-muted)]">
                  ({rawTables.length})
                </span>
              )}
            </div>
            {isLoadingTables ? (
              <div className="flex items-center justify-center py-8 px-4">
                <p className="text-2xs text-[var(--fg-muted)] text-center">
                  {t("common.loading") || "Loading..."}
                </p>
              </div>
            ) : currentWs && !isConnected ? (
              <div className="flex items-center justify-center py-8 px-4">
                <p className="text-2xs text-[var(--fg-muted)] text-center">
                  {t("sidebar.noConnections") || "Not Connected"}
                </p>
              </div>
            ) : displayTables.length === 0 ? (
              <div className="flex items-center justify-center py-8 px-4">
                <p className="text-2xs text-[var(--fg-muted)] text-center">
                  {searchQuery
                    ? t("common.noResults")
                    : isEmptyTables
                      ? t("sidebar.noTables")
                      : t("sidebar.noConnections") || "Not Connected"}
                </p>
              </div>
            ) : (
              <div
                ref={tableListRef}
                className="flex-1 overflow-y-auto"
                tabIndex={0}
                data-table-list="true"
                onScroll={() => {
                  clearTableHoverTimer();
                  hoveredTableNameRef.current = null;
                  if (tableHoverTooltip) setTableHoverTooltip(null);
                }}
                onKeyDown={(e) => {
                  if ((e.metaKey || e.ctrlKey) && !e.shiftKey && !e.altKey && e.key.toLowerCase() === "a") {
                    e.preventDefault();
                    setSelectedTableNames(new Set(displayTables.map((item) => item.name)));
                    setTableSelectionAnchor(displayTables.length > 0 ? 0 : null);
                  }
                }}
              >
                <div
                  className="relative"
                  style={{ height: rowVirtualizer.getTotalSize() }}
                >
                  {rowVirtualizer.getVirtualItems().map((virtualRow) => {
                    const tbl = displayTables[virtualRow.index];
                    if (!tbl) return null;
                    const effectiveSelectedTable = optimisticSelectedTable || selectedTableName;
                    const hasMultiSelection = selectedTableNames.size > 0;
                    const isSelected = hasMultiSelection
                      ? selectedTableNames.has(tbl.name)
                      : tbl.name === effectiveSelectedTable;
                    return (
                      <div
                        key={tbl.name}
                        className="absolute left-0 right-0 px-1"
                        style={{ transform: `translateY(${virtualRow.start}px)` }}
                      >
                        <div
                          className={cn(
                            "flex items-center h-[24px] px-3 rounded-[var(--radius-btn)] cursor-pointer",
                            isSelected
                              ? "bg-[var(--sidebar-active)] text-[var(--sidebar-accent)]"
                              : "hover:bg-[var(--sidebar-hover)]"
                          )}
                          onPointerDown={(e) => {
                            if (e.button !== 0) return;
                            e.preventDefault();
                            clearTableHoverTimer();
                            hoveredTableNameRef.current = null;
                            setTableHoverTooltip(null);
                            const rowIndex = virtualRow.index;
                            const isMod = e.metaKey || e.ctrlKey;
                            const isShift = e.shiftKey;

                            if (isShift) {
                              const anchor = tableSelectionAnchor ?? rowIndex;
                              selectTableRange(anchor, rowIndex);
                              return;
                            }

                            if (isMod) {
                              setSelectedTableNames((prev) => {
                                const next = new Set(prev);
                                if (next.has(tbl.name)) {
                                  next.delete(tbl.name);
                                } else {
                                  next.add(tbl.name);
                                }
                                return next;
                              });
                              setTableSelectionAnchor(rowIndex);
                              return;
                            }

                            setSelectedTableNames(new Set([tbl.name]));
                            setTableSelectionAnchor(rowIndex);
                            setOptimisticSelectedTable(tbl.name);
                            requestAnimationFrame(() => handleOpenTable(tbl.name));
                          }}
                          onContextMenu={(e) => handleContextMenu(e, tbl.name)}
                        >
                          <Table2 className={cn("h-3 w-3 mr-2 flex-shrink-0", isSelected ? "text-[var(--sidebar-accent)]" : "text-[var(--fg-muted)]")} />
                          <span
                            className={cn("text-[length:var(--size-font-2xs)] truncate flex-1", isSelected ? "text-[var(--sidebar-accent)] font-medium" : "text-[var(--sidebar-fg)]")}
                            onMouseEnter={(e) => {
                              scheduleTableHoverTooltip(tbl.name, e.clientX, e.clientY);
                            }}
                            onMouseMove={(e) => {
                              if (tableHoverTooltip?.text === tbl.name) {
                                setTableHoverTooltip((prev) => {
                                  if (!prev || prev.text !== tbl.name) return prev;
                                  if (prev.x === e.clientX && prev.y === e.clientY) return prev;
                                  return { ...prev, x: e.clientX, y: e.clientY };
                                });
                                return;
                              }
                              scheduleTableHoverTooltip(tbl.name, e.clientX, e.clientY);
                            }}
                            onMouseLeave={() => {
                              clearTableHoverTimer();
                              hoveredTableNameRef.current = null;
                              setTableHoverTooltip((prev) => (prev?.text === tbl.name ? null : prev));
                            }}
                          >
                            {tbl.name}
                          </span>
                        </div>
                      </div>
                    );
                  })}
                </div>
              </div>
            )}
          </div>
        )}
      </div>

      {/* 表名跟随鼠标 tooltip（固定层，避免被列表容器裁剪） */}
      {tableHoverTooltip && tableHoverTooltipPosition && createPortal(
        <div
          className={cn(
            "fixed z-[140] pointer-events-none max-w-[420px] px-2.5 py-1.5",
            "rounded-[var(--radius-btn)] border text-[11px] leading-[1.35]",
            "bg-[var(--surface-elevated)] text-[var(--fg)] border-[var(--border-color)]",
            "shadow-[var(--shadow-lg)] animate-fade-in",
            "whitespace-pre-wrap break-all"
          )}
          style={{ left: tableHoverTooltipPosition.left, top: tableHoverTooltipPosition.top }}
        >
          {tableHoverTooltip.text}
        </div>,
        document.body
      )}

      {/* 右键菜单 */}
      {contextMenu && currentWs && createPortal(
        <div
          ref={menuRef}
          className={cn(
            "fixed z-[100] min-w-[180px] py-1 rounded-[var(--radius-menu)] shadow-lg border animate-fade-in",
            "bg-[var(--surface-elevated)] border-[var(--border-color)]"
          )}
          style={{ left: contextMenu.x, top: contextMenu.y }}
        >
          <button
            className="w-full px-2.5 py-1 text-xs text-left hover:bg-[var(--sidebar-hover)] text-[var(--fg)] flex items-center gap-2"
            onClick={() => {
              handleOpenTable(contextMenu.tableName);
              setContextMenu(null);
            }}
          >
            <ExternalLink className="h-3.5 w-3.5" /> {t("contextMenu.openInNewTab") || "Open"}
          </button>
          <button
            className="w-full px-2.5 py-1 text-xs text-left hover:bg-[var(--sidebar-hover)] text-[var(--fg)] flex items-center gap-2"
            onClick={() => {
              copyToClipboard(contextMenu.tableName);
              setContextMenu(null);
            }}
          >
            <Copy className="h-3.5 w-3.5" /> {t("contextMenu.copyTableName") || "Copy Name"}
          </button>
          <div className="h-px bg-[var(--border-subtle)] my-1" />
          <button
            className="w-full px-2.5 py-1 text-xs text-left hover:bg-[var(--sidebar-hover)] text-[var(--fg)] flex items-center gap-2"
            onClick={() => {
              addTab({
                type: "table", title: contextMenu.tableName,
                connectionId: currentWs.connectionId, database: currentWs.database, table: contextMenu.tableName,
                closable: true, initialSubView: "data",
              });
              setContextMenu(null);
            }}
          >
            <Table2 className="h-3.5 w-3.5" /> {t("contextMenu.viewData") || "View Data"}
          </button>
          <button
            className="w-full px-2.5 py-1 text-xs text-left hover:bg-[var(--sidebar-hover)] text-[var(--fg)] flex items-center gap-2"
            onClick={() => {
              addTab({
                type: "table", title: contextMenu.tableName,
                connectionId: currentWs.connectionId, database: currentWs.database, table: contextMenu.tableName,
                closable: true, initialSubView: "structure",
              });
              setContextMenu(null);
            }}
          >
            <Columns className="h-3.5 w-3.5" /> {t("contextMenu.viewStructure") || "Structure"}
          </button>
          <button
            className="w-full px-2.5 py-1 text-xs text-left hover:bg-[var(--sidebar-hover)] text-[var(--fg)] flex items-center gap-2"
            onClick={() => {
              addTab({
                type: "table", title: contextMenu.tableName,
                connectionId: currentWs.connectionId, database: currentWs.database, table: contextMenu.tableName,
                closable: true, initialSubView: "info",
              });
              setContextMenu(null);
            }}
          >
            <FileCode className="h-3.5 w-3.5" /> {t("contextMenu.viewDDL") || "View DDL"}
          </button>
          <button
            className="w-full px-2.5 py-1 text-xs text-left hover:bg-[var(--sidebar-hover)] text-[var(--fg)] flex items-center gap-2"
            onClick={() => {
              addTab({
                type: "doc",
                title: `${t("doc.prefix")} - ${contextMenu.tableName}`,
                connectionId: currentWs.connectionId, database: currentWs.database, table: contextMenu.tableName,
                closable: true,
              });
              setContextMenu(null);
            }}
          >
            <Eye className="h-3.5 w-3.5" /> {t("contextMenu.tableDoc") || "Table Doc"}
          </button>
          <div className="h-px bg-[var(--border-subtle)] my-1" />
          {(["csv", "json", "sql"] as const).map((fmt) => (
            <button
              key={fmt}
              className="w-full px-2.5 py-1 text-xs text-left hover:bg-[var(--sidebar-hover)] text-[var(--fg)] flex items-center gap-2"
              onClick={async () => {
                const tblName = contextMenu.tableName;
                const connId = currentWs.connectionId;
                const db = currentWs.database;
                setContextMenu(null);
                try {
                  const mod = await import("@/lib/wails/services/ExportService");
                  const taskId = await mod.ExportTableStream(connId, db, tblName, fmt);
                  if (!taskId) return; // 用户取消了路径选择
                  console.log(`[流式导出] 任务已启动: taskId=${taskId} format=${fmt}`);
                } catch (e: any) {
                  useUIStore.getState().addToast("error", `导出失败: ${e?.message || e}`);
                  console.error(`[流式导出] 启动失败:`, e);
                }
              }}
            >
              <Download className="h-3.5 w-3.5" /> {t(`contextMenu.export${fmt.toUpperCase()}` as any) || `导出 ${fmt.toUpperCase()}`}
            </button>
          ))}
          <div className="h-px bg-[var(--border-subtle)] my-1" />
          <button
            className="w-full px-2.5 py-1 text-xs text-left hover:bg-[var(--sidebar-hover)] text-[var(--danger)] flex items-center gap-2"
            onClick={() => {
              if (confirm(t("contextMenu.truncateConfirm", { table: contextMenu.tableName }))) {
                import("@/lib/wails/services/DatabaseService").then((mod) => {
                  mod.TruncateTable(currentWs.connectionId, currentWs.database, contextMenu.tableName)
                    .then(() => loadTables(currentWs.connectionId, currentWs.database))
                    .catch((e: any) => alert(t("contextMenu.truncateFailed") + e));
                });
              }
              setContextMenu(null);
            }}
          >
            <Trash2 className="h-3.5 w-3.5" /> {t("contextMenu.truncateTable") || "Truncate"}
          </button>
          <button
            className="w-full px-2.5 py-1 text-xs text-left hover:bg-[var(--sidebar-hover)] text-[var(--danger)] flex items-center gap-2"
            onClick={() => {
              if (confirm(t("contextMenu.dropConfirm", { table: contextMenu.tableName }))) {
                import("@/lib/wails/services/DatabaseService").then((mod) => {
                  mod.DropTable(currentWs.connectionId, currentWs.database, contextMenu.tableName)
                    .then(() => loadTables(currentWs.connectionId, currentWs.database))
                    .catch((e: any) => alert(t("contextMenu.dropFailed") + e));
                });
              }
              setContextMenu(null);
            }}
          >
            <Trash2 className="h-3.5 w-3.5" /> {t("contextMenu.dropTable") || "Drop"}
          </button>
        </div>,
        document.body
      )}
    </div>
  );
}
