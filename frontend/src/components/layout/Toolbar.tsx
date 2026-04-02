import React from "react";
import {
  PanelLeftClose,
  PanelLeftOpen,
  Plus,
  RefreshCw,
  Settings,
  Moon,
  Sun,
  Sparkles,
  Search,
} from "lucide-react";
import { Button } from "@/components/ui/button";
import {
  Tooltip,
  TooltipContent,
  TooltipTrigger,
} from "@/components/ui/tooltip";
import { useUIStore } from "@/stores/ui";
import { useThemeStore } from "@/stores/theme";
import { cn } from "@/lib/utils";
import { useTranslation } from "@/i18n";

interface ToolbarProps {
  onNewConnection: () => void;
  onOpenSettings: () => void;
  onOpenSearch: () => void;
  onToggleAI: () => void;
}

export function Toolbar({
  onNewConnection,
  onOpenSettings,
  onOpenSearch,
  onToggleAI,
}: ToolbarProps) {
  const { sidebarCollapsed, toggleSidebar } = useUIStore();
  const { resolved, setTheme } = useThemeStore();
  const { t } = useTranslation();

  return (
    <div
      className={cn(
        "h-12 flex items-center px-3 gap-1 border-b vibrancy titlebar-drag",
        "bg-[var(--toolbar-bg)] border-[var(--toolbar-border)]"
      )}
    >
      {/* macOS 红绿灯按钮占位 */}
      <div className="w-16 flex-shrink-0" />

      {/* 侧边栏切换 */}
      <div className="titlebar-no-drag">
        <Tooltip>
          <TooltipTrigger asChild>
            <Button variant="ghost" size="icon" onClick={toggleSidebar}>
              {sidebarCollapsed ? (
                <PanelLeftOpen className="h-4 w-4" />
              ) : (
                <PanelLeftClose className="h-4 w-4" />
              )}
            </Button>
          </TooltipTrigger>
          <TooltipContent>
            {sidebarCollapsed ? t("toolbar.expandSidebar") : t("toolbar.collapseSidebar")}
          </TooltipContent>
        </Tooltip>
      </div>

      <div className="flex-1" />

      {/* 中央操作区 */}
      <div className="titlebar-no-drag flex items-center gap-1">
        <Tooltip>
          <TooltipTrigger asChild>
            <Button variant="ghost" size="icon" onClick={onOpenSearch}>
              <Search className="h-4 w-4" />
            </Button>
          </TooltipTrigger>
          <TooltipContent>{t("toolbar.quickSearch")} (⌘P)</TooltipContent>
        </Tooltip>

        <Tooltip>
          <TooltipTrigger asChild>
            <Button variant="ghost" size="icon" onClick={onNewConnection}>
              <Plus className="h-4 w-4" />
            </Button>
          </TooltipTrigger>
          <TooltipContent>{t("toolbar.newConnection")}</TooltipContent>
        </Tooltip>

        <Tooltip>
          <TooltipTrigger asChild>
            <Button variant="ghost" size="icon" onClick={onToggleAI}>
              <Sparkles className="h-4 w-4" />
            </Button>
          </TooltipTrigger>
          <TooltipContent>{t("toolbar.aiAssistant")}</TooltipContent>
        </Tooltip>
      </div>

      <div className="flex-1" />

      {/* 右侧操作区 */}
      <div className="titlebar-no-drag flex items-center gap-1">
        <Tooltip>
          <TooltipTrigger asChild>
            <Button
              variant="ghost"
              size="icon"
              onClick={() => setTheme(resolved === "dark" ? "light" : "dark")}
            >
              {resolved === "dark" ? (
                <Sun className="h-4 w-4" />
              ) : (
                <Moon className="h-4 w-4" />
              )}
            </Button>
          </TooltipTrigger>
          <TooltipContent>
            {resolved === "dark" ? t("toolbar.switchToLight") : t("toolbar.switchToDark")}
          </TooltipContent>
        </Tooltip>

        <Tooltip>
          <TooltipTrigger asChild>
            <Button variant="ghost" size="icon" onClick={onOpenSettings}>
              <Settings className="h-4 w-4" />
            </Button>
          </TooltipTrigger>
          <TooltipContent>{t("toolbar.settings")}</TooltipContent>
        </Tooltip>
      </div>
    </div>
  );
}
