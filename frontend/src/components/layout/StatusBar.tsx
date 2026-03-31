import React from "react";
import { cn } from "@/lib/utils";
import { useConnectionStore } from "@/stores/connection";
import { useTabsStore } from "@/stores/tabs";
import { useTranslation } from "@/i18n";
import { Database, Table2, Clock, ChevronDown } from "lucide-react";

interface StatusBarProps {
  queryDuration?: number;
  rowCount?: number;
  onSwitchDatabase?: () => void;
}

export function StatusBar({ queryDuration, rowCount, onSwitchDatabase }: StatusBarProps) {
  const { activeConnectionId, connections, connectionStates, databases } =
    useConnectionStore();
  const { activeTabId, tabs } = useTabsStore();
  const { t } = useTranslation();

  const activeConn = connections.find((c) => c.id === activeConnectionId);
  const connState = activeConnectionId
    ? connectionStates[activeConnectionId]
    : undefined;
  const activeTab = tabs.find((tab) => tab.id === activeTabId);

  const dbList = activeConnectionId ? databases[activeConnectionId] : [];
  const totalTables = dbList?.reduce((sum, db) => sum + db.tableCount, 0) || 0;

  return (
    <div
      className={cn(
        "h-5 flex items-center px-2 text-2xs border-t select-none gap-3",
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
          <span className="text-2xs">{activeConn.name}</span>
        </div>
      )}

      {/* 数据库信息 */}
      {activeTab?.database && (
        <button
          className="flex items-center gap-0.5 hover:text-[var(--fg)] transition-colors"
          onClick={onSwitchDatabase}
          title={t("statusBar.switchDatabase")}
        >
          <Database className="h-2.5 w-2.5" />
          <span className="text-2xs">{activeTab.database}</span>
          {onSwitchDatabase && <ChevronDown className="h-2 w-2" />}
        </button>
      )}

      {/* 表数量 */}
      {totalTables > 0 && (
        <div className="flex items-center gap-0.5">
          <Table2 className="h-2.5 w-2.5" />
          <span className="text-2xs">{totalTables} {t("statusBar.tables")}</span>
        </div>
      )}

      <div className="flex-1" />

      {/* 查询结果统计 */}
      {rowCount !== undefined && (
        <span className="text-2xs">{rowCount} {t("statusBar.rows")}</span>
      )}

      {queryDuration !== undefined && (
        <div className="flex items-center gap-0.5">
          <Clock className="h-2.5 w-2.5" />
          <span className="text-2xs">
            {queryDuration < 1000
              ? `${Math.round(queryDuration)}ms`
              : `${(queryDuration / 1000).toFixed(2)}s`}
          </span>
        </div>
      )}
    </div>
  );
}
