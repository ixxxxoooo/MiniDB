import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import { useUIStore } from "@/stores/ui";
import { useConnectionStore } from "@/stores/connection";
import type { ColumnInfo, ColumnMeta } from "@/types/database";
import * as QueryService from "@/lib/wails/services/QueryService";
import { reportTabError } from "./tabFeedback";
import { t } from "@/i18n";
import { makeRowRemap, remapEditedCells, remapIndexSet } from "./rowRemap";

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

  // 表结构已加载但没有任何主键列：已有行无法被唯一定位，编辑/删除只会被静默丢弃，必须在入口拒绝。
  const lacksPrimaryKey = useMemo(
    () => structureColumns.length > 0 && !structureColumns.some((c: any) => c.isPrimary),
    [structureColumns],
  );

  const isReadOnlyConnection = () =>
    Boolean(useConnectionStore.getState().connections.find((c) => c.id === connectionId)?.readOnly);

  const handleCellEdit = useCallback((rowIdx: number, column: string, value: unknown) => {
    if (isReadOnlyConnection()) {
      useUIStore.getState().addToast("error", t("connection.readOnlyBlocked"));
      return;
    }
    if (lacksPrimaryKey && !newRowIndexes.has(rowIdx)) {
      useUIStore.getState().addToast("error", t("table.noPrimaryKey"));
      return;
    }
    const key = `${rowIdx}:${column}`;
    setEditedCells((prev) => ({ ...prev, [key]: value }));
    setData((prev) => {
      const next = [...prev];
      next[rowIdx] = { ...next[rowIdx], [column]: value };
      return next;
    });
  }, [lacksPrimaryKey, newRowIndexes]);

  // 缓存后端返回的新行默认值
  const cachedDefaults = useRef<Record<string, unknown> | null>(null);
  useEffect(() => {
    if (!connectionId || !database || !table) return;
    QueryService.BuildNewRowDefaults(connectionId, database, table)
      .then((defaults) => { cachedDefaults.current = defaults; })
      .catch(() => { cachedDefaults.current = null; });
  }, [connectionId, database, table]);

  const handleAddRow = useCallback((setSelectedRowIndex: (index: number | null) => void) => {
    if (isReadOnlyConnection()) {
      useUIStore.getState().addToast("error", t("connection.readOnlyBlocked"));
      return;
    }
    const emptyRow: Record<string, unknown> = cachedDefaults.current
      ? { ...cachedDefaults.current }
      : Object.fromEntries(columns.map((c) => [c.name, null]));
    setData((prev) => {
      const newIdx = prev.length;
      setNewRowIndexes((state) => new Set(state).add(newIdx));
      setSelectedRowIndex(newIdx);
      return [...prev, emptyRow];
    });
  }, [columns]);

  // 删除一批行：新增行（尚未入库）直接移除并重排索引；已有行标记为待删除，提交时才真正删除。
  // 任一已有行无法定位（无主键）时整体拒绝，避免部分生效。onNewRowsRemoved 在新增行被移除后调用，
  // 因为此时行下标发生了位移，调用方需要清掉旧的选中状态。
  const handleDeleteRows = useCallback((indexes: number[], onNewRowsRemoved?: () => void) => {
    const unique = Array.from(new Set(indexes)).filter((i) => i >= 0);
    if (unique.length === 0) return;
    if (isReadOnlyConnection()) {
      useUIStore.getState().addToast("error", t("connection.readOnlyBlocked"));
      return;
    }
    const newRows = unique.filter((i) => newRowIndexes.has(i));
    const existingRows = unique.filter((i) => !newRowIndexes.has(i));
    if (existingRows.length > 0 && lacksPrimaryKey) {
      useUIStore.getState().addToast("error", t("table.noPrimaryKey"));
      return;
    }

    if (newRows.length > 0) {
      const removed = new Set(newRows);
      const remap = makeRowRemap(removed);
      setData((prev) => prev.filter((_, i) => !removed.has(i)));
      setNewRowIndexes((prev) => remapIndexSet(prev, remap));
      setEditedCells((prev) => remapEditedCells(prev, remap));
      setPendingDeleteIndexes((prev) => remapIndexSet(prev, remap));
      onNewRowsRemoved?.();
    }
    if (existingRows.length > 0) {
      setPendingDeleteIndexes((prev) => {
        const next = new Set(prev);
        for (const i of existingRows) next.add(i);
        return next;
      });
    }
  }, [lacksPrimaryKey, newRowIndexes]);

  const commitChanges = useCallback(async () => {
    if (!connectionId || !database || !table) return false;
    if (Object.keys(editedCells).length === 0 && newRowIndexes.size === 0 && pendingDeleteIndexes.size === 0) return false;

    const pkCols = structureColumns.filter((c: any) => c.isPrimary).map((c: any) => c.name);

    try {
      const deletePKs: Record<string, unknown>[] = [];
      for (const delIdx of pendingDeleteIndexes) {
        if (newRowIndexes.has(delIdx)) continue;
        if (pkCols.length === 0) {
          useUIStore.getState().addToast("error", t("table.noPrimaryKey"));
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
      if (pkCols.length === 0 && Object.keys(editedCells).some((key) => !newRowIndexes.has(parseInt(key.split(":")[0])))) {
        // 防御：即使入口被绕过，也绝不能把"无法保存的修改"当作成功并清空编辑状态
        useUIStore.getState().addToast("error", t("table.noPrimaryKey"));
        return false;
      }
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
        toastMessage: t("err.commitFailed"),
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
    handleDeleteRows,
    commitChanges,
  };
}
