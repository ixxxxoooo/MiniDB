import { useState, useCallback, useRef } from "react";
import * as AIService from "@/lib/wails/services/AIService";

// withLoading 包装：并发多个 AI 操作时，loading 保持 true 直到全部完成
function useLoadingTracker() {
  const [loading, setLoading] = useState(false);
  const pendingRef = useRef(0);

  const begin = useCallback(() => {
    pendingRef.current += 1;
    setLoading(true);
  }, []);

  const end = useCallback(() => {
    pendingRef.current = Math.max(0, pendingRef.current - 1);
    if (pendingRef.current === 0) {
      setLoading(false);
    }
  }, []);

  return { loading, begin, end };
}

export function useAI(connId: string, dbName: string) {
  const { loading, begin, end } = useLoadingTracker();

  const nl2sql = useCallback(
    async (prompt: string) => {
      begin();
      try {
        return await AIService.NaturalLanguageToSQL(connId, dbName, prompt);
      } catch (e: any) {
        throw new Error(e?.message || "AI 请求失败");
      } finally {
        end();
      }
    },
    [connId, dbName, begin, end]
  );

  const explainSQL = useCallback(
    async (sql: string) => {
      begin();
      try {
        return await AIService.ExplainSQL(sql);
      } catch (e: any) {
        throw new Error(e?.message || "AI 请求失败");
      } finally {
        end();
      }
    },
    [begin, end]
  );

  const analyzeData = useCallback(
    async (columns: string[], rows: Record<string, any>[], question: string) => {
      begin();
      try {
        return await AIService.AnalyzeData(columns, rows, question);
      } catch (e: any) {
        throw new Error(e?.message || "AI 请求失败");
      } finally {
        end();
      }
    },
    [begin, end]
  );

  const generateDoc = useCallback(
    async (tableName: string) => {
      begin();
      try {
        return await AIService.GenerateTableDoc(connId, dbName, tableName);
      } catch (e: any) {
        throw new Error(e?.message || "AI 请求失败");
      } finally {
        end();
      }
    },
    [connId, dbName, begin, end]
  );

  const diagnoseError = useCallback(
    async (sql: string, errorMsg: string) => {
      begin();
      try {
        return await AIService.DiagnoseError(sql, errorMsg);
      } catch (e: any) {
        throw new Error(e?.message || "AI 请求失败");
      } finally {
        end();
      }
    },
    [begin, end]
  );

  return { loading, nl2sql, explainSQL, analyzeData, generateDoc, diagnoseError };
}