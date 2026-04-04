import React, { useCallback, useEffect, useMemo, useRef, useState } from "react";
import { createPortal } from "react-dom";
import { ChevronDown, Hash, Key, Plus, Trash2, Undo2 } from "lucide-react";
import { useTranslation } from "@/i18n";
import { cn } from "@/lib/utils";
import { useUIStore } from "@/stores/ui";
import type { DatabaseDriver } from "@/types/connection";
import type { ColumnInfo } from "@/types/database";
import * as DatabaseService from "../../../wailsjs/go/services/DatabaseService";
import { TipBtn } from "./TipBtn";
import {
  getDataTypes,
  INDEX_COL_DEFS,
  STRUCTURE_COL_DEFS,
  type EditingIndexRow,
  type EditingStructureCol,
} from "./tabTypes";

export function StructureView({
  connectionId,
  database: dbName,
  tableName,
  driver,
  columns,
  indexes,
  onRefresh,
  onHasEditsChange,
  commitRef,
  deleteRef,
  insertRef,
}: {
  connectionId: string;
  database: string;
  tableName: string;
  driver?: DatabaseDriver;
  columns: ColumnInfo[];
  indexes: any[];
  onRefresh: () => Promise<void>;
  onHasEditsChange: (hasEdits: boolean) => void;
  commitRef: React.MutableRefObject<((source?: "shortcut" | "button") => Promise<void>) | null>;
  deleteRef: React.MutableRefObject<(() => void) | null>;
  insertRef: React.MutableRefObject<(() => void) | null>;
}) {

  const { t } = useTranslation();
  const addToast = useUIStore((state) => state.addToast);
  const [workingCols, setWorkingCols] = useState<EditingStructureCol[]>([]);
  const [originalCols, setOriginalCols] = useState<EditingStructureCol[]>([]);
  const [editingCell, setEditingCell] = useState<{ uid: string; key: string } | null>(null);
  const [editValue, setEditValue] = useState("");
  const editInputRef = useRef<HTMLInputElement | HTMLSelectElement>(null);
  const [selectedUid, setSelectedUid] = useState<string | null>(null);
  const [topHeight, setTopHeight] = useState<number>(0);
  const containerRef = useRef<HTMLDivElement>(null);
  const topGridRef = useRef<HTMLDivElement>(null);
  const indexGridRef = useRef<HTMLDivElement>(null);
  const resizingRef = useRef(false);
  const resizeRafRef = useRef<number | null>(null);
  const pendingTopHeightRef = useRef<number | null>(null);
  const [workingIndexes, setWorkingIndexes] = useState<EditingIndexRow[]>([]);
  const [originalIndexes, setOriginalIndexes] = useState<EditingIndexRow[]>([]);
  const [selectedIndexUid, setSelectedIndexUid] = useState<string | null>(null);
  const [editingIndexCell, setEditingIndexCell] = useState<{ uid: string; key: "name" | "columns" } | null>(null);
  const [indexEditValue, setIndexEditValue] = useState("");
  const indexInputRef = useRef<HTMLInputElement>(null);
  const [typeDropdownOpen, setTypeDropdownOpen] = useState(false);
  const [typeFilter, setTypeFilter] = useState("");
  const [typeHighlightIdx, setTypeHighlightIdx] = useState(-1);
  const [typeDropdownPos, setTypeDropdownPos] = useState<{ top: number; left: number; width: number }>({ top: 0, left: 0, width: 180 });
  const typeInputRef = useRef<HTMLInputElement>(null);
  const typeDropdownRef = useRef<HTMLDivElement>(null);
  const [topViewportHeight, setTopViewportHeight] = useState(0);
  const [indexViewportHeight, setIndexViewportHeight] = useState(0);
  const [columnRowHeight, setColumnRowHeight] = useState(26);
  const [indexRowHeight, setIndexRowHeight] = useState(26);

  useEffect(() => {
    const mapped: EditingStructureCol[] = columns.map((c, i) => ({
      ...c,
      __uid: `orig_${i}_${c.name}`,
    }));
    setWorkingCols(mapped);
    setOriginalCols(mapped.map((c) => ({ ...c })));
    setEditingCell(null);
    setSelectedUid(null);
  }, [columns]);

  useEffect(() => {
    const mappedIndexes: EditingIndexRow[] = (indexes || []).map((idx: any, i: number) => ({
      __uid: `idx_${i}_${idx.name}`,
      name: idx.name || "",
      type: idx.type || "BTREE",
      isUnique: !!idx.isUnique,
      columns: Array.isArray(idx.columns) ? idx.columns : [],
      isPrimary: !!idx.isPrimary,
    }));
    setWorkingIndexes(mappedIndexes);
    setOriginalIndexes(mappedIndexes.map((idx) => ({ ...idx, columns: [...(idx.columns || [])] })));
    setSelectedIndexUid(null);
    setEditingIndexCell(null);
    setIndexEditValue("");
  }, [indexes]);

  useEffect(() => {
    if (editingIndexCell && indexInputRef.current) {
      indexInputRef.current.focus();
      indexInputRef.current.select();
    }
  }, [editingIndexCell]);

  useEffect(() => {
    if (containerRef.current && topHeight === 0) {
      setTopHeight(Math.floor(containerRef.current.clientHeight * 0.6));
    }
  }, [topHeight]);

  useEffect(() => {
    if (typeof ResizeObserver === "undefined") return;
    const topEl = topGridRef.current;
    const idxEl = indexGridRef.current;
    const observer = new ResizeObserver((entries) => {
      for (const entry of entries) {
        if (entry.target === topEl) {
          setTopViewportHeight(Math.floor(entry.contentRect.height));
        } else if (entry.target === idxEl) {
          setIndexViewportHeight(Math.floor(entry.contentRect.height));
        }
      }
    });
    if (topEl) observer.observe(topEl);
    if (idxEl) observer.observe(idxEl);
    return () => observer.disconnect();
  }, []);

  useEffect(() => {
    const row = topGridRef.current?.querySelector("tbody tr") as HTMLTableRowElement | null;
    if (!row) return;
    const h = Math.round(row.getBoundingClientRect().height);
    if (h > 10) setColumnRowHeight(h);
  }, [workingCols.length, topHeight]);

  useEffect(() => {
    const row = indexGridRef.current?.querySelector("tbody tr") as HTMLTableRowElement | null;
    if (!row) return;
    const h = Math.round(row.getBoundingClientRect().height);
    if (h > 10) setIndexRowHeight(h);
  }, [workingIndexes.length, topHeight]);

  const hasEdits = useMemo(() => {
    const hasColumnEdits = (() => {
      if (workingCols.length !== originalCols.length) return true;
      for (let i = 0; i < workingCols.length; i++) {
        const w = workingCols[i];
        if (w.__status === "new" || w.__status === "deleted") return true;
        const o = originalCols.find((c) => c.__uid === w.__uid);
        if (!o) return true;
        if (w.name !== o.name || w.type !== o.type || w.nullable !== o.nullable ||
          (w.defaultValue ?? "") !== (o.defaultValue ?? "") || w.comment !== o.comment) return true;
      }
      return false;
    })();
    if (hasColumnEdits) return true;

    if (workingIndexes.length !== originalIndexes.length) return true;
    for (let i = 0; i < workingIndexes.length; i++) {
      const w = workingIndexes[i];
      if (w.__status === "new" || w.__status === "deleted") return true;
      const o = originalIndexes.find((idx) => idx.__uid === w.__uid);
      if (!o) return true;
      const wCols = (w.columns || []).join(",");
      const oCols = (o.columns || []).join(",");
      if (w.name !== o.name || w.type !== o.type || w.isUnique !== o.isUnique || wCols !== oCols) return true;
    }
    return false;
  }, [workingCols, originalCols, workingIndexes, originalIndexes]);

  useEffect(() => {
    onHasEditsChange(hasEdits);
  }, [hasEdits, onHasEditsChange]);

  const toColumnPayload = (c: EditingStructureCol) => ({
    uid: c.__uid,
    status: c.__status === "new" ? "new" : c.__status === "deleted" ? "deleted" : c.__status === "modified" ? "modified" : "",
    name: c.name,
    type: c.type,
    nullable: c.nullable,
    defaultValue: c.defaultValue === null || c.defaultValue === undefined ? undefined : String(c.defaultValue),
    comment: c.comment ?? "",
  });

  const toIndexPayload = (idx: EditingIndexRow) => ({
    uid: idx.__uid,
    status: idx.__status === "new" ? "new" : idx.__status === "deleted" ? "deleted" : "",
    name: idx.name,
    type: idx.type || "BTREE",
    isUnique: idx.isUnique,
    isPrimary: idx.isPrimary,
    columns: idx.columns || [],
  });

  const commitStructureChanges = useCallback(async (source: "shortcut" | "button" = "button") => {
    const wp = workingCols.map(toColumnPayload);
    const op = originalCols.map(toColumnPayload);
    const wi = workingIndexes.map(toIndexPayload);
    try {
      console.log("[Structure] 调用后端应用结构变更: cols=%d idx=%d", wp.length, wi.length);
      await DatabaseService.ApplyTableStructureChanges(
        connectionId, dbName, tableName,
        wp as any, op as any, wi as any
      );
      console.log("[Structure] 提交成功，刷新结构");
      addToast("success", t("structure.commitSuccess"), 1200, "top-center");
      await onRefresh();
    } catch (e: any) {
      console.error("[Structure] 结构变更失败:", e);
      addToast("error", `${t("structure.commitFailed")}: ${e?.message || e}`);
    }
  }, [workingCols, originalCols, workingIndexes, tableName, connectionId, dbName, onRefresh, t, addToast]);

  useEffect(() => {
    commitRef.current = commitStructureChanges;
  }, [commitStructureChanges, commitRef]);

  const handleAddColumn = useCallback(() => {
    const uid = `new_${Date.now()}_${Math.random().toString(36).slice(2, 7)}`;
    const newRow: EditingStructureCol = {
      name: "", type: "varchar(255)", nullable: true, defaultValue: null,
      isPrimary: false, isAutoIncrement: false, comment: "", maxLength: null,
      characterSet: "", collation: "", extra: "", foreignKey: "",
      __status: "new", __uid: uid,
    };
    setWorkingCols((prev) => [...prev, newRow]);
    setSelectedIndexUid(null);
    requestAnimationFrame(() => {
      setEditingCell({ uid, key: "name" });
      setEditValue("");
      setSelectedUid(uid);
    });
  }, []);

  const handleDeleteSelected = useCallback(() => {
    if (!selectedUid) return;
    setWorkingCols((prev) => prev.map((c) => {
      if (c.__uid !== selectedUid) return c;
      if (c.__status === "new") return { ...c, __status: "deleted" as const };
      return { ...c, __status: "deleted" as const };
    }).filter((c) => !(c.__status === "deleted" && c.__uid.startsWith("new_"))));
    setSelectedUid(null);
  }, [selectedUid]);

  const handleRevertAll = useCallback(() => {
    setWorkingCols(originalCols.map((c) => ({ ...c })));
    setWorkingIndexes(originalIndexes.map((idx) => ({ ...idx, columns: [...(idx.columns || [])] })));
    setEditingCell(null);
    setEditingIndexCell(null);
    setSelectedUid(null);
    setSelectedIndexUid(null);
    setIndexEditValue("");
  }, [originalCols, originalIndexes]);

  const updateDropdownPos = useCallback(() => {
    if (!typeInputRef.current) return;
    const td = typeInputRef.current.closest("td");
    const rect = td ? td.getBoundingClientRect() : typeInputRef.current.getBoundingClientRect();
    setTypeDropdownPos({
      top: rect.bottom + 1,
      left: rect.left,
      width: rect.width,
    });
  }, []);

  const handleCellDoubleClick = useCallback((uid: string, key: string, currentValue: unknown) => {
    const colDef = STRUCTURE_COL_DEFS.find((d) => d.key === key);
    if (!colDef || !colDef.editable) return;
    const row = workingCols.find((c) => c.__uid === uid);
    if (row?.__status === "deleted") return;

    if (colDef.isCheckbox) {
      setWorkingCols((prev) => prev.map((c) => {
        if (c.__uid !== uid) return c;
        return { ...c, [key]: !c[key as keyof EditingStructureCol] };
      }));
      return;
    }

    setEditingCell({ uid, key });
    const strVal = currentValue === null || currentValue === undefined ? "" : String(currentValue);
    if (colDef.isTypeSelect) {
      setTypeFilter("");
      setTypeDropdownOpen(true);
      setTypeHighlightIdx(-1);
      requestAnimationFrame(() => updateDropdownPos());
    }
    setEditValue(strVal);
  }, [workingCols, updateDropdownPos]);

  const commitCellEdit = useCallback(() => {
    if (!editingCell) return;
    const { uid, key } = editingCell;
    setWorkingCols((prev) => prev.map((c) => {
      if (c.__uid !== uid) return c;
      const updated = { ...c, [key]: key === "defaultValue" && editValue === "" ? null : editValue };
      if (c.__status !== "new") {
        const orig = originalCols.find((o) => o.__uid === uid);
        if (orig) {
          const changed = updated.name !== orig.name || updated.type !== orig.type ||
            updated.nullable !== orig.nullable || (updated.defaultValue ?? "") !== (orig.defaultValue ?? "") ||
            updated.comment !== orig.comment;
          updated.__status = changed ? "modified" : undefined;
        }
      }
      return updated;
    }));
    setEditingCell(null);
    setTypeDropdownOpen(false);
    setTypeHighlightIdx(-1);
  }, [editingCell, editValue, originalCols]);

  const cancelCellEdit = useCallback(() => {
    setEditingCell(null);
    setTypeDropdownOpen(false);
    setTypeHighlightIdx(-1);
  }, []);

  useEffect(() => {
    if (editingCell && editInputRef.current) {
      editInputRef.current.focus();
      if (editInputRef.current instanceof HTMLInputElement) {
        editInputRef.current.select();
      }
    }
  }, [editingCell]);

  useEffect(() => {
    if (!typeDropdownOpen) return;
    const handler = (e: MouseEvent) => {
      if (typeDropdownRef.current && !typeDropdownRef.current.contains(e.target as Node) &&
        typeInputRef.current && !typeInputRef.current.contains(e.target as Node)) {
        commitCellEdit();
      }
    };
    document.addEventListener("mousedown", handler);
    return () => document.removeEventListener("mousedown", handler);
  }, [typeDropdownOpen, commitCellEdit]);

  useEffect(() => {
    if (!typeDropdownOpen || typeHighlightIdx < 0 || !typeDropdownRef.current) return;
    const items = typeDropdownRef.current.children;
    if (items[typeHighlightIdx]) {
      (items[typeHighlightIdx] as HTMLElement).scrollIntoView({ block: "nearest" });
    }
  }, [typeHighlightIdx, typeDropdownOpen]);

  const handleSplitDragStart = useCallback((e: React.MouseEvent) => {
    e.preventDefault();
    resizingRef.current = true;
    const startY = e.clientY;
    const startH = topHeight;
    const containerH = containerRef.current?.clientHeight || 600;
    const onMove = (ev: MouseEvent) => {
      if (!resizingRef.current) return;
      const newH = Math.max(100, Math.min(containerH - 100, startH + ev.clientY - startY));
      pendingTopHeightRef.current = newH;
      if (resizeRafRef.current !== null) return;
      resizeRafRef.current = requestAnimationFrame(() => {
        resizeRafRef.current = null;
        if (pendingTopHeightRef.current !== null) {
          setTopHeight(pendingTopHeightRef.current);
        }
      });
    };
    const onUp = () => {
      resizingRef.current = false;
      if (resizeRafRef.current !== null) {
        cancelAnimationFrame(resizeRafRef.current);
        resizeRafRef.current = null;
      }
      if (pendingTopHeightRef.current !== null) {
        setTopHeight(pendingTopHeightRef.current);
        pendingTopHeightRef.current = null;
      }
      document.removeEventListener("mousemove", onMove);
      document.removeEventListener("mouseup", onUp);
    };
    document.addEventListener("mousemove", onMove);
    document.addEventListener("mouseup", onUp);
  }, [topHeight]);

  const handleAddIndex = useCallback(() => {
    const newIndexUid = `new_idx_${Date.now()}`;
    setWorkingIndexes((prev) => {
      if (prev.some((item) => item.__status === "new" && !item.name && item.columns.length === 0)) {
        return prev;
      }
      return [
        ...prev,
        {
          __uid: newIndexUid,
          __status: "new",
          name: "",
          type: "BTREE",
          isUnique: false,
          columns: [],
          isPrimary: false,
        },
      ];
    });
    setSelectedUid(null);
    setEditingIndexCell({ uid: newIndexUid, key: "name" });
    setSelectedIndexUid(newIndexUid);
    setIndexEditValue("");
  }, []);

  const startEditIndexCell = useCallback((uid: string, key: "name" | "columns", value: string) => {
    setEditingIndexCell({ uid, key });
    setIndexEditValue(value);
  }, []);

  const cancelEditIndexCell = useCallback(() => {
    setEditingIndexCell(null);
    setIndexEditValue("");
  }, []);

  const commitEditIndexCell = useCallback(() => {
    if (!editingIndexCell) return;
    const nextValue = indexEditValue.trim();
    setWorkingIndexes((prev) => prev.map((item) => {
      if (item.__uid !== editingIndexCell.uid) return item;
      if (editingIndexCell.key === "name") {
        return { ...item, name: nextValue };
      }
      return {
        ...item,
        columns: nextValue
          ? nextValue.split(",").map((part) => part.trim()).filter(Boolean)
          : [],
      };
    }));
    setEditingIndexCell(null);
    setIndexEditValue("");
  }, [editingIndexCell, indexEditValue]);

  const handleToggleInlineIndexUnique = useCallback((uid: string, checked: boolean) => {
    setWorkingIndexes((prev) => prev.map((item) => (
      item.__uid === uid ? { ...item, isUnique: checked } : item
    )));
  }, []);

  const handleDeleteSelectedIndex = useCallback(() => {
    if (!selectedIndexUid) return;
    setWorkingIndexes((prev) => prev.map((item) => {
      if (item.__uid !== selectedIndexUid) return item;
      return { ...item, __status: "deleted" as const };
    }).filter((item) => !(item.__status === "deleted" && item.__uid.startsWith("new_idx_"))));
    setSelectedIndexUid(null);
    setEditingIndexCell((prev) => (prev?.uid === selectedIndexUid ? null : prev));
    setIndexEditValue("");
  }, [selectedIndexUid]);

  const deleteSelectedStructureItem = useCallback(() => {
    if (selectedIndexUid) {
      handleDeleteSelectedIndex();
      return;
    }
    if (selectedUid) {
      handleDeleteSelected();
    }
  }, [handleDeleteSelected, handleDeleteSelectedIndex, selectedIndexUid, selectedUid]);

  useEffect(() => {
    deleteRef.current = deleteSelectedStructureItem;
  }, [deleteRef, deleteSelectedStructureItem]);

  const insertStructureItem = useCallback(() => {
    if (selectedIndexUid) {
      handleAddIndex();
      return;
    }
    handleAddColumn();
  }, [handleAddColumn, handleAddIndex, selectedIndexUid]);

  useEffect(() => {
    insertRef.current = insertStructureItem;
  }, [insertRef, insertStructureItem]);

  const inputCls = cn(
    "w-full h-full border-2 border-[var(--accent)] outline-none text-xs px-1 rounded-sm",
    "bg-[var(--surface)] text-[var(--fg)] font-medium",
    "absolute inset-0 z-20"
  );

  const allDataTypes = useMemo(() => getDataTypes(driver), [driver]);
  const filteredTypes = useMemo(() => {
    if (!typeFilter.trim()) return allDataTypes;
    const lower = typeFilter.toLowerCase();
    return allDataTypes.filter((item) => item.toLowerCase().includes(lower));
  }, [typeFilter, allDataTypes]);

  const visibleCols = useMemo(() => workingCols.filter((c) => !(c.__status === "deleted" && c.__uid.startsWith("new_"))), [workingCols]);
  const visibleIndexes = useMemo(
    () => workingIndexes.filter((idx) => !(idx.__status === "deleted" && idx.__uid.startsWith("new_idx_"))),
    [workingIndexes]
  );
  const columnFillerRows = useMemo(() => {
    const headerHeight = 30;
    const rowH = Math.max(20, columnRowHeight);
    const safetyRows = 8; // 额外缓冲，避免底部露白
    const targetRows = topViewportHeight > 0
      ? Math.max(8, Math.ceil(Math.max(0, topViewportHeight - headerHeight) / rowH) + safetyRows)
      : 14;
    return Math.max(0, Math.min(400, targetRows - visibleCols.length));
  }, [columnRowHeight, topViewportHeight, visibleCols.length]);

  const indexFillerRows = useMemo(() => {
    const headerHeight = 30;
    const rowH = Math.max(20, indexRowHeight);
    const safetyRows = 8; // 额外缓冲，避免底部露白
    const targetRows = indexViewportHeight > 0
      ? Math.max(8, Math.ceil(Math.max(0, indexViewportHeight - headerHeight) / rowH) + safetyRows)
      : 10;
    return Math.max(0, Math.min(400, targetRows - visibleIndexes.length));
  }, [indexRowHeight, indexViewportHeight, visibleIndexes.length]);

  return (
    <div ref={containerRef} className="flex-1 flex flex-col overflow-hidden">
      <div className="flex items-center h-6 px-2 gap-1 border-b border-[var(--border-color)] bg-[var(--surface-secondary)] flex-shrink-0">
        <span className="text-[length:var(--size-font-xs)] font-medium text-[var(--fg-secondary)]">
          {t("structure.columns")} ({visibleCols.filter((c) => c.__status !== "deleted").length})
        </span>
        <div className="flex-1" />
        <TipBtn
          tip={t("structure.addColumn")}
          className="h-4 w-4 flex items-center justify-center rounded-[var(--radius-btn)] text-[var(--fg-secondary)] hover:bg-[var(--sidebar-hover)] transition-colors"
          onClick={handleAddColumn}
        >
          <Plus className="h-2.5 w-2.5" />
        </TipBtn>
        <TipBtn
          tip={t("structure.deleteSelectedColumn")}
          shortcut="⌫"
          className={cn(
            "h-4 w-4 flex items-center justify-center rounded-[var(--radius-btn)] transition-colors",
            selectedUid ? "text-[var(--fg-secondary)] hover:bg-[var(--danger)]/10 hover:text-[var(--danger)]" : "text-[var(--fg-muted)] opacity-40 cursor-not-allowed"
          )}
          onClick={handleDeleteSelected}
          disabled={!selectedUid}
        >
          <Trash2 className="h-2.5 w-2.5" />
        </TipBtn>
        {hasEdits && (
          <TipBtn
            tip={t("structure.revertAll")}
            className="h-4 w-4 flex items-center justify-center rounded-[var(--radius-btn)] text-[var(--fg-secondary)] hover:bg-[var(--sidebar-hover)] transition-colors"
            onClick={handleRevertAll}
          >
            <Undo2 className="h-2.5 w-2.5" />
          </TipBtn>
        )}
      </div>

      <div ref={topGridRef} className="overflow-auto flex-shrink-0" style={{ height: topHeight > 0 ? topHeight : "60%" }}>
        <table className="w-full border-collapse" style={{ minWidth: "max-content", tableLayout: "fixed" }}>
          <colgroup>
            <col style={{ width: 32 }} />
            {STRUCTURE_COL_DEFS.map((def) => (
              <col key={def.key} style={{ width: def.minWidth }} />
            ))}
          </colgroup>
          <thead className="sticky top-0 z-10">
            <tr>
              <th className="data-grid-header border-b border-[var(--border-color)] shadow-[-1px_0_0_0_var(--border-color)_inset]">#</th>
              {STRUCTURE_COL_DEFS.map((def) => (
                <th key={def.key} className="data-grid-header border-b border-[var(--border-color)] shadow-[-1px_0_0_0_var(--border-color)_inset]">
                  {def.label}
                </th>
              ))}
            </tr>
          </thead>
          <tbody>
            {visibleCols.map((col, idx) => {
              const isDeleted = col.__status === "deleted";
              const isNew = col.__status === "new";
              const isSelected = selectedUid === col.__uid;
              return (
                <tr
                  key={col.__uid}
                  className={cn(
                    "group transition-colors cursor-default",
                    isSelected
                      ? "bg-[var(--row-selected)] ring-1 ring-inset ring-[var(--accent)]"
                      : idx % 2 === 0 ? "bg-[var(--surface)]" : "bg-[var(--row-stripe)]",
                    !isSelected && !isDeleted && "hover:bg-[var(--row-hover)]",
                    isDeleted && "bg-[var(--row-delete-bg)]",
                    isNew && "bg-[var(--row-new-bg)]",
                  )}
                  onClick={() => {
                    setSelectedUid(col.__uid);
                    setSelectedIndexUid(null);
                  }}
                >
                  <td className="data-grid-cell text-center text-[var(--fg-muted)] border-r border-[var(--border-subtle)]">
                    <div className="flex items-center justify-center gap-0.5">
                      {col.isPrimary && <Key className="h-2.5 w-2.5 text-[var(--warning)]" />}
                      {!col.isPrimary && <span>{idx + 1}</span>}
                    </div>
                  </td>
                  {STRUCTURE_COL_DEFS.map((def) => {
                    const cellValue = col[def.key as keyof EditingStructureCol];
                    const isEditing = editingCell?.uid === col.__uid && editingCell?.key === def.key;
                    const isCheckbox = !!def.isCheckbox;
                    const isTypeSelect = !!def.isTypeSelect;
                    const isEditableCell = def.editable && !isDeleted;
                    const orig = originalCols.find((o) => o.__uid === col.__uid);
                    const cellModified = orig && def.editable &&
                      String(col[def.key as keyof EditingStructureCol] ?? "") !== String(orig[def.key as keyof EditingStructureCol] ?? "");

                    return (
                      <td
                        key={def.key}
                        className={cn(
                          "data-grid-cell overflow-hidden relative",
                          isEditableCell && "cursor-text",
                          isDeleted && "text-[var(--fg-muted)]",
                          cellModified && !isNew && "border-l-2 border-l-[var(--warning)] bg-[var(--cell-edit-bg)]/30",
                          isNew && "bg-[var(--row-new-bg)]",
                        )}
                        onClick={(e) => {
                          if (isEditableCell && (isTypeSelect || isCheckbox)) {
                            e.stopPropagation();
                            handleCellDoubleClick(col.__uid, def.key, cellValue);
                          }
                        }}
                        onDoubleClick={() => isEditableCell && !isTypeSelect && !isCheckbox && handleCellDoubleClick(col.__uid, def.key, cellValue)}
                      >
                        {isEditing && isTypeSelect ? (
                          <>
                            <div className="absolute inset-[1px] z-10">
                              <div className="relative flex h-full items-center">
                                <input
                                  ref={typeInputRef as React.RefObject<HTMLInputElement>}
                                  className={cn(
                                    "w-full h-full border border-[var(--accent)] outline-none text-[length:var(--size-font-xs)] px-1.5 rounded-[var(--radius-sm)] box-border",
                                    "bg-[var(--surface)] text-[var(--fg)] font-medium pr-5"
                                  )}
                                  value={editValue}
                                  onChange={(e) => {
                                    const val = e.target.value;
                                    setEditValue(val);
                                    setTypeFilter(val);
                                    setTypeDropdownOpen(true);
                                    setTypeHighlightIdx(-1);
                                    requestAnimationFrame(() => updateDropdownPos());
                                  }}
                                  onFocus={() => {
                                    setTypeFilter("");
                                    setTypeDropdownOpen(true);
                                    setTypeHighlightIdx(-1);
                                    requestAnimationFrame(() => updateDropdownPos());
                                  }}
                                  onKeyDown={(e) => {
                                    if (e.key === "ArrowDown") {
                                      e.preventDefault();
                                      setTypeDropdownOpen(true);
                                      setTypeHighlightIdx((prev) => Math.min(prev + 1, filteredTypes.length - 1));
                                    } else if (e.key === "ArrowUp") {
                                      e.preventDefault();
                                      setTypeHighlightIdx((prev) => Math.max(prev - 1, 0));
                                    } else if (e.key === "Enter") {
                                      e.preventDefault();
                                      if (typeHighlightIdx >= 0 && typeHighlightIdx < filteredTypes.length) {
                                        setEditValue(filteredTypes[typeHighlightIdx]);
                                      }
                                      commitCellEdit();
                                    } else if (e.key === "Escape") {
                                      cancelCellEdit();
                                    } else if (e.key === "Tab") {
                                      e.preventDefault();
                                      if (typeHighlightIdx >= 0 && typeHighlightIdx < filteredTypes.length) {
                                        setEditValue(filteredTypes[typeHighlightIdx]);
                                      }
                                      commitCellEdit();
                                    }
                                  }}
                                  onClick={(e) => e.stopPropagation()}
                                />
                                <button
                                  className="absolute right-0 top-0 bottom-0 w-5 flex items-center justify-center text-[var(--fg-muted)] hover:text-[var(--fg)] transition-colors"
                                  tabIndex={-1}
                                  onMouseDown={(e) => {
                                    e.preventDefault();
                                    e.stopPropagation();
                                    setTypeDropdownOpen((prev) => {
                                      if (!prev) requestAnimationFrame(() => updateDropdownPos());
                                      return !prev;
                                    });
                                  }}
                                >
                                  <ChevronDown className="h-3 w-3" />
                                </button>
                              </div>
                            </div>
                            {typeDropdownOpen && createPortal(
                              <div
                                ref={typeDropdownRef}
                                className="fixed z-[9999] max-h-[240px] overflow-auto rounded-[var(--radius-menu)] border border-[var(--border-color)] bg-[var(--surface)] shadow-lg"
                                style={{ top: typeDropdownPos.top, left: typeDropdownPos.left, width: typeDropdownPos.width }}
                              >
                                {filteredTypes.length === 0 ? (
                                  <div className="px-2 py-2 text-xs text-[var(--fg-muted)] text-center">
                                    {t("structure.noMatchingTypes")}
                                  </div>
                                ) : (
                                  filteredTypes.map((item, itemIdx) => (
                                    <div
                                      key={item}
                                      className={cn(
                                        "px-2 py-[5px] text-xs cursor-pointer transition-colors",
                                        itemIdx === typeHighlightIdx
                                          ? "bg-[var(--accent)] text-white"
                                          : item.toLowerCase() === editValue.toLowerCase()
                                            ? "bg-[var(--accent)]/15 text-[var(--accent)]"
                                            : "hover:bg-[var(--row-hover)] text-[var(--fg)]"
                                      )}
                                      onMouseDown={(e) => {
                                        e.preventDefault();
                                        setEditValue(item);
                                        setWorkingCols((prev) => prev.map((c) => {
                                          if (c.__uid !== col.__uid) return c;
                                          const updated = { ...c, type: item };
                                          if (c.__status !== "new") {
                                            const origC = originalCols.find((o) => o.__uid === col.__uid);
                                            if (origC) {
                                              const changed = updated.name !== origC.name || updated.type !== origC.type ||
                                                updated.nullable !== origC.nullable || (updated.defaultValue ?? "") !== (origC.defaultValue ?? "") ||
                                                updated.comment !== origC.comment;
                                              updated.__status = changed ? "modified" : undefined;
                                            }
                                          }
                                          return updated;
                                        }));
                                        setEditingCell(null);
                                        setTypeDropdownOpen(false);
                                      }}
                                      onMouseEnter={() => setTypeHighlightIdx(itemIdx)}
                                    >
                                      {item}
                                    </div>
                                  ))
                                )}
                              </div>,
                              document.body
                            )}
                          </>
                        ) : isEditing && isCheckbox ? null : isEditing ? (
                          <input
                            ref={editInputRef as React.RefObject<HTMLInputElement>}
                            className={cn(
                              "w-full h-full border-2 border-[var(--accent)] outline-none text-xs px-1 rounded-sm",
                              "bg-[var(--surface)] text-[var(--fg)] font-medium",
                              "absolute inset-0 z-20"
                            )}
                            value={editValue}
                            onChange={(e) => setEditValue(e.target.value)}
                            onBlur={commitCellEdit}
                            onKeyDown={(e) => {
                              if (e.key === "Enter") { e.preventDefault(); commitCellEdit(); }
                              if (e.key === "Escape") cancelCellEdit();
                              if (e.key === "Tab") { e.preventDefault(); commitCellEdit(); }
                            }}
                            onClick={(e) => e.stopPropagation()}
                          />
                        ) : isCheckbox ? (
                          <div className="flex items-center justify-center h-full">
                            <span className={cn("cursor-pointer select-none", isEditableCell ? "" : "opacity-50 cursor-not-allowed")} onClick={(e) => {
                              e.stopPropagation();
                              if (isEditableCell) handleCellDoubleClick(col.__uid, def.key, cellValue);
                            }}>
                              {cellValue ? "YES" : "NO"}
                            </span>
                          </div>
                        ) : isTypeSelect ? (
                          <div className="flex items-center h-full group/type">
                            <span className="truncate flex-1">
                              {cellValue === null || cellValue === undefined || cellValue === "" ? (
                                <span className="text-[var(--fg-muted)] italic opacity-50">{t("query.empty")}</span>
                              ) : String(cellValue)}
                            </span>
                            {isEditableCell && (
                              <ChevronDown className="h-3 w-3 text-[var(--fg-muted)] opacity-0 group-hover/type:opacity-100 transition-opacity flex-shrink-0 ml-0.5" />
                            )}
                          </div>
                        ) : (
                          <span className={cn("truncate block", def.key === "name" && "font-medium")}>
                            {cellValue === null || cellValue === undefined || cellValue === "" ? (
                              <span className="text-[var(--fg-muted)] italic opacity-50">{t("query.empty")}</span>
                            ) : String(cellValue)}
                          </span>
                        )}
                      </td>
                    );
                  })}
                </tr>
              );
            })}
            {Array.from({ length: columnFillerRows }).map((_, fillerIdx) => {
              const visualIdx = visibleCols.length + fillerIdx;
              return (
                <tr
                  key={`col_filler_${fillerIdx}`}
                  className={cn(
                    "transition-colors cursor-default",
                    visualIdx % 2 === 0 ? "bg-[var(--surface)]" : "bg-[var(--row-stripe)]",
                    "hover:bg-[var(--row-hover)]"
                  )}
                  onDoubleClick={handleAddColumn}
                >
                  <td className="data-grid-cell text-center text-[var(--fg-muted)] border-r border-[var(--border-subtle)]">
                    {visualIdx + 1}
                  </td>
                  {STRUCTURE_COL_DEFS.map((def) => (
                    <td key={`col_filler_${fillerIdx}_${def.key}`} className="data-grid-cell">
                      <span className="opacity-0 select-none">.</span>
                    </td>
                  ))}
                </tr>
              );
            })}
          </tbody>
        </table>
      </div>

      <div className="h-[3px] flex-shrink-0 cursor-row-resize group relative bg-[var(--surface-secondary)]" onMouseDown={handleSplitDragStart}>
        <div className="absolute inset-x-0 top-1/2 -translate-y-1/2 h-px bg-[var(--border-color)] transition-colors group-hover:bg-[var(--accent)]/50" />
        <div className="absolute left-1/2 top-1/2 -translate-x-1/2 -translate-y-1/2 w-5 h-1 rounded-full opacity-0 group-hover:opacity-100 bg-[var(--accent)]/40 transition-opacity" />
      </div>

      <div className="flex items-center h-6 px-2 gap-1 border-b border-[var(--border-color)] bg-[var(--surface-secondary)] flex-shrink-0">
        <span className="text-[length:var(--size-font-xs)] font-medium text-[var(--fg-secondary)]">
          Indexes ({visibleIndexes.length})
        </span>
        <div className="flex-1" />
        <TipBtn
          tip={t("structure.addIndex")}
          className="h-4 w-4 flex items-center justify-center rounded-[var(--radius-btn)] text-[var(--fg-secondary)] hover:bg-[var(--sidebar-hover)] transition-colors"
          onClick={handleAddIndex}
        >
          <Plus className="h-2.5 w-2.5" />
        </TipBtn>
        <TipBtn
          tip={t("structure.deleteSelectedColumn")}
          shortcut="⌫"
          className={cn(
            "h-4 w-4 flex items-center justify-center rounded-[var(--radius-btn)] transition-colors",
            selectedIndexUid
              ? "text-[var(--fg-secondary)] hover:bg-[var(--danger)]/10 hover:text-[var(--danger)]"
              : "text-[var(--fg-muted)] opacity-40 cursor-not-allowed"
          )}
          onClick={() => void handleDeleteSelectedIndex()}
          disabled={!selectedIndexUid}
        >
          <Trash2 className="h-2.5 w-2.5" />
        </TipBtn>
      </div>

      <div ref={indexGridRef} className="flex-1 overflow-auto">
        <table className="w-full border-collapse" style={{ minWidth: "max-content", tableLayout: "fixed" }}>
          <colgroup>
            {INDEX_COL_DEFS.map((def) => (
              <col key={def.key} style={{ width: def.minWidth }} />
            ))}
          </colgroup>
          <thead className="sticky top-0 z-10">
            <tr>
              {INDEX_COL_DEFS.map((def) => (
                <th key={def.key} className="data-grid-header border-b border-[var(--border-color)] shadow-[-1px_0_0_0_var(--border-color)_inset]">
                  {def.label}
                </th>
              ))}
            </tr>
          </thead>
          <tbody>
            {visibleIndexes.map((idx, i) => {
              const isNew = idx.__status === "new";
              const isDeleted = idx.__status === "deleted";
              const isEditingName = editingIndexCell?.uid === idx.__uid && editingIndexCell.key === "name";
              const isEditingColumns = editingIndexCell?.uid === idx.__uid && editingIndexCell.key === "columns";
              const columnsText = (idx.columns || []).join(", ");
              return (
                <tr
                  key={idx.__uid}
                  className={cn(
                    "group transition-colors cursor-default",
                    selectedIndexUid === idx.__uid
                      ? "bg-[var(--row-selected)] ring-1 ring-inset ring-[var(--accent)]"
                      : isDeleted
                        ? "bg-[var(--row-delete-bg)]"
                      : isNew
                        ? "bg-[var(--row-new-bg)]"
                        : i % 2 === 0 ? "bg-[var(--surface)]" : "bg-[var(--row-stripe)]",
                    selectedIndexUid !== idx.__uid && !isNew && !isDeleted && "hover:bg-[var(--row-hover)]"
                  )}
                  onClick={() => {
                    setSelectedIndexUid(idx.__uid);
                    setSelectedUid(null);
                  }}
                >
                  <td className={cn("data-grid-cell font-medium relative", isDeleted && "text-[var(--fg-muted)]")} title={idx.name || t("structure.indexNamePlaceholder")}>
                    {isNew && !isDeleted && isEditingName ? (
                      <input
                        ref={indexInputRef}
                        className={cn(inputCls, "w-full")}
                        value={indexEditValue}
                        onChange={(e) => setIndexEditValue(e.target.value)}
                        onBlur={commitEditIndexCell}
                        onKeyDown={(e) => {
                          if (e.key === "Enter") {
                            e.preventDefault();
                            commitEditIndexCell();
                          } else if (e.key === "Escape") {
                            e.preventDefault();
                            cancelEditIndexCell();
                          }
                        }}
                        placeholder={t("structure.indexNamePlaceholder")}
                      />
                    ) : (
                      <div className={cn("flex items-center gap-1", isNew && !isDeleted && "cursor-text")} onDoubleClick={() => isNew && !isDeleted && startEditIndexCell(idx.__uid, "name", idx.name)}>
                        {idx.isPrimary && <Key className="h-2.5 w-2.5 text-[var(--warning)] flex-shrink-0" />}
                        {!idx.isPrimary && idx.isUnique && <Hash className="h-2.5 w-2.5 text-[var(--accent)] flex-shrink-0" />}
                        <span className={cn("truncate", !idx.name && "text-[var(--fg-muted)]")}>{idx.name || t("structure.indexNamePlaceholder")}</span>
                      </div>
                    )}
                  </td>
                  <td className={cn("data-grid-cell text-[var(--fg-muted)]", isDeleted && "text-[var(--fg-muted)]")}>{idx.type || "BTREE"}</td>
                  <td className={cn("data-grid-cell text-center", isDeleted && "text-[var(--fg-muted)]")}>
                    {isNew && !isDeleted ? (
                      <label className="inline-flex items-center justify-center cursor-pointer">
                        <input type="checkbox" checked={idx.isUnique} onChange={(e) => handleToggleInlineIndexUnique(idx.__uid, e.target.checked)} />
                      </label>
                    ) : (
                      idx.isUnique ? "TRUE" : "FALSE"
                    )}
                  </td>
                  <td className={cn("data-grid-cell text-[var(--fg-secondary)] relative", isDeleted && "text-[var(--fg-muted)]")} title={columnsText || t("structure.indexColumnsPlaceholder")}>
                    {isNew && !isDeleted && isEditingColumns ? (
                      <input
                        ref={indexInputRef}
                        className={cn(inputCls, "w-full")}
                        value={indexEditValue}
                        onChange={(e) => setIndexEditValue(e.target.value)}
                        onBlur={commitEditIndexCell}
                        onKeyDown={(e) => {
                          if (e.key === "Enter") {
                            e.preventDefault();
                            commitEditIndexCell();
                          } else if (e.key === "Escape") {
                            e.preventDefault();
                            cancelEditIndexCell();
                          }
                        }}
                        placeholder={t("structure.indexColumnsPlaceholder")}
                      />
                    ) : (
                      <div className={cn("truncate", isNew && !isDeleted && "cursor-text", !columnsText && "text-[var(--fg-muted)]")} onDoubleClick={() => isNew && !isDeleted && startEditIndexCell(idx.__uid, "columns", columnsText)}>
                        {columnsText || t("structure.indexColumnsPlaceholder")}
                      </div>
                    )}
                  </td>
                </tr>
              );
            })}
            {Array.from({ length: indexFillerRows }).map((_, fillerIdx) => {
              const visualIdx = visibleIndexes.length + fillerIdx;
              return (
                <tr
                  key={`idx_filler_${fillerIdx}`}
                  className={cn(
                    "transition-colors cursor-default",
                    visualIdx % 2 === 0 ? "bg-[var(--surface)]" : "bg-[var(--row-stripe)]",
                    "hover:bg-[var(--row-hover)]"
                  )}
                  onDoubleClick={handleAddIndex}
                >
                  {INDEX_COL_DEFS.map((def) => (
                    <td key={`idx_filler_${fillerIdx}_${def.key}`} className="data-grid-cell">
                      <span className="opacity-0 select-none">.</span>
                    </td>
                  ))}
                </tr>
              );
            })}
          </tbody>
        </table>
      </div>
    </div>
  );
}
