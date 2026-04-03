import React, { useEffect, useLayoutEffect, useRef, useState } from "react";
import { createPortal } from "react-dom";
import {
  Braces,
  Copy,
  ClipboardCopy,
  FileCode,
  Trash2,
  RefreshCw,
  Eye,
  Download,
} from "lucide-react";
import { cn } from "@/lib/utils";
import { useTranslation } from "@/i18n";

export interface ContextMenuPosition {
  x: number;
  y: number;
}

interface RowContextMenuProps {
  position: ContextMenuPosition | null;
  onClose: () => void;
  onCopyCell: () => void;
  onCopyRow: () => void;
  onCopyAsInsert: () => void;
  onFormatJSON?: () => void;
  onDeleteRow: () => void;
  onRefresh: () => void;
  onPreview: () => void;
  onDownloadPage?: () => void;
  showCopyAsInsert?: boolean;
  showFormatJSON?: boolean;
}

export function RowContextMenu({
  position,
  onClose,
  onCopyCell,
  onCopyRow,
  onCopyAsInsert,
  onFormatJSON,
  onDeleteRow,
  onRefresh,
  onPreview,
  onDownloadPage,
  showCopyAsInsert = true,
  showFormatJSON = false,
}: RowContextMenuProps) {
  const { t } = useTranslation();
  const menuRef = useRef<HTMLDivElement>(null);
  const [resolvedPosition, setResolvedPosition] = useState<ContextMenuPosition | null>(position);

  useEffect(() => {
    if (!position) return;

    const handlePointerDown = (e: PointerEvent) => {
      const target = e.target as Node | null;
      if (target && menuRef.current?.contains(target)) return;
      onClose();
    };
    const handleEscape = (e: KeyboardEvent) => {
      if (e.key === "Escape") {
        onClose();
      }
    };

    document.addEventListener("pointerdown", handlePointerDown, true);
    window.addEventListener("keydown", handleEscape);
    return () => {
      document.removeEventListener("pointerdown", handlePointerDown, true);
      window.removeEventListener("keydown", handleEscape);
    };
  }, [position, onClose]);

  useLayoutEffect(() => {
    if (!position || !menuRef.current) return;

    const menuRect = menuRef.current.getBoundingClientRect();
    const viewportWidth = window.innerWidth;
    const viewportHeight = window.innerHeight;
    const padding = 8;

    let left = position.x;
    let top = position.y;

    if (left + menuRect.width > viewportWidth - padding) {
      left = viewportWidth - menuRect.width - padding;
    }
    if (left < padding) {
      left = padding;
    }

    // 底部放不下时，菜单向上弹出
    if (top + menuRect.height > viewportHeight - padding) {
      top = position.y - menuRect.height;
    }
    if (top < padding) {
      top = padding;
    }

    setResolvedPosition({ x: left, y: top });
  }, [position]);

  if (!position) return null;

  return createPortal(
    <div
      ref={menuRef}
      className={cn(
        "fixed z-[101] min-w-[180px] rounded-[var(--radius-menu)] py-1 shadow-lg border animate-fade-in",
        "bg-[var(--surface-elevated)] border-[var(--border-color)]"
      )}
      style={{ left: resolvedPosition?.x ?? position.x, top: resolvedPosition?.y ?? position.y }}
      onContextMenu={(e) => e.preventDefault()}
    >
      <MenuItem icon={Eye} label={t("contextMenu.previewRow")} shortcut="Space" onClick={onPreview} />
      <Separator />
      <MenuItem icon={Copy} label={t("contextMenu.copyCell")} shortcut="⌘C" onClick={onCopyCell} />
      {showFormatJSON && onFormatJSON && <MenuItem icon={Braces} label={t("contextMenu.formatJSON")} onClick={onFormatJSON} />}
      <MenuItem icon={ClipboardCopy} label={t("contextMenu.copyRow")} onClick={onCopyRow} />
      {showCopyAsInsert && <MenuItem icon={FileCode} label={t("contextMenu.copyAsInsert")} onClick={onCopyAsInsert} />}
      <Separator />
      {onDownloadPage && (
        <>
          <MenuItem icon={Download} label={t("contextMenu.downloadCSV")} onClick={onDownloadPage} />
          <Separator />
        </>
      )}
      <MenuItem icon={RefreshCw} label={t("common.refresh")} shortcut="⌘R" onClick={onRefresh} />
      <Separator />
      <MenuItem icon={Trash2} label={t("contextMenu.deleteRow")} onClick={onDeleteRow} danger />
    </div>,
    document.body
  );
}

function MenuItem({
  icon: Icon,
  label,
  shortcut,
  onClick,
  danger,
}: {
  icon: React.ElementType;
  label: string;
  shortcut?: string;
  onClick: () => void;
  danger?: boolean;
}) {
  return (
    <button
      className={cn(
        "w-full flex items-center gap-2 px-3 py-1.5 text-[length:var(--size-font-xs)] transition-colors",
        danger
          ? "text-[var(--danger)] hover:bg-red-50 dark:hover:bg-red-900/20"
          : "text-[var(--fg)] hover:bg-[var(--row-hover)]"
      )}
      onClick={onClick}
    >
      <Icon className="h-3.5 w-3.5" />
      <span className="flex-1 text-left">{label}</span>
      {shortcut && (
        <span className="text-2xs text-[var(--fg-muted)]">{shortcut}</span>
      )}
    </button>
  );
}

function Separator() {
  return <div className="my-1 h-px bg-[var(--border-subtle)]" />;
}
