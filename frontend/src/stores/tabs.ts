import { create } from "zustand";
import { generateId } from "@/lib/utils";
import type { ColumnMeta } from "@/types/database";

export type TabType = "table" | "query" | "ddl" | "doc";

export interface QueryResultItem {
  columns: ColumnMeta[];
  rows: Record<string, unknown>[];
  total: number;
  duration: number;
  error?: string;
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
  /** 表页面初始子视图：data / structure / info */
  initialSubView?: "data" | "structure" | "info";
  /** 查询视图缓存的结果 */
  queryResults?: QueryResultItem[];
  /** 查询视图当前激活的结果索引 */
  queryActiveIdx?: number;
}

interface TabsStore {
  tabs: Tab[];
  activeTabId: string | null;

  addTab: (tab: Omit<Tab, "id">) => string;
  removeTab: (id: string) => void;
  setActiveTab: (id: string) => void;
  updateTab: (id: string, updates: Partial<Tab>) => void;
  closeOtherTabs: (id: string) => void;
  closeAllTabs: () => void;
}

export const useTabsStore = create<TabsStore>()((set, get) => ({
  tabs: [],
  activeTabId: null,

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

    if (existing) {
      // 如果指定了子视图，更新已有标签的子视图
      if (tabData.initialSubView) {
        set((s) => ({
          tabs: s.tabs.map((t) => t.id === existing.id ? { ...t, initialSubView: tabData.initialSubView } : t),
          activeTabId: existing.id,
        }));
      } else {
        set({ activeTabId: existing.id });
      }
      return existing.id;
    }

    set((s) => ({
      tabs: [...s.tabs, tab],
      activeTabId: id,
    }));
    return id;
  },

  removeTab: (id) =>
    set((s) => {
      const idx = s.tabs.findIndex((t) => t.id === id);
      const tabs = s.tabs.filter((t) => t.id !== id);
      let activeTabId = s.activeTabId;

      if (activeTabId === id) {
        if (tabs.length > 0) {
          const newIdx = Math.min(idx, tabs.length - 1);
          activeTabId = tabs[newIdx].id;
        } else {
          activeTabId = null;
        }
      }

      return { tabs, activeTabId };
    }),

  setActiveTab: (activeTabId) => set({ activeTabId }),

  updateTab: (id, updates) =>
    set((s) => ({
      tabs: s.tabs.map((t) => (t.id === id ? { ...t, ...updates } : t)),
    })),

  closeOtherTabs: (id) =>
    set((s) => ({
      tabs: s.tabs.filter((t) => t.id === id || !t.closable),
      activeTabId: id,
    })),

  closeAllTabs: () => set({ tabs: [], activeTabId: null }),
}));
