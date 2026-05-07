import type { ColumnInfo, ColumnMeta } from "@/types/database";

type TemporalKind = "date" | "time" | "datetime";

function pad2(v: number): string {
  return String(v).padStart(2, "0");
}

function resolveTemporalKind(type: string | undefined): TemporalKind | null {
  const t = (type || "").trim().toLowerCase();
  if (t.includes("datetime") || t.includes("timestamp")) return "datetime";
  if (t.includes("date") && !t.includes("datetime") && !t.includes("timestamp")) return "date";
  if (t.includes("time") && !t.includes("datetime") && !t.includes("timestamp")) return "time";
  return null;
}

function formatCurrentTemporalValue(kind: TemporalKind, now: Date): string {
  const date = `${now.getFullYear()}-${pad2(now.getMonth() + 1)}-${pad2(now.getDate())}`;
  const time = `${pad2(now.getHours())}:${pad2(now.getMinutes())}:${pad2(now.getSeconds())}`;
  if (kind === "date") return date;
  if (kind === "time") return time;
  return `${date} ${time}`;
}

function normalizeColumnDefault(rawDefault: string | null | undefined): string | null {
  if (rawDefault === null || rawDefault === undefined) return null;
  const trimmed = String(rawDefault).trim();
  if (!trimmed) return null;
  return trimmed.replace(/^['"]|['"]$/g, "").trim();
}

function isCurrentTemporalDefault(rawDefault: string | null | undefined): boolean {
  const normalized = normalizeColumnDefault(rawDefault);
  if (!normalized) return false;
  return /^(current_(timestamp|date|time)(?:\(\d*\))?|local(timestamp|time)(?:\(\d*\))?|now\(\))$/i.test(normalized);
}

export function buildNewTableRow(
  columns: ColumnMeta[],
  structureColumns: ColumnInfo[],
  now: Date = new Date(),
): Record<string, unknown> {
  const structureByName = new Map<string, ColumnInfo>();
  for (const info of structureColumns) {
    structureByName.set(info.name, info);
  }

  const row: Record<string, unknown> = {};
  for (const col of columns) {
    const info = structureByName.get(col.name);
    const kind = resolveTemporalKind(info?.type || col.type);
    const defaultValue = info?.defaultValue ?? null;
    const nullable = info?.nullable ?? col.nullable ?? true;

    if (info?.isAutoIncrement) {
      row[col.name] = null;
    } else if (kind && (isCurrentTemporalDefault(defaultValue) || (!nullable && normalizeColumnDefault(defaultValue) === null))) {
      row[col.name] = formatCurrentTemporalValue(kind, now);
    } else {
      row[col.name] = null;
    }
  }
  return row;
}
