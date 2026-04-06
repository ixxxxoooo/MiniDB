import { create } from "zustand";
import { persist } from "zustand/middleware";
import { generateId } from "@/lib/utils";
import type { ColumnMeta } from "@/types/database";

export type TabType = "table" | "query" | "ddl" | "doc";

export interface QueryResultItem {
  columns: ColumnMeta[];
  rows: Record<string, unknown>[];
  total: number;
  duration: number;
  error?: string;
  autoLimited?: boolean;
  sql: string;
}

export interface Tab {
  id: string;
  type: TabType;
  title: string;
  icon?: string;
  connectionId: string;
  database: string;
  table?: string;
  closable: boolean;
  sql?: string;
  dirty?: boolean;
  /** 表页面初始子视图：data / structure / info / doc */
  initialSubView?: "data" | "structure" | "info" | "doc";
  /** 查询视图缓存的结果 */
  queryResults?: QueryResultItem[];
  /** 查询视图当前激活的结果索引 */
  queryActiveIdx?: number;
}

interface TabsStore {
  tabs: Tab[];
  activeTabId: string | null;
  workspaceActiveTab: Record<string, string>;

  addTab: (tab: Omit<Tab, "id">) => string;
  removeTab: (id: string) => void;
  setActiveTab: (id: string) => void;
  updateTab: (id: string, updates: Partial<Tab>) => void;
  closeOtherTabs: (id: string) => void;
  closeAllTabs: (workspaceId?: string) => void;
  switchWorkspace: (workspaceId: string) => void;
}

