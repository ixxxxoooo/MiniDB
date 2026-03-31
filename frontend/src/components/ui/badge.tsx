import React from "react";
import { cn } from "@/lib/utils";

interface BadgeProps extends React.HTMLAttributes<HTMLDivElement> {
  variant?: "default" | "secondary" | "success" | "warning" | "danger";
}

export function Badge({
  className,
  variant = "default",
  ...props
}: BadgeProps) {
  return (
    <div
      className={cn(
        "inline-flex items-center rounded-full px-2 py-0.5 text-2xs font-medium",
        {
          "bg-[var(--accent)] text-[var(--accent-fg)]": variant === "default",
          "bg-[var(--surface-secondary)] text-[var(--fg-secondary)]":
            variant === "secondary",
          "bg-green-100 text-green-700 dark:bg-green-900/30 dark:text-green-400":
            variant === "success",
          "bg-yellow-100 text-yellow-700 dark:bg-yellow-900/30 dark:text-yellow-400":
            variant === "warning",
          "bg-red-100 text-red-700 dark:bg-red-900/30 dark:text-red-400":
            variant === "danger",
        },
        className
      )}
      {...props}
    />
  );
}
