import React, { useEffect, useState } from "react";
import { DDLViewer } from "@/components/table/DDLViewer";
import { type Tab } from "@/stores/tabs";
import * as DatabaseService from "../../../wailsjs/go/services/DatabaseService";

export function DDLView({ tab }: { tab: Tab }) {
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
