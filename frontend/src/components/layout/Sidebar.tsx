import React, { useState } from "react";
import {
  Database,
  Table2,
  ChevronRight,
  ChevronDown,
  Plug,
  PlugZap,
  Circle,
  Plus,
  MoreHorizontal,
} from "lucide-react";
import { cn } from "@/lib/utils";
import { useConnectionStore } from "@/stores/connection";
import { useUIStore } from "@/stores/ui";
import { useTabsStore } from "@/stores/tabs";
import { useDatabase } from "@/hooks/useDatabase";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import type { ConnectionConfig } from "@/types/connection";
import { DRIVER_LABELS } from "@/types/connection";

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
  const { connect, loadTables } = useDatabase();

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
      {/* 侧边栏头部 */}
      <div className="px-4 py-3 flex items-center justify-between border-b border-[var(--sidebar-border)]">
        <span className="text-sm font-semibold text-[var(--fg)]">连接</span>
        <Button variant="ghost" size="icon" className="h-6 w-6" onClick={onNewConnection}>
          <Plus className="h-3.5 w-3.5" />
        </Button>
      </div>

      {/* 连接列表 */}
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
                // 展开连接节点时，自动连接
                if (nodeId.startsWith("conn:") && !expandedNodes.has(nodeId)) {
                  const state = connectionStates[conn.id];
                  if (!state || state.status !== "connected") {
                    connect(conn.id);
                  }
                }
                // 展开数据库节点时，加载表列表
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
            />
          ))
        )}
      </div>
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
