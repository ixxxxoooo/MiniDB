import { useCallback, useEffect, useMemo, useState } from "react";
import { useUIStore } from "@/stores/ui";
import type { ColumnInfo, ColumnMeta } from "@/types/database";
import * as QueryService from "@/lib/wails/services/QueryService";
import { reportTabError } from "./tabFeedback";

interface RowUpdatePayload {
  primaryKey: Record<string, unknown>;
  changes: Record<string, unknown>;
}
import type { FilterCondition } from "@/components/table/DataGridToolbar";

// 表数据编辑状态：单元格编辑、新增行、删除行、提交事务
export function useTableDataEditor(params: {
  connectionId?: string;
  database?: string;
  table?: string;
  columns: ColumnMeta[];
  structureColumns: ColumnInfo[];
  initialData: Record<string, unknown>[];
  originalData: Record<string, unknown>[];
}) {
  const {
    connectionId,
    database,
    table,
    columns,
    structureColumns,
    initialData,
    originalData,
  } = params;

  const [data, setData] = useState<Record<string, unknown>[]>(initialData);
  const [editedCells, setEditedCells] = useState<Record<string, unknown>>({});
  const [newRowIndexes, setNewRowIndexes] = useState<Set<number>>(new Set());
  const [pendingDeleteIndexes, setPendingDeleteIndexes] = useState<Set<number>>(new Set());

  useEffect(() => {
    setData(initialData);
  }, [initialData]);

  const resetEditState = useCallback(() => {
    setEditedCells({});
    setNewRowIndexes(new Set());
    setPendingDeleteIndexes(new Set());
  }, []);

  const syncData = useCallback((rows: Record<string, unknown>[]) => {
    setData(rows);
  }, []);

  const handleCellEdit = useCallback((rowIdx: number, column: string, value: unknown) => {
    const key = `${rowIdx}:${column}`;
    setEditedCells((prev) => ({ ...prev, [key]: value }));
    setData((prev) => {
      const next = [...prev];
      next[rowIdx] = { ...next[rowIdx], [column]: value };
      return next;
    });
  }, []);

  const handleAddRow = useCallback((setSelectedRowIndex: (index: number | null) => void) => {
    const emptyRow: Record<string, unknown> = {};
    for (const col of columns) {
      emptyRow[col.name] = null;
    }
    setData((prev) => {
      const newIdx = prev.length;
      setNewRowIndexes((state) => new Set(state).add(newIdx));
      setSelectedRowIndex(newIdx);
      return [...prev, emptyRow];
    });
  }, [columns]);

  const handleDeleteSelectedRow = useCallback((selectedRowIndex: number | null, setSelectedRowIndex: (index: number | null) => void) => {
    if (selectedRowIndex === null) return;
    const isNewRow = newRowIndexes.has(selectedRowIndex);
    if (isNewRow) {
      setData((prev) => prev.filter((_, i) => i !== selectedRowIndex));
      setNewRowIndexes((prev) => {
        const next = new Set<number>();
        for (const idx of prev) {
          if (idx < selectedRowIndex) next.add(idx);
          else if (idx > selectedRowIndex) next.add(idx - 1);
        }
        return next;
      });
      setEditedCells((prev) => {
        const next: Record<string, unknown> = {};
        for (const [key, val] of Object.entries(prev)) {
          const rowIdx = parseInt(key.split(":")[0]);
          const col = key.split(":")[1];
          if (rowIdx < selectedRowIndex) next[key] = val;
          else if (rowIdx > selectedRowIndex) next[`${rowIdx - 1}:${col}`] = val;
        }
        return next;
      });
      setSelectedRowIndex(null);
    } else {
      setPendingDeleteIndexes((prev) => new Set(prev).add(selectedRowIndex));
    }
  }, [newRowIndexes]);

  const commitChanges = useCallback(async () => {
    if (!connectionId || !database || !table) return false;
    if (Object.keys(editedCells).length === 0 && newRowIndexes.size === 0 && pendingDeleteIndexes.size === 0) return false;

    const pkCols = structureColumns.filter((c: any) => c.isPrimary).map((c: any) => c.name);

    try {
      const deletePKs: Record<string, unknown>[] = [];
      for (const delIdx of pendingDeleteIndexes) {
        if (newRowIndexes.has(delIdx)) continue;
        if (pkCols.length === 0) {
          useUIStore.getState().addToast("error", "无法删除：未找到主键列");
          return false;
        }
        const origRow = originalData[delIdx];
        if (!origRow) continue;
        const pk: Record<string, unknown> = {};
        for (const col of pkCols) pk[col] = origRow[col];
        deletePKs.push(pk);
      }

      const inserts: Record<string, unknown>[] = [];
      for (const newIdx of newRowIndexes) {
        if (pendingDeleteIndexes.has(newIdx)) continue;
        const row = data[newIdx];
        if (!row) continue;
        const rowData: Record<string, unknown> = {};
        for (const col of columns) {
          if (row[col.name] !== null && row[col.name] !== undefined) {
            rowData[col.name] = row[col.name];
          }
        }
        if (Object.keys(rowData).length === 0) continue;
        inserts.push(rowData);
      }

      const updates: { primaryKey: Record<string, unknown>; changes: Record<string, unknown> }[] = [];
      if (pkCols.length > 0) {
        const changesByRow: Record<number, Record<string, unknown>> = {};
        for (const [key, val] of Object.entries(editedCells)) {
          const [rowStr, col] = key.split(":");
          const rowIdx = parseInt(rowStr);
          if (newRowIndexes.has(rowIdx) || pendingDeleteIndexes.has(rowIdx)) continue;
          if (!changesByRow[rowIdx]) changesByRow[rowIdx] = {};
          changesByRow[rowIdx][col] = val;
        }
        for (const [rowIdxStr, changes] of Object.entries(changesByRow)) {
          const rowIdx = parseInt(rowIdxStr);
          const origRow = originalData[rowIdx];
          if (!origRow) continue;
          const pk: Record<string, unknown> = {};
          for (const col of pkCols) pk[col] = origRow[col];
          updates.push({ primaryKey: pk, changes });
        }
      }

      await QueryService.CommitTableDataChanges(
        connectionId,
        database,
        table,
        deletePKs as any,
        inserts as any,
        updates as any
      );

      resetEditState();
      return true;
    } catch (e: any) {
      reportTabError({
        logTitle: "[TableView] 事务提交失败:",
        toastMessage: "事务提交失败",
        error: e,
      });
      return false;
    }
  }, [
    columns,
    connectionId,
    data,
    database,
    editedCells,
    newRowIndexes,
    originalData,
    pendingDeleteIndexes,
    resetEditState,
    structureColumns,
    table,
  ]);

  const hasEdits = useMemo(() => {
    return Object.keys(editedCells).length > 0 || newRowIndexes.size > 0 || pendingDeleteIndexes.size > 0;
  }, [editedCells, newRowIndexes, pendingDeleteIndexes]);

  return {
    data,
    setData,
    syncData,
    editedCells,
    newRowIndexes,
    pendingDeleteIndexes,
    hasEdits,
    resetEditState,
    handleCellEdit,
    handleAddRow,
    handleDeleteSelectedRow,
    commitChanges,
  };
}
