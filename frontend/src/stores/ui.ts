import { create } from "zustand";
import { persist } from "zustand/middleware";

// 布局模式：compact 紧凑模式，default 标准模式（默认）
export type LayoutMode = "compact" | "default";

// 轻量 toast 通知
export interface ToastItem {
  id: string;
  type: "info" | "success" | "error";
  message: string;
  placement?: "bottom-right" | "top-center";
}

// 导出任务状态
export interface ExportTask {
  taskId: string;
  status: "progress" | "done" | "error" | "cancelled";
  current: number;
  total: number;
  fileName: string;
  filePath: string;
  error?: string;
}

interface UIStore {
  sidebarCollapsed: boolean;
  sidebarWidth: number;
  previewVisible: boolean;
  previewWidth: number;
  statusBarVisible: boolean;
  pageSize: number;
  showDataRowNumbers: boolean;
  layoutMode: LayoutMode;
  showScrollbar: boolean;
  toasts: ToastItem[];
  exportTasks: ExportTask[];

  toggleSidebar: () => void;
  setSidebarWidth: (width: number) => void;
  togglePreview: () => void;
  setPreviewVisible: (visible: boolean) => void;
  setPreviewWidth: (width: number) => void;
  setPageSize: (size: number) => void;
  setShowDataRowNumbers: (show: boolean) => void;
  setLayoutMode: (mode: LayoutMode) => void;
  setShowScrollbar: (show: boolean) => void;
  addToast: (
    type: ToastItem["type"],
    message: string,
    durationMs?: number,
    placement?: ToastItem["placement"],
  ) => void;
  removeToast: (id: string) => void;
  updateExportTask: (task: ExportTask) => void;
  removeExportTask: (taskId: string) => void;
}

export const useUIStore = create<UIStore>()(
  persist(
    (set) => ({
      sidebarCollapsed: false,
      sidebarWidth: 260,
      previewVisible: false,
      previewWidth: 320,
      statusBarVisible: false,
      pageSize: 100,
      showDataRowNumbers: true,
      layoutMode: "default",
      showScrollbar: true,
      toasts: [],
      exportTasks: [],

      toggleSidebar: () =>
        set((s) => ({ sidebarCollapsed: !s.sidebarCollapsed })),
      setSidebarWidth: (sidebarWidth) => set({ sidebarWidth }),
      togglePreview: () => set((s) => ({ previewVisible: !s.previewVisible })),
      setPreviewVisible: (previewVisible) => set({ previewVisible }),
      setPreviewWidth: (previewWidth) => set({ previewWidth }),
      setPageSize: (pageSize) => set({ pageSize }),
      setShowDataRowNumbers: (showDataRowNumbers) => set({ showDataRowNumbers }),
      setLayoutMode: (layoutMode) => set({ layoutMode }),
      setShowScrollbar: (showScrollbar) => set({ showScrollbar }),
      addToast: (type, message, durationMs = 3000, placement = "bottom-right") => {
        const id = `toast_${Date.now()}_${Math.random().toString(36).slice(2, 6)}`;
        set((s) => ({ toasts: [...s.toasts, { id, type, message, placement }] }));
        if (durationMs > 0) {
          setTimeout(() => {
            set((s) => ({ toasts: s.toasts.filter((t) => t.id !== id) }));
          }, durationMs);
        }
        return id;
      },
      removeToast: (id) => set((s) => ({ toasts: s.toasts.filter((t) => t.id !== id) })),
      updateExportTask: (task) => set((s) => {
        const idx = s.exportTasks.findIndex((t) => t.taskId === task.taskId);
        if (idx >= 0) {
          const updated = [...s.exportTasks];
          updated[idx] = task;
          return { exportTasks: updated };
        }
        return { exportTasks: [...s.exportTasks, task] };
      }),
      removeExportTask: (taskId) => set((s) => ({
        exportTasks: s.exportTasks.filter((t) => t.taskId !== taskId),
      })),
    }),
    {
      name: "tableplus-ai-ui",
      partialize: (state) => {
        const { toasts, exportTasks, ...rest } = state;
        return rest;
      },
    }
  )
);
