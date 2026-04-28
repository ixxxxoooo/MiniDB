import React, { useEffect, useState } from "react";
import { DDLViewer } from "@/components/table/DDLViewer";
import { type Tab } from "@/stores/tabs";
import { useConnectionStore } from "@/stores/connection";
import * as DatabaseService from "@/lib/wails/services/DatabaseService";

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

  return <DDLViewer ddl={ddl} tableName={tab.table || ""} />;
}
