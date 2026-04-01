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
            {sidebarCollapsed ? "展开侧边栏" : "收起侧边栏"}
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
          <TooltipContent>快速搜索 (⌘P)</TooltipContent>
        </Tooltip>

        <Tooltip>
          <TooltipTrigger asChild>
            <Button variant="ghost" size="icon" onClick={onNewConnection}>
              <Plus className="h-4 w-4" />
            </Button>
          </TooltipTrigger>
          <TooltipContent>新建连接</TooltipContent>
        </Tooltip>

        <Tooltip>
          <TooltipTrigger asChild>
            <Button variant="ghost" size="icon" onClick={onToggleAI}>
              <Sparkles className="h-4 w-4" />
            </Button>
          </TooltipTrigger>
          <TooltipContent>AI 助手</TooltipContent>
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
            {resolved === "dark" ? "切换到浅色主题" : "切换到深色主题"}
          </TooltipContent>
        </Tooltip>

        <Tooltip>
          <TooltipTrigger asChild>
            <Button variant="ghost" size="icon" onClick={onOpenSettings}>
              <Settings className="h-4 w-4" />
            </Button>
          </TooltipTrigger>
          <TooltipContent>设置</TooltipContent>
        </Tooltip>
      </div>
    </div>
  );
}
