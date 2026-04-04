import { useEffect } from "react";
import { useUIStore } from "@/stores/ui";
import { useTabsStore } from "@/stores/tabs";
import type { TableSubView } from "./tabTypes";
import { isEditableTarget, isGridTarget } from "./tabUtils";

// 表视图快捷键：筛选、提交、刷新、视图切换、行选择与预览
export function useTableViewKeyboardShortcuts(params: {
  tabId: string;
  subView: TableSubView;
  showFilter: boolean;
  setSubView: (view: TableSubView) => void;
  setShowFilter: React.Dispatch<React.SetStateAction<boolean>>;
  structureCommitRef: React.MutableRefObject<((source?: "shortcut" | "button") => Promise<void>) | null>;
  structureDeleteRef: React.MutableRefObject<(() => void) | null>;
  structureInsertRef: React.MutableRefObject<(() => void) | null>;
  commitChanges: (source?: "shortcut" | "button") => void | Promise<void>;
  deleteDataRow: () => void;
  insertDataRow: () => void;
  loadStructure: (force?: boolean) => Promise<void>;
  loadDDL: (force?: boolean) => Promise<void>;
  loadDoc: (force?: boolean) => Promise<void>;
  reloadDataView: () => void;
  selectedRow: Record<string, unknown> | null;
  selectedRowIndex: number | null;
  setSelectedRowIndex: React.Dispatch<React.SetStateAction<number | null>>;
  dataLength: number;
  previewVisible: boolean;
  gridContainerRef: React.RefObject<HTMLDivElement | null>;
}) {
  const {
    tabId,
    subView,
    showFilter,
    setSubView,
    setShowFilter,
    structureCommitRef,
    structureDeleteRef,
    structureInsertRef,
    commitChanges,
    deleteDataRow,
    insertDataRow,
    loadStructure,
    loadDDL,
    loadDoc,
    reloadDataView,
    selectedRow,
    selectedRowIndex,
    setSelectedRowIndex,
    dataLength,
    previewVisible,
    gridContainerRef,
  } = params;

  const setPreviewVisible = useUIStore((state) => state.setPreviewVisible);
  const activeTabId = useTabsStore((state) => state.activeTabId);

  useEffect(() => {
    const handler = (e: KeyboardEvent) => {
      if (activeTabId !== tabId) return;
      if ((e.metaKey || e.ctrlKey) && e.key === "f") {
        e.preventDefault();
        setShowFilter((value) => !value);
      }
      if (e.key === "Escape" && subView === "data" && showFilter) {
        e.preventDefault();
        setShowFilter(false);
        return;
      }
      if ((e.metaKey || e.ctrlKey) && e.key === "s") {
        e.preventDefault();
        if (subView === "structure" && structureCommitRef.current) {
          void structureCommitRef.current("shortcut");
        } else {
          void commitChanges("shortcut");
        }
      }
      if ((e.metaKey || e.ctrlKey) && e.key === "r") {
        e.preventDefault();
        if (subView === "structure") {
          void loadStructure(true);
        } else if (subView === "info") {
          void loadDDL(true);
        } else if (subView === "doc") {
          void loadDoc(true);
        } else {
          reloadDataView();
        }
      }
      if ((e.key === "Delete" || e.key === "Backspace") && !e.metaKey && !e.ctrlKey && !e.altKey) {
        const target = e.target as HTMLElement;
        if (!isEditableTarget(target)) {
          if (subView === "data") {
            e.preventDefault();
            deleteDataRow();
            return;
          }
          if (subView === "structure") {
            e.preventDefault();
            structureDeleteRef.current?.();
            return;
          }
        }
      }
      if (e.key === "Insert" && !e.metaKey && !e.ctrlKey && !e.altKey) {
        const target = e.target as HTMLElement;
        if (!isEditableTarget(target)) {
          if (subView === "data") {
            e.preventDefault();
            insertDataRow();
            return;
          }
          if (subView === "structure") {
            e.preventDefault();
            structureInsertRef.current?.();
            return;
          }
        }
      }
      if (e.metaKey && e.ctrlKey) {
        const subViews: TableSubView[] = ["data", "structure", "info", "doc"];
        const currentIndex = subViews.indexOf(subView);
        if (e.key === "[") {
          e.preventDefault();
          e.stopPropagation();
          setSubView(subViews[(currentIndex - 1 + subViews.length) % subViews.length]);
          return;
        }
        if (e.key === "]") {
          e.preventDefault();
          e.stopPropagation();
          setSubView(subViews[(currentIndex + 1) % subViews.length]);
          return;
        }
      }
      if ((e.key === "ArrowDown" || e.key === "ArrowUp") && subView === "data") {
        const target = e.target as HTMLElement;
        if (!isEditableTarget(target)) {
          e.preventDefault();
          const currentIndex = selectedRowIndex ?? -1;
          const nextIndex = e.key === "ArrowDown"
            ? Math.min(currentIndex + 1, dataLength - 1)
            : Math.max(currentIndex - 1, 0);
          setSelectedRowIndex(nextIndex);
          requestAnimationFrame(() => {
            const container = gridContainerRef.current;
            if (!container) return;
            const rowElement = container.querySelector(`tbody tr:nth-child(${nextIndex + 1})`) as HTMLElement;
            rowElement?.scrollIntoView({ block: "nearest" });
          });
          return;
        }
      }
      if (e.code === "Space" && selectedRow && subView === "data") {
        const target = e.target as HTMLElement;
        if (!isEditableTarget(target) && isGridTarget(target)) {
          e.preventDefault();
          setPreviewVisible(!previewVisible);
        }
      }
    };

    window.addEventListener("keydown", handler);
    return () => window.removeEventListener("keydown", handler);
  }, [
    activeTabId,
    tabId,
    showFilter,
    commitChanges,
    dataLength,
    gridContainerRef,
    loadDDL,
    loadDoc,
    loadStructure,
    previewVisible,
    reloadDataView,
    selectedRow,
    selectedRowIndex,
    setPreviewVisible,
    setSelectedRowIndex,
    setShowFilter,
    setSubView,
    structureCommitRef,
    structureDeleteRef,
    structureInsertRef,
    subView,
    deleteDataRow,
    insertDataRow,
  ]);
}
