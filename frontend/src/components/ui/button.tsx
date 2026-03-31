import React from "react";
import { cn } from "@/lib/utils";

export interface ButtonProps
  extends React.ButtonHTMLAttributes<HTMLButtonElement> {
  variant?: "default" | "ghost" | "outline" | "destructive" | "link";
  size?: "default" | "sm" | "lg" | "icon";
}

const Button = React.forwardRef<HTMLButtonElement, ButtonProps>(
  ({ className, variant = "default", size = "default", ...props }, ref) => {
    return (
      <button
        ref={ref}
        className={cn(
          "inline-flex items-center justify-center rounded-[var(--radius-btn)] font-medium transition-colors",
          "focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-[var(--accent)] focus-visible:ring-offset-1",
          "disabled:pointer-events-none disabled:opacity-50",
          {
            "bg-[var(--accent)] text-[var(--accent-fg)] hover:bg-[var(--accent-hover)]":
              variant === "default",
            "hover:bg-[var(--sidebar-hover)] text-[var(--fg)]":
              variant === "ghost",
            "border border-[var(--border-color)] bg-transparent hover:bg-[var(--surface-secondary)] text-[var(--fg)]":
              variant === "outline",
            "bg-[var(--danger)] text-white hover:bg-red-600":
              variant === "destructive",
            "text-[var(--accent)] underline-offset-4 hover:underline":
              variant === "link",
          },
          {
            "h-[var(--size-btn)] px-3 py-1 text-[length:var(--size-font-sm)]": size === "default",
            "h-[var(--size-btn-sm)] px-2.5 text-[length:var(--size-font-xs)]": size === "sm",
            "h-[calc(var(--size-btn)+6px)] px-6 text-[length:var(--size-font-base)]": size === "lg",
            "h-[var(--size-btn)] w-[var(--size-btn)] p-0": size === "icon",
          },
          className
        )}
        {...props}
      />
    );
  }
);
Button.displayName = "Button";

export { Button };
