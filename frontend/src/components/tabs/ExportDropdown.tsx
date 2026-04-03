import React, { useEffect, useRef, useState } from "react";
import { Download } from "lucide-react";
import { useTranslation } from "@/i18n";
import { TipBtn } from "./TipBtn";

// =========== 导出格式下拉按钮 ===========
export function ExportDropdown({ onExport }: { onExport: (format: "csv" | "json" | "sql") => void }) {
  const [open, setOpen] = useState(false);
  const ref = useRef<HTMLDivElement>(null);
  const { t } = useTranslation();

  useEffect(() => {
    if (!open) return;
    const handler = (e: MouseEvent) => {
      if (ref.current && !ref.current.contains(e.target as Node)) setOpen(false);
    };
    document.addEventListener("mousedown", handler);
    return () => document.removeEventListener("mousedown", handler);
  }, [open]);

  return (
    <div className="relative" ref={ref}>
      <TipBtn
        tip={t("logViewer.exportTitle")}
        className="h-[var(--size-btn-sm)] w-[var(--size-btn-sm)] flex items-center justify-center rounded-[var(--radius-btn)] hover:bg-[var(--sidebar-hover)] transition-colors"
        onClick={() => setOpen((v) => !v)}
      >
        <Download className="h-2.5 w-2.5 text-[var(--fg-secondary)]" />
      </TipBtn>
      {open && (
        <div className="absolute right-0 top-full z-[100] mt-0.5 min-w-[120px] py-0.5 rounded-[var(--radius-menu)] shadow-lg border bg-[var(--surface-elevated)] border-[var(--border-color)]">
          {(["csv", "json", "sql"] as const).map((fmt) => (
            <button
              key={fmt}
              className="w-full px-2.5 py-1 text-xs text-left hover:bg-[var(--sidebar-hover)] text-[var(--fg)] flex items-center gap-2"
              onClick={() => { onExport(fmt); setOpen(false); }}
            >
              <Download className="h-3 w-3" /> {fmt.toUpperCase()}
            </button>
          ))}
        </div>
      )}
    </div>
  );
}
