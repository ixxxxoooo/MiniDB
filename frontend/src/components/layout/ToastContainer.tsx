import React, { useEffect } from "react";
import { AlertCircle, Check, FileDown, Info, StopCircle, X } from "lucide-react";
import { useUIStore, type ExportTask } from "@/stores/ui";
import { useTranslation } from "@/i18n";
import { cn } from "@/lib/utils";
import { EventsOn } from "@/lib/wails/runtime";
import * as ExportService from "@/lib/wails/services/ExportService";

/** 全局 Toast 通知 + 导出进度容器 */
export function ToastContainer() {
  const { toasts, removeToast, exportTasks, updateExportTask, removeExportTask } = useUIStore();

  // 监听后端导出进度事件
  useEffect(() => {
    const off = EventsOn("export:progress", (event: ExportTask) => {
      if (!event || !event.taskId) return;
      updateExportTask(event);
      // 完成/失败/取消 3 秒后自动移除
      if (event.status === "done" || event.status === "error" || event.status === "cancelled") {
        setTimeout(() => removeExportTask(event.taskId), 4000);
      }
    });
    return () => { off(); };
  }, [updateExportTask, removeExportTask]);

  if (toasts.length === 0 && exportTasks.length === 0) return null;

  const iconMap = {
    info: <Info className="h-3.5 w-3.5 text-blue-500 flex-shrink-0" />,
    success: <Check className="h-3.5 w-3.5 text-green-500 flex-shrink-0" />,
    error: <AlertCircle className="h-3.5 w-3.5 text-red-500 flex-shrink-0" />,
  };
  const topCenterToasts = toasts.filter((t) => t.placement === "top-center");
  const bottomRightToasts = toasts.filter((t) => t.placement !== "top-center");

  return (
    <>
      {topCenterToasts.length > 0 && (
        <div className="fixed top-14 left-1/2 -translate-x-1/2 z-[220] flex flex-col gap-1.5 pointer-events-none max-w-[420px]">
          {topCenterToasts.map((t) => (
            <div
              key={t.id}
              className={cn(
                "pointer-events-auto flex items-center gap-1.5 px-3 py-1.5 rounded-[var(--radius-btn)] shadow-lg border text-xs animate-fade-in",
                "bg-[var(--surface-elevated)] border-[var(--success)]/35 text-[var(--fg)]"
              )}
            >
              {iconMap[t.type]}
              <span>{t.message}</span>
              <button className="ml-1 text-[var(--fg-muted)] hover:text-[var(--fg)]" onClick={() => removeToast(t.id)}>
                <X className="h-3 w-3" />
              </button>
            </div>
          ))}
        </div>
      )}
      <div className="fixed bottom-4 right-4 z-[200] flex flex-col gap-2 pointer-events-none max-w-[380px]">
        {/* 导出任务进度面板 */}
        {exportTasks.map((task) => (
          <ExportTaskCard key={task.taskId} task={task} />
        ))}
        {/* 普通 toast */}
        {bottomRightToasts.map((t) => (
          <div
            key={t.id}
            className={cn(
              "pointer-events-auto flex items-center gap-2 px-3 py-2 rounded-[var(--radius-panel)] shadow-lg border text-xs animate-fade-in",
              "bg-[var(--surface-elevated)] border-[var(--border-color)] text-[var(--fg)]"
            )}
          >
            {iconMap[t.type]}
            <span>{t.message}</span>
            <button className="ml-1 text-[var(--fg-muted)] hover:text-[var(--fg)]" onClick={() => removeToast(t.id)}>
              <X className="h-3 w-3" />
            </button>
          </div>
        ))}
      </div>
    </>
  );
}

