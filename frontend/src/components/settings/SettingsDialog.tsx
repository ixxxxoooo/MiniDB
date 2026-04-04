import React, { useState, useEffect } from "react";
import { X, Bot, Settings as SettingsIcon, Keyboard, Info } from "lucide-react";
import { Button } from "@/components/ui/button";
import { cn } from "@/lib/utils";
import { AISettings } from "./AISettings";
import { GeneralSettings } from "./GeneralSettings";
import { ShortcutsSettings } from "./ShortcutsSettings";
import { AboutSettings } from "./AboutSettings";
import { useTranslation } from "@/i18n";
import { useUIStore } from "@/stores/ui";

interface SettingsDialogProps {
  open: boolean;
  onClose: () => void;
}

type SettingsTab = "general" | "shortcuts" | "about" | "ai";

export function SettingsDialog({ open, onClose }: SettingsDialogProps) {
  const [activeTab, setActiveTab] = useState<SettingsTab>("general");
  const { t } = useTranslation();
  const { layoutMode } = useUIStore();
  const isCompact = layoutMode === "compact";

  // ESC 关闭弹窗
  useEffect(() => {
    if (!open) return;
    const handleEsc = (e: KeyboardEvent) => {
      if (e.key === "Escape") { e.preventDefault(); onClose(); }
    };
    window.addEventListener("keydown", handleEsc);
    return () => window.removeEventListener("keydown", handleEsc);
  }, [open, onClose]);

  if (!open) return null;

  const TABS: { id: SettingsTab; label: string; icon: React.ElementType }[] = [
    { id: "general", label: t("settings.general"), icon: SettingsIcon },
    { id: "ai", label: t("settings.aiConfig"), icon: Bot },
    { id: "shortcuts", label: t("generalSettings.shortcutsTitle"), icon: Keyboard },
    { id: "about", label: t("settings.about"), icon: Info },
  ];

  return (
    <>
      <div
        className="fixed inset-0 z-50 bg-black/40 backdrop-blur-sm"
        onClick={onClose}
      />
      <div
        className={cn(
          "fixed z-50 top-1/2 left-1/2 -translate-x-1/2 -translate-y-1/2",
          "rounded-[var(--radius-panel)] shadow-lg border overflow-hidden flex",
          "bg-[var(--surface)] border-[var(--border-color)]",
          "w-[760px] h-[560px]"
        )}
      >
        {/* 左侧导航 */}
        <div className="w-40 border-r border-[var(--border-color)] bg-[var(--surface-secondary)] py-2.5 flex-shrink-0">
          <h2 className="px-3 mb-2 text-[length:var(--size-font-2xs)] font-semibold">
            {t("settings.title")}
          </h2>
          {TABS.map((tab) => {
            const Icon = tab.icon;
            return (
              <button
                key={tab.id}
                className={cn(
                  "w-full flex items-center gap-1.5 px-3 py-1.5 text-[length:var(--size-font-2xs)] transition-colors",
                  activeTab === tab.id
                    ? "bg-[var(--accent)]/10 text-[var(--accent)] font-medium"
                    : "text-[var(--fg-secondary)] hover:text-[var(--fg)] hover:bg-[var(--sidebar-hover)]"
                )}
                onClick={() => setActiveTab(tab.id)}
              >
                <Icon className="h-3.5 w-3.5" />
                {tab.label}
              </button>
            );
          })}
        </div>

        {/* 右侧内容 */}
        <div className="flex-1 flex flex-col min-h-0">
          <div className="flex items-center justify-end px-3 py-1.5 border-b border-[var(--border-color)] flex-shrink-0">
            <Button variant="ghost" size="icon" className="h-6 w-6" onClick={onClose}>
              <X className="h-3.5 w-3.5" />
            </Button>
          </div>
          <div className="flex-1 overflow-y-auto p-3">
            {activeTab === "general" && <GeneralSettings />}
            {activeTab === "shortcuts" && <ShortcutsSettings />}
            {activeTab === "about" && <AboutSettings />}
            {activeTab === "ai" && <AISettings />}
          </div>
        </div>
      </div>
    </>
  );
}
