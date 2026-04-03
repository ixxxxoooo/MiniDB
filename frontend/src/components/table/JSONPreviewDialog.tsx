import React, { useEffect, useMemo } from "react";
import { createPortal } from "react-dom";
import { Copy, X } from "lucide-react";
import Prism from "prismjs";
import { cn, copyToClipboard } from "@/lib/utils";
import { useTranslation } from "@/i18n";

interface JSONPreviewDialogProps {
  open: boolean;
  formattedJSON: string;
  onClose: () => void;
}

export function JSONPreviewDialog({
  open,
  formattedJSON,
  onClose,
}: JSONPreviewDialogProps) {
  const { t } = useTranslation();

  useEffect(() => {
    if (!open) return;
    const onKeyDown = (e: KeyboardEvent) => {
      if (e.key === "Escape") {
        e.preventDefault();
        onClose();
      }
    };
    window.addEventListener("keydown", onKeyDown);
    return () => window.removeEventListener("keydown", onKeyDown);
  }, [open, onClose]);

  const highlightedHTML = useMemo(() => {
    try {
      const lang = Prism.languages.json ? "json" : "javascript";
      return Prism.highlight(formattedJSON, Prism.languages[lang], lang);
    } catch {
      return formattedJSON
        .replace(/&/g, "&amp;")
        .replace(/</g, "&lt;")
        .replace(/>/g, "&gt;");
    }
  }, [formattedJSON]);

  if (!open) return null;

  return createPortal(
    <>
      <div className="fixed inset-0 z-[120] bg-black/25 backdrop-blur-sm" onClick={onClose} />
      <div
        className={cn(
          "fixed z-[121] left-1/2 top-1/2 -translate-x-1/2 -translate-y-1/2",
          "w-[min(92vw,900px)] h-[min(84vh,720px)] rounded-[var(--radius-panel)] border shadow-xl overflow-hidden",
          "bg-[var(--surface)] border-[var(--border-color)]"
        )}
      >
        <div className="h-[var(--size-toolbar)] px-[var(--size-padding)] border-b border-[var(--border-color)] flex items-center justify-between">
          <div className="text-[length:var(--size-font-sm)] font-medium text-[var(--fg)]">
            {t("jsonViewer.title")}
          </div>
          <div className="flex items-center gap-1.5">
            <button
              className="h-[var(--size-btn-sm)] px-2 rounded-[var(--radius-btn)] text-[length:var(--size-font-xs)] text-[var(--fg-secondary)] hover:bg-[var(--sidebar-hover)] transition-colors inline-flex items-center gap-1"
              onClick={() => copyToClipboard(formattedJSON)}
            >
              <Copy className="h-3 w-3" />
              <span>{t("jsonViewer.copyJSON")}</span>
            </button>
            <button
              className="h-[var(--size-btn-sm)] w-[var(--size-btn-sm)] rounded-[var(--radius-btn)] text-[var(--fg-secondary)] hover:bg-[var(--sidebar-hover)] transition-colors inline-flex items-center justify-center"
              onClick={onClose}
            >
              <X className="h-3.5 w-3.5" />
            </button>
          </div>
        </div>
        <div className="h-[calc(100%-var(--size-toolbar))] overflow-auto p-[var(--size-padding)] bg-[var(--surface-secondary)]">
          <pre className="markdown-content text-[length:var(--size-font-xs)] leading-5 font-mono rounded-[var(--radius-input)] border border-[var(--border-color)] bg-[var(--surface)] p-3 min-h-full">
            <code className="language-code" dangerouslySetInnerHTML={{ __html: highlightedHTML }} />
          </pre>
        </div>
      </div>
    </>,
    document.body
  );
}
