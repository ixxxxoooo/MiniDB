import React, { useState, useRef, useEffect } from "react";
import { createPortal } from "react-dom";
import { Database, Unplug, X } from "lucide-react";
import { cn } from "@/lib/utils";
import { useConnectionStore } from "@/stores/connection";
import { useTranslation } from "@/i18n";

interface ContextMenuState {
  x: number;
  y: number;
  workspaceId: string;
}

export function WorkspaceBar() {
  const { workspaces, activeWorkspaceId, setActiveWorkspace, removeWorkspace, connections } = useConnectionStore();
  const [contextMenu, setContextMenu] = useState<ContextMenuState | null>(null);
  const menuRef = useRef<HTMLDivElement>(null);
  const { t } = useTranslation();

  useEffect(() => {
    if (!contextMenu) return;
    const handler = (e: MouseEvent) => {
      if (menuRef.current && !menuRef.current.contains(e.target as Node)) {
        setContextMenu(null);
      }
    };
    document.addEventListener("mousedown", handler);
    return () => document.removeEventListener("mousedown", handler);
  }, [contextMenu]);

  // 根据需求，如果只有一个数据库连接时不显示最左边的库选项卡
  if (workspaces.length <= 1) {
    return null;
  }

  const handleContextMenu = (e: React.MouseEvent, workspaceId: string) => {
    e.preventDefault();
    e.stopPropagation();
    setContextMenu({ x: e.clientX, y: e.clientY, workspaceId });
  };

  return (
    <>
      <div
        className={cn(
          "w-[60px] flex flex-col items-center py-3 gap-2 border-r flex-shrink-0 z-20",
          "bg-[var(--sidebar-bg)] border-[var(--sidebar-border)]"
        )}
      >
        {workspaces.map((ws, index) => {
          const isActive = activeWorkspaceId === ws.id;
          const conn = connections.find((c) => c.id === ws.connectionId);
          // 图标下的文字优先使用 database，如果没有 database 则使用 connection 名称
          const displayName = ws.database || conn?.name || "Unknown";

          return (
            <React.Fragment key={ws.id}>
              <div className="relative group flex flex-col items-center w-full px-1.5">
                <button
                  onClick={() => setActiveWorkspace(ws.id)}
                  onContextMenu={(e) => handleContextMenu(e, ws.id)}
                  className={cn(
                    "w-full h-[52px] rounded-[var(--radius-panel)] flex flex-col items-center justify-center transition-colors relative",
                    isActive
                      ? "bg-[var(--sidebar-active)] text-[var(--fg)]"
                      : "hover:bg-[var(--sidebar-hover)] text-[var(--fg-secondary)]"
                  )}
                  title={`${conn?.name || ""} - ${ws.database || "Default"}`}
                >
                  {/* 被选中的标志 */}
                  {isActive && (
                    <div className="absolute left-[-6px] top-1/2 -translate-y-1/2 w-1 h-6 rounded-r-md bg-[var(--accent)]" />
                  )}
                  <Database
                    className={cn(
                      "h-5 w-5 mb-1",
                      isActive ? "text-[var(--accent)]" : "text-[var(--fg-muted)] group-hover:text-[var(--fg-secondary)]"
                    )}
                  />
                  <span className="text-[10px] leading-none max-w-full truncate px-1">
                    {displayName}
                  </span>
                </button>
                
                {/* 悬浮显示的 X（无按钮形状） */}
                <button
                  className={cn(
                    "absolute top-1 right-2 opacity-0 group-hover:opacity-100 transition-opacity flex items-center justify-center",
                    "text-[var(--fg-muted)] hover:text-[var(--fg)] z-10 p-0.5"
                  )}
                  onClick={(e) => {
                    e.stopPropagation();
                    removeWorkspace(ws.id);
                  }}
                  title={t("sidebar.disconnect") || "Close"}
                >
                  <X className="h-3 w-3" />
                </button>
              </div>

              {/* 分隔线 */}
              {index < workspaces.length - 1 && (
                <div className="w-[32px] h-px bg-[var(--border-color)] opacity-40 my-1 flex-shrink-0" />
              )}
            </React.Fragment>
          );
        })}
      </div>

      {contextMenu &&
        createPortal(
          <div
            ref={menuRef}
            className={cn(
              "fixed z-[200] min-w-[140px] py-1 rounded-[var(--radius-menu)] shadow-lg border animate-fade-in",
              "bg-[var(--surface-elevated)] border-[var(--border-color)]"
            )}
            style={{ left: contextMenu.x, top: contextMenu.y }}
          >
            <button
              className="w-full px-3 py-1.5 text-xs text-left hover:bg-[var(--sidebar-hover)] text-[var(--danger)] flex items-center gap-2"
              onClick={() => {
                const wsId = contextMenu.workspaceId;
                setContextMenu(null);
                removeWorkspace(wsId);
                // 这里可选：如果该 connectionId 下没有任何 workspace 了，是否要调用 disconnect?
              }}
            >
              <Unplug className="h-3.5 w-3.5" />
              {t("sidebar.disconnect") || "Close"}
            </button>
          </div>,
          document.body
        )}
    </>
  );
}
