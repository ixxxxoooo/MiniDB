import React from "react";
import { useThemeStore } from "@/stores/theme";
import { Monitor, Sun, Moon } from "lucide-react";
import { cn } from "@/lib/utils";

export function GeneralSettings() {
  const { theme, setTheme } = useThemeStore();

  const themes = [
    { id: "light" as const, label: "浅色", icon: Sun },
    { id: "dark" as const, label: "深色", icon: Moon },
    { id: "system" as const, label: "跟随系统", icon: Monitor },
  ];

  return (
    <div className="space-y-5">
      <div>
        <h3 className="text-sm font-semibold mb-1">通用设置</h3>
        <p className="text-xs text-[var(--fg-secondary)]">自定义应用外观与行为</p>
      </div>

      {/* 主题选择 */}
      <div>
        <label className="text-xs font-medium text-[var(--fg-secondary)] mb-2 block">
          主题
        </label>
        <div className="flex gap-3">
          {themes.map(({ id, label, icon: Icon }) => (
            <button
              key={id}
              className={cn(
                "flex-1 flex flex-col items-center gap-2 py-4 rounded-lg border transition-colors",
                theme === id
                  ? "border-[var(--accent)] bg-[var(--accent)]/5"
                  : "border-[var(--border-color)] hover:border-[var(--fg-muted)]"
              )}
              onClick={() => setTheme(id)}
            >
              <Icon
                className={cn(
                  "h-5 w-5",
                  theme === id
                    ? "text-[var(--accent)]"
                    : "text-[var(--fg-secondary)]"
                )}
              />
              <span
                className={cn(
                  "text-xs",
                  theme === id
                    ? "text-[var(--accent)] font-medium"
                    : "text-[var(--fg-secondary)]"
                )}
              >
                {label}
              </span>
            </button>
          ))}
        </div>
      </div>

      {/* 默认每页行数 */}
      <div>
        <label className="text-xs font-medium text-[var(--fg-secondary)] mb-1.5 block">
          默认每页行数
        </label>
        <select
          className={cn(
            "w-full h-9 rounded-md border px-3 text-sm",
            "bg-[var(--surface)] border-[var(--border-color)] text-[var(--fg)]"
          )}
          defaultValue="100"
        >
          <option value="50">50</option>
          <option value="100">100</option>
          <option value="200">200</option>
          <option value="500">500</option>
          <option value="1000">1000</option>
        </select>
      </div>
    </div>
  );
}
