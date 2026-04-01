import React, { useState, useCallback, useEffect, useRef } from "react";
import { Sidebar } from "./Sidebar";
import { TabBar } from "@/components/tabs/TabBar";
import { TabContent } from "@/components/tabs/TabContent";
import { ConnectionDialog } from "@/components/connection/ConnectionDialog";
import { SettingsDialog } from "@/components/settings/SettingsDialog";
import { AIPanel } from "@/components/ai/AIPanel";
import { DatabaseSwitcher } from "./DatabaseSwitcher";
import { WorkspaceBar } from "./WorkspaceBar";
import { useConnectionStore } from "@/stores/connection";
import { useTabsStore } from "@/stores/tabs";
import { useUIStore } from "@/stores/ui";
import { useThemeStore } from "@/stores/theme";
import { useDatabase } from "@/hooks/useDatabase";
import { CommandPalette } from "./CommandPalette";
import { useKeyboard } from "@/hooks/useKeyboard";
import { useTranslation } from "@/i18n";
import type { ConnectionConfig } from "@/types/connection";
import { cn } from "@/lib/utils";
import {
  Sidebar as SidebarIcon,
  Database,
  Plus,
  Settings,
  Moon,
  Sun,
  Sparkles,
  Search,
  ChevronDown,
  ScrollText,
  X,
  Loader2,
  RefreshCw,
} from "lucide-react";
import { Button } from "@/components/ui/button";



/**
 * 标题栏双击切换最大化 hook
 * 通过 mousedown 时间间隔检测双击，兼容 -webkit-app-region: drag 区域
 */
function useTitlebarDoubleClick() {
  const lastClickRef = useRef<{ time: number; x: number; y: number }>({
    time: 0, x: 0, y: 0,
  });

  const handleMouseDown = useCallback((e: React.MouseEvent) => {
    if (e.button !== 0) return;
    const now = Date.now();
    const last = lastClickRef.current;
    const timeDelta = now - last.time;
    const distX = Math.abs(e.clientX - last.x);
    const distY = Math.abs(e.clientY - last.y);

    if (timeDelta < 400 && distX < 5 && distY < 5) {
      // 双击检测成功，切换最大化/还原
      import("../../../wailsjs/runtime/runtime").then((r) =>
        r.WindowToggleMaximise()
      );
      lastClickRef.current = { time: 0, x: 0, y: 0 };
    } else {
      lastClickRef.current = { time: now, x: e.clientX, y: e.clientY };
    }
  }, []);

  return { handleMouseDown };
}

// 窗口控制按钮组件（自绘 macOS 红绿灯）
function WindowControls() {
  const [hovered, setHovered] = useState(false);

  const handleClose = () => {
    import("../../../wailsjs/runtime/runtime").then((r) => r.Quit());
  };
  const handleMinimise = () => {
    import("../../../wailsjs/runtime/runtime").then((r) => r.WindowMinimise());
  };
  const handleMaximise = () => {
    import("../../../wailsjs/runtime/runtime").then((r) => r.WindowToggleMaximise());
  };

  return (
    <div
      className="flex items-center gap-[7px] px-3 titlebar-no-drag flex-shrink-0"
      onMouseEnter={() => setHovered(true)}
      onMouseLeave={() => setHovered(false)}
    >
      {/* 关闭 */}
      <button
        onClick={handleClose}
        className="w-[12px] h-[12px] rounded-full flex items-center justify-center transition-colors focus:outline-none"
        style={{ backgroundColor: hovered ? "#ff5f57" : "var(--fg-muted)" }}
        title="关闭"
      >
        {hovered && (
          <svg width="6" height="6" viewBox="0 0 6 6" fill="none">
            <path d="M0.5 0.5L5.5 5.5M5.5 0.5L0.5 5.5" stroke="#4a0002" strokeWidth="1.2" strokeLinecap="round" />
          </svg>
        )}
      </button>
      {/* 最小化 */}
      <button
        onClick={handleMinimise}
        className="w-[12px] h-[12px] rounded-full flex items-center justify-center transition-colors focus:outline-none"
        style={{ backgroundColor: hovered ? "#febc2e" : "var(--fg-muted)" }}
        title="最小化"
      >
        {hovered && (
          <svg width="6" height="2" viewBox="0 0 6 2" fill="none">
            <path d="M0.5 1H5.5" stroke="#995700" strokeWidth="1.2" strokeLinecap="round" />
          </svg>
        )}
      </button>
      {/* 最大化 */}
      <button
        onClick={handleMaximise}
        className="w-[12px] h-[12px] rounded-full flex items-center justify-center transition-colors focus:outline-none"
        style={{ backgroundColor: hovered ? "#28c840" : "var(--fg-muted)" }}
        title="最大化"
      >
        {hovered && (
          <svg width="6" height="6" viewBox="0 0 6 6" fill="none">
            <path d="M1 4.5L3 1.5L5 4.5" stroke="#006500" strokeWidth="1.2" strokeLinecap="round" strokeLinejoin="round" />
          </svg>
        )}
      </button>
    </div>
  );
}

