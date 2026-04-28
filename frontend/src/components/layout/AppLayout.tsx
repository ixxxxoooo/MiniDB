import React, { Suspense, lazy, useState, useCallback, useEffect, useMemo, useRef } from "react";
import { Sidebar } from "./Sidebar";
import { TabBar } from "@/components/tabs/TabBar";
import { TabContent } from "@/components/tabs/TabContent";
import { ConnectionDialog } from "@/components/connection/ConnectionDialog";
import { SettingsDialog } from "@/components/settings/SettingsDialog";
import { DatabaseSwitcher } from "./DatabaseSwitcher";
import { WorkspaceBar } from "./WorkspaceBar";
import { useConnectionStore } from "@/stores/connection";
import { useTabsStore } from "@/stores/tabs";
import { useUIStore, type ExportTask } from "@/stores/ui";
import { useThemeStore } from "@/stores/theme";
import { useDatabase } from "@/hooks/useDatabase";
import { CommandPalette } from "./CommandPalette";
import { useKeyboard } from "@/hooks/useKeyboard";
import { useTranslation } from "@/i18n";
import { DRIVER_LABELS, type ConnectionConfig } from "@/types/connection";
import { DRIVER_COLORS } from "@/components/icons/DatabaseIcons";
import { cn } from "@/lib/utils";
import {
  Database,
  Plug,
  Unplug,
  Settings,
  Moon,
  Sun,
  Sparkles,
  Search,
  ScrollText,
  RefreshCw,
  X,
  Loader2,
  Check,
  AlertCircle,
  Info,
  StopCircle,
  FileDown,
} from "lucide-react";

import { Button } from "@/components/ui/button";
import { Tooltip, TooltipContent, TooltipTrigger } from "@/components/ui/tooltip";
import { EventsOn } from "@/lib/wails/runtime";
import * as ExportService from "@/lib/wails/services/ExportService";

const AIPanel = lazy(() => import("@/components/ai/AIPanel").then((m) => ({ default: m.AIPanel })));

type RGB = { r: number; g: number; b: number };

