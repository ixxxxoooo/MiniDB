import { create } from "zustand";
import type { ConnectionConfig, ConnectionState } from "@/types/connection";
import type { DatabaseInfo, TableInfo, ColumnInfo } from "@/types/database";

interface ConnectionStore {
  connections: ConnectionConfig[];
  connectionStates: Record<string, ConnectionState>;
  activeConnectionId: string | null;

  // 数据库树结构缓存
  databases: Record<string, DatabaseInfo[]>;
  tables: Record<string, TableInfo[]>;
  expandedNodes: Set<string>;

  setConnections: (connections: ConnectionConfig[]) => void;
  addConnection: (conn: ConnectionConfig) => void;
  updateConnection: (conn: ConnectionConfig) => void;
  removeConnection: (id: string) => void;
  setActiveConnection: (id: string | null) => void;
  setConnectionState: (id: string, state: Partial<ConnectionState>) => void;
  setDatabases: (connId: string, dbs: DatabaseInfo[]) => void;
  setTables: (key: string, tables: TableInfo[]) => void;
  toggleNode: (nodeId: string) => void;
}

export const useConnectionStore = create<ConnectionStore>()((set) => ({
  connections: [],
  connectionStates: {},
  activeConnectionId: null,
  databases: {},
  tables: {},
  expandedNodes: new Set<string>(),

  setConnections: (connections) => set({ connections }),
  addConnection: (conn) =>
    set((s) => ({ connections: [...s.connections, conn] })),
  updateConnection: (conn) =>
    set((s) => ({
      connections: s.connections.map((c) => (c.id === conn.id ? conn : c)),
    })),
  removeConnection: (id) =>
    set((s) => ({
      connections: s.connections.filter((c) => c.id !== id),
    })),
  setActiveConnection: (activeConnectionId) => set({ activeConnectionId }),
  setConnectionState: (id, state) =>
    set((s) => ({
      connectionStates: {
        ...s.connectionStates,
        [id]: { ...s.connectionStates[id], ...state } as ConnectionState,
      },
    })),
  setDatabases: (connId, dbs) =>
    set((s) => ({ databases: { ...s.databases, [connId]: dbs } })),
  setTables: (key, tables) =>
    set((s) => ({ tables: { ...s.tables, [key]: tables } })),
  toggleNode: (nodeId) =>
    set((s) => {
      const next = new Set(s.expandedNodes);
      if (next.has(nodeId)) {
        next.delete(nodeId);
      } else {
        next.add(nodeId);
      }
      return { expandedNodes: next };
    }),
}));
