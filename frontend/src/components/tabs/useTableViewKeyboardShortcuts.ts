import { useEffect } from "react";
import { useUIStore } from "@/stores/ui";
import { useTabsStore } from "@/stores/tabs";
import type { TableSubView } from "./tabTypes";
import { isEditableTarget, isGridShortcutContext } from "./tabUtils";

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
  copySelectedRows: () => void;
  deleteDataRow: () => void;
  insertDataRow: () => void;
  loadStructure: (force?: boolean) => Promise<void>;
  loadDDL: (force?: boolean) => Promise<void>;
  loadDoc: (force?: boolean) => Promise<void>;
  reloadDataView: () => void;
  selectedRow: Record<string, unknown> | null;
  selectedRowIndex: number | null;
  setSelectedRowIndex: React.Dispatch<React.SetStateAction<number | null>>;
  setSelectedRowIndexes: React.Dispatch<React.SetStateAction<Set<number>>>;
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
    copySelectedRows,
    deleteDataRow,
    insertDataRow,
    loadStructure,
    loadDDL,
    loadDoc,
    reloadDataView,
    selectedRow,
    selectedRowIndex,
    setSelectedRowIndex,
    setSelectedRowIndexes,
    dataLength,
    previewVisible,
    gridContainerRef,
  } = params;

  const setPreviewVisible = useUIStore((state) => state.setPreviewVisible);
  const activeTabId = useTabsStore((state) => state.activeTabId);

  useEffect(() => {
    const isDataGridShortcutContext = (target: EventTarget | null) => {
      if (subView !== "data") return false;
      return isGridShortcutContext(target, gridContainerRef.current);
    };

    const handler = (e: KeyboardEvent) => {
      if (activeTabId !== tabId) return;
      const isKeyA = (e.key && e.key.toLowerCase() === "a") || e.code === "KeyA";
      const isKeyC = (e.key && e.key.toLowerCase() === "c") || e.code === "KeyC";
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
      if ((e.metaKey || e.ctrlKey) && !e.altKey && !e.shiftKey && isKeyA) {
        const target = e.target as HTMLElement;
        if (isDataGridShortcutContext(target)) {
          e.preventDefault();
          const next = new Set<number>();
          for (let i = 0; i < dataLength; i += 1) next.add(i);
          setSelectedRowIndexes(next);
          setSelectedRowIndex(dataLength > 0 ? 0 : null);
          return;
        }
      }
      if ((e.metaKey || e.ctrlKey) && !e.altKey && !e.shiftKey && isKeyC) {
        const target = e.target as HTMLElement;
        if (isDataGridShortcutContext(target)) {
          e.preventDefault();
          copySelectedRows();
          return;
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
          // 同步 selectedRowIndexes，否则 selectedRow 可能与选中状态不一致
          setSelectedRowIndexes(new Set<number>([nextIndex]));
          requestAnimationFrame(() => {
            const container = gridContainerRef.current;
            if (!container) return;
            const rowElement = container.querySelector(`tbody tr:nth-child(${nextIndex + 1})`) as HTMLElement;
            rowElement?.scrollIntoView({ block: "nearest" });
          });
          return;
        }
      }
      // 空格切换预览：只要焦点不在可编辑元素上，且在 grid 区域内即可触发
      const isSpaceKey = e.code === "Space" || e.key === " ";
      if (isSpaceKey && selectedRow && subView === "data") {
        const target = e.target as HTMLElement;
        if (!e.repeat && isGridShortcutContext(target, gridContainerRef.current, { allowNeutralFocus: true })) {
          e.preventDefault();
          const previewActuallyOpen = Boolean(previewVisible && selectedRow && selectedRowIndex !== null);
          setPreviewVisible(!previewActuallyOpen);
        }
      }
    };

    window.addEventListener("keydown", handler, { capture: true });
    return () => window.removeEventListener("keydown", handler, { capture: true });
  }, [
    activeTabId,
    tabId,
    showFilter,
    commitChanges,
    copySelectedRows,
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
    setSelectedRowIndexes,
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
