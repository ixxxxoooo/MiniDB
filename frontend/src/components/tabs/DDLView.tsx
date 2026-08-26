import React, { useEffect, useState, lazy, Suspense } from "react";
import { type Tab } from "@/stores/tabs";
import { useConnectionStore } from "@/stores/connection";
import * as DatabaseService from "@/lib/wails/services/DatabaseService";
import { LazyLoadingPlaceholder } from "@/components/ui/LazyLoadingPlaceholder";

// DDLViewer 依赖 monaco（体积大），按需懒加载
const DDLViewer = lazy(() =>
  import("@/components/table/DDLViewer").then((m) => ({ default: m.DDLViewer }))
);

export function DDLView({ tab, isActive = true }: { tab: Tab; isActive?: boolean }) {
  const [ddl, setDDL] = useState("");
  const isConnectionReady = useConnectionStore(
    (s) => s.connectionStates[tab.connectionId || ""]?.status === "connected"
  );

  useEffect(() => {
    if (!isActive || !isConnectionReady) return;
    if (tab.connectionId && tab.database && tab.table) {
      DatabaseService.GetDDL(tab.connectionId, tab.database, tab.table)
        .then(setDDL)
        .catch(() => setDDL("-- 获取 DDL 失败"));
    }
  }, [isActive, isConnectionReady, tab.connectionId, tab.database, tab.table]);

  return (
    <Suspense fallback={<LazyLoadingPlaceholder />}>
      <DDLViewer ddl={ddl} tableName={tab.table || ""} />
    </Suspense>
  );
}