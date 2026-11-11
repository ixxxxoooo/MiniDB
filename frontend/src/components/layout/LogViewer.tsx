import React, { useCallback, useEffect, useState } from "react";
import { Loader2, ScrollText, X } from "lucide-react";
import { Button } from "@/components/ui/button";
import { useTranslation } from "@/i18n";
import { cn } from "@/lib/utils";

/** 日志查看器弹窗 */
export function LogViewer({ onClose }: { onClose: () => void }) {
  const [logContent, setLogContent] = useState("");
  const [logPath, setLogPath] = useState("");
  const [loading, setLoading] = useState(true);
  const contentRef = React.useRef<HTMLPreElement>(null);
  const { t } = useTranslation();

  const loadLog = useCallback(async () => {
    setLoading(true);
    try {
      const [content, path] = await Promise.all([
        import("@/lib/wails/services/SettingsService").then((m) => m.GetLogContent()),
        import("@/lib/wails/services/SettingsService").then((m) => m.GetLogPath()),
      ]);
      setLogContent(content || t("logViewer.noLogs"));
      setLogPath(path || "");
    } catch (e: any) {
      setLogContent(t("logViewer.loadFailed") + (e?.message || e));
    } finally {
      setLoading(false);
    }
  }, [t]);

  useEffect(() => {
    loadLog();
  }, [loadLog]);

  useEffect(() => {
    if (!loading && contentRef.current) {
      contentRef.current.scrollTop = contentRef.current.scrollHeight;
    }
  }, [loading, logContent]);

  // ESC 关闭日志弹窗
  useEffect(() => {
    const handleEsc = (e: KeyboardEvent) => {
      if (e.key === "Escape") { e.preventDefault(); onClose(); }
    };
    window.addEventListener("keydown", handleEsc);
    return () => window.removeEventListener("keydown", handleEsc);
  }, [onClose]);

  return (
    <>
      <div className="fixed inset-0 z-50 bg-black/40 backdrop-blur-sm" onClick={onClose} />
      <div
        className={cn(
          "fixed z-50 top-1/2 left-1/2 -translate-x-1/2 -translate-y-1/2",
          "w-[800px] h-[560px] rounded-[var(--radius-panel)] shadow-lg border overflow-hidden flex flex-col",
          "bg-[var(--surface)] border-[var(--border-color)]"
        )}
      >
        <div className="flex items-center justify-between px-4 py-2.5 border-b border-[var(--border-color)] bg-[var(--surface-secondary)]">
          <div className="flex items-center gap-2">
            <ScrollText className="h-4 w-4 text-[var(--fg-secondary)]" />
            <span className="text-sm font-medium">{t("logViewer.title")}</span>
            {logPath && (
              <span className="text-2xs text-[var(--fg-muted)] font-mono">{logPath}</span>
            )}
          </div>
          <div className="flex items-center gap-1">
            <Button variant="ghost" size="sm" className="h-7 text-xs" onClick={loadLog}>
              {t("logViewer.refresh")}
            </Button>
            <Button variant="ghost" size="icon" className="h-7 w-7" onClick={onClose}>
              <X className="h-3.5 w-3.5" />
            </Button>
          </div>
        </div>
        <pre
          ref={contentRef}
          className="flex-1 overflow-auto p-4 text-xs font-mono text-[var(--fg)] bg-[var(--surface)] leading-relaxed whitespace-pre-wrap break-all"
        >
          {loading ? (
            <div className="flex items-center gap-2 text-[var(--fg-muted)]">
              <Loader2 className="h-4 w-4 animate-spin" />
              {t("logViewer.loading")}
            </div>
          ) : logContent}
        </pre>
      </div>
    </>
  );
}
