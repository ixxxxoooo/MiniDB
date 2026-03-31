import React, { useState, useCallback, useEffect } from "react";
import { Toolbar } from "./Toolbar";
import { Sidebar } from "./Sidebar";
import { StatusBar } from "./StatusBar";
import { TabBar } from "@/components/tabs/TabBar";
import { TabContent } from "@/components/tabs/TabContent";
import { ConnectionDialog } from "@/components/connection/ConnectionDialog";
import { SettingsDialog } from "@/components/settings/SettingsDialog";
import { AIPanel } from "@/components/ai/AIPanel";
import { useUIStore } from "@/stores/ui";
import { useConnectionStore } from "@/stores/connection";
import { useTabsStore } from "@/stores/tabs";
import { useDatabase } from "@/hooks/useDatabase";
import { CommandPalette } from "./CommandPalette";
import { useKeyboard } from "@/hooks/useKeyboard";
import type { ConnectionConfig } from "@/types/connection";

export function AppLayout() {
  const [connDialogOpen, setConnDialogOpen] = useState(false);
  const [editingConnection, setEditingConnection] =
    useState<ConnectionConfig | null>(null);
  const [settingsOpen, setSettingsOpen] = useState(false);
  const [aiPanelOpen, setAIPanelOpen] = useState(false);
  const [searchOpen, setSearchOpen] = useState(false);

  const { activeTabId, tabs, addTab, removeTab } = useTabsStore();
  const activeTab = tabs.find((t) => t.id === activeTabId);
  const { activeConnectionId } = useConnectionStore();

  const {
    loadConnections,
    saveConnection,
    testConnection,
    connect,
  } = useDatabase();

  // 启动时加载连接列表
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
      // 保存后自动连接
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

  // 全局快捷键
  useKeyboard({
    "mod+k": () => {
      setSearchOpen(true);
    },
    "mod+t": () => {
      if (activeConnectionId) {
        addTab({
          type: "query",
          title: "新查询",
          connectionId: activeConnectionId,
          database: activeTab?.database || "",
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
  });

  return (
    <div className="h-full flex flex-col bg-[var(--surface)]">
      <Toolbar
        onNewConnection={handleNewConnection}
        onOpenSettings={() => setSettingsOpen(true)}
        onOpenSearch={() => setSearchOpen(true)}
        onToggleAI={() => setAIPanelOpen(!aiPanelOpen)}
      />

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
            currentDatabase={activeTab?.database}
            currentTable={activeTab?.table}
          />
        )}
      </div>

      <StatusBar />

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
    </div>
  );
}
