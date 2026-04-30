import { create } from "zustand";
import { persist } from "zustand/middleware";
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

  // 多工作区机制（用于替代原有的左侧树根节点切换）
  workspaces: { id: string; connectionId: string; database: string }[];
  activeWorkspaceId: string | null;

  setConnections: (connections: ConnectionConfig[]) => void;
  addConnection: (conn: ConnectionConfig) => void;
  updateConnection: (conn: ConnectionConfig) => void;
  removeConnection: (id: string) => void;
  setActiveConnection: (id: string | null) => void;
  setConnectionState: (id: string, state: Partial<ConnectionState>) => void;
  setDatabases: (connId: string, dbs: DatabaseInfo[]) => void;
  setTables: (key: string, tables: TableInfo[]) => void;
  toggleNode: (nodeId: string) => void;

  addWorkspace: (connectionId: string, database: string) => void;
  removeWorkspace: (id: string) => void;
  setActiveWorkspace: (id: string | null) => void;
}

export const useConnectionStore = create<ConnectionStore>()(
  persist(
    (set) => ({
      connections: [],
      connectionStates: {},
      activeConnectionId: null,
      databases: {},
      tables: {},
      expandedNodes: new Set<string>(),

      workspaces: [],
      activeWorkspaceId: null,

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

      addWorkspace: (connectionId, database) =>
        set((s) => {
          const id = `${connectionId}:${database}`;
          const nextConnectionStates = s.connectionStates[connectionId]
            ? {
                ...s.connectionStates,
                [connectionId]: {
                  ...s.connectionStates[connectionId],
                  currentDatabase: database,
                },
              }
            : s.connectionStates;
          const existing = s.workspaces.find((w) => w.id === id);
          if (existing) {
            return {
              activeWorkspaceId: id,
              activeConnectionId: connectionId,
              connectionStates: nextConnectionStates,
            };
          }
          const newWorkspace = { id, connectionId, database };
          return {
            workspaces: [...s.workspaces, newWorkspace],
            activeWorkspaceId: id,
            activeConnectionId: connectionId,
            connectionStates: nextConnectionStates,
          };
        }),
      removeWorkspace: (id) =>
        set((s) => {
          const newWs = s.workspaces.filter((w) => w.id !== id);
          let nextActive = s.activeWorkspaceId;
          if (nextActive === id) {
            if (newWs.length > 0) {
              nextActive = newWs[newWs.length - 1].id;
            } else {
              nextActive = null;
            }
          }
          const nextWs = nextActive ? newWs.find((w) => w.id === nextActive) : undefined;
          const nextConnectionStates = nextWs && s.connectionStates[nextWs.connectionId]
            ? {
                ...s.connectionStates,
                [nextWs.connectionId]: {
                  ...s.connectionStates[nextWs.connectionId],
                  currentDatabase: nextWs.database,
                },
              }
            : s.connectionStates;
          return {
            workspaces: newWs,
            activeWorkspaceId: nextActive,
            activeConnectionId: nextActive ? newWs.find((w) => w.id === nextActive)?.connectionId || null : null,
            connectionStates: nextConnectionStates,
          };
        }),
      setActiveWorkspace: (id) =>
        set((s) => {
          const ws = s.workspaces.find((w) => w.id === id);
          const nextConnectionStates = ws && s.connectionStates[ws.connectionId]
            ? {
                ...s.connectionStates,
                [ws.connectionId]: {
                  ...s.connectionStates[ws.connectionId],
                  currentDatabase: ws.database,
                },
              }
            : s.connectionStates;
          return {
            activeWorkspaceId: id,
            activeConnectionId: ws ? ws.connectionId : s.activeConnectionId,
            connectionStates: nextConnectionStates,
          };
        }),
    }),
    {
      name: "tableplus-ai-connection",
      partialize: (state) => ({
        activeConnectionId: state.activeConnectionId,
        workspaces: state.workspaces,
        activeWorkspaceId: state.activeWorkspaceId,
      }),
      onRehydrateStorage: () => (state) => {
        if (!state) return;
        if (state.activeWorkspaceId && !state.workspaces.some((w) => w.id === state.activeWorkspaceId)) {
          state.activeWorkspaceId = state.workspaces.length > 0 ? state.workspaces[state.workspaces.length - 1].id : null;
        }
        if (state.activeWorkspaceId) {
          const ws = state.workspaces.find((w) => w.id === state.activeWorkspaceId);
          if (ws) {
            state.activeConnectionId = ws.connectionId;
          }
        }
      },
    }
  )
);
