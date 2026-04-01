import { create } from "zustand";
import { persist } from "zustand/middleware";

// 布局模式：compact 紧凑模式（默认），default 标准模式
export type LayoutMode = "compact" | "default";

interface UIStore {
  sidebarCollapsed: boolean;
  sidebarWidth: number;
  previewVisible: boolean;
  previewWidth: number;
  statusBarVisible: boolean;
  pageSize: number;
  layoutMode: LayoutMode;
  showScrollbar: boolean;

  toggleSidebar: () => void;
  setSidebarWidth: (width: number) => void;
  togglePreview: () => void;
  setPreviewVisible: (visible: boolean) => void;
  setPreviewWidth: (width: number) => void;
  setPageSize: (size: number) => void;
  setLayoutMode: (mode: LayoutMode) => void;
  setShowScrollbar: (show: boolean) => void;
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
      layoutMode: "compact",
      showScrollbar: true,

      toggleSidebar: () =>
        set((s) => ({ sidebarCollapsed: !s.sidebarCollapsed })),
      setSidebarWidth: (sidebarWidth) => set({ sidebarWidth }),
      togglePreview: () => set((s) => ({ previewVisible: !s.previewVisible })),
      setPreviewVisible: (previewVisible) => set({ previewVisible }),
      setPreviewWidth: (previewWidth) => set({ previewWidth }),
      setPageSize: (pageSize) => set({ pageSize }),
      setLayoutMode: (layoutMode) => set({ layoutMode }),
      setShowScrollbar: (showScrollbar) => set({ showScrollbar }),
    }),
    { name: "tableplus-ai-ui" }
  )
);