function normalizeHexColor(color: string): string | null {
  const value = color.trim();
  if (!/^#([0-9a-fA-F]{3}|[0-9a-fA-F]{6})$/.test(value)) return null;
  if (value.length === 4) {
    const r = value[1];
    const g = value[2];
    const b = value[3];
    return `#${r}${r}${g}${g}${b}${b}`.toLowerCase();
  }
  return value.toLowerCase();
}

function hexToRgb(hex: string): RGB | null {
  const normalized = normalizeHexColor(hex);
  if (!normalized) return null;
  const value = normalized.slice(1);
  const r = Number.parseInt(value.slice(0, 2), 16);
  const g = Number.parseInt(value.slice(2, 4), 16);
  const b = Number.parseInt(value.slice(4, 6), 16);
  return { r, g, b };
}

function mixHex(baseHex: string, targetHex: string, amount: number): string {
  const base = hexToRgb(baseHex);
  const target = hexToRgb(targetHex);
  if (!base || !target) return baseHex;
  const mix = (a: number, b: number) => Math.round(a + (b - a) * amount);
  const r = mix(base.r, target.r).toString(16).padStart(2, "0");
  const g = mix(base.g, target.g).toString(16).padStart(2, "0");
  const b = mix(base.b, target.b).toString(16).padStart(2, "0");
  return `#${r}${g}${b}`;
}


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
      import("@/lib/wails/runtime").then((r) =>
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
  const { t } = useTranslation();

  const handleClose = () => {
    import("@/lib/wails/runtime").then((r) => r.Quit());
  };
  const handleMinimise = () => {
    import("@/lib/wails/runtime").then((r) => r.WindowMinimise());
  };
  const handleMaximise = () => {
    import("@/lib/wails/runtime").then((r) => r.WindowToggleMaximise());
  };

  return (
    <div
      className="flex items-center gap-[7px] px-3 titlebar-no-drag flex-shrink-0"
      onMouseEnter={() => setHovered(true)}
      onMouseLeave={() => setHovered(false)}
    >
      {/* 关闭 */}
      <Tooltip delayDuration={250}>
        <TooltipTrigger asChild>
          <button
            onClick={handleClose}
            className="w-[12px] h-[12px] rounded-full flex items-center justify-center transition-colors focus:outline-none"
            style={{ backgroundColor: hovered ? "#ff5f57" : "var(--fg-muted)" }}
          >
            {hovered && (
              <svg width="6" height="6" viewBox="0 0 6 6" fill="none">
                <path d="M0.5 0.5L5.5 5.5M5.5 0.5L0.5 5.5" stroke="#4a0002" strokeWidth="1.2" strokeLinecap="round" />
              </svg>
            )}
          </button>
        </TooltipTrigger>
        <TooltipContent side="bottom">{t("common.close")}</TooltipContent>
      </Tooltip>
      {/* 最小化 */}
      <Tooltip delayDuration={250}>
        <TooltipTrigger asChild>
          <button
            onClick={handleMinimise}
            className="w-[12px] h-[12px] rounded-full flex items-center justify-center transition-colors focus:outline-none"
            style={{ backgroundColor: hovered ? "#febc2e" : "var(--fg-muted)" }}
          >
            {hovered && (
              <svg width="6" height="2" viewBox="0 0 6 2" fill="none">
                <path d="M0.5 1H5.5" stroke="#995700" strokeWidth="1.2" strokeLinecap="round" />
              </svg>
            )}
          </button>
        </TooltipTrigger>
        <TooltipContent side="bottom">{t("window.minimize")}</TooltipContent>
      </Tooltip>
      {/* 最大化 */}
      <Tooltip delayDuration={250}>
        <TooltipTrigger asChild>
          <button
            onClick={handleMaximise}
            className="w-[12px] h-[12px] rounded-full flex items-center justify-center transition-colors focus:outline-none"
            style={{ backgroundColor: hovered ? "#28c840" : "var(--fg-muted)" }}
          >
            {hovered && (
              <svg width="6" height="6" viewBox="0 0 6 6" fill="none">
                <path d="M1 4.5L3 1.5L5 4.5" stroke="#006500" strokeWidth="1.2" strokeLinecap="round" strokeLinejoin="round" />
              </svg>
            )}
          </button>
        </TooltipTrigger>
        <TooltipContent side="bottom">{t("window.maximize")}</TooltipContent>
      </Tooltip>
    </div>
  );
}

export function AppLayout() {
  const [connDialogOpen, setConnDialogOpen] = useState(false);
  const [editingConnection, setEditingConnection] =
    useState<ConnectionConfig | null>(null);
  const [settingsOpen, setSettingsOpen] = useState(false);
  const [aiPanelOpen, setAIPanelOpen] = useState(false);
  const [aiPanelWidth, setAIPanelWidth] = useState(510);
  const [searchOpen, setSearchOpen] = useState(false);
  const [dbSwitcherOpen, setDbSwitcherOpen] = useState(false);
  const [logViewerOpen, setLogViewerOpen] = useState(false);

  // 标题栏双击最大化/还原
  const { handleMouseDown: handleTitlebarMouseDown } = useTitlebarDoubleClick();

  const { t } = useTranslation();
  const activeTab = useTabsStore((s) =>
    s.activeTabId ? s.tabs.find((tab) => tab.id === s.activeTabId) : undefined
  );
  const addTab = useTabsStore((s) => s.addTab);
  const switchWorkspace = useTabsStore((s) => s.switchWorkspace);
  const activeConnectionId = useConnectionStore((s) => s.activeConnectionId);
  const databases = useConnectionStore((s) => s.databases);
  const connections = useConnectionStore((s) => s.connections);
  const connectionStates = useConnectionStore((s) => s.connectionStates);
  const activeWorkspaceId = useConnectionStore((s) => s.activeWorkspaceId);
  const workspaces = useConnectionStore((s) => s.workspaces);
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
  const restoredSessionRef = useRef(false);

  useEffect(() => {
    loadConnections();
  }, [loadConnections]);

  useEffect(() => {
    if (restoredSessionRef.current) return;
    if (connections.length === 0) return;

    const { workspaces: wsList, activeWorkspaceId: activeWsId, setActiveWorkspace } = useConnectionStore.getState();
    const restoredWorkspace = (activeWsId ? wsList.find((ws) => ws.id === activeWsId) : undefined) || wsList[0];
    const restoredConnId = restoredWorkspace?.connectionId || activeConnectionId;
    if (!restoredConnId || !connections.some((c) => c.id === restoredConnId)) return;
    restoredSessionRef.current = true;

    if (restoredWorkspace) {
      setActiveWorkspace(restoredWorkspace.id);
      switchWorkspace(restoredWorkspace.id);
    }

    const connState = useConnectionStore.getState().connectionStates[restoredConnId];
    const shouldReconnect = !connState || connState.status === "disconnected" || connState.status === "error";
    if (!shouldReconnect) return;

    void (async () => {
      try {
        await connect(restoredConnId);
        if (restoredWorkspace?.database) {
          await loadTables(restoredConnId, restoredWorkspace.database);
        }
      } catch (e) {
        console.error("[AppLayout] 恢复上次会话失败:", e);
      }
    })();
  }, [activeConnectionId, connect, connections, loadTables, switchWorkspace]);

  useEffect(() => {
    if (activeWorkspaceId) {
      switchWorkspace(activeWorkspaceId);
    }
  }, [activeWorkspaceId, switchWorkspace]);

  useEffect(() => {
    if (!activeWorkspaceId) return;
    const ws = workspaces.find((item) => item.id === activeWorkspaceId);
    if (!ws) return;

    let cancelled = false;
    void (async () => {
      try {
        const currentState = useConnectionStore.getState().connectionStates[ws.connectionId];
        const status = currentState?.status;
        if (status === "connecting") return;

        const shouldReconnect = !status || status === "disconnected" || status === "error";
        if (shouldReconnect) {
          await connect(ws.connectionId);
          if (cancelled) return;
        }

        await loadTables(ws.connectionId, ws.database);
      } catch (e) {
        if (!cancelled) {
          console.error("[AppLayout] 切换工作区后自动检测连接失败:", e);
        }
      }
    })();

    return () => {
      cancelled = true;
    };
  }, [activeWorkspaceId, connect, loadTables, workspaces]);

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
  const activeWorkspace = workspaces.find((ws) => ws.id === activeWorkspaceId);
  const currentDb =
    activeTab?.database ||
    activeWorkspace?.database ||
    databases[activeConnectionId || ""]?.find((db) => db.tableCount > 0)?.name ||
    databases[activeConnectionId || ""]?.[0]?.name ||
    "";

  const connectionThemeVars = useMemo<React.CSSProperties>(() => {
    if (!activeConn) return {};
    const baseAccent =
      normalizeHexColor(activeConn.color || "") ||
      normalizeHexColor(DRIVER_COLORS[activeConn.type]) ||
      null;
    if (!baseAccent) return {};
    const rgb = hexToRgb(baseAccent);
    if (!rgb) return {};

    const accentHover =
      resolved === "dark"
        ? mixHex(baseAccent, "#ffffff", 0.14)
        : mixHex(baseAccent, "#000000", 0.12);

    const rowSelectedAlpha = resolved === "dark" ? 0.28 : 0.16;
    const sidebarActiveAlpha = resolved === "dark" ? 0.22 : 0.14;

    return {
      "--accent": baseAccent,
      "--accent-hover": accentHover,
      "--sidebar-accent": baseAccent,
      "--tab-active-border": baseAccent,
      "--row-selected": `rgba(${rgb.r}, ${rgb.g}, ${rgb.b}, ${rowSelectedAlpha})`,
      "--sidebar-active": `rgba(${rgb.r}, ${rgb.g}, ${rgb.b}, ${sidebarActiveAlpha})`,
    } as React.CSSProperties;
  }, [activeConn, resolved]);

  // 重连当前激活连接，保持在当前数据库不跳转
  const handleReconnect = useCallback(async () => {
    if (!activeConnectionId) return;
    const currentDatabase = currentDb;
    setReconnecting(true);
    try {
      await disconnect(activeConnectionId);
      await connect(activeConnectionId);
      // 重连后恢复到之前的数据库，而不是跳到第一个
      if (currentDatabase) {
        const { addWorkspace } = useConnectionStore.getState();
        addWorkspace(activeConnectionId, currentDatabase);
        await loadTables(activeConnectionId, currentDatabase);
      }
      console.log("[AppLayout] 重连成功: id=%s db=%s", activeConnectionId, currentDatabase);
    } catch (e: any) {
      console.error("[AppLayout] 重连失败:", e);
    } finally {
      setReconnecting(false);
    }
  }, [activeConnectionId, currentDb, disconnect, connect, loadTables]);

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
      const { activeTabId, tabs, removeTab } = useTabsStore.getState();
      if (!activeTabId) return;
      const tab = tabs.find((t) => t.id === activeTabId);
      if (tab?.closable) removeTab(activeTabId);
    },
    "mod+n": () => handleNewConnection(),
    // 导航：切换当前工作区 Tab ⌘] / ⌘[
    "mod+]": () => {
      const { activeWorkspaceId } = useConnectionStore.getState();
      if (!activeWorkspaceId) return;
      const { tabs, activeTabId, setActiveTab } = useTabsStore.getState();
      const workspaceTabs = tabs.filter((t) => `${t.connectionId}:${t.database}` === activeWorkspaceId);
      if (workspaceTabs.length <= 1) return;
      const idx = workspaceTabs.findIndex((t) => t.id === activeTabId);
      if (idx === -1) return;
      const nextIdx = (idx + 1) % workspaceTabs.length;
      setActiveTab(workspaceTabs[nextIdx].id);
    },
    "mod+[": () => {
      const { activeWorkspaceId } = useConnectionStore.getState();
      if (!activeWorkspaceId) return;
      const { tabs, activeTabId, setActiveTab } = useTabsStore.getState();
      const workspaceTabs = tabs.filter((t) => `${t.connectionId}:${t.database}` === activeWorkspaceId);
      if (workspaceTabs.length <= 1) return;
      const idx = workspaceTabs.findIndex((t) => t.id === activeTabId);
      if (idx === -1) return;
      const prevIdx = (idx - 1 + workspaceTabs.length) % workspaceTabs.length;
      setActiveTab(workspaceTabs[prevIdx].id);
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
        showScrollbar ? "show-scrollbar" : "hide-scrollbar"
      )}
      style={connectionThemeVars}
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

        <div className="titlebar-no-drag flex items-center gap-[var(--size-gap-sm)] ml-1" onMouseDown={(e) => e.stopPropagation()}>
          <Tooltip delayDuration={300}>
            <TooltipTrigger asChild>
              <button
                className="flex items-center justify-center h-[var(--size-btn)] w-[var(--size-btn)] rounded-full text-[var(--fg-secondary)] hover:bg-[var(--sidebar-hover)] transition-colors"
                onClick={handleNewConnection}
              >
                <Plug className="h-[var(--size-btn-icon-sm)] w-[var(--size-btn-icon-sm)]" />
              </button>
            </TooltipTrigger>
            <TooltipContent side="bottom">{`${t("toolbar.newConnection")} (⌘N)`}</TooltipContent>
          </Tooltip>

          <Tooltip delayDuration={300}>
            <TooltipTrigger asChild>
              <button
                className={cn(
                  "flex items-center justify-center h-[var(--size-btn)] w-[var(--size-btn)] rounded-full transition-colors",
                  activeConnectionId
                    ? "text-[var(--fg-muted)] hover:bg-[var(--sidebar-hover)] hover:text-[var(--danger)]"
                    : "text-[var(--fg-muted)] opacity-35 cursor-not-allowed"
                )}
                onClick={async () => {
                  if (!activeConnectionId) return;
                  await disconnect(activeConnectionId);
                }}
                disabled={!activeConnectionId}
              >
                <Unplug className="h-[var(--size-btn-icon-sm)] w-[var(--size-btn-icon-sm)]" />
              </button>
            </TooltipTrigger>
            <TooltipContent side="bottom">{t("sidebar.disconnect")}</TooltipContent>
          </Tooltip>

          <div className="w-px h-3 bg-[var(--border-color)] mx-0.5" />

          {activeConn && (connState?.status === "connected" || reconnecting) && (
            <Tooltip delayDuration={300}>
              <TooltipTrigger asChild>
                <button
                  className={cn(
                    "flex items-center justify-center rounded-[var(--radius-btn)] font-medium transition-colors h-[var(--size-btn)] w-[var(--size-btn)]",
                    "hover:bg-[var(--sidebar-hover)] text-[var(--fg-secondary)]",
                    reconnecting && "opacity-50 pointer-events-none cursor-not-allowed"
                  )}
                  onClick={() => setDbSwitcherOpen(true)}
                >
                  <Database className="h-[var(--size-btn-icon-sm)] w-[var(--size-btn-icon-sm)]" />
                </button>
              </TooltipTrigger>
              <TooltipContent side="bottom">{`${t("toolbar.switchDatabase")} (⌘K)`}</TooltipContent>
            </Tooltip>
          )}

          {activeConnectionId && (connState?.status === "connected" || reconnecting) && (
            <Tooltip delayDuration={300}>
              <TooltipTrigger asChild>
                <button
                  className={cn(
                    "px-1.5 rounded-[var(--radius-btn)] font-mono text-[var(--fg-secondary)] hover:bg-[var(--sidebar-hover)] hover:text-[var(--fg)] transition-colors",
                    "h-[var(--size-btn)] text-[length:var(--size-font-xs)]",
                    reconnecting && "opacity-50 pointer-events-none cursor-not-allowed"
                  )}
                  onClick={() => {
                    if (reconnecting) return;
                    addTab({
                      type: "query",
                      title: t("tabs.newQuery"),
                      connectionId: activeConnectionId,
                      database: currentDb,
                      closable: true,
                      sql: "",
                    });
                  }}
                >
                  SQL
                </button>
              </TooltipTrigger>
              <TooltipContent side="bottom">{`${t("toolbar.sqlQuery")} (⌘T)`}</TooltipContent>
            </Tooltip>
          )}
        </div>

        <div className="flex-1" />

        {/* 居中区域：当前连接信息 */}
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
              {/* 连接名 */}
              <span className="text-[length:var(--size-font-2xs)] text-[var(--fg)] font-medium truncate max-w-[120px]">
                {activeConn.name}
              </span>
              {/* 当前数据库名 */}
              {currentDb && (
                <>
                  <span className="text-[length:var(--size-font-2xs)] text-[var(--fg-muted)]">/</span>
                  <span className="text-[length:var(--size-font-2xs)] text-[var(--fg-secondary)] truncate max-w-[100px]">
                    {currentDb}
                  </span>
                </>
              )}
              <span className="text-[length:var(--size-font-2xs)] text-[var(--fg-muted)]">·</span>
              {/* 数据库类型 + 版本号 */}
              <span className="text-[length:var(--size-font-2xs)] text-[var(--fg-muted)] truncate max-w-[180px]">
                {DRIVER_LABELS[(activeConn.type as keyof typeof DRIVER_LABELS) || "mysql"]}
                {connState?.serverVersion ? ` ${connState.serverVersion}` : ""}
              </span>
              {/* 重连按钮 */}
              <Tooltip delayDuration={300}>
                <TooltipTrigger asChild>
                  <button
                    className={cn(
                      "flex items-center justify-center h-4 w-4 rounded-full hover:bg-[var(--fg-muted)]/15 transition-colors ml-0.5",
                      reconnecting && "animate-spin"
                    )}
                    onClick={handleReconnect}
                    disabled={reconnecting}
                  >
                    <RefreshCw className="h-2.5 w-2.5 text-[var(--fg-secondary)]" />
                  </button>
                </TooltipTrigger>
                <TooltipContent side="bottom">{t("toolbar.reconnect")}</TooltipContent>
              </Tooltip>
            </div>
          </div>
        )}

        <div className="flex-1" />

        {/* 右侧功能区，阻止 mousedown 冒泡避免误触最大化 */}
        <div className="titlebar-no-drag flex items-center gap-[var(--size-gap-sm)] mr-0.5" onMouseDown={(e) => e.stopPropagation()}>
          <Tooltip delayDuration={300}>
            <TooltipTrigger asChild>
              <button
                className="flex items-center justify-center h-[var(--size-btn)] w-[var(--size-btn)] rounded-[var(--radius-btn)] hover:bg-[var(--sidebar-hover)] transition-colors"
                onClick={() => setSearchOpen(true)}
              >
                <Search className="h-[var(--size-btn-icon-sm)] w-[var(--size-btn-icon-sm)] text-[var(--fg-secondary)]" />
              </button>
            </TooltipTrigger>
            <TooltipContent side="bottom">{`${t("toolbar.quickSearch")} (⌘P)`}</TooltipContent>
          </Tooltip>
          <Tooltip delayDuration={300}>
            <TooltipTrigger asChild>
              <button
                className="flex items-center justify-center h-[var(--size-btn)] w-[var(--size-btn)] rounded-[var(--radius-btn)] hover:bg-[var(--sidebar-hover)] transition-colors"
                onClick={() => setAIPanelOpen(!aiPanelOpen)}
              >
                <Sparkles className="h-[var(--size-btn-icon-sm)] w-[var(--size-btn-icon-sm)] text-[var(--fg-secondary)]" />
              </button>
            </TooltipTrigger>
            <TooltipContent side="bottom">{t("toolbar.aiAssistant")}</TooltipContent>
          </Tooltip>
          <Tooltip delayDuration={300}>
            <TooltipTrigger asChild>
              <button
                className="flex items-center justify-center h-[var(--size-btn)] w-[var(--size-btn)] rounded-[var(--radius-btn)] hover:bg-[var(--sidebar-hover)] transition-colors"
                onClick={() => setTheme(resolved === "dark" ? "light" : "dark")}
              >
                {resolved === "dark"
                  ? <Sun className="h-[var(--size-btn-icon-sm)] w-[var(--size-btn-icon-sm)] text-[var(--fg-secondary)]" />
                  : <Moon className="h-[var(--size-btn-icon-sm)] w-[var(--size-btn-icon-sm)] text-[var(--fg-secondary)]" />
                }
              </button>
            </TooltipTrigger>
            <TooltipContent side="bottom">{resolved === "dark" ? t("toolbar.switchToLight") : t("toolbar.switchToDark")}</TooltipContent>
          </Tooltip>
          <Tooltip delayDuration={300}>
            <TooltipTrigger asChild>
              <button
                className="flex items-center justify-center h-[var(--size-btn)] w-[var(--size-btn)] rounded-[var(--radius-btn)] hover:bg-[var(--sidebar-hover)] transition-colors"
                onClick={() => setSettingsOpen(true)}
              >
                <Settings className="h-[var(--size-btn-icon-sm)] w-[var(--size-btn-icon-sm)] text-[var(--fg-secondary)]" />
              </button>
            </TooltipTrigger>
            <TooltipContent side="bottom">{`${t("toolbar.settings")} (⌘,)`}</TooltipContent>
          </Tooltip>
        </div>
      </div>

      {/* ====== 主内容区 ====== */}
      <div className="absolute top-[var(--size-toolbar)] bottom-0 left-0 right-0 flex overflow-hidden">
        <WorkspaceBar onEditConnection={handleEditConnection} />
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
          <Suspense fallback={null}>
            <AIPanel
              open={aiPanelOpen}
              onClose={() => setAIPanelOpen(false)}
              currentConnectionId={activeConnectionId || ""}
              currentDatabase={currentDb}
              currentTable={activeTab?.table}
              width={aiPanelWidth}
              onWidthChange={setAIPanelWidth}
            />
          </Suspense>
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

      {/* 全局 Toast 通知 */}
      <ToastContainer />
    </div>
  );
}

/** 全局 Toast 通知 + 导出进度容器 */
function ToastContainer() {
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
