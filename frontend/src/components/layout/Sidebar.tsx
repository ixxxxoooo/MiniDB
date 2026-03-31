import React, { useState, useEffect, useRef, useCallback } from "react";
import { createPortal } from "react-dom";
import {
  Database,
  Table2,
  ChevronRight,
  ChevronDown,
  PlugZap,
  Plus,
  MoreHorizontal,
  Copy,
  FileCode,
  Download,
  Trash2,
  Eye,
  ExternalLink,
  Unplug,
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
import type { ConnectionConfig } from "@/types/connection";

interface ContextMenuState {
  x: number;
  y: number;
  type: "connection" | "table";
  connectionId: string;
  database?: string;
  tableName?: string;
}

interface SidebarProps {
  onNewConnection: () => void;
  onEditConnection: (conn: ConnectionConfig) => void;
}

export function Sidebar({ onNewConnection, onEditConnection }: SidebarProps) {
  const { sidebarCollapsed, sidebarWidth, setSidebarWidth } = useUIStore();
  const resizingRef = useRef(false);
  const {
    connections,
    connectionStates,
    activeConnectionId,
    databases,
    tables,
    expandedNodes,
    setActiveConnection,
    toggleNode,
  } = useConnectionStore();
  const { addTab } = useTabsStore();
  const { connect, disconnect, loadTables } = useDatabase();
  const { t } = useTranslation();
  const [contextMenu, setContextMenu] = useState<ContextMenuState | null>(null);
  const [searchQuery, setSearchQuery] = useState("");
  const searchInputRef = useRef<HTMLInputElement>(null);
  const menuRef = useRef<HTMLDivElement>(null);

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

  const handleDisconnect = useCallback(async (connId: string) => {
    await disconnect(connId);
    setContextMenu(null);
  }, [disconnect]);

  // 获取当前活跃连接和数据库列表
  const activeConn = connections.find((c) => c.id === activeConnectionId);
  const connState = activeConnectionId ? connectionStates[activeConnectionId] : undefined;
  const isConnected = connState?.status === "connected";
  const dbList = activeConnectionId ? databases[activeConnectionId] || [] : [];
  // 判断是否为单数据库模式（只有一个数据库时不显示数据库层级）
  const isSingleDb = dbList.length <= 1;
  const singleDbName = dbList[0]?.name || "";

  // 获取当前数据库的表列表（单数据库模式直接展示）
  const getTablesForDb = (connId: string, dbName: string) => {
    return tables[`${connId}:${dbName}`] || [];
  };

  // 根据搜索过滤表
  const filterTables = (tableList: { name: string; type: string }[]) => {
    if (!searchQuery.trim()) return tableList;
    const q = searchQuery.toLowerCase();
    return tableList.filter((t) => t.name.toLowerCase().includes(q));
  };

  // 折叠模式：只显示图标
  if (sidebarCollapsed) {
    return (
      <div
        className={cn(
          "w-10 flex flex-col items-center py-[var(--size-padding-sm)] gap-[var(--size-gap-sm)] border-r",
          "bg-[var(--sidebar-bg)] border-[var(--sidebar-border)]"
        )}
      >
        {connections.map((conn) => {
          const state = connectionStates[conn.id];
          const isActive = activeConnectionId === conn.id;
          return (
            <button
              key={conn.id}
              onClick={() => setActiveConnection(conn.id)}
              className={cn(
                "h-[var(--size-btn)] w-[var(--size-btn)] rounded-[var(--radius-btn)] flex items-center justify-center transition-colors relative",
                isActive
                  ? "bg-[var(--sidebar-active)]"
                  : "hover:bg-[var(--sidebar-hover)]"
              )}
              title={conn.name}
            >
              <Database className="h-[var(--size-btn-icon-sm)] w-[var(--size-btn-icon-sm)] text-[var(--fg-secondary)]" />
              {state?.status === "connected" && (
                <div className="absolute -top-0.5 -right-0.5 w-1.5 h-1.5 rounded-full bg-[var(--success)]" />
              )}
            </button>
          );
        })}
        <button
          onClick={onNewConnection}
          className="h-[var(--size-btn)] w-[var(--size-btn)] rounded-[var(--radius-btn)] flex items-center justify-center hover:bg-[var(--sidebar-hover)] transition-colors"
        >
          <Plus className="h-[var(--size-btn-icon-sm)] w-[var(--size-btn-icon-sm)] text-[var(--fg-muted)]" />
        </button>
      </div>
    );
  }

  const handleSidebarResizeStart = useCallback((e: React.MouseEvent) => {
    e.preventDefault();
    resizingRef.current = true;
    const startX = e.clientX;
    const startW = sidebarWidth;
    const onMove = (ev: MouseEvent) => {
      if (!resizingRef.current) return;
      const newW = Math.max(180, Math.min(500, startW + ev.clientX - startX));
      setSidebarWidth(newW);
    };
    const onUp = () => {
      resizingRef.current = false;
      document.removeEventListener("mousemove", onMove);
      document.removeEventListener("mouseup", onUp);
    };
    document.addEventListener("mousemove", onMove);
    document.addEventListener("mouseup", onUp);
  }, [sidebarWidth, setSidebarWidth]);

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
      {/* 搜索框 — 参考 TablePlus 顶部搜索 */}
      {isConnected && (
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
      <div className="flex-1 overflow-y-auto py-0.5">
        {connections.length === 0 ? (
          <div className="px-3 py-6 text-center">
            <Database className="h-6 w-6 mx-auto mb-1.5 text-[var(--fg-muted)]" />
            <p className="text-xs text-[var(--fg-secondary)]">{t("sidebar.noConnections")}</p>
            <p className="text-2xs text-[var(--fg-muted)] mt-0.5">
              {t("sidebar.addConnection")}
            </p>
          </div>
        ) : isConnected && activeConnectionId ? (
          /* 已连接状态：根据数据库数量决定布局 */
          isSingleDb ? (
            /* 单数据库模式：直接展示表列表，不显示数据库层级 */
            <SingleDbTableList
              connectionId={activeConnectionId}
              dbName={singleDbName}
              tables={filterTables(getTablesForDb(activeConnectionId, singleDbName))}
              onOpenTable={(table) => {
                addTab({
                  type: "table",
                  title: table,
                  connectionId: activeConnectionId,
                  database: singleDbName,
                  table,
                  closable: true,
                });
              }}
              onContextMenu={(e, table) => {
                e.preventDefault();
                e.stopPropagation();
                setContextMenu({
                  x: e.clientX, y: e.clientY,
                  type: "table", connectionId: activeConnectionId,
                  database: singleDbName, tableName: table,
                });
              }}
            />
          ) : (
            /* 多数据库模式：展示数据库层级 */
            dbList.map((db) => {
              const dbNodeId = `db:${activeConnectionId}:${db.name}`;
              const isDbExpanded = expandedNodes.has(dbNodeId);
              const dbTables = filterTables(getTablesForDb(activeConnectionId, db.name));

              return (
                <div key={db.name}>
                  <div
                    className={cn(
                      "flex items-center h-6 px-2 cursor-pointer group transition-colors",
                      "hover:bg-[var(--sidebar-hover)]"
                    )}
                    onClick={() => {
                      toggleNode(dbNodeId);
                      if (!expandedNodes.has(dbNodeId)) {
                        loadTables(activeConnectionId, db.name);
                      }
                    }}
                  >
                    <span className="flex items-center justify-center w-3.5 h-3.5 mr-0.5 flex-shrink-0">
                      {isDbExpanded ? (
                        <ChevronDown className="h-2.5 w-2.5 text-[var(--fg-muted)]" />
                      ) : (
                        <ChevronRight className="h-2.5 w-2.5 text-[var(--fg-muted)]" />
                      )}
                    </span>
                    <Database className="h-3 w-3 mr-1 text-[var(--fg-secondary)] flex-shrink-0" />
                    <span className="text-xs truncate flex-1">{db.name}</span>
                    <span className="text-2xs text-[var(--fg-muted)] ml-1">{db.tableCount}</span>
                  </div>

                  {isDbExpanded && (
                    <div className="ml-3">
                      {dbTables.map((tbl) => (
                        <div
                          key={tbl.name}
                          className={cn(
                            "flex items-center h-[22px] px-2 cursor-pointer transition-colors",
                            "hover:bg-[var(--sidebar-hover)]"
                          )}
                          onClick={() => {
                            addTab({
                              type: "table",
                              title: tbl.name,
                              connectionId: activeConnectionId,
                              database: db.name,
                              table: tbl.name,
                              closable: true,
                            });
                          }}
                          onContextMenu={(e) => {
                            e.preventDefault();
                            e.stopPropagation();
                            setContextMenu({
                              x: e.clientX, y: e.clientY,
                              type: "table", connectionId: activeConnectionId,
                              database: db.name, tableName: tbl.name,
                            });
                          }}
                        >
                          <Table2 className="h-2.5 w-2.5 mr-1.5 text-[var(--fg-muted)] flex-shrink-0" />
                          <span className="text-xs truncate">{tbl.name}</span>
                        </div>
                      ))}
                      {dbTables.length === 0 && !searchQuery && (
                        <div className="px-2 py-1 text-2xs text-[var(--fg-muted)]">
                          {t("common.loading")}
                        </div>
                      )}
                    </div>
                  )}
                </div>
              );
            })
          )
        ) : (
          /* 未连接状态：展示连接列表 */
          connections.map((conn) => (
            <ConnectionTreeNode
              key={conn.id}
              connection={conn}
              state={connectionStates[conn.id]}
              isActive={activeConnectionId === conn.id}
              databases={databases[conn.id] || []}
              tables={tables}
              expandedNodes={expandedNodes}
              searchQuery={searchQuery}
              onSelect={() => setActiveConnection(conn.id)}
              onToggle={(nodeId) => {
                toggleNode(nodeId);
                if (nodeId.startsWith("conn:") && !expandedNodes.has(nodeId)) {
                  const state = connectionStates[conn.id];
                  if (!state || state.status !== "connected") {
                    connect(conn.id);
                  }
                }
                if (nodeId.startsWith("db:") && !expandedNodes.has(nodeId)) {
                  const parts = nodeId.split(":");
                  const dbName = parts.slice(2).join(":");
                  loadTables(conn.id, dbName);
                }
              }}
              onEdit={() => onEditConnection(conn)}
              onOpenTable={(db, table) => {
                addTab({
                  type: "table",
                  title: table,
                  connectionId: conn.id,
                  database: db,
                  table,
                  closable: true,
                });
              }}
              onContextMenu={(e, type, connId, db, table) => {
                e.preventDefault();
                e.stopPropagation();
                setContextMenu({ x: e.clientX, y: e.clientY, type, connectionId: connId, database: db, tableName: table });
              }}
            />
          ))
        )}
      </div>

      {/* 右键菜单 */}
      {contextMenu && createPortal(
        <div
          ref={menuRef}
          className={cn(
            "fixed z-[100] min-w-[180px] py-0.5 rounded-[var(--radius-menu)] shadow-lg border animate-fade-in",
            "bg-[var(--surface-elevated)] border-[var(--border-color)]"
          )}
          style={{ left: contextMenu.x, top: contextMenu.y }}
        >
          {contextMenu.type === "connection" && (
            <>
              {connectionStates[contextMenu.connectionId]?.status === "connected" ? (
                <button
                  className="w-full px-2.5 py-1 text-xs text-left hover:bg-[var(--sidebar-hover)] text-[var(--fg)] flex items-center gap-1.5"
                  onClick={() => handleDisconnect(contextMenu.connectionId)}
                >
                  <Unplug className="h-3 w-3" /> {t("sidebar.disconnect")}
                </button>
              ) : (
                <button
                  className="w-full px-2.5 py-1 text-xs text-left hover:bg-[var(--sidebar-hover)] text-[var(--fg)] flex items-center gap-1.5"
                  onClick={() => {
                    connect(contextMenu.connectionId);
                    setContextMenu(null);
                  }}
                >
                  <PlugZap className="h-3 w-3" /> {t("sidebar.connect")}
                </button>
              )}
              <button
                className="w-full px-2.5 py-1 text-xs text-left hover:bg-[var(--sidebar-hover)] text-[var(--fg)] flex items-center gap-1.5"
                onClick={() => {
                  const conn = connections.find((c) => c.id === contextMenu.connectionId);
                  if (conn) onEditConnection(conn);
                  setContextMenu(null);
                }}
              >
                <MoreHorizontal className="h-3 w-3" /> {t("sidebar.editConnection")}
              </button>
            </>
          )}
          {contextMenu.type === "table" && contextMenu.database && contextMenu.tableName && (
            <>
              <button
                className="w-full px-2.5 py-1 text-xs text-left hover:bg-[var(--sidebar-hover)] text-[var(--fg)] flex items-center gap-1.5"
                onClick={() => {
                  addTab({
                    type: "table",
                    title: contextMenu.tableName!,
                    connectionId: contextMenu.connectionId,
                    database: contextMenu.database!,
                    table: contextMenu.tableName!,
                    closable: true,
                  });
                  setContextMenu(null);
                }}
              >
                <ExternalLink className="h-3 w-3" /> {t("contextMenu.openInNewTab")}
              </button>
              <button
                className="w-full px-2.5 py-1 text-xs text-left hover:bg-[var(--sidebar-hover)] text-[var(--fg)] flex items-center gap-1.5"
                onClick={() => {
                  copyToClipboard(contextMenu.tableName!);
                  setContextMenu(null);
                }}
              >
                <Copy className="h-3 w-3" /> {t("contextMenu.copyTableName")}
              </button>
              <div className="h-px bg-[var(--border-subtle)] my-0.5" />
              <button
                className="w-full px-2.5 py-1 text-xs text-left hover:bg-[var(--sidebar-hover)] text-[var(--fg)] flex items-center gap-1.5"
                onClick={() => {
                  addTab({
                    type: "table", title: contextMenu.tableName!,
                    connectionId: contextMenu.connectionId,
                    database: contextMenu.database!, table: contextMenu.tableName!,
                    closable: true, initialSubView: "data",
                  });
                  setContextMenu(null);
                }}
              >
                <Table2 className="h-3 w-3" /> {t("contextMenu.viewData")}
              </button>
              <button
                className="w-full px-2.5 py-1 text-xs text-left hover:bg-[var(--sidebar-hover)] text-[var(--fg)] flex items-center gap-1.5"
                onClick={() => {
                  addTab({
                    type: "table", title: contextMenu.tableName!,
                    connectionId: contextMenu.connectionId,
                    database: contextMenu.database!, table: contextMenu.tableName!,
                    closable: true, initialSubView: "structure",
                  });
                  setContextMenu(null);
                }}
              >
                <Columns className="h-3 w-3" /> {t("contextMenu.viewStructure")}
              </button>
              <button
                className="w-full px-2.5 py-1 text-xs text-left hover:bg-[var(--sidebar-hover)] text-[var(--fg)] flex items-center gap-1.5"
                onClick={() => {
                  addTab({
                    type: "table", title: contextMenu.tableName!,
                    connectionId: contextMenu.connectionId,
                    database: contextMenu.database!, table: contextMenu.tableName!,
                    closable: true, initialSubView: "info",
                  });
                  setContextMenu(null);
                }}
              >
                <FileCode className="h-3 w-3" /> {t("contextMenu.viewDDL")}
              </button>
              <button
                className="w-full px-2.5 py-1 text-xs text-left hover:bg-[var(--sidebar-hover)] text-[var(--fg)] flex items-center gap-1.5"
                onClick={() => {
                  addTab({
                    type: "doc",
                    title: `${t("doc.prefix")} - ${contextMenu.tableName}`,
                    connectionId: contextMenu.connectionId,
                    database: contextMenu.database!, table: contextMenu.tableName!,
                    closable: true,
                  });
                  setContextMenu(null);
                }}
              >
                <Eye className="h-3 w-3" /> {t("contextMenu.tableDoc")}
              </button>
              <div className="h-px bg-[var(--border-subtle)] my-0.5" />
              <button
                className="w-full px-2.5 py-1 text-xs text-left hover:bg-[var(--sidebar-hover)] text-[var(--fg)] flex items-center gap-1.5"
                onClick={() => {
                  addTab({
                    type: "query",
                    title: `${t("export.prefix")} - ${contextMenu.tableName}`,
                    connectionId: contextMenu.connectionId,
                    database: contextMenu.database!, table: contextMenu.tableName!,
                    closable: true,
                    sql: `SELECT * FROM ${contextMenu.tableName};`,
                  });
                  setContextMenu(null);
                }}
              >
                <Download className="h-3 w-3" /> {t("contextMenu.exportData")}
              </button>
              <div className="h-px bg-[var(--border-subtle)] my-0.5" />
              <button
                className="w-full px-2.5 py-1 text-xs text-left hover:bg-[var(--sidebar-hover)] text-[var(--danger)] flex items-center gap-1.5"
                onClick={() => {
                  if (confirm(t("contextMenu.truncateConfirm", { table: contextMenu.tableName! }))) {
                    import("../../../wailsjs/go/services/DatabaseService").then((mod) => {
                      mod.TruncateTable(contextMenu.connectionId, contextMenu.database!, contextMenu.tableName!)
                        .then(() => loadTables(contextMenu.connectionId, contextMenu.database!))
                        .catch((e: any) => alert(t("contextMenu.truncateFailed") + e));
                    });
                  }
                  setContextMenu(null);
                }}
              >
                <Trash2 className="h-3 w-3" /> {t("contextMenu.truncateTable")}
              </button>
              <button
                className="w-full px-2.5 py-1 text-xs text-left hover:bg-[var(--sidebar-hover)] text-[var(--danger)] flex items-center gap-1.5"
                onClick={() => {
                  if (confirm(t("contextMenu.dropConfirm", { table: contextMenu.tableName! }))) {
                    import("../../../wailsjs/go/services/DatabaseService").then((mod) => {
                      mod.DropTable(contextMenu.connectionId, contextMenu.database!, contextMenu.tableName!)
                        .then(() => loadTables(contextMenu.connectionId, contextMenu.database!))
                        .catch((e: any) => alert(t("contextMenu.dropFailed") + e));
                    });
                  }
                  setContextMenu(null);
                }}
              >
                <Trash2 className="h-3 w-3" /> {t("contextMenu.dropTable")}
              </button>
            </>
          )}
        </div>,
        document.body
      )}
    </div>
  );
}

/** 单数据库模式的表列表 — 直接展示，无数据库层级 */
function SingleDbTableList({
  connectionId,
  dbName,
  tables: tableList,
  onOpenTable,
  onContextMenu,
}: {
  connectionId: string;
  dbName: string;
  tables: { name: string; type: string }[];
  onOpenTable: (table: string) => void;
  onContextMenu: (e: React.MouseEvent, table: string) => void;
}) {
  const { t } = useTranslation();

  if (tableList.length === 0) {
    return (
      <div className="px-3 py-4 text-center">
        <Table2 className="h-5 w-5 mx-auto mb-1 text-[var(--fg-muted)]" />
        <p className="text-2xs text-[var(--fg-muted)]">{t("common.loading")}</p>
      </div>
    );
  }

  return (
    <div>
      {tableList.map((tbl) => (
        <div
          key={tbl.name}
          className={cn(
            "flex items-center h-[22px] px-2.5 cursor-pointer transition-colors",
            "hover:bg-[var(--sidebar-hover)]"
          )}
          onClick={() => onOpenTable(tbl.name)}
          onContextMenu={(e) => onContextMenu(e, tbl.name)}
        >
          <Table2 className="h-2.5 w-2.5 mr-1.5 text-[var(--fg-muted)] flex-shrink-0" />
          <span className="text-xs truncate">{tbl.name}</span>
        </div>
      ))}
    </div>
  );
}

/** 连接树节点（未连接状态使用） */
interface ConnectionTreeNodeProps {
  connection: ConnectionConfig;
  state?: { status: string };
  isActive: boolean;
  databases: { name: string; tableCount: number }[];
  tables: Record<string, { name: string; type: string }[]>;
  expandedNodes: Set<string>;
  searchQuery: string;
  onSelect: () => void;
  onToggle: (nodeId: string) => void;
  onEdit: () => void;
  onOpenTable: (db: string, table: string) => void;
  onContextMenu: (e: React.MouseEvent, type: "connection" | "table", connId: string, db?: string, table?: string) => void;
}

function ConnectionTreeNode({
  connection,
  state,
  isActive,
  databases,
  tables,
  expandedNodes,
  searchQuery,
  onSelect,
  onToggle,
  onEdit,
  onOpenTable,
  onContextMenu,
}: ConnectionTreeNodeProps) {
  const connNodeId = `conn:${connection.id}`;
  const isExpanded = expandedNodes.has(connNodeId);
  const isConnected = state?.status === "connected";
  const { t } = useTranslation();

  return (
    <div>
      <div
        className={cn(
          "flex items-center h-7 px-2 cursor-pointer group transition-colors",
          isActive ? "bg-[var(--sidebar-active)]" : "hover:bg-[var(--sidebar-hover)]"
        )}
        onClick={() => { onSelect(); onToggle(connNodeId); }}
        onContextMenu={(e) => onContextMenu(e, "connection", connection.id)}
      >
        <span className="flex items-center justify-center w-3.5 h-3.5 mr-0.5 flex-shrink-0">
          {isExpanded ? (
            <ChevronDown className="h-2.5 w-2.5 text-[var(--fg-muted)]" />
          ) : (
            <ChevronRight className="h-2.5 w-2.5 text-[var(--fg-muted)]" />
          )}
        </span>
        <span
          className="w-2 h-2 rounded-full mr-1.5 flex-shrink-0"
          style={{ backgroundColor: connection.color || "#007aff" }}
        />
        <span className="text-xs truncate flex-1 text-[var(--sidebar-fg)]">
          {connection.name}
        </span>
        {isConnected && (
          <PlugZap className="h-2.5 w-2.5 text-[var(--success)] flex-shrink-0" />
        )}
        <button
          className="h-4 w-4 flex items-center justify-center opacity-0 group-hover:opacity-100 transition-opacity ml-0.5"
          onClick={(e) => { e.stopPropagation(); onEdit(); }}
        >
          <MoreHorizontal className="h-2.5 w-2.5 text-[var(--fg-muted)]" />
        </button>
      </div>

      {isExpanded && isConnected && (
        <div className="ml-3">
          {databases.map((db) => {
            const dbNodeId = `db:${connection.id}:${db.name}`;
            const isDbExpanded = expandedNodes.has(dbNodeId);
            const dbTables = tables[`${connection.id}:${db.name}`] || [];
            const filteredTables = searchQuery
              ? dbTables.filter((t) => t.name.toLowerCase().includes(searchQuery.toLowerCase()))
              : dbTables;

            return (
              <div key={db.name}>
                <div
                  className="flex items-center h-6 px-2 cursor-pointer hover:bg-[var(--sidebar-hover)] transition-colors"
                  onClick={() => onToggle(dbNodeId)}
                >
                  <span className="flex items-center justify-center w-3.5 h-3.5 mr-0.5 flex-shrink-0">
                    {isDbExpanded ? (
                      <ChevronDown className="h-2.5 w-2.5 text-[var(--fg-muted)]" />
                    ) : (
                      <ChevronRight className="h-2.5 w-2.5 text-[var(--fg-muted)]" />
                    )}
                  </span>
                  <Database className="h-3 w-3 mr-1 text-[var(--fg-secondary)] flex-shrink-0" />
                  <span className="text-xs truncate flex-1">{db.name}</span>
                  <span className="text-2xs text-[var(--fg-muted)] ml-1">{db.tableCount}</span>
                </div>

                {isDbExpanded && (
                  <div className="ml-4">
                    {filteredTables.map((tbl) => (
                      <div
                        key={tbl.name}
                        className="flex items-center h-[22px] px-2 cursor-pointer hover:bg-[var(--sidebar-hover)] transition-colors"
                        onClick={() => onOpenTable(db.name, tbl.name)}
                        onContextMenu={(e) => onContextMenu(e, "table", connection.id, db.name, tbl.name)}
                      >
                        <Table2 className="h-2.5 w-2.5 mr-1.5 text-[var(--fg-muted)] flex-shrink-0" />
                        <span className="text-xs truncate">{tbl.name}</span>
                      </div>
                    ))}
                    {filteredTables.length === 0 && !searchQuery && (
                      <div className="px-2 py-1 text-2xs text-[var(--fg-muted)]">
                        {t("common.loading")}
                      </div>
                    )}
                  </div>
                )}
              </div>
            );
          })}
        </div>
      )}

      {isExpanded && !isConnected && (
        <div className="ml-6 px-2 py-1 text-2xs text-[var(--fg-muted)]">
          {t("sidebar.notConnected")}
        </div>
      )}
    </div>
  );
}