/** 单个导出任务卡片（带进度条） */
function ExportTaskCard({ task }: { task: ExportTask }) {
  const { removeExportTask } = useUIStore();
  const { t } = useTranslation();
  const percent = task.total > 0 ? Math.round((task.current / task.total) * 100) : 0;
  const isDone = task.status === "done";
  const canOpenFile = isDone && !!task.filePath;

  const formatRows = (n: number) => n.toLocaleString();

  const handleStop = () => {
    ExportService.CancelExport(task.taskId);
  };

  const handleOpenFile = async () => {
    if (!canOpenFile) return;
    try {
      const mod = await import("@/lib/wails/services/ExportService");
      await mod.OpenExportedFile(task.filePath);
    } catch (e) {
      console.error("打开导出文件失败", e);
    }
  };

  return (
    <div
      className={cn(
        "pointer-events-auto rounded-[var(--radius-panel)] shadow-lg border overflow-hidden animate-fade-in transition-colors",
        "bg-[var(--surface-elevated)] border-[var(--border-color)] text-[var(--fg)]",
        canOpenFile && "cursor-pointer hover:border-[var(--accent)] hover:bg-[var(--sidebar-hover)]/40"
      )}
      onClick={canOpenFile ? handleOpenFile : undefined}
      role={canOpenFile ? "button" : undefined}
      tabIndex={canOpenFile ? 0 : undefined}
      onKeyDown={canOpenFile ? (e) => {
        if (e.key === "Enter" || e.key === " ") {
          e.preventDefault();
          handleOpenFile();
        }
      } : undefined}
    >
      {/* 标题行 */}
      <div className="flex items-center gap-2 px-3 pt-2.5 pb-1">
        <FileDown className="h-3.5 w-3.5 text-[var(--accent)] flex-shrink-0" />
        <span className="text-xs font-medium truncate flex-1">{task.fileName || t("logViewer.exportTitle")}</span>
        {(task.status === "done" || task.status === "error" || task.status === "cancelled") && (
          <button
            className="text-[var(--fg-muted)] hover:text-[var(--fg)]"
            onClick={(e) => {
              e.stopPropagation();
              removeExportTask(task.taskId);
            }}
          >
            <X className="h-3 w-3" />
          </button>
        )}
      </div>

      {/* 进度区 */}
      <div className="px-3 pb-2.5">
        {task.status === "progress" && (
          <>
            <div className="flex items-center justify-between text-2xs text-[var(--fg-secondary)] mb-1">
              <span>{formatRows(task.current)}{task.total > 0 ? ` / ${formatRows(task.total)} ${t("common.rows")}` : ` ${t("common.rows")}`}</span>
              {task.total > 0 && <span>{percent}%</span>}
            </div>
            {/* 进度条 */}
            <div className="h-1.5 rounded-full bg-[var(--border-color)] overflow-hidden mb-1.5">
              <div
                className="h-full rounded-full bg-[var(--accent)] transition-all duration-300"
                style={{ width: task.total > 0 ? `${percent}%` : "30%", animation: task.total === 0 ? "pulse 1.5s infinite" : undefined }}
              />
            </div>
            <div className="flex justify-end">
              <button
                className="flex items-center gap-1 text-2xs text-[var(--fg-muted)] hover:text-[var(--danger)] transition-colors"
                onClick={handleStop}
              >
                <StopCircle className="h-3 w-3" />
                <span>{t("common.stop")}</span>
              </button>
            </div>
          </>
        )}
        {task.status === "done" && (
          <div className="flex items-center gap-1.5 text-2xs text-green-500">
            <Check className="h-3 w-3" />
            <span>{t("logViewer.exportDone")} · {formatRows(task.current)} {t("common.rows")}</span>
          </div>
        )}
        {task.status === "error" && (
          <div className="flex items-center gap-1.5 text-2xs text-red-500">
            <AlertCircle className="h-3 w-3" />
            <span className="truncate">{task.error || t("logViewer.exportFailed")}</span>
          </div>
        )}
        {task.status === "cancelled" && (
          <div className="flex items-center gap-1.5 text-2xs text-[var(--fg-muted)]">
            <StopCircle className="h-3 w-3" />
            <span>{t("logViewer.exportCancelled")} · {formatRows(task.current)} {t("common.rows")}</span>
          </div>
        )}
      </div>
    </div>
  );
}
