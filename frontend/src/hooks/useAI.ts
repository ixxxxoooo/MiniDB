import { useState, useCallback } from "react";
import * as AIService from "../../wailsjs/go/services/AIService";

export function useAI(connId: string, dbName: string) {
  const [loading, setLoading] = useState(false);

  const nl2sql = useCallback(
    async (prompt: string) => {
      setLoading(true);
      try {
        const result = await AIService.NaturalLanguageToSQL(connId, dbName, prompt);
        return result;
      } catch (e: any) {
        throw new Error(e?.message || "AI 请求失败");
      } finally {
        setLoading(false);
      }
    },
    [connId, dbName]
  );

  const explainSQL = useCallback(async (sql: string) => {
    setLoading(true);
    try {
      return await AIService.ExplainSQL(sql);
    } catch (e: any) {
      throw new Error(e?.message || "AI 请求失败");
    } finally {
      setLoading(false);
    }
  }, []);

  const analyzeData = useCallback(
    async (columns: string[], rows: Record<string, any>[], question: string) => {
      setLoading(true);
      try {
        return await AIService.AnalyzeData(columns, rows, question);
      } catch (e: any) {
        throw new Error(e?.message || "AI 请求失败");
      } finally {
        setLoading(false);
      }
    },
    []
  );

  const generateDoc = useCallback(
    async (tableName: string) => {
      setLoading(true);
      try {
        return await AIService.GenerateTableDoc(connId, dbName, tableName);
      } catch (e: any) {
        throw new Error(e?.message || "AI 请求失败");
      } finally {
        setLoading(false);
      }
    },
    [connId, dbName]
  );

  const diagnoseError = useCallback(async (sql: string, errorMsg: string) => {
    setLoading(true);
    try {
      return await AIService.DiagnoseError(sql, errorMsg);
    } catch (e: any) {
      throw new Error(e?.message || "AI 请求失败");
    } finally {
      setLoading(false);
    }
  }, []);

  return { loading, nl2sql, explainSQL, analyzeData, generateDoc, diagnoseError };
}
