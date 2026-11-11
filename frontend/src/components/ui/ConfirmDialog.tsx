import React, { useEffect, useRef, useState } from "react";
import { Button } from "@/components/ui/button";
import { useConfirmStore } from "@/stores/confirm";
import { useTranslation } from "@/i18n";
import { cn } from "@/lib/utils";

/** 全局确认框宿主：挂在应用根部，由 requestConfirm() 驱动。 */
export function ConfirmDialogHost() {
  const current = useConfirmStore((s) => s.current);
  const resolve = useConfirmStore((s) => s.resolve);
  const { t } = useTranslation();
  const [typed, setTyped] = useState("");
  const inputRef = useRef<HTMLInputElement>(null);

  useEffect(() => {
    setTyped("");
    if (current?.requireText) requestAnimationFrame(() => inputRef.current?.focus());
  }, [current]);

  useEffect(() => {
    if (!current) return;
    const onKey = (e: KeyboardEvent) => {
      if (e.key === "Escape") {
        e.preventDefault();
        e.stopPropagation();
        resolve(false);
      }
    };
    window.addEventListener("keydown", onKey, true);
    return () => window.removeEventListener("keydown", onKey, true);
  }, [current, resolve]);

  if (!current) return null;
  const canConfirm = !current.requireText || typed === current.requireText;

  return (
    <>
      <div className="fixed inset-0 z-[300] bg-black/40 backdrop-blur-sm" onClick={() => resolve(false)} />
      <div
        role="alertdialog"
        aria-modal="true"
        className={cn(
          "fixed z-[301] top-1/2 left-1/2 -translate-x-1/2 -translate-y-1/2 w-[400px] max-w-[92vw]",
          "rounded-[var(--radius-panel)] border shadow-lg p-4",
          "bg-[var(--surface-elevated)] border-[var(--border-color)] text-[var(--fg)]"
        )}
      >
        <div className="text-sm font-semibold">{current.title}</div>
        {current.message && (
          <div className="mt-2 text-xs text-[var(--fg-secondary)] leading-5 whitespace-pre-wrap break-words">{current.message}</div>
        )}
        {current.requireText && (
          <div className="mt-3">
            <div className="text-2xs text-[var(--fg-muted)] mb-1">
              {t("dialog.typeToConfirm", { text: current.requireText })}
            </div>
            <input
              ref={inputRef}
              value={typed}
              onChange={(e) => setTyped(e.target.value)}
              onKeyDown={(e) => {
                if (e.key === "Enter" && canConfirm) resolve(true);
              }}
              className="w-full h-7 px-2 rounded-[var(--radius-btn)] border border-[var(--border-color)] bg-[var(--surface)] text-xs font-mono focus:outline-none focus:border-[var(--accent)]"
            />
          </div>
        )}
        <div className="mt-4 flex justify-end gap-2">
          <Button size="sm" variant="outline" onClick={() => resolve(false)}>
            {current.cancelText || t("common.cancel")}
          </Button>
          <Button size="sm" variant={current.danger ? "destructive" : "default"} disabled={!canConfirm} onClick={() => resolve(true)}>
            {current.confirmText || t("common.confirm")}
          </Button>
        </div>
      </div>
    </>
  );
}
