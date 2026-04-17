import * as React from "react";
import * as TooltipPrimitive from "@radix-ui/react-tooltip";
import { cn } from "@/lib/utils";

const TOOLTIP_DELAY_MS = 700;

const TooltipProvider = ({
  delayDuration = TOOLTIP_DELAY_MS,
  ...props
}: React.ComponentPropsWithoutRef<typeof TooltipPrimitive.Provider>) => (
  <TooltipPrimitive.Provider
    delayDuration={Math.max(delayDuration, TOOLTIP_DELAY_MS)}
    {...props}
  />
);

const Tooltip = ({
  delayDuration,
  ...props
}: React.ComponentPropsWithoutRef<typeof TooltipPrimitive.Root>) => (
  <TooltipPrimitive.Root
    delayDuration={
      typeof delayDuration === "number"
        ? Math.max(delayDuration, TOOLTIP_DELAY_MS)
        : TOOLTIP_DELAY_MS
    }
    {...props}
  />
);
const TooltipTrigger = TooltipPrimitive.Trigger;

const TooltipContent = React.forwardRef<
  React.ElementRef<typeof TooltipPrimitive.Content>,
  React.ComponentPropsWithoutRef<typeof TooltipPrimitive.Content>
>(({ className, sideOffset = 4, ...props }, ref) => (
  <TooltipPrimitive.Portal>
    <TooltipPrimitive.Content
      ref={ref}
      sideOffset={sideOffset}
      collisionPadding={8}
      className={cn(
        "z-[10000] max-w-[360px] overflow-hidden px-2.5 py-1.5",
        "rounded-[var(--radius-btn)] border text-[11px] leading-[1.35] select-none",
        "bg-[var(--surface-elevated)] text-[var(--fg)] border-[var(--border-color)]",
        "shadow-[var(--shadow-lg)] animate-fade-in",
        className
      )}
      {...props}
    />
  </TooltipPrimitive.Portal>
));
TooltipContent.displayName = TooltipPrimitive.Content.displayName;

export { Tooltip, TooltipTrigger, TooltipContent, TooltipProvider, TOOLTIP_DELAY_MS };
