import { create } from "zustand";
import { persist } from "zustand/middleware";

export interface SQLHistoryItem {
  id: string;
  sql: string;
  database: string;
  connectionId: string;
  executedAt: string;
  favorite: boolean;
}

interface SQLHistoryState {
  history: SQLHistoryItem[];
  /** 添加一条执行记录 */
  addHistory: (item: Omit<SQLHistoryItem, "id" | "executedAt" | "favorite">) => void;
  /** 切换收藏状态 */
  toggleFavorite: (id: string) => void;
  /** 删除一条记录 */
  removeHistory: (id: string) => void;
  /** 清空全部历史（保留收藏） */
  clearHistory: () => void;
}

const MAX_HISTORY = 200;

export const useSQLHistoryStore = create<SQLHistoryState>()(
  persist(
    (set) => ({
      history: [],

      addHistory: (item) =>
        set((state) => {
          // 如果与最近一条完全相同的 SQL，跳过
          if (state.history.length > 0 && state.history[0].sql.trim() === item.sql.trim()) {
            return state;
          }
          const newItem: SQLHistoryItem = {
            ...item,
            id: `${Date.now()}-${Math.random().toString(36).slice(2, 8)}`,
            executedAt: new Date().toISOString(),
            favorite: false,
          };
          const newHistory = [newItem, ...state.history].slice(0, MAX_HISTORY);
          return { history: newHistory };
        }),

      toggleFavorite: (id) =>
        set((state) => ({
          history: state.history.map((h) =>
            h.id === id ? { ...h, favorite: !h.favorite } : h
          ),
        })),

      removeHistory: (id) =>
        set((state) => ({
          history: state.history.filter((h) => h.id !== id),
        })),

      clearHistory: () =>
        set((state) => ({
          history: state.history.filter((h) => h.favorite),
        })),
    }),
    {
      name: "tableplus-sql-history",
    }
  )
);
