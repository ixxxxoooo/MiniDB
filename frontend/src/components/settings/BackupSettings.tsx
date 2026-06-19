import React, { useState } from "react";
import { Download, Upload, AlertTriangle } from "lucide-react";
import { useTranslation } from "@/i18n";
import { useUIStore } from "@/stores/ui";
import { useDatabase } from "@/hooks/useDatabase";
import * as ConnectionService from "@/lib/wails/services/ConnectionService";
import { cn } from "@/lib/utils";

export function BackupSettings() {
  const { t } = useTranslation();
  const { loadConnections } = useDatabase();
  const [exporting, setExporting] = useState(false);
  const [importing, setImporting] = useState(false);

  const handleExport = async () => {
    setExporting(true);
    try {
      const filePath = await ConnectionService.ExportConnections();
      if (!filePath) return;
      useUIStore.getState().addToast("success", t("backupSettings.exportSuccess", { path: filePath }));
    } catch (e: unknown) {
      const message = e instanceof Error ? e.message : String(e ?? "未知错误");
      useUIStore.getState().addToast("error", `${t("backupSettings.exportFailed")}: ${message}`);
    } finally {
      setExporting(false);
    }
  };

  const handleImport = async () => {
    setImporting(true);
    try {
      const result = await ConnectionService.ImportConnections();
      if (!result) return;
      if (result.imported === 0 && result.skipped === 0) return;
      await loadConnections();
      useUIStore.getState().addToast(
        "success",
        t("backupSettings.importSuccess", { imported: result.imported, skipped: result.skipped })
      );
    } catch (e: unknown) {
      const message = e instanceof Error ? e.message : String(e ?? "未知错误");
      useUIStore.getState().addToast("error", `${t("backupSettings.importFailed")}: ${message}`);
    } finally {
      setImporting(false);
    }
  };

  return (
    <div className="space-y-[var(--size-gap)]">
      <div>
        <h3 className="text-[length:var(--size-font-xs)] font-semibold mb-0.5">{t("backupSettings.title")}</h3>
        <p className="text-[length:var(--size-font-2xs)] text-[var(--fg-secondary)]">{t("backupSettings.description")}</p>
      </div>

      <div className="flex items-start gap-2 rounded-[var(--radius-btn)] border border-[var(--warning)]/30 bg-[var(--warning)]/5 px-2.5 py-2">
        <AlertTriangle className="h-3.5 w-3.5 text-[var(--warning)] flex-shrink-0 mt-0.5" />
        <p className="text-[length:var(--size-font-2xs)] text-[var(--fg-secondary)] leading-relaxed">
          {t("backupSettings.passwordWarning")}
        </p>
      </div>

      <div className="flex flex-col gap-2">
        <button
          className={cn(
            "flex items-center justify-center gap-2 h-8 px-3 rounded-[var(--radius-btn)] text-[length:var(--size-font-2xs)] font-medium transition-colors",
            "border border-[var(--border-color)] hover:bg-[var(--sidebar-hover)] text-[var(--fg)]",
            exporting && "opacity-60 cursor-not-allowed"
          )}
          disabled={exporting}
          onClick={() => void handleExport()}
        >
          <Download className="h-3.5 w-3.5" />
          {exporting ? t("backupSettings.exporting") : t("backupSettings.export")}
        </button>

        <button
          className={cn(
            "flex items-center justify-center gap-2 h-8 px-3 rounded-[var(--radius-btn)] text-[length:var(--size-font-2xs)] font-medium transition-colors",
            "border border-[var(--border-color)] hover:bg-[var(--sidebar-hover)] text-[var(--fg)]",
            importing && "opacity-60 cursor-not-allowed"
          )}
          disabled={importing}
          onClick={() => void handleImport()}
        >
          <Upload className="h-3.5 w-3.5" />
          {importing ? t("backupSettings.importing") : t("backupSettings.import")}
        </button>
      </div>
    </div>
  );
}
