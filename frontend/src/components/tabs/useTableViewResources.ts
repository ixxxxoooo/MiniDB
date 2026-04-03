import { useCallback, useEffect, useState } from "react";
import type { ColumnInfo } from "@/types/database";
import * as DatabaseService from "../../../wailsjs/go/services/DatabaseService";
import * as DocService from "../../../wailsjs/go/services/DocService";
import { reportTabError } from "./tabFeedback";

// 表视图附属资源加载：结构、DDL、文档
export function useTableViewResources(params: {
  connectionId?: string;
  database?: string;
  table?: string;
}) {
  const { connectionId, database, table } = params;
  const [structureColumns, setStructureColumns] = useState<ColumnInfo[]>([]);
  const [indexes, setIndexes] = useState<any[]>([]);
  const [ddl, setDDL] = useState("");
  const [docContent, setDocContent] = useState("");
  const [structureLoaded, setStructureLoaded] = useState(false);
  const [ddlLoaded, setDDLLoaded] = useState(false);
  const [docLoaded, setDocLoaded] = useState(false);

  const loadStructure = useCallback(async (force = false) => {
    if (!connectionId || !database || !table) return;
    if (!force && structureLoaded) return;
    try {
      const [cols, idxs] = await Promise.all([
        DatabaseService.GetColumns(connectionId, database, table),
        DatabaseService.GetIndexes(connectionId, database, table),
      ]);
      setStructureColumns((cols || []) as unknown as ColumnInfo[]);
      setIndexes(idxs || []);
      setStructureLoaded(true);
    } catch (e) {
      reportTabError({
        logTitle: "[TableView] 加载表结构失败:",
        toastMessage: "加载表结构失败",
        error: e,
      });
    }
  }, [connectionId, database, table, structureLoaded]);

  const loadDDL = useCallback(async (force = false) => {
    if (!connectionId || !database || !table) return;
    if (!force && ddlLoaded) return;
    try {
      const result = await DatabaseService.GetDDL(connectionId, database, table);
      setDDL(result || "");
      setDDLLoaded(true);
    } catch {
      setDDL("-- 获取 DDL 失败");
    }
  }, [connectionId, database, table, ddlLoaded]);

  const loadDoc = useCallback(async (force = false) => {
    if (!connectionId || !database || !table) return;
    if (!force && docLoaded) return;
    try {
      const doc = await DocService.GetTableDoc(connectionId, database, table);
      setDocContent(doc || "");
      setDocLoaded(true);
    } catch {
      setDocContent("");
    }
  }, [connectionId, database, table, docLoaded]);

  useEffect(() => {
    setStructureLoaded(false);
    setDDLLoaded(false);
    setDocLoaded(false);
    setStructureColumns([]);
    setIndexes([]);
    setDDL("");
    setDocContent("");
  }, [connectionId, database, table]);

  return {
    structureColumns,
    indexes,
    ddl,
    docContent,
    loadStructure,
    loadDDL,
    loadDoc,
    setDocContent,
  };
}
