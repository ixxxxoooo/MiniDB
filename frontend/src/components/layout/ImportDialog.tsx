import React, { useCallback, useEffect, useMemo, useRef, useState } from "react";
import { AlertCircle, FileUp, Loader2, X } from "lucide-react";
import { Button } from "@/components/ui/button";
import { useTranslation } from "@/i18n";
import { cn } from "@/lib/utils";
import { EventsOn } from "@/lib/wails/runtime";
import * as DatabaseService from "@/lib/wails/services/DatabaseService";
import * as ImportService from "@/lib/wails/services/ImportService";
import { useUIStore } from "@/stores/ui";

interface TargetColumn {
  name: string;
  nullable: boolean;
  hasDefault: boolean;
  isAutoIncrement: boolean;
}

interface Preview {
  format: string;
  columns: string[];
  rows: string[][];
  totalRows: number;
}

interface ImportProgress {
  taskId: string;
  status: "progress" | "done" | "error" | "cancelled";
  current: number;
  total: number;
  error: string;
  rolledBack: boolean;
}

const SKIP = -1;

/** 把 CSV/JSON 文件导入到已有表：选文件 → 预览与列映射 → 后台导入并显示进度。 */
export function ImportDialog({ target, onClose }: { target: { connId: string; database: string; table: string }; onClose: () => void }) {
  const { t } = useTranslation();
  const [filePath, setFilePath] = useState("");
  const [hasHeader, setHasHeader] = useState(true);
  const [delimiter, setDelimiter] = useState("");
  const [preview, setPreview] = useState<Preview | null>(null);
  const [targetCols, setTargetCols] = useState<TargetColumn[]>([]);
  const [mapping, setMapping] = useState<Record<string, number>>({});
  const [atomic, setAtomic] = useState(true);
  const [emptyAsNull, setEmptyAsNull] = useState(true);
  const [error, setError] = useState("");
  const [loading, setLoading] = useState(false);
  const [progress, setProgress] = useState<ImportProgress | null>(null);
  const running = progress?.status === "progress";
  const startedRef = useRef(false);

  const isCSV = preview ? preview.format === "csv" : true;

  // 目标表列信息
  useEffect(() => {
    DatabaseService.GetColumns(target.connId, target.database, target.table)
      .then((cols) =>
        setTargetCols(
          (cols || []).map((c: any) => ({
            name: c.name,
            nullable: !!c.nullable,
            hasDefault: c.defaultValue != null,
            isAutoIncrement: !!c.isAutoIncrement,
          })),
        ),
      )
      .catch((e: any) => setError(e?.message || String(e)));
  }, [target]);

  // 自动映射：优先同名（忽略大小写），无表头时按位置
  const autoMap = useCallback((p: Preview, cols: TargetColumn[], header: boolean) => {
    const next: Record<string, number> = {};
    cols.forEach((c, i) => {
      const byName = p.columns.findIndex((s) => s.toLowerCase() === c.name.toLowerCase());
      if (byName >= 0) next[c.name] = byName;
      else if (!header && p.format === "csv" && i < p.columns.length) next[c.name] = i;
      else next[c.name] = SKIP;
    });
    setMapping(next);
  }, []);

  const loadPreview = useCallback(
    async (path: string, header: boolean, delim: string) => {
      setLoading(true);
      setError("");
      try {
        const p = (await ImportService.PreviewImportFile(path, { format: "", hasHeader: header, delimiter: delim })) as Preview;
        setPreview(p);
        return p;
      } catch (e: any) {
        setPreview(null);
        setError(`${t("import.readFailed")}: ${e?.message || e}`);
        return null;
      } finally {
        setLoading(false);
      }
    },
    [t],
  );

  const pickFile = useCallback(async () => {
    try {
      const path = await ImportService.PickImportFile();
      if (!path) return false;
      setFilePath(path);
      const p = await loadPreview(path, hasHeader, delimiter);
      if (p) autoMap(p, targetCols, hasHeader);
      return true;
    } catch (e: any) {
      setError(e?.message || String(e));
      return false;
    }
  }, [loadPreview, autoMap, targetCols, hasHeader, delimiter]);

  // 打开时自动弹出文件选择；等目标列加载完再选，才能正确自动映射。
  useEffect(() => {
    if (startedRef.current || targetCols.length === 0) return;
    startedRef.current = true;
    void pickFile().then((picked) => {
      if (!picked) onClose();
    });
  }, [targetCols, pickFile, onClose]);

  // 修改表头/分隔符后重新解析
  const reparse = async (header: boolean, delim: string) => {
    if (!filePath) return;
    const p = await loadPreview(filePath, header, delim);
    if (p) autoMap(p, targetCols, header);
  };

  // 进度事件
  useEffect(() => {
    return EventsOn<ImportProgress & { fileName: string }>("import:progress", (ev) => {
      if (!ev || !ev.taskId || !ev.taskId.startsWith(`import_${target.table}_`)) return;
      setProgress(ev);
      if (ev.status === "done") {
        useUIStore.getState().addToast("success", t("import.done", { n: ev.current.toLocaleString(), table: target.table }), 5000);
        onClose();
      }
    });
  }, [target.table, t, onClose]);

  useEffect(() => {
    const onKey = (e: KeyboardEvent) => {
      if (e.key === "Escape" && !running) onClose();
    };
    window.addEventListener("keydown", onKey);
    return () => window.removeEventListener("keydown", onKey);
  }, [onClose, running]);

  const mappedColumns = useMemo(
    () => targetCols.filter((c) => (mapping[c.name] ?? SKIP) !== SKIP),
    [targetCols, mapping],
  );
  const missingRequired = useMemo(
    () => targetCols.filter((c) => (mapping[c.name] ?? SKIP) === SKIP && !c.nullable && !c.hasDefault && !c.isAutoIncrement),
    [targetCols, mapping],
  );

  const start = async () => {
    if (!preview || mappedColumns.length === 0) {
      setError(t("import.noMapping"));
      return;
    }
    setError("");
    try {
      await ImportService.StartImport({
        connId: target.connId,
        database: target.database,
        table: target.table,
        filePath,
        parse: { format: "", hasHeader, delimiter },
        columns: mappedColumns.map((c) => ({ source: mapping[c.name], target: c.name })),
        import: { atomic, emptyAsNull, batchSize: 0 },
      } as any);
      setProgress({ taskId: "", status: "progress", current: 0, total: preview.totalRows, error: "", rolledBack: false });
    } catch (e: any) {
      setError(e?.message || String(e));
    }
  };

  const stop = () => {
    if (progress?.taskId) void ImportService.CancelImport(progress.taskId);
  };

  const percent = progress && progress.total > 0 ? Math.min(100, Math.round((progress.current / progress.total) * 100)) : 0;
  const finished = progress && progress.status !== "progress";

  return (
    <>
      <div className="fixed inset-0 z-50 bg-black/40 backdrop-blur-sm" onClick={() => !running && onClose()} />
      <div
        className={cn(
          "fixed z-50 top-1/2 left-1/2 -translate-x-1/2 -translate-y-1/2 w-[860px] max-w-[96vw] max-h-[90vh]",
          "rounded-[var(--radius-panel)] shadow-lg border overflow-hidden flex flex-col",
          "bg-[var(--surface)] border-[var(--border-color)]",
        )}
      >
        <div className="flex items-center justify-between px-4 py-2.5 border-b border-[var(--border-color)] bg-[var(--surface-secondary)]">
          <div className="flex items-center gap-2 min-w-0">
            <FileUp className="h-4 w-4 text-[var(--fg-secondary)]" />
            <span className="text-sm font-medium">{t("import.title")}</span>
            <span className="text-2xs text-[var(--fg-muted)] truncate">
              {target.database ? `${target.database}.` : ""}
              {target.table}
            </span>
          </div>
          <Button variant="ghost" size="icon" className="h-7 w-7" onClick={onClose} disabled={running} aria-label="close">
            <X className="h-3.5 w-3.5" />
          </Button>
        </div>

        <div className="flex-1 overflow-auto p-4 space-y-3 text-xs">
          {error && (
            <div className="flex items-start gap-1.5 text-[var(--danger)] whitespace-pre-wrap">
              <AlertCircle className="h-3.5 w-3.5 mt-0.5 flex-shrink-0" />
              <span>{error}</span>
            </div>
          )}

          {loading && !preview && (
            <div className="flex items-center justify-center py-10 text-[var(--fg-muted)] gap-2">
              <Loader2 className="h-4 w-4 animate-spin" />
            </div>
          )}

          {filePath && (
            <div className="flex flex-wrap items-center gap-3">
              <span className="text-[var(--fg-secondary)]">{t("import.file")}</span>
              <span className="font-mono truncate max-w-[420px]" title={filePath}>{filePath.split(/[\\/]/).pop()}</span>
              <Button variant="outline" size="sm" className="h-6 text-2xs" disabled={running} onClick={() => void pickFile()}>
                {t("import.pickAgain")}
              </Button>
              {preview && <span className="text-[var(--fg-muted)]">{t("import.totalRows", { n: preview.totalRows.toLocaleString() })}</span>}
              {isCSV && (
                <>
                  <label className="flex items-center gap-1 cursor-pointer select-none">
                    <input
                      type="checkbox"
                      checked={hasHeader}
                      disabled={running}
                      onChange={(e) => {
                        setHasHeader(e.target.checked);
                        void reparse(e.target.checked, delimiter);
                      }}
                    />
                    {t("import.hasHeader")}
                  </label>
                  <label className="flex items-center gap-1">
                    {t("import.delimiter")}
                    <select
                      className="h-6 rounded border border-[var(--border-color)] bg-[var(--surface)] px-1"
                      value={delimiter}
                      disabled={running}
                      onChange={(e) => {
                        setDelimiter(e.target.value);
                        void reparse(hasHeader, e.target.value);
                      }}
                    >
                      <option value="">{t("import.delimAuto")}</option>
                      <option value=",">,</option>
                      <option value=";">;</option>
                      <option value="|">|</option>
                      <option value="tab">{t("import.delimTab")}</option>
                    </select>
                  </label>
                </>
              )}
            </div>
          )}

          {preview && (
            <>
              <div>
                <div className="mb-1 font-medium text-[var(--fg-secondary)]">{t("import.mapping")}</div>
                <div className="border border-[var(--border-color)] rounded overflow-auto max-h-[220px]">
                  <table className="w-full border-collapse">
                    <thead className="sticky top-0 bg-[var(--surface-secondary)] text-[var(--fg-secondary)]">
                      <tr>
                        <th className="px-2 py-1 text-left font-medium w-1/2">{t("import.target")}</th>
                        <th className="px-2 py-1 text-left font-medium">{t("import.source")}</th>
                      </tr>
                    </thead>
                    <tbody>
                      {targetCols.map((c) => (
                        <tr key={c.name} className="border-t border-[var(--border-subtle)]">
                          <td className="px-2 py-1 font-mono">
                            {c.name}
                            {!c.nullable && !c.hasDefault && !c.isAutoIncrement && <span className="text-[var(--danger)] ml-1">*</span>}
                          </td>
                          <td className="px-2 py-1">
                            <select
                              className="h-6 w-full rounded border border-[var(--border-color)] bg-[var(--surface)] px-1"
                              value={mapping[c.name] ?? SKIP}
                              disabled={running}
                              onChange={(e) => setMapping((m) => ({ ...m, [c.name]: Number(e.target.value) }))}
                            >
                              <option value={SKIP}>{t("import.skip")}</option>
                              {preview.columns.map((s, i) => (
                                <option key={i} value={i}>{s}</option>
                              ))}
                            </select>
                          </td>
                        </tr>
                      ))}
                    </tbody>
                  </table>
                </div>
                {missingRequired.length > 0 && (
                  <div className="mt-1 text-[var(--warning)]">
                    {t("import.missingRequired", { cols: missingRequired.map((c) => c.name).join(", ") })}
                  </div>
                )}
              </div>

              <div>
                <div className="mb-1 font-medium text-[var(--fg-secondary)]">{t("import.preview", { n: preview.rows.length })}</div>
                <div className="border border-[var(--border-color)] rounded overflow-auto max-h-[180px]">
                  <table className="w-full border-collapse font-mono">
                    <thead className="sticky top-0 bg-[var(--surface-secondary)] text-[var(--fg-secondary)]">
                      <tr>
                        {preview.columns.map((c, i) => (
                          <th key={i} className="px-2 py-1 text-left font-medium whitespace-nowrap">{c}</th>
                        ))}
                      </tr>
                    </thead>
                    <tbody>
                      {preview.rows.map((r, ri) => (
                        <tr key={ri} className="border-t border-[var(--border-subtle)]">
                          {r.map((v, ci) => (
                            <td key={ci} className="px-2 py-0.5 whitespace-nowrap max-w-[200px] truncate">{v}</td>
                          ))}
                        </tr>
                      ))}
                    </tbody>
                  </table>
                </div>
              </div>

              <div className="space-y-1">
                <label className="flex items-center gap-1.5 cursor-pointer select-none">
                  <input type="checkbox" checked={atomic} disabled={running} onChange={(e) => setAtomic(e.target.checked)} />
                  {t("import.atomic")}
                </label>
                <div className="text-2xs text-[var(--fg-muted)] pl-5">{t("import.atomicHint")}</div>
                {isCSV && (
                  <label className="flex items-center gap-1.5 cursor-pointer select-none">
                    <input type="checkbox" checked={emptyAsNull} disabled={running} onChange={(e) => setEmptyAsNull(e.target.checked)} />
                    {t("import.emptyAsNull")}
                  </label>
                )}
              </div>
            </>
          )}

          {progress && (
            <div className="rounded border border-[var(--border-color)] p-3">
              {running && (
                <>
                  <div className="flex items-center justify-between mb-1">
                    <span>{t("import.running", { cur: progress.current.toLocaleString(), total: progress.total.toLocaleString() })}</span>
                    <span>{percent}%</span>
                  </div>
                  <div className="h-1.5 rounded-full bg-[var(--border-color)] overflow-hidden">
                    <div className="h-full bg-[var(--accent)] transition-all duration-300" style={{ width: `${percent}%` }} />
                  </div>
                </>
              )}
              {finished && (
                <div className={cn("whitespace-pre-wrap", progress.status === "error" ? "text-[var(--danger)]" : "text-[var(--fg-secondary)]")}>
                  <div className="font-medium">{progress.status === "error" ? t("import.failed") : t("import.cancelled")}</div>
                  {progress.error && <div>{progress.error}</div>}
                  <div className="mt-1">
                    {progress.rolledBack ? t("import.rolledBack") : progress.current > 0 ? t("import.keptRows", { n: progress.current.toLocaleString() }) : ""}
                  </div>
                </div>
              )}
            </div>
          )}
        </div>

        <div className="flex justify-end gap-2 px-4 py-2.5 border-t border-[var(--border-color)] bg-[var(--surface-secondary)]">
          {running ? (
            <Button variant="outline" size="sm" onClick={stop} disabled={!progress?.taskId}>
              {t("import.stop")}
            </Button>
          ) : (
            <>
              <Button variant="ghost" size="sm" onClick={onClose}>
                {t("common.cancel")}
              </Button>
              <Button size="sm" onClick={() => void start()} disabled={!preview || mappedColumns.length === 0}>
                {t("import.start")}
              </Button>
            </>
          )}
        </div>
      </div>
    </>
  );
}
