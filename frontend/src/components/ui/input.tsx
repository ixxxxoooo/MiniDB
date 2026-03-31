import React from "react";
import { cn } from "@/lib/utils";

export interface InputProps
  extends React.InputHTMLAttributes<HTMLInputElement> {}

const Input = React.forwardRef<HTMLInputElement, InputProps>(
  ({ className, type, ...props }, ref) => {
    return (
      <input
        type={type}
        ref={ref}
        className={cn(
          "flex h-[var(--size-input)] w-full rounded-[var(--radius-input)] border border-[var(--border-color)] bg-[var(--surface)] px-3 py-1 text-[length:var(--size-font-sm)]",
          "text-[var(--fg)] placeholder:text-[var(--fg-muted)]",
          "transition-colors focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-[var(--accent)]",
          "disabled:cursor-not-allowed disabled:opacity-50",
          className
        )}
        {...props}
      />
    );
  }
);
Input.displayName = "Input";

export { Input };
