import React from "react";
import { Tooltip, TooltipContent, TooltipTrigger } from "@/components/ui/tooltip";

// 带快捷键提示的按钮包装组件
export function TipBtn({ tip, shortcut, children, ...rest }: {
  tip: string;
  shortcut?: string;
  children: React.ReactNode;
} & React.ButtonHTMLAttributes<HTMLButtonElement>) {
  return (
    <Tooltip delayDuration={400}>
      <TooltipTrigger asChild>
        <button {...rest}>{children}</button>
      </TooltipTrigger>
      <TooltipContent side="top" className="flex items-center gap-1.5">
        <span>{tip}</span>
        {shortcut && (
          <kbd className="ml-1 px-1 py-0.5 rounded border border-[var(--border-color)] bg-[var(--surface-secondary)] text-2xs font-mono text-[var(--fg-secondary)]">
            {shortcut}
          </kbd>
        )}
      </TooltipContent>
    </Tooltip>
  );
}
