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
  Upload,
  Trash2,
  Eye,
  ExternalLink,
  Unplug,
} from "lucide-react";
import { cn, copyToClipboard } from "@/lib/utils";
import { useConnectionStore } from "@/stores/connection";
import { useUIStore } from "@/stores/ui";
import { useTabsStore } from "@/stores/tabs";
import { useDatabase } from "@/hooks/useDatabase";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import type { ConnectionConfig } from "@/types/connection";

// 右键菜单的位置和上下文
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
  const { sidebarCollapsed } = useUIStore();
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
  const [contextMenu, setContextMenu] = useState<ContextMenuState | null>(null);
  const menuRef = useRef<HTMLDivElement>(null);

  // 点击外部关闭菜单
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

  if (sidebarCollapsed) {
    return (
      <div
        className={cn(
          "w-12 flex flex-col items-center py-3 gap-2 border-r vibrancy",
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
                "w-8 h-8 rounded-lg flex items-center justify-center transition-colors relative",
                isActive
                  ? "bg-[var(--sidebar-active)]"
                  : "hover:bg-[var(--sidebar-hover)]"
              )}
              title={conn.name}
            >
              <Database className="h-4 w-4 text-[var(--fg-secondary)]" />
              {state?.status === "connected" && (
                <div className="absolute -top-0.5 -right-0.5 w-2 h-2 rounded-full bg-[var(--success)]" />
              )}
            </button>
          );
        })}
        <button
          onClick={onNewConnection}
          className="w-8 h-8 rounded-lg flex items-center justify-center hover:bg-[var(--sidebar-hover)] transition-colors"
        >
          <Plus className="h-4 w-4 text-[var(--fg-muted)]" />
        </button>
      </div>
    );
  }

  return (
    <div
      className={cn(
        "flex flex-col border-r vibrancy overflow-hidden",
        "bg-[var(--sidebar-bg)] border-[var(--sidebar-border)]"
      )}
      style={{ width: 260 }}
    >
      {/* 连接列表（不再需要头部标题栏） */}
      <div className="flex-1 overflow-y-auto py-1">
        {connections.length === 0 ? (
          <div className="px-4 py-8 text-center">
            <Database className="h-8 w-8 mx-auto mb-2 text-[var(--fg-muted)]" />
            <p className="text-sm text-[var(--fg-secondary)]">暂无连接</p>
            <p className="text-xs text-[var(--fg-muted)] mt-1">
              点击 + 添加数据库连接
            </p>
          </div>
        ) : (
          connections.map((conn) => (
            <ConnectionTreeNode
              key={conn.id}
              connection={conn}
              state={connectionStates[conn.id]}
              isActive={activeConnectionId === conn.id}
              databases={databases[conn.id] || []}
              tables={tables}
              expandedNodes={expandedNodes}
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

      {/* 右键菜单（Portal 到 body 防止被遮挡） */}
      {contextMenu && createPortal(
        <div
          ref={menuRef}
          className={cn(
            "fixed z-[100] min-w-[200px] py-1 rounded-lg shadow-lg border animate-fade-in",
            "bg-[var(--surface-elevated)] border-[var(--border-color)]"
          )}
          style={{ left: contextMenu.x, top: contextMenu.y }}
        >
          {contextMenu.type === "connection" && (
            <>
              {connectionStates[contextMenu.connectionId]?.status === "connected" ? (
                <button
                  className="w-full px-3 py-1.5 text-xs text-left hover:bg-[var(--sidebar-hover)] text-[var(--fg)] flex items-center gap-2"
                  onClick={() => {
                    disconnect(contextMenu.connectionId);
                    setContextMenu(null);
                  }}
                >
                  <Unplug className="h-3 w-3" /> 断开连接
                </button>
              ) : (
                <button
                  className="w-full px-3 py-1.5 text-xs text-left hover:bg-[var(--sidebar-hover)] text-[var(--fg)] flex items-center gap-2"
                  onClick={() => {
                    connect(contextMenu.connectionId);
                    setContextMenu(null);
                  }}
                >
                  <PlugZap className="h-3 w-3" /> 连接
                </button>
              )}
              <button
                className="w-full px-3 py-1.5 text-xs text-left hover:bg-[var(--sidebar-hover)] text-[var(--fg)] flex items-center gap-2"
                onClick={() => {
                  const conn = connections.find((c) => c.id === contextMenu.connectionId);
                  if (conn) onEditConnection(conn);
                  setContextMenu(null);
                }}
              >
                <MoreHorizontal className="h-3 w-3" /> 编辑连接
              </button>
            </>
          )}
          {contextMenu.type === "table" && contextMenu.database && contextMenu.tableName && (
            <>
              <button
                className="w-full px-3 py-1.5 text-xs text-left hover:bg-[var(--sidebar-hover)] text-[var(--fg)] flex items-center gap-2"
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
                <ExternalLink className="h-3 w-3" /> 在新标签页打开
              </button>
              <button
                className="w-full px-3 py-1.5 text-xs text-left hover:bg-[var(--sidebar-hover)] text-[var(--fg)] flex items-center gap-2"
                onClick={() => {
                  copyToClipboard(contextMenu.tableName!);
                  setContextMenu(null);
                }}
              >
                <Copy className="h-3 w-3" /> 复制表名
              </button>
              <button
                className="w-full px-3 py-1.5 text-xs text-left hover:bg-[var(--sidebar-hover)] text-[var(--fg)] flex items-center gap-2"
                onClick={() => {
                  addTab({
                    type: "ddl",
                    title: `DDL - ${contextMenu.tableName}`,
                    connectionId: contextMenu.connectionId,
                    database: contextMenu.database!,
                    table: contextMenu.tableName!,
                    closable: true,
                  });
                  setContextMenu(null);
                }}
              >
                <FileCode className="h-3 w-3" /> 查看 DDL
              </button>
              <button
                className="w-full px-3 py-1.5 text-xs text-left hover:bg-[var(--sidebar-hover)] text-[var(--fg)] flex items-center gap-2"
                onClick={() => {
                  addTab({
                    type: "doc",
                    title: `文档 - ${contextMenu.tableName}`,
                    connectionId: contextMenu.connectionId,
                    database: contextMenu.database!,
                    table: contextMenu.tableName!,
                    closable: true,
                  });
                  setContextMenu(null);
                }}
              >
                <Eye className="h-3 w-3" /> 表文档
              </button>
              <div className="h-px bg-[var(--border-subtle)] my-1" />
              <button
                className="w-full px-3 py-1.5 text-xs text-left hover:bg-[var(--sidebar-hover)] text-[var(--fg)] flex items-center gap-2"
                onClick={() => {
                  addTab({
                    type: "query",
                    title: `导出 - ${contextMenu.tableName}`,
                    connectionId: contextMenu.connectionId,
                    database: contextMenu.database!,
                    table: contextMenu.tableName!,
                    closable: true,
                    sql: `SELECT * FROM ${contextMenu.tableName};`,
                  });
                  setContextMenu(null);
                }}
              >
                <Download className="h-3 w-3" /> 导出数据
              </button>
              <div className="h-px bg-[var(--border-subtle)] my-1" />
              <button
                className="w-full px-3 py-1.5 text-xs text-left hover:bg-[var(--sidebar-hover)] text-[var(--danger)] flex items-center gap-2"
                onClick={() => {
                  if (confirm(`确定要 TRUNCATE 表 ${contextMenu.tableName} 吗？此操作不可逆！`)) {
                    import("../../../wailsjs/go/services/DatabaseService").then((mod) => {
                      mod.TruncateTable(contextMenu.connectionId, contextMenu.database!, contextMenu.tableName!)
                        .then(() => loadTables(contextMenu.connectionId, contextMenu.database!))
                        .catch((e: any) => alert("TRUNCATE 失败: " + e));
                    });
                  }
                  setContextMenu(null);
                }}
              >
                <Trash2 className="h-3 w-3" /> TRUNCATE 表
              </button>
              <button
                className="w-full px-3 py-1.5 text-xs text-left hover:bg-[var(--sidebar-hover)] text-[var(--danger)] flex items-center gap-2"
                onClick={() => {
                  if (confirm(`确定要 DROP 表 ${contextMenu.tableName} 吗？此操作不可逆！`)) {
                    import("../../../wailsjs/go/services/DatabaseService").then((mod) => {
                      mod.DropTable(contextMenu.connectionId, contextMenu.database!, contextMenu.tableName!)
                        .then(() => loadTables(contextMenu.connectionId, contextMenu.database!))
                        .catch((e: any) => alert("DROP 失败: " + e));
                    });
                  }
                  setContextMenu(null);
                }}
              >
                <Trash2 className="h-3 w-3" /> DROP 表
              </button>
            </>
          )}
        </div>,
        document.body
      )}
    </div>
  );
}

interface ConnectionTreeNodeProps {
  connection: ConnectionConfig;
  state?: { status: string };
  isActive: boolean;
  databases: { name: string; tableCount: number }[];
  tables: Record<string, { name: string; type: string }[]>;
  expandedNodes: Set<string>;
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
  onSelect,
  onToggle,
  onEdit,
  onOpenTable,
  onContextMenu,
}: ConnectionTreeNodeProps) {
  const connNodeId = `conn:${connection.id}`;
  const isExpanded = expandedNodes.has(connNodeId);
  const isConnected = state?.status === "connected";

  return (
    <div>
      {/* 连接节点 */}
      <div
        className={cn(
          "flex items-center px-2 py-1.5 mx-1 rounded-md cursor-pointer group transition-colors",
          isActive ? "bg-[var(--sidebar-active)]" : "hover:bg-[var(--sidebar-hover)]"
        )}
        onClick={() => {
          onSelect();
          onToggle(connNodeId);
        }}
        onContextMenu={(e) => onContextMenu(e, "connection", connection.id)}
      >
        <span className="flex items-center justify-center w-4 h-4 mr-1">
          {isExpanded ? (
            <ChevronDown className="h-3 w-3 text-[var(--fg-muted)]" />
          ) : (
            <ChevronRight className="h-3 w-3 text-[var(--fg-muted)]" />
          )}
        </span>
        <span
          className="w-2.5 h-2.5 rounded-full mr-2 flex-shrink-0"
          style={{ backgroundColor: connection.color || "#007aff" }}
        />
        <span className="text-sm truncate flex-1 text-[var(--sidebar-fg)]">
          {connection.name}
        </span>
        {isConnected && (
          <PlugZap className="h-3 w-3 text-[var(--success)] flex-shrink-0" />
        )}
        <button
          className="h-5 w-5 flex items-center justify-center opacity-0 group-hover:opacity-100 transition-opacity ml-1"
          onClick={(e) => {
            e.stopPropagation();
            onEdit();
          }}
        >
          <MoreHorizontal className="h-3 w-3 text-[var(--fg-muted)]" />
        </button>
      </div>

      {/* 数据库子节点 */}
      {isExpanded && isConnected && (
        <div className="ml-4">
          {databases.map((db) => {
            const dbNodeId = `db:${connection.id}:${db.name}`;
            const isDbExpanded = expandedNodes.has(dbNodeId);
            const dbTables = tables[`${connection.id}:${db.name}`] || [];

            return (
              <div key={db.name}>
                <div
                  className="flex items-center px-2 py-1 mx-1 rounded-md cursor-pointer hover:bg-[var(--sidebar-hover)] transition-colors"
                  onClick={() => onToggle(dbNodeId)}
                >
                  <span className="flex items-center justify-center w-4 h-4 mr-1">
                    {isDbExpanded ? (
                      <ChevronDown className="h-3 w-3 text-[var(--fg-muted)]" />
                    ) : (
                      <ChevronRight className="h-3 w-3 text-[var(--fg-muted)]" />
                    )}
                  </span>
                  <Database className="h-3.5 w-3.5 mr-1.5 text-[var(--fg-secondary)]" />
                  <span className="text-sm truncate flex-1">{db.name}</span>
                  <Badge variant="secondary" className="ml-1 text-2xs">
                    {db.tableCount}
                  </Badge>
                </div>

                {isDbExpanded && (
                  <div className="ml-5">
                    {dbTables.map((t) => (
                      <div
                        key={t.name}
                        className="flex items-center px-2 py-1 mx-1 rounded-md cursor-pointer hover:bg-[var(--sidebar-hover)] transition-colors"
                        onClick={() => onOpenTable(db.name, t.name)}
                        onContextMenu={(e) => onContextMenu(e, "table", connection.id, db.name, t.name)}
                      >
                        <Table2 className="h-3 w-3 mr-1.5 text-[var(--fg-muted)]" />
                        <span className="text-xs truncate">{t.name}</span>
                      </div>
                    ))}
                    {dbTables.length === 0 && (
                      <div className="px-2 py-1 text-xs text-[var(--fg-muted)]">
                        加载中...
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
        <div className="ml-8 px-2 py-1 text-xs text-[var(--fg-muted)]">
          未连接 — 双击连接
        </div>
      )}
    </div>
  );
}
