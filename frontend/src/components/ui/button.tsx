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
          "inline-flex items-center justify-center rounded-md font-medium transition-colors",
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
            "h-9 px-4 py-2 text-sm": size === "default",
            "h-7 px-3 text-xs": size === "sm",
            "h-11 px-8 text-base": size === "lg",
            "h-8 w-8 p-0": size === "icon",
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
