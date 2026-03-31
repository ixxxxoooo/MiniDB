import { useCallback } from "react";
import { useConnectionStore } from "@/stores/connection";
import * as ConnectionService from "../../wailsjs/go/services/ConnectionService";
import * as DatabaseService from "../../wailsjs/go/services/DatabaseService";
import type { ConnectionConfig } from "@/types/connection";

export function useDatabase() {
  const {
    setConnections,
    addConnection,
    updateConnection,
    removeConnection,
    setConnectionState,
    setDatabases,
    setTables,
  } = useConnectionStore();

  const loadConnections = useCallback(async () => {
    try {
      const conns = await ConnectionService.GetConnections();
      setConnections((conns || []) as any as ConnectionConfig[]);
    } catch (e) {
      console.error("加载连接列表失败:", e);
    }
  }, [setConnections]);

  const saveConnection = useCallback(
    async (conn: ConnectionConfig) => {
      try {
        await ConnectionService.SaveConnection(conn as any);
        addConnection(conn);
      } catch (e) {
        console.error("保存连接失败:", e);
        throw e;
      }
    },
    [addConnection]
  );

  const deleteConnection = useCallback(
    async (id: string) => {
      try {
        await ConnectionService.DeleteConnection(id);
        removeConnection(id);
      } catch (e) {
        console.error("删除连接失败:", e);
      }
    },
    [removeConnection]
  );

  const testConnection = useCallback(
    async (conn: ConnectionConfig): Promise<boolean> => {
      try {
        const result = await ConnectionService.TestConnection(conn as any);
        // result 可能是 [boolean, string]
        if (Array.isArray(result)) {
          return result[0] as boolean;
        }
        return !!result;
      } catch (e) {
        console.error("测试连接失败:", e);
        return false;
      }
    },
    []
  );

  const connect = useCallback(
    async (id: string) => {
      setConnectionState(id, { id, status: "connecting", databases: [], currentDatabase: "" });
      try {
        const result = await ConnectionService.Connect(id);
        const success = Array.isArray(result) ? result[0] : !!result;
        if (success) {
          setConnectionState(id, { id, status: "connected", databases: [], currentDatabase: "" });
          // 加载数据库列表
          const dbs = await DatabaseService.GetDatabases(id);
          setDatabases(id, (dbs || []) as any);

          // 对每个数据库加载表列表
          for (const db of dbs || []) {
            try {
              const tables = await DatabaseService.GetTables(id, db.name);
              setTables(`${id}:${db.name}`, (tables || []) as any);
            } catch {}
          }
        } else {
          const errMsg = Array.isArray(result) ? result[1] : "连接失败";
          setConnectionState(id, { id, status: "error", error: String(errMsg), databases: [], currentDatabase: "" });
        }
      } catch (e: any) {
        setConnectionState(id, { id, status: "error", error: e?.message || "连接失败", databases: [], currentDatabase: "" });
      }
    },
    [setConnectionState, setDatabases, setTables]
  );

  const disconnect = useCallback(
    async (id: string) => {
      try {
        await ConnectionService.Disconnect(id);
        setConnectionState(id, { id, status: "disconnected", databases: [], currentDatabase: "" });
      } catch (e) {
        console.error("断开连接失败:", e);
      }
    },
    [setConnectionState]
  );

  const loadTables = useCallback(
    async (connId: string, dbName: string) => {
      try {
        const tables = await DatabaseService.GetTables(connId, dbName);
        setTables(`${connId}:${dbName}`, (tables || []) as any);
      } catch (e) {
        console.error("加载表列表失败:", e);
      }
    },
    [setTables]
  );

  return {
    loadConnections,
    saveConnection,
    deleteConnection,
    testConnection,
    connect,
    disconnect,
    loadTables,
  };
}
