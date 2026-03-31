import React from "react";
import { cn } from "@/lib/utils";
import { useConnectionStore } from "@/stores/connection";
import { useTabsStore } from "@/stores/tabs";
import { Database, Table2, Clock, Zap } from "lucide-react";

interface StatusBarProps {
  queryDuration?: number;
  rowCount?: number;
}

export function StatusBar({ queryDuration, rowCount }: StatusBarProps) {
  const { activeConnectionId, connections, connectionStates, databases } =
    useConnectionStore();
  const { activeTabId, tabs } = useTabsStore();

  const activeConn = connections.find((c) => c.id === activeConnectionId);
  const connState = activeConnectionId
    ? connectionStates[activeConnectionId]
    : undefined;
  const activeTab = tabs.find((t) => t.id === activeTabId);

  const dbList = activeConnectionId ? databases[activeConnectionId] : [];
  const totalTables = dbList?.reduce((sum, db) => sum + db.tableCount, 0) || 0;

  return (
    <div
      className={cn(
        "h-6 flex items-center px-3 text-2xs border-t select-none gap-4",
        "bg-[var(--surface-secondary)] border-[var(--border-color)] text-[var(--fg-secondary)]"
      )}
    >
      {/* 连接状态 */}
      {activeConn && (
        <div className="flex items-center gap-1">
          <div
            className={cn("w-1.5 h-1.5 rounded-full", {
              "bg-[var(--success)]": connState?.status === "connected",
              "bg-[var(--warning)]": connState?.status === "connecting",
              "bg-[var(--fg-muted)]":
                !connState || connState.status === "disconnected",
              "bg-[var(--danger)]": connState?.status === "error",
            })}
          />
          <span>{activeConn.name}</span>
        </div>
      )}

      {/* 数据库信息 */}
      {activeTab?.database && (
        <div className="flex items-center gap-1">
          <Database className="h-3 w-3" />
          <span>{activeTab.database}</span>
        </div>
      )}

      {/* 表数量 */}
      {totalTables > 0 && (
        <div className="flex items-center gap-1">
          <Table2 className="h-3 w-3" />
          <span>{totalTables} 张表</span>
        </div>
      )}

      <div className="flex-1" />

      {/* 查询结果统计 */}
      {rowCount !== undefined && (
        <div className="flex items-center gap-1">
          <span>{rowCount} 行</span>
        </div>
      )}

      {queryDuration !== undefined && (
        <div className="flex items-center gap-1">
          <Clock className="h-3 w-3" />
          <span>
            {queryDuration < 1000
              ? `${Math.round(queryDuration)}ms`
              : `${(queryDuration / 1000).toFixed(2)}s`}
          </span>
        </div>
      )}
    </div>
  );
}