export function AppLayout() {
  const [connDialogOpen, setConnDialogOpen] = useState(false);
  const [editingConnection, setEditingConnection] =
    useState<ConnectionConfig | null>(null);
  const [settingsOpen, setSettingsOpen] = useState(false);
  const [aiPanelOpen, setAIPanelOpen] = useState(false);
  const [aiPanelWidth, setAIPanelWidth] = useState(400);
  const [searchOpen, setSearchOpen] = useState(false);
  const [dbSwitcherOpen, setDbSwitcherOpen] = useState(false);
  const [logViewerOpen, setLogViewerOpen] = useState(false);

  // 标题栏双击最大化/还原
  const { handleMouseDown: handleTitlebarMouseDown } = useTitlebarDoubleClick();

  const { t } = useTranslation();
  const { activeTabId, tabs, addTab, removeTab, switchWorkspace } = useTabsStore();
  const activeTab = tabs.find((t) => t.id === activeTabId);
  const { activeConnectionId, databases, connections, connectionStates, activeWorkspaceId } = useConnectionStore();
  const { sidebarCollapsed, toggleSidebar, layoutMode, showScrollbar } = useUIStore();
  const { resolved, setTheme } = useThemeStore();

  const {
    loadConnections,
    saveConnection,
    testConnection,
    connect,
    disconnect,
    loadTables,
  } = useDatabase();

  const [reconnecting, setReconnecting] = useState(false);

  useEffect(() => {
    loadConnections();
  }, [loadConnections]);

  useEffect(() => {
    if (activeWorkspaceId) {
      switchWorkspace(activeWorkspaceId);
    }
  }, [activeWorkspaceId, switchWorkspace]);

  const handleNewConnection = useCallback(() => {
    setEditingConnection(null);
    setConnDialogOpen(true);
  }, []);

  const handleEditConnection = useCallback((conn: ConnectionConfig) => {
    setEditingConnection(conn);
    setConnDialogOpen(true);
  }, []);

  const handleSaveConnection = useCallback(
    async (conn: ConnectionConfig) => {
      await saveConnection(conn);
      await connect(conn.id);
    },
    [saveConnection, connect]
  );

  const handleTestConnection = useCallback(
    async (conn: ConnectionConfig): Promise<boolean> => {
      return testConnection(conn);
    },
    [testConnection]
  );

  const activeConn = connections.find((c) => c.id === activeConnectionId);
  const connState = activeConnectionId ? connectionStates[activeConnectionId] : undefined;
  const currentDb = activeTab?.database || databases[activeConnectionId || ""]?.[0]?.name || "";

  // 重连当前激活连接
  const handleReconnect = useCallback(async () => {
    if (!activeConnectionId) return;
    setReconnecting(true);
    try {
      await disconnect(activeConnectionId);
      await connect(activeConnectionId);
      console.log("[AppLayout] 重连成功: id=%s", activeConnectionId);
    } catch (e: any) {
      console.error("[AppLayout] 重连失败:", e);
    } finally {
      setReconnecting(false);
    }
  }, [activeConnectionId, disconnect, connect]);

  useKeyboard({
    // 搜索命令面板：⌘P
    "mod+p": () => setSearchOpen(true),
    // 切换数据库：⌘K
    "mod+k": () => {
      if (activeConnectionId && connState?.status === "connected") {
        setDbSwitcherOpen(true);
      }
    },
    "mod+t": () => {
      if (activeConnectionId) {
        addTab({
          type: "query",
          title: t("tabs.newQuery"),
          connectionId: activeConnectionId,
          database: currentDb,
          closable: true,
          sql: "",
        });
      }
    },
    "mod+,": () => setSettingsOpen(true),
    "mod+w": () => {
      if (activeTabId) {
        const tab = tabs.find((t) => t.id === activeTabId);
        if (tab?.closable) removeTab(activeTabId);
      }
    },
    "mod+n": () => handleNewConnection(),
    // 导航：切换 Tab ⌘] / ⌘[
    "mod+]": () => {
      if (tabs.length <= 1) return;
      const idx = tabs.findIndex((t) => t.id === activeTabId);
      const nextIdx = (idx + 1) % tabs.length;
      useTabsStore.getState().setActiveTab(tabs[nextIdx].id);
    },
    "mod+[": () => {
      if (tabs.length <= 1) return;
      const idx = tabs.findIndex((t) => t.id === activeTabId);
      const prevIdx = (idx - 1 + tabs.length) % tabs.length;
      useTabsStore.getState().setActiveTab(tabs[prevIdx].id);
    },
    // 切换工作区 ⇧⌘] / ⇧⌘[
    "mod+shift+]": () => {
      const { workspaces, activeWorkspaceId, setActiveWorkspace } = useConnectionStore.getState();
      if (workspaces.length <= 1) return;
      const idx = workspaces.findIndex((w) => w.id === activeWorkspaceId);
      const nextIdx = (idx + 1) % workspaces.length;
      setActiveWorkspace(workspaces[nextIdx].id);
    },
    "mod+shift+[": () => {
      const { workspaces, activeWorkspaceId, setActiveWorkspace } = useConnectionStore.getState();
      if (workspaces.length <= 1) return;
      const idx = workspaces.findIndex((w) => w.id === activeWorkspaceId);
      const prevIdx = (idx - 1 + workspaces.length) % workspaces.length;
      setActiveWorkspace(workspaces[prevIdx].id);
    },
  });

  return (
    <div
      className={cn(
        "h-full relative bg-[var(--surface)] overflow-hidden",
        layoutMode === "compact" && "compact",
        !showScrollbar && "hide-scrollbar"
      )}
    >
      {/* ====== 顶部工具栏 ======
       * Frameless 模式，自绘红绿灯在最左侧
       * 双击标题栏空白区域切换最大化/还原（模拟 macOS 原生行为）
       */}
      <div
        className={cn(
          "absolute top-0 left-0 right-0 h-[var(--size-toolbar)] z-40",
          "flex items-center border-b titlebar-drag",
          "bg-[var(--toolbar-bg)] border-[var(--toolbar-border)]"
        )}
        style={{ paddingRight: "var(--size-padding-sm)" }}
        onMouseDown={handleTitlebarMouseDown}
      >
        {/* 自绘窗口控制按钮（红绿灯），阻止 mousedown 冒泡避免误触最大化 */}
        <div onMouseDown={(e) => e.stopPropagation()}>
          <WindowControls />
        </div>

        {/* 左侧功能区，阻止 mousedown 冒泡避免误触最大化 */}
        <div className="titlebar-no-drag flex items-center gap-[var(--size-gap-sm)] ml-1" onMouseDown={(e) => e.stopPropagation()}>
          {activeConn && connState?.status === "connected" && (
            <button
              className={cn(
                "flex items-center justify-center rounded-[var(--radius-btn)] font-medium transition-colors h-[var(--size-btn)] w-[var(--size-btn)]",
                "hover:bg-[var(--sidebar-hover)] text-[var(--fg-secondary)]"
              )}
              onClick={() => setDbSwitcherOpen(true)}
              title={`${t("toolbar.switchDatabase")} (⌘K)`}
            >
              <Database className="h-[var(--size-btn-icon-sm)] w-[var(--size-btn-icon-sm)]" />
            </button>
          )}

          <div className="w-px h-3 bg-[var(--border-color)] mx-0.5" />

          <button
            className="flex items-center justify-center h-[var(--size-btn)] w-[var(--size-btn)] rounded-[var(--radius-btn)] hover:bg-[var(--sidebar-hover)] transition-colors"
            onClick={handleNewConnection}
            title={`${t("toolbar.newConnection")} (⌘N)`}
          >
            <Plus className="h-[var(--size-btn-icon-sm)] w-[var(--size-btn-icon-sm)] text-[var(--fg-secondary)]" />
          </button>

          {activeConnectionId && connState?.status === "connected" && (
            <button
              className={cn(
                "px-1.5 rounded-[var(--radius-btn)] font-mono text-[var(--fg-secondary)] hover:bg-[var(--sidebar-hover)] hover:text-[var(--fg)] transition-colors",
                "h-[var(--size-btn)] text-[length:var(--size-font-xs)]"
              )}
              onClick={() => {
                addTab({
                  type: "query",
                  title: t("tabs.newQuery"),
                  connectionId: activeConnectionId,
                  database: currentDb,
                  closable: true,
                  sql: "",
                });
              }}
              title={`${t("toolbar.sqlQuery")} (⌘T)`}
            >
              SQL
            </button>
          )}
        </div>

        <div className="flex-1" />

        {/* 居中胶囊：当前连接状态 */}
        {activeConn && (
          <div
            className="titlebar-no-drag absolute left-1/2 top-1/2 -translate-x-1/2 -translate-y-1/2"
            onMouseDown={(e) => e.stopPropagation()}
          >
            <div
              className={cn(
                "flex items-center gap-1.5 px-3 h-[var(--size-btn)] rounded-full border transition-colors",
                "bg-[var(--surface-secondary)] border-[var(--border-color)]"
              )}
            >
              {/* 连接状态指示灯 */}
              <div
                className={cn(
                  "w-1.5 h-1.5 rounded-full flex-shrink-0",
                  connState?.status === "connected" ? "bg-[var(--success)]" :
                  connState?.status === "connecting" ? "bg-yellow-400 animate-pulse" :
                  "bg-[var(--danger)]"
                )}
              />
              <span className="text-[length:var(--size-font-2xs)] text-[var(--fg)] font-medium truncate max-w-[120px]">
                {activeConn.name}
              </span>
              <span className="text-[length:var(--size-font-2xs)] text-[var(--fg-muted)]">·</span>
              <span className="text-[length:var(--size-font-2xs)] text-[var(--fg-muted)] truncate max-w-[140px]">
                {activeConn.host}:{activeConn.port}
              </span>
              {/* 重连按钮 */}
              <button
                className={cn(
                  "flex items-center justify-center h-4 w-4 rounded-full hover:bg-[var(--fg-muted)]/15 transition-colors ml-0.5",
                  reconnecting && "animate-spin"
                )}
                onClick={handleReconnect}
                disabled={reconnecting}
                title={t("toolbar.reconnect")}
              >
                <RefreshCw className="h-2.5 w-2.5 text-[var(--fg-secondary)]" />
              </button>
            </div>
          </div>
        )}

        <div className="flex-1" />

        {/* 右侧功能区，阻止 mousedown 冒泡避免误触最大化 */}
        <div className="titlebar-no-drag flex items-center gap-[var(--size-gap-sm)] mr-0.5" onMouseDown={(e) => e.stopPropagation()}>
          <button
            className="flex items-center justify-center h-[var(--size-btn)] w-[var(--size-btn)] rounded-[var(--radius-btn)] hover:bg-[var(--sidebar-hover)] transition-colors"
            onClick={() => setSearchOpen(true)}
            title={`${t("toolbar.quickSearch")} (⌘P)`}
          >
            <Search className="h-[var(--size-btn-icon-sm)] w-[var(--size-btn-icon-sm)] text-[var(--fg-secondary)]" />
          </button>
          <button
            className="flex items-center justify-center h-[var(--size-btn)] w-[var(--size-btn)] rounded-[var(--radius-btn)] hover:bg-[var(--sidebar-hover)] transition-colors"
            onClick={() => setAIPanelOpen(!aiPanelOpen)}
            title={t("toolbar.aiAssistant")}
          >
            <Sparkles className="h-[var(--size-btn-icon-sm)] w-[var(--size-btn-icon-sm)] text-[var(--fg-secondary)]" />
          </button>
          <button
            className="flex items-center justify-center h-[var(--size-btn)] w-[var(--size-btn)] rounded-[var(--radius-btn)] hover:bg-[var(--sidebar-hover)] transition-colors"
            onClick={() => setLogViewerOpen(true)}
            title={t("toolbar.viewLogs")}
          >
            <ScrollText className="h-[var(--size-btn-icon-sm)] w-[var(--size-btn-icon-sm)] text-[var(--fg-secondary)]" />
          </button>
          <button
            className="flex items-center justify-center h-[var(--size-btn)] w-[var(--size-btn)] rounded-[var(--radius-btn)] hover:bg-[var(--sidebar-hover)] transition-colors"
            onClick={() => setTheme(resolved === "dark" ? "light" : "dark")}
            title={resolved === "dark" ? t("toolbar.switchToLight") : t("toolbar.switchToDark")}
          >
            {resolved === "dark"
              ? <Sun className="h-[var(--size-btn-icon-sm)] w-[var(--size-btn-icon-sm)] text-[var(--fg-secondary)]" />
              : <Moon className="h-[var(--size-btn-icon-sm)] w-[var(--size-btn-icon-sm)] text-[var(--fg-secondary)]" />
            }
          </button>
          <button
            className="flex items-center justify-center h-[var(--size-btn)] w-[var(--size-btn)] rounded-[var(--radius-btn)] hover:bg-[var(--sidebar-hover)] transition-colors"
            onClick={() => setSettingsOpen(true)}
            title={`${t("toolbar.settings")} (⌘,)`}
          >
            <Settings className="h-[var(--size-btn-icon-sm)] w-[var(--size-btn-icon-sm)] text-[var(--fg-secondary)]" />
          </button>
        </div>
      </div>

      {/* ====== 主内容区 ====== */}
      <div className="absolute top-[var(--size-toolbar)] bottom-0 left-0 right-0 flex overflow-hidden">
        <WorkspaceBar />
        <Sidebar
          onNewConnection={handleNewConnection}
          onEditConnection={handleEditConnection}
        />

        <div className="flex-1 relative min-w-0">
          <div className="absolute top-0 left-0 right-0 z-10">
            <TabBar />
          </div>
          <div className="absolute top-[var(--size-tab)] bottom-0 left-0 right-0 overflow-hidden">
            <TabContent />
          </div>
        </div>

        {aiPanelOpen && (
          <AIPanel
            open={aiPanelOpen}
            onClose={() => setAIPanelOpen(false)}
            currentConnectionId={activeConnectionId || ""}
            currentDatabase={activeTab?.database}
            currentTable={activeTab?.table}
            width={aiPanelWidth}
            onWidthChange={setAIPanelWidth}
          />
        )}
      </div>

      {/* ====== 浮动弹窗层 ====== */}
      <DatabaseSwitcher
        open={dbSwitcherOpen}
        onClose={() => setDbSwitcherOpen(false)}
        connectionId={activeConnectionId || ""}
        currentDatabase={currentDb}
        onSelect={(dbName) => {
          if (activeConnectionId) {
            const { toggleNode, expandedNodes, addWorkspace } = useConnectionStore.getState();
            const connNodeId = `conn:${activeConnectionId}`;
            const dbNodeId = `db:${activeConnectionId}:${dbName}`;
            if (!expandedNodes.has(connNodeId)) toggleNode(connNodeId);
            if (!expandedNodes.has(dbNodeId)) toggleNode(dbNodeId);
            
            // 加入工作区并激活
            addWorkspace(activeConnectionId, dbName);
            loadTables(activeConnectionId, dbName);
            
            if (activeTabId) {
              const { tabs, updateTab } = useTabsStore.getState();
              // 如果只是想要让当前 tab 跟随数据库可继续保留该代码，或根据业务需求切断关联
              // updateTab(activeTabId, { database: dbName });
            }
          }
        }}
      />

      <ConnectionDialog
        open={connDialogOpen}
        connection={editingConnection}
        onClose={() => setConnDialogOpen(false)}
        onSave={handleSaveConnection}
        onTest={handleTestConnection}
      />
      <SettingsDialog
        open={settingsOpen}
        onClose={() => setSettingsOpen(false)}
      />
      <CommandPalette
        open={searchOpen}
        onClose={() => setSearchOpen(false)}
        onOpenSettings={() => setSettingsOpen(true)}
        onNewConnection={handleNewConnection}
      />

      {/* 日志查看弹窗 */}
      {logViewerOpen && <LogViewer onClose={() => setLogViewerOpen(false)} />}
    </div>
  );
}

/** 日志查看器弹窗 */
function LogViewer({ onClose }: { onClose: () => void }) {
  const [logContent, setLogContent] = useState("");
  const [logPath, setLogPath] = useState("");
  const [loading, setLoading] = useState(true);
  const contentRef = React.useRef<HTMLPreElement>(null);
  const { t } = useTranslation();

  const loadLog = useCallback(async () => {
    setLoading(true);
    try {
      const [content, path] = await Promise.all([
        import("../../../wailsjs/go/services/SettingsService").then((m) => m.GetLogContent()),
        import("../../../wailsjs/go/services/SettingsService").then((m) => m.GetLogPath()),
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
