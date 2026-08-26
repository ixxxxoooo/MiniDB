import { useState, useCallback, useRef } from "react";
import * as QueryService from "@/lib/wails/services/QueryService";
import * as DatabaseService from "@/lib/wails/services/DatabaseService";
import type { QueryResult, ColumnMeta } from "@/types/database";

export function useQuery(connId: string, dbName: string) {
  const [loading, setLoading] = useState(false);
  const [result, setResult] = useState<QueryResult | null>(null);
  const [error, setError] = useState<string | null>(null);

  // 请求序号：快速连续执行时只保留最后一次请求的结果，避免旧响应覆盖新结果
  const requestSeqRef = useRef(0);

  const executeSQL = useCallback(
    async (sql: string) => {
      const seq = ++requestSeqRef.current;
      setLoading(true);
      setError(null);
      try {
        const res = await QueryService.ExecuteSQL(connId, dbName, sql);
        if (seq !== requestSeqRef.current) return null; // 过期响应
        if (res.error) {
          setError(res.error);
        }
        setResult(res);
        return res;
      } catch (e: any) {
        if (seq !== requestSeqRef.current) return null;
        setError(e?.message || "查询执行失败");
        return null;
      } finally {
        if (seq === requestSeqRef.current) {
          setLoading(false);
        }
      }
    },
    [connId, dbName]
  );

  const queryTableData = useCallback(
    async (table: string, page: number, pageSize: number) => {
      const seq = ++requestSeqRef.current;
      setLoading(true);
      setError(null);
      try {
        const res = await QueryService.QueryTableData(
          connId, dbName, table, page, pageSize, [], []
        );
        if (seq !== requestSeqRef.current) return null;
        if (res.error) {
          setError(res.error);
        }
        setResult(res);
        return res;
      } catch (e: any) {
        if (seq !== requestSeqRef.current) return null;
        setError(e?.message || "查询执行失败");
        return null;
      } finally {
        if (seq === requestSeqRef.current) {
          setLoading(false);
        }
      }
    },
    [connId, dbName]
  );

  const getDDL = useCallback(
    async (table: string) => {
      try {
        return await DatabaseService.GetDDL(connId, dbName, table);
      } catch (e: any) {
        return `-- 获取 DDL 失败: ${e?.message || "未知错误"}`;
      }
    },
    [connId, dbName]
  );

  const getColumns = useCallback(
    async (table: string) => {
      try {
        return await DatabaseService.GetColumns(connId, dbName, table);
      } catch {
        return [];
      }
    },
    [connId, dbName]
  );

  const deleteRow = useCallback(
    async (table: string, primaryKey: Record<string, any>) => {
      try {
        await QueryService.DeleteRow(connId, dbName, table, primaryKey);
        return true;
      } catch (e) {
        console.error("删除行失败:", e);
        return false;
      }
    },
    [connId, dbName]
  );

  const generateInsertSQL = useCallback(
    async (table: string, row: Record<string, any>) => {
      try {
        return await QueryService.GenerateInsertSQL(table, row);
      } catch {
        return "";
      }
    },
    []
  );

  return {
    loading,
    result,
    error,
    executeSQL,
    queryTableData,
    getDDL,
    getColumns,
    deleteRow,
    generateInsertSQL,
  };
}