export const useTabsStore = create<TabsStore>()(
  persist(
    (set, get) => ({
      tabs: [],
      activeTabId: null,
      workspaceActiveTab: {},

      addTab: (tabData) => {
        const id = generateId();
        const tab: Tab = { ...tabData, id };

        // 检查是否已有相同的表标签
        const existing = get().tabs.find(
          (t) =>
            t.type === tabData.type &&
            t.connectionId === tabData.connectionId &&
            t.database === tabData.database &&
            t.table === tabData.table &&
            t.type !== "query"
        );

        const workspaceId = `${tabData.connectionId}:${tabData.database}`;

        if (existing) {
          if (tabData.initialSubView) {
            set((s) => ({
              tabs: s.tabs.map((t) => t.id === existing.id ? { ...t, initialSubView: tabData.initialSubView } : t),
              activeTabId: existing.id,
              workspaceActiveTab: { ...s.workspaceActiveTab, [workspaceId]: existing.id }
            }));
          } else {
            set((s) => ({
              activeTabId: existing.id,
              workspaceActiveTab: { ...s.workspaceActiveTab, [workspaceId]: existing.id }
            }));
          }
          return existing.id;
        }

        set((s) => ({
          tabs: [...s.tabs, tab],
          activeTabId: id,
          workspaceActiveTab: { ...s.workspaceActiveTab, [workspaceId]: id }
        }));
        return id;
      },

      removeTab: (id) =>
        set((s) => {
          const idx = s.tabs.findIndex((t) => t.id === id);
          if (idx === -1) return {};
          const tabToRemove = s.tabs[idx];
          const workspaceId = `${tabToRemove.connectionId}:${tabToRemove.database}`;
          const tabs = s.tabs.filter((t) => t.id !== id);

          let activeTabId = s.activeTabId;
          let newWorkspaceActive = { ...s.workspaceActiveTab };

          if (activeTabId === id) {
            const workspaceTabs = tabs.filter(t => `${t.connectionId}:${t.database}` === workspaceId);
            if (workspaceTabs.length > 0) {
              const oldWsTabs = s.tabs.filter(t => `${t.connectionId}:${t.database}` === workspaceId);
              const wsIdx = oldWsTabs.findIndex(t => t.id === id);
              const newIdx = Math.min(wsIdx, workspaceTabs.length - 1);
              activeTabId = workspaceTabs[newIdx].id;
              newWorkspaceActive[workspaceId] = activeTabId;
            } else {
              activeTabId = null;
              delete newWorkspaceActive[workspaceId];
            }
          } else if (newWorkspaceActive[workspaceId] === id) {
            const workspaceTabs = tabs.filter(t => `${t.connectionId}:${t.database}` === workspaceId);
            if (workspaceTabs.length > 0) {
              newWorkspaceActive[workspaceId] = workspaceTabs[workspaceTabs.length - 1].id;
            } else {
              delete newWorkspaceActive[workspaceId];
            }
          }

          return { tabs, activeTabId, workspaceActiveTab: newWorkspaceActive };
        }),

      setActiveTab: (activeTabId) => set((s) => {
        const tab = s.tabs.find(t => t.id === activeTabId);
        if (!tab) return { activeTabId };
        const workspaceId = `${tab.connectionId}:${tab.database}`;
        return {
          activeTabId,
          workspaceActiveTab: { ...s.workspaceActiveTab, [workspaceId]: activeTabId }
        };
      }),

      updateTab: (id, updates) =>
        set((s) => ({
          tabs: s.tabs.map((t) => (t.id === id ? { ...t, ...updates } : t)),
        })),

      closeOtherTabs: (id) =>
        set((s) => {
          const targetTab = s.tabs.find(t => t.id === id);
          if (!targetTab) return {};
          const workspaceId = `${targetTab.connectionId}:${targetTab.database}`;

          const newTabs = s.tabs.filter((t) => {
             const tWs = `${t.connectionId}:${t.database}`;
             return tWs !== workspaceId || t.id === id || !t.closable;
          });
          return {
            tabs: newTabs,
            activeTabId: id,
            workspaceActiveTab: { ...s.workspaceActiveTab, [workspaceId]: id }
          };
        }),

      closeAllTabs: (workspaceId) => set((s) => {
        if (!workspaceId) {
          return { tabs: s.tabs.filter(t => !t.closable), activeTabId: null, workspaceActiveTab: {} };
        }
        const newTabs = s.tabs.filter((t) => {
           const tWs = `${t.connectionId}:${t.database}`;
           return tWs !== workspaceId || !t.closable;
        });

        let newActiveTabId = s.activeTabId;
        let newWorkspaceActive = { ...s.workspaceActiveTab };

        const activeTab = s.tabs.find(t => t.id === s.activeTabId);
        if (activeTab && `${activeTab.connectionId}:${activeTab.database}` === workspaceId) {
           const remainingWsTabs = newTabs.filter(t => `${t.connectionId}:${t.database}` === workspaceId);
           if (remainingWsTabs.length > 0) {
             newActiveTabId = remainingWsTabs[remainingWsTabs.length - 1].id;
             newWorkspaceActive[workspaceId] = newActiveTabId;
           } else {
             newActiveTabId = null;
             delete newWorkspaceActive[workspaceId];
           }
        } else {
           delete newWorkspaceActive[workspaceId];
        }

        return { tabs: newTabs, activeTabId: newActiveTabId, workspaceActiveTab: newWorkspaceActive };
      }),

      switchWorkspace: (workspaceId) => set((s) => {
         let newActiveTabId = s.workspaceActiveTab[workspaceId] || null;
         if (!newActiveTabId) {
           const wsTabs = s.tabs.filter(t => `${t.connectionId}:${t.database}` === workspaceId);
           if (wsTabs.length > 0) {
             newActiveTabId = wsTabs[0].id;
           }
         }
         return { activeTabId: newActiveTabId };
      }),
    }),
    {
      name: "tableplus-ai-tabs",
      partialize: (state) => ({
        tabs: state.tabs.map((tab) => ({ ...tab, queryResults: undefined })),
        activeTabId: state.activeTabId,
        workspaceActiveTab: state.workspaceActiveTab,
      }),
      onRehydrateStorage: () => (state) => {
        if (!state) return;
        const validTabIds = new Set(state.tabs.map((tab) => tab.id));
        if (state.activeTabId && !validTabIds.has(state.activeTabId)) {
          state.activeTabId = null;
        }
        const nextWorkspaceActive: Record<string, string> = {};
        for (const [workspaceId, tabId] of Object.entries(state.workspaceActiveTab || {})) {
          const tab = state.tabs.find((item) => item.id === tabId);
          if (tab && `${tab.connectionId}:${tab.database}` === workspaceId) {
            nextWorkspaceActive[workspaceId] = tabId;
          }
        }
        state.workspaceActiveTab = nextWorkspaceActive;
      },
    }
  )
);
