import React, { useState } from "react";
import { hasDirtyTabs } from "@/lib/tabGuards";
import { requestConfirm } from "@/stores/confirm";
import { useTabsStore } from "@/stores/tabs";
import { Tooltip, TooltipContent, TooltipTrigger } from "@/components/ui/tooltip";
import { useTranslation } from "@/i18n";

// 窗口控制按钮组件（自绘 macOS 红绿灯）
export function WindowControls() {
  const [hovered, setHovered] = useState(false);
  const { t } = useTranslation();

  const handleClose = async () => {
    // 有未保存修改时，退出前确认，避免关窗直接丢失编辑
    if (hasDirtyTabs()) {
      const ok = await requestConfirm({
        title: t("dialog.quitUnsavedTitle"),
        message: t("dialog.unsavedMessage", {
          names: useTabsStore.getState().tabs.filter((tab) => tab.dirty).map((tab) => tab.title).join("、"),
        }),
        confirmText: t("dialog.discard"),
        danger: true,
      });
      if (!ok) return;
    }
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
