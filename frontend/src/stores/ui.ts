import { create } from "zustand";
import { persist } from "zustand/middleware";

interface UIStore {
  sidebarCollapsed: boolean;
  sidebarWidth: number;
  previewVisible: boolean;
  previewWidth: number;
  statusBarVisible: boolean;
  pageSize: number;

  toggleSidebar: () => void;
  setSidebarWidth: (width: number) => void;
  togglePreview: () => void;
  setPreviewVisible: (visible: boolean) => void;
  setPreviewWidth: (width: number) => void;
  setPageSize: (size: number) => void;
}

export const useUIStore = create<UIStore>()(
  persist(
    (set) => ({
      sidebarCollapsed: false,
      sidebarWidth: 260,
      previewVisible: false,
      previewWidth: 320,
      statusBarVisible: true,
      pageSize: 100,

      toggleSidebar: () =>
        set((s) => ({ sidebarCollapsed: !s.sidebarCollapsed })),
      setSidebarWidth: (sidebarWidth) => set({ sidebarWidth }),
      togglePreview: () => set((s) => ({ previewVisible: !s.previewVisible })),
      setPreviewVisible: (previewVisible) => set({ previewVisible }),
      setPreviewWidth: (previewWidth) => set({ previewWidth }),
      setPageSize: (pageSize) => set({ pageSize }),
    }),
    { name: "tableplus-ai-ui" }
  )
);
