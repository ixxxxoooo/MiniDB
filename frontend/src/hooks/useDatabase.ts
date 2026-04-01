import { useCallback } from "react";
import { useConnectionStore } from "@/stores/connection";
import * as ConnectionService from "../../wailsjs/go/services/ConnectionService";
import * as DatabaseService from "../../wailsjs/go/services/DatabaseService";
import type { ConnectionConfig } from "@/types/connection";

export function useDatabase() {
  const {
    connections,
    setConnections,
    addConnection,
    updateConnection,
    removeConnection,
    setConnectionState,
    setDatabases,
    setTables,
    toggleNode,
    expandedNodes,
    setActiveConnection,
    addWorkspace,
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
        // 判断是新增还是更新
        const existing = connections.find((c) => c.id === conn.id);
        if (existing) {
          updateConnection(conn);
        } else {
          addConnection(conn);
        }
      } catch (e) {
        console.error("保存连接失败:", e);
        throw e;
      }
    },
    [addConnection, updateConnection, connections]
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
          setActiveConnection(id);

          // 并行获取数据库列表和服务器版本
          const [dbs, serverVersion] = await Promise.all([
            DatabaseService.GetDatabases(id),
            DatabaseService.GetServerVersion(id).catch(() => ""),
          ]);
          const dbList = (dbs || []) as any;
          setDatabases(id, dbList);

          const initialDatabase =
            dbList.find((db: { tableCount?: number }) => (db.tableCount || 0) > 0)?.name ||
            dbList[0]?.name ||
            "";

          // 添加默认 Workspace
          addWorkspace(id, initialDatabase);
          setConnectionState(id, { id, status: "connected", databases: [], currentDatabase: initialDatabase, serverVersion: serverVersion || "" });

          // 自动展开连接节点
          const connNodeId = `conn:${id}`;
          if (!expandedNodes.has(connNodeId)) {
            toggleNode(connNodeId);
          }

          // 对每个数据库加载表列表，并自动展开第一个库
          for (let i = 0; i < dbList.length; i++) {
            const db = dbList[i];
            try {
              const tables = await DatabaseService.GetTables(id, db.name);
              setTables(`${id}:${db.name}`, (tables || []) as any);

              // 如果只有一个库（即指定了库），自动展开
              if (dbList.length === 1) {
                const dbNodeId = `db:${id}:${db.name}`;
                if (!expandedNodes.has(dbNodeId)) {
                  toggleNode(dbNodeId);
                }
              }
            } catch (e) {
              console.error(`加载表列表失败: ${db.name}`, e);
            }
          }
        } else {
          const errMsg = Array.isArray(result) ? result[1] : "连接失败";
          setConnectionState(id, { id, status: "error", error: String(errMsg), databases: [], currentDatabase: "" });
        }
      } catch (e: any) {
        setConnectionState(id, { id, status: "error", error: e?.message || "连接失败", databases: [], currentDatabase: "" });
      }
    },
    [setConnectionState, setDatabases, setTables, setActiveConnection, toggleNode, expandedNodes, addWorkspace]
  );

  const disconnect = useCallback(
    async (id: string) => {
      try {
        await ConnectionService.Disconnect(id);
        setConnectionState(id, { id, status: "disconnected", databases: [], currentDatabase: "" });
        // 收起侧边栏节点
        const connNodeId = `conn:${id}`;
        const { expandedNodes, toggleNode: toggle } = useConnectionStore.getState();
        if (expandedNodes.has(connNodeId)) toggle(connNodeId);
        // 清空该连接的数据库和表数据
        setDatabases(id, []);
      } catch (e) {
        console.error("断开连接失败:", e);
      }
    },
    [setConnectionState, setDatabases]
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
