import React, { useState, useCallback, useEffect } from "react";
import { Sidebar } from "./Sidebar";
import { StatusBar } from "./StatusBar";
import { TabBar } from "@/components/tabs/TabBar";
import { TabContent } from "@/components/tabs/TabContent";
import { ConnectionDialog } from "@/components/connection/ConnectionDialog";
import { SettingsDialog } from "@/components/settings/SettingsDialog";
import { AIPanel } from "@/components/ai/AIPanel";
import { DatabaseSwitcher } from "./DatabaseSwitcher";
import { useConnectionStore } from "@/stores/connection";
import { useTabsStore } from "@/stores/tabs";
import { useUIStore } from "@/stores/ui";
import { useThemeStore } from "@/stores/theme";
import { useDatabase } from "@/hooks/useDatabase";
import { CommandPalette } from "./CommandPalette";
import { useKeyboard } from "@/hooks/useKeyboard";
import type { ConnectionConfig } from "@/types/connection";
import { cn } from "@/lib/utils";
import {
  PanelLeftClose,
  PanelLeftOpen,
  Plus,
  Settings,
  Moon,
  Sun,
  Sparkles,
  Search,
  ChevronDown,
  Code,
  ScrollText,
  X,
  Loader2,
} from "lucide-react";
import { Button } from "@/components/ui/button";

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

  const { activeTabId, tabs, addTab, removeTab } = useTabsStore();
  const activeTab = tabs.find((t) => t.id === activeTabId);
  const { activeConnectionId, databases, connections, connectionStates } = useConnectionStore();
  const { sidebarCollapsed, toggleSidebar } = useUIStore();
  const { resolved, setTheme } = useThemeStore();

  const {
    loadConnections,
    saveConnection,
    testConnection,
    connect,
    loadTables,
  } = useDatabase();

  useEffect(() => {
    loadConnections();
  }, [loadConnections]);

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

  useKeyboard({
    "mod+k": () => setSearchOpen(true),
    "mod+t": () => {
      if (activeConnectionId) {
        addTab({
          type: "query",
          title: "新查询",
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
  });

  return (
    <div className="h-full flex flex-col bg-[var(--surface)]">
      {/* ====== 顶部工具栏 ====== */}
      <div
        className={cn(
          "h-[38px] flex items-center px-2 gap-0.5 border-b vibrancy titlebar-drag flex-shrink-0",
          "bg-[var(--toolbar-bg)] border-[var(--toolbar-border)]"
        )}
        style={{ zIndex: 40 }}
      >
        {/* macOS 红绿灯按钮占位 */}
        <div className="w-[68px] flex-shrink-0" />

        {/* 左侧功能区 */}
        <div className="titlebar-no-drag flex items-center gap-0.5">
          <Button variant="ghost" size="icon" className="h-7 w-7" onClick={toggleSidebar} title={sidebarCollapsed ? "展开侧边栏" : "收起侧边栏"}>
            {sidebarCollapsed ? <PanelLeftOpen className="h-3.5 w-3.5" /> : <PanelLeftClose className="h-3.5 w-3.5" />}
          </Button>

          {activeConn && connState?.status === "connected" && (
            <button
              className={cn(
                "flex items-center gap-1.5 px-2.5 py-1.5 rounded-md text-xs font-medium transition-colors",
                "hover:bg-[var(--sidebar-hover)] text-[var(--fg)]"
              )}
              onClick={() => setDbSwitcherOpen(true)}
              title="切换数据库"
            >
              <div
                className="w-2 h-2 rounded-full flex-shrink-0"
                style={{ backgroundColor: activeConn.color || "#007aff" }}
              />
              <span className="max-w-[100px] truncate">{activeConn.name}</span>
              {currentDb && (
                <>
                  <span className="text-[var(--fg-muted)]">:</span>
                  <span className="text-[var(--fg-secondary)] max-w-[80px] truncate">{currentDb}</span>
                </>
              )}
              <ChevronDown className="h-3 w-3 text-[var(--fg-muted)]" />
            </button>
          )}

          <Button variant="ghost" size="icon" className="h-7 w-7" onClick={handleNewConnection} title="新建连接 (⌘N)">
            <Plus className="h-3.5 w-3.5" />
          </Button>

          {activeConnectionId && connState?.status === "connected" && (
            <Button variant="ghost" size="icon" className="h-7 w-7" onClick={() => {
              addTab({
                type: "query",
                title: "SQL Query",
                connectionId: activeConnectionId,
                database: currentDb,
                closable: true,
                sql: "",
              });
            }} title="SQL 查询 (⌘T)">
              <Code className="h-3.5 w-3.5" />
            </Button>
          )}
        </div>

        {/* 中间弹性空间 */}
        <div className="flex-1" />

        {/* 居中功能区 */}
        <div className="titlebar-no-drag flex items-center justify-center gap-0.5">
          <Button variant="ghost" size="icon" className="h-7 w-7" onClick={() => setSearchOpen(true)} title="快速搜索 (⌘K)">
            <Search className="h-3.5 w-3.5" />
          </Button>
          <Button variant="ghost" size="icon" className="h-7 w-7" onClick={() => setAIPanelOpen(!aiPanelOpen)} title="AI 助手">
            <Sparkles className="h-3.5 w-3.5" />
          </Button>
        </div>

        {/* 中间弹性空间 */}
        <div className="flex-1" />

        {/* 右侧功能区 */}
        <div className="titlebar-no-drag flex items-center gap-0.5">
          <Button variant="ghost" size="icon" className="h-7 w-7" onClick={() => setLogViewerOpen(true)} title="查看日志">
            <ScrollText className="h-3.5 w-3.5" />
          </Button>
          <Button
            variant="ghost" size="icon" className="h-7 w-7"
            onClick={() => setTheme(resolved === "dark" ? "light" : "dark")}
            title={resolved === "dark" ? "切换到浅色主题" : "切换到深色主题"}
          >
            {resolved === "dark" ? <Sun className="h-3.5 w-3.5" /> : <Moon className="h-3.5 w-3.5" />}
          </Button>
          <Button variant="ghost" size="icon" className="h-7 w-7" onClick={() => setSettingsOpen(true)} title="设置 (⌘,)">
            <Settings className="h-3.5 w-3.5" />
          </Button>
        </div>
      </div>

      {/* ====== 主内容区 ====== */}
      <div className="flex-1 flex overflow-hidden">
        <Sidebar
          onNewConnection={handleNewConnection}
          onEditConnection={handleEditConnection}
        />

        <div className="flex-1 flex flex-col min-w-0">
          <TabBar />
          <div className="flex-1 overflow-hidden">
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

      {/* ====== 底部状态栏 ====== */}
      <StatusBar onSwitchDatabase={() => {
        if (activeConnectionId) setDbSwitcherOpen(true);
      }} />

      {/* ====== 浮动弹窗层 ====== */}
      <DatabaseSwitcher
        open={dbSwitcherOpen}
        onClose={() => setDbSwitcherOpen(false)}
        connectionId={activeConnectionId || ""}
        currentDatabase={currentDb}
        onSelect={(dbName) => {
          if (activeConnectionId) {
            const { toggleNode, expandedNodes } = useConnectionStore.getState();
            const connNodeId = `conn:${activeConnectionId}`;
            const dbNodeId = `db:${activeConnectionId}:${dbName}`;
            // 展开连接节点和选中的数据库节点
            if (!expandedNodes.has(connNodeId)) toggleNode(connNodeId);
            if (!expandedNodes.has(dbNodeId)) toggleNode(dbNodeId);
            loadTables(activeConnectionId, dbName);
            if (activeTabId) {
              const { updateTab } = useTabsStore.getState();
              updateTab(activeTabId, { database: dbName });
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

  const loadLog = useCallback(async () => {
    setLoading(true);
    try {
      const [content, path] = await Promise.all([
        import("../../../wailsjs/go/services/SettingsService").then((m) => m.GetLogContent()),
        import("../../../wailsjs/go/services/SettingsService").then((m) => m.GetLogPath()),
      ]);
      setLogContent(content || "暂无日志");
      setLogPath(path || "");
    } catch (e: any) {
      setLogContent("加载日志失败: " + (e?.message || e));
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    loadLog();
  }, [loadLog]);

  useEffect(() => {
    if (!loading && contentRef.current) {
      contentRef.current.scrollTop = contentRef.current.scrollHeight;
    }
  }, [loading, logContent]);

  return (
    <>
      <div className="fixed inset-0 z-50 bg-black/40 backdrop-blur-sm" onClick={onClose} />
      <div
        className={cn(
          "fixed z-50 top-1/2 left-1/2 -translate-x-1/2 -translate-y-1/2",
          "w-[800px] h-[560px] rounded-xl shadow-lg border overflow-hidden flex flex-col",
          "bg-[var(--surface)] border-[var(--border-color)]"
        )}
      >
        <div className="flex items-center justify-between px-4 py-2.5 border-b border-[var(--border-color)] bg-[var(--surface-secondary)]">
          <div className="flex items-center gap-2">
            <ScrollText className="h-4 w-4 text-[var(--fg-secondary)]" />
            <span className="text-sm font-medium">应用日志</span>
            {logPath && (
              <span className="text-2xs text-[var(--fg-muted)] font-mono">{logPath}</span>
            )}
          </div>
          <div className="flex items-center gap-1">
            <Button variant="ghost" size="sm" className="h-7 text-xs" onClick={loadLog}>
              刷新
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
              加载中...
            </div>
          ) : logContent}
        </pre>
      </div>
    </>
  );
}
