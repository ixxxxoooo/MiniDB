import React, { useState } from "react";
import { X, Bot, Settings as SettingsIcon, Palette } from "lucide-react";
import { Button } from "@/components/ui/button";
import { cn } from "@/lib/utils";
import { AISettings } from "./AISettings";
import { GeneralSettings } from "./GeneralSettings";

interface SettingsDialogProps {
  open: boolean;
  onClose: () => void;
}

type SettingsTab = "general" | "ai" | "appearance";

const TABS: { id: SettingsTab; label: string; icon: React.ElementType }[] = [
  { id: "general", label: "通用", icon: SettingsIcon },
  { id: "ai", label: "AI 配置", icon: Bot },
  { id: "appearance", label: "外观", icon: Palette },
];

export function SettingsDialog({ open, onClose }: SettingsDialogProps) {
  const [activeTab, setActiveTab] = useState<SettingsTab>("general");

  if (!open) return null;

  return (
    <>
      <div
        className="fixed inset-0 z-50 bg-black/40 backdrop-blur-sm"
        onClick={onClose}
      />
      <div
        className={cn(
          "fixed z-50 top-1/2 left-1/2 -translate-x-1/2 -translate-y-1/2",
          "w-[640px] h-[480px] rounded-xl shadow-lg border overflow-hidden flex",
          "bg-[var(--surface)] border-[var(--border-color)]"
        )}
      >
        {/* 左侧导航 */}
        <div className="w-44 border-r border-[var(--border-color)] bg-[var(--surface-secondary)] py-4">
          <h2 className="px-4 mb-3 text-base font-semibold">设置</h2>
          {TABS.map((tab) => {
            const Icon = tab.icon;
            return (
              <button
                key={tab.id}
                className={cn(
                  "w-full flex items-center gap-2 px-4 py-2 text-sm transition-colors",
                  activeTab === tab.id
                    ? "bg-[var(--accent)]/10 text-[var(--accent)] font-medium"
                    : "text-[var(--fg-secondary)] hover:text-[var(--fg)] hover:bg-[var(--sidebar-hover)]"
                )}
                onClick={() => setActiveTab(tab.id)}
              >
                <Icon className="h-4 w-4" />
                {tab.label}
              </button>
            );
          })}
        </div>

        {/* 右侧内容 */}
        <div className="flex-1 flex flex-col">
          <div className="flex items-center justify-end px-4 py-3 border-b border-[var(--border-color)]">
            <Button variant="ghost" size="icon" onClick={onClose}>
              <X className="h-4 w-4" />
            </Button>
          </div>
          <div className="flex-1 overflow-y-auto p-5">
            {activeTab === "general" && <GeneralSettings />}
            {activeTab === "ai" && <AISettings />}
            {activeTab === "appearance" && <GeneralSettings />}
          </div>
        </div>
      </div>
    </>
  );
}
