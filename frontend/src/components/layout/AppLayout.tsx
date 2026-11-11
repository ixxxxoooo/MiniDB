import React, { useState, useCallback, useEffect, useMemo, useRef } from "react";
import { removeTabGuarded } from "@/lib/tabGuards";
import { ConfirmDialogHost } from "@/components/ui/ConfirmDialog";
import { Sidebar } from "./Sidebar";
import { TabBar } from "@/components/tabs/TabBar";
import { TabContent } from "@/components/tabs/TabContent";
import { ConnectionDialog } from "@/components/connection/ConnectionDialog";
import { SettingsDialog } from "@/components/settings/SettingsDialog";
import { DatabaseSwitcher } from "./DatabaseSwitcher";
import { WorkspaceBar } from "./WorkspaceBar";
import { WindowControls } from "./WindowControls";
import { ToastContainer } from "./ToastContainer";
import { LogViewer } from "./LogViewer";
import { ProcessListDialog } from "./ProcessListDialog";
import { ImportDialog } from "./ImportDialog";
import { UpdateBadge, UpdateNotifier } from "@/components/update/UpdateNotifier";
import { useConnectionStore } from "@/stores/connection";
import { useTabsStore } from "@/stores/tabs";
import { useUIStore, type ExportTask } from "@/stores/ui";
import { useThemeStore } from "@/stores/theme";
import { useDatabase } from "@/hooks/useDatabase";
import { CommandPalette } from "./CommandPalette";
import { AIPanel } from "@/components/ai/AIPanel";
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
  BarChart3,
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
import * as SchemaIndexService from "@/lib/wails/services/SchemaIndexService";
import type { SchemaIndexStatus } from "@/types/ai";

type RGB = { r: number; g: number; b: number };
type SchemaState = "ready" | "refreshing" | "stale" | "error" | "missing";

function getSchemaState(status: SchemaIndexStatus | null): SchemaState {
  if (!status || !status.exists) return "missing";
  if (status.refreshing) return "refreshing";
  if (status.lastError) return "error";
  if (status.dirty || status.stale) return "stale";
  return "ready";
}

function formatTimestamp(value?: string) {
  if (!value) return "—";
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) return value;
  return date.toLocaleString();
}

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

export function AppLayout() {
  const [connDialogOpen, setConnDialogOpen] = useState(false);
  const [editingConnection, setEditingConnection] =
    useState<ConnectionConfig | null>(null);
  const [settingsOpen, setSettingsOpen] = useState(false);
  const [aiPanelOpen, setAIPanelOpen] = useState(false);
  const [aiPanelWidth, setAIPanelWidth] = useState(510);
  const [searchOpen, setSearchOpen] = useState(false);
  const [dbSwitcherOpen, setDbSwitcherOpen] = useState(false);
  const [schemaPanelOpen, setSchemaPanelOpen] = useState(false);
  const [schemaStatus, setSchemaStatus] = useState<SchemaIndexStatus | null>(null);
  const [schemaStatusLoading, setSchemaStatusLoading] = useState(false);
  const schemaPanelRef = useRef<HTMLDivElement>(null);

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
  const { sidebarCollapsed, toggleSidebar, layoutMode, showScrollbar, logViewerOpen, setLogViewerOpen, processListConnId, setProcessListConnId, importTarget, setImportTarget } = useUIStore();
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
  const previousWorkspaceIdRef = useRef<string | null>(null);

  useEffect(() => {
    loadConnections();
  }, [loadConnections]);

  // 后台保活：连接被中断时状态灯变黄并提示，自动重连成功后恢复
  useEffect(() => {
    return EventsOn<{ connId: string; status: "lost" | "connected"; error?: string }>("connection:status", (ev) => {
      if (!ev?.connId) return;
      const store = useConnectionStore.getState();
      const name = store.connections.find((c) => c.id === ev.connId)?.name || ev.connId;
      if (ev.status === "lost") {
        store.setConnectionState(ev.connId, { id: ev.connId, status: "connecting" });
        useUIStore.getState().addToast("error", t("connection.lostReconnecting", { name }), 5000);
      } else {
        store.setConnectionState(ev.connId, { id: ev.connId, status: "connected", error: undefined });
        useUIStore.getState().addToast("success", t("connection.recovered", { name }));
      }
    });
  }, [t]);

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

    const workspaceChanged = previousWorkspaceIdRef.current !== activeWorkspaceId;
    previousWorkspaceIdRef.current = activeWorkspaceId;

    let cancelled = false;
    void (async () => {
      try {
        const currentState = useConnectionStore.getState().connectionStates[ws.connectionId];
        const status = currentState?.status;
        if (status === "connecting") return;

        const shouldReconnect = !status || status === "disconnected" || status === "error";
        if (shouldReconnect) {
          // 仅在切换工作区时自动重连，避免用户手动断开后被 connect 引用变化再次触发连接
          if (!workspaceChanged) return;
          await connect(ws.connectionId);
          if (cancelled) return;
        }

        const latestStatus = useConnectionStore.getState().connectionStates[ws.connectionId]?.status;
        if (latestStatus !== "connected") return;

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
  const isConnectionActive = connState?.status === "connected";
  const activeWorkspace = workspaces.find((ws) => ws.id === activeWorkspaceId);
  const availableDatabases = databases[activeConnectionId || ""] || [];
  const availableDatabaseNames = new Set(availableDatabases.map((db) => db.name));
  const activeTabWorkspaceId = activeTab ? `${activeTab.connectionId}:${activeTab.database}` : "";
  const activeTabDb =
    activeTab && (!activeWorkspaceId || activeTabWorkspaceId === activeWorkspaceId)
      ? activeTab.database
      : "";
  const workspaceDb = activeWorkspace?.database || activeTabDb;
  const fallbackDb =
    availableDatabases.find((db) => db.tableCount > 0)?.name ||
    availableDatabases[0]?.name ||
    "";
  const preferredDb =
    workspaceDb ||
    connState?.currentDatabase ||
    fallbackDb;
  const currentDb =
    !isConnectionActive ? "" :
    !preferredDb ? "" :
      workspaceDb ? workspaceDb :
      availableDatabaseNames.size === 0 || availableDatabaseNames.has(preferredDb) ? preferredDb :
        connState?.currentDatabase && availableDatabaseNames.has(connState.currentDatabase) ? connState.currentDatabase :
          fallbackDb;
  const schemaState = getSchemaState(schemaStatus);

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

  useEffect(() => {
    if (!schemaPanelOpen) return;
    const onMouseDown = (event: MouseEvent) => {
      if (schemaPanelRef.current && !schemaPanelRef.current.contains(event.target as Node)) {
        setSchemaPanelOpen(false);
      }
    };
    document.addEventListener("mousedown", onMouseDown);
    return () => document.removeEventListener("mousedown", onMouseDown);
  }, [schemaPanelOpen]);

  useEffect(() => {
    if (!activeConnectionId || !currentDb) {
      setSchemaStatus(null);
      return;
    }

    let cancelled = false;
    const loadSchemaStatus = async () => {
      try {
        const next = await SchemaIndexService.GetSchemaIndexStatus(activeConnectionId, currentDb);
        if (!cancelled) {
          setSchemaStatus(next as SchemaIndexStatus);
        }
      } catch {
        if (!cancelled) {
          setSchemaStatus(null);
        }
      }
    };

    void loadSchemaStatus();
    const off = EventsOn("schema:index_status_changed", () => {
      void loadSchemaStatus();
    });
    return () => {
      cancelled = true;
      off();
    };
  }, [activeConnectionId, currentDb]);

  // 重连当前激活连接，保持在当前数据库不跳转
  const handleReconnect = useCallback(async () => {
    if (!activeConnectionId) return;
    const currentDatabase = currentDb;
    setReconnecting(true);
    try {
      await disconnect(activeConnectionId, { preserveTabs: true });
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

  const handleRefreshSchemaIndex = useCallback(async () => {
    if (!activeConnectionId || !currentDb) return;
    setSchemaStatusLoading(true);
    try {
      const next = await SchemaIndexService.RefreshSchemaIndex(activeConnectionId, currentDb);
      setSchemaStatus(next as SchemaIndexStatus);
    } catch (error) {
      console.error("刷新 schema 索引失败:", error);
    } finally {
      setSchemaStatusLoading(false);
    }
  }, [activeConnectionId, currentDb]);

  useKeyboard({
    // 搜索命令面板：⌘P
    "mod+p": () => setSearchOpen(true),
    // 切换 AI 面板：⌘I
    "mod+i": () => setAIPanelOpen((prev) => !prev),
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
      const { activeTabId, tabs } = useTabsStore.getState();
      if (!activeTabId) return;
      const tab = tabs.find((t) => t.id === activeTabId);
      if (tab?.closable) void removeTabGuarded(activeTabId);
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
              {activeConn.readOnly && (
                <span className="px-1 rounded-[var(--radius-sm)] bg-[var(--warning)]/15 text-[var(--warning)] text-[length:var(--size-font-2xs)] font-medium">
                  {t("connection.readOnlyBadge")}
                </span>
              )}
              {/* 当前数据库名 */}
              {isConnectionActive && currentDb && (
                <>
                  <span className="text-[length:var(--size-font-2xs)] text-[var(--fg-muted)]">/</span>
                  <span className="text-[length:var(--size-font-2xs)] text-[var(--fg-secondary)] truncate max-w-[100px]">
                    {currentDb}
                  </span>
                </>
              )}
              {isConnectionActive && (
                <>
                  <span className="text-[length:var(--size-font-2xs)] text-[var(--fg-muted)]">·</span>
                  {/* 数据库类型 + 版本号 */}
                  <span className="text-[length:var(--size-font-2xs)] text-[var(--fg-muted)] truncate max-w-[180px]">
                    {DRIVER_LABELS[(activeConn.type as keyof typeof DRIVER_LABELS) || "mysql"]}
                    {connState?.serverVersion ? ` ${connState.serverVersion}` : ""}
                  </span>
                </>
              )}
              <Tooltip delayDuration={300}>
                <TooltipTrigger asChild>
                  <button
                    type="button"
                    className={cn(
                      "flex items-center justify-center h-4 w-4 rounded-full transition-colors ml-0.5",
                      "hover:bg-[var(--fg-muted)]/15",
                      schemaState === "ready" && "text-[var(--success)]",
                      schemaState === "refreshing" && "text-[var(--warning)]",
                      schemaState === "stale" && "text-[var(--warning)]",
                      schemaState === "error" && "text-[var(--danger)]",
                      schemaState === "missing" && "text-[var(--fg-muted)]"
                    )}
                    onClick={() => setSchemaPanelOpen((open) => !open)}
                  >
                    <BarChart3 className={cn("h-2.5 w-2.5", schemaState === "refreshing" && "animate-pulse")} />
                  </button>
                </TooltipTrigger>
                <TooltipContent side="bottom">{t("statusBar.schemaIndex")}</TooltipContent>
              </Tooltip>
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
            {schemaPanelOpen && (
              <div
                ref={schemaPanelRef}
                className={cn(
                  "absolute left-1/2 top-[calc(100%+8px)] -translate-x-1/2 z-[80] min-w-[260px] rounded-[var(--radius-panel)] border shadow-lg p-3 text-xs",
                  "bg-[var(--surface-elevated)] border-[var(--border-color)] text-[var(--fg)]"
                )}
              >
                <div className="flex items-center justify-between gap-2 mb-2">
                  <div className="font-medium">{t("statusBar.schemaIndex")}</div>
                  {schemaStatus?.dirty && (
                    <span className="text-[var(--warning)]">{t("statusBar.schemaDirty")}</span>
                  )}
                </div>

                <div className="space-y-1.5 text-[var(--fg-secondary)]">
                  <div className="flex items-center justify-between gap-2">
                    <span>{t("statusBar.schemaState")}</span>
                    <span>
                      {schemaState === "refreshing" ? t("statusBar.schemaRefreshing") :
                        schemaState === "stale" ? t("statusBar.schemaStale") :
                          schemaState === "error" ? t("statusBar.schemaError") :
                            schemaState === "missing" ? t("statusBar.schemaMissing") :
                              t("statusBar.schemaReady")}
                    </span>
                  </div>
                  <div className="flex items-center justify-between gap-2">
                    <span>{t("statusBar.lastRefreshed")}</span>
                    <span className="text-right">{formatTimestamp(schemaStatus?.lastRefreshedAt)}</span>
                  </div>
                  <div className="flex items-center justify-between gap-2">
                    <span>{t("statusBar.indexedTables")}</span>
                    <span>{schemaStatus?.tableCount ?? 0}</span>
                  </div>
                  <div className="flex items-center justify-between gap-2">
                    <span>{t("statusBar.schemaSource")}</span>
                    <span>{schemaStatus?.source || "—"}</span>
                  </div>
                  {schemaStatus?.lastError && (
                    <div className="rounded-[var(--radius-btn)] bg-[var(--danger)]/10 text-[var(--danger)] px-2 py-1 flex items-start gap-1">
                      <AlertCircle className="h-3 w-3 mt-0.5 flex-shrink-0" />
                      <span className="break-all">{schemaStatus.lastError}</span>
                    </div>
                  )}
                </div>

                <div className="mt-3 flex justify-end">
                  <button
                    type="button"
                    className={cn(
                      "inline-flex items-center gap-1 rounded-[var(--radius-btn)] border px-2 py-1 transition-colors",
                      "border-[var(--border-color)] hover:bg-[var(--sidebar-hover)] disabled:opacity-60"
                    )}
                    onClick={handleRefreshSchemaIndex}
                    disabled={!activeConnectionId || !currentDb || schemaStatusLoading}
                  >
                    <RefreshCw className={cn("h-3 w-3", schemaStatusLoading && "animate-spin")} />
                    <span>{t("statusBar.refreshSchemaIndex")}</span>
                  </button>
                </div>
              </div>
            )}
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
            <TooltipContent side="bottom">{`${t("toolbar.aiAssistant")} (⌘I)`}</TooltipContent>
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
          <UpdateBadge />
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
          <AIPanel
            open={aiPanelOpen}
            onClose={() => setAIPanelOpen(false)}
            currentConnectionId={activeConnectionId || ""}
            currentDatabase={currentDb}
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

      {processListConnId && <ProcessListDialog connectionId={processListConnId} onClose={() => setProcessListConnId(null)} />}
      {importTarget && <ImportDialog target={importTarget} onClose={() => setImportTarget(null)} />}

      {/* 日志查看弹窗 */}
      {logViewerOpen && <LogViewer onClose={() => setLogViewerOpen(false)} />}

      {/* 全局 Toast 通知 */}
      <ToastContainer />
      <UpdateNotifier />
      <ConfirmDialogHost />
    </div>
  );
}

