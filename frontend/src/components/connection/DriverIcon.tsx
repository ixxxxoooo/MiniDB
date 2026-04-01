import React from "react";
import { cn } from "@/lib/utils";
import type { DatabaseDriver } from "@/types/connection";

type DriverVisual = {
  accent: string;
  surface: string;
  border: string;
};

const DRIVER_VISUALS: Record<DatabaseDriver, DriverVisual> = {
  mysql: {
    accent: "#1f7aff",
    surface: "rgba(31, 122, 255, 0.12)",
    border: "rgba(31, 122, 255, 0.22)",
  },
  postgres: {
    accent: "#3e63dd",
    surface: "rgba(62, 99, 221, 0.12)",
    border: "rgba(62, 99, 221, 0.22)",
  },
  sqlite: {
    accent: "#0f9d8a",
    surface: "rgba(15, 157, 138, 0.12)",
    border: "rgba(15, 157, 138, 0.22)",
  },
  tidb: {
    accent: "#eb6f12",
    surface: "rgba(235, 111, 18, 0.12)",
    border: "rgba(235, 111, 18, 0.22)",
  },
  starrocks: {
    accent: "#f43f5e",
    surface: "rgba(244, 63, 94, 0.12)",
    border: "rgba(244, 63, 94, 0.22)",
  },
};

export function getDriverVisual(driver?: string): DriverVisual {
  if (!driver) return DRIVER_VISUALS.mysql;
  return DRIVER_VISUALS[(driver as DatabaseDriver) || "mysql"] || DRIVER_VISUALS.mysql;
}

function DriverGlyph({ driver }: { driver: DatabaseDriver }) {
  switch (driver) {
    case "postgres":
      return (
        <svg viewBox="0 0 24 24" fill="none" className="h-full w-full">
          <path
            d="M8 10.2C8 6.6 10 4 13 4c2.9 0 4.7 2.1 4.7 5.2v3.3c0 2.3-1.4 3.9-3.6 3.9-1.8 0-3-1-3.6-2.6v2.8c0 1.4-.9 2.3-2.1 2.3-1 0-1.8-.7-1.8-1.7 0-.8.4-1.4 1-1.8l.8-.5V10.2Z"
            stroke="currentColor"
            strokeWidth="1.8"
            strokeLinecap="round"
            strokeLinejoin="round"
          />
          <path d="M10.9 10h3.4" stroke="currentColor" strokeWidth="1.8" strokeLinecap="round" />
          <path d="M14.3 8.1v5" stroke="currentColor" strokeWidth="1.8" strokeLinecap="round" />
        </svg>
      );
    case "sqlite":
      return (
        <svg viewBox="0 0 24 24" fill="none" className="h-full w-full">
          <path
            d="M7 3.8h7.6l3.4 3.4V19a1.2 1.2 0 0 1-1.2 1.2H7A1.2 1.2 0 0 1 5.8 19V5A1.2 1.2 0 0 1 7 3.8Z"
            stroke="currentColor"
            strokeWidth="1.8"
            strokeLinejoin="round"
          />
          <path d="M14.6 3.8v4h4" stroke="currentColor" strokeWidth="1.8" strokeLinejoin="round" />
          <path d="M8.5 11.2h6.7" stroke="currentColor" strokeWidth="1.8" strokeLinecap="round" />
          <path d="M8.5 15.3h6.7" stroke="currentColor" strokeWidth="1.8" strokeLinecap="round" />
        </svg>
      );
    case "tidb":
      return (
        <svg viewBox="0 0 24 24" fill="none" className="h-full w-full">
          <rect x="4.5" y="4.5" width="5.5" height="5.5" rx="1.2" stroke="currentColor" strokeWidth="1.8" />
          <rect x="14" y="4.5" width="5.5" height="5.5" rx="1.2" stroke="currentColor" strokeWidth="1.8" />
          <rect x="4.5" y="14" width="5.5" height="5.5" rx="1.2" stroke="currentColor" strokeWidth="1.8" />
          <rect x="14" y="14" width="5.5" height="5.5" rx="1.2" stroke="currentColor" strokeWidth="1.8" />
          <path d="M10 7.3h4" stroke="currentColor" strokeWidth="1.8" strokeLinecap="round" />
          <path d="M7.3 10v4" stroke="currentColor" strokeWidth="1.8" strokeLinecap="round" />
          <path d="M16.7 10v4" stroke="currentColor" strokeWidth="1.8" strokeLinecap="round" />
          <path d="M10 16.7h4" stroke="currentColor" strokeWidth="1.8" strokeLinecap="round" />
        </svg>
      );
    case "starrocks":
      return (
        <svg viewBox="0 0 24 24" fill="none" className="h-full w-full">
          <path
            d="m12 4.2 2.1 4.3 4.8.7-3.4 3.3.8 4.9-4.3-2.3-4.3 2.3.8-4.9-3.4-3.3 4.8-.7L12 4.2Z"
            stroke="currentColor"
            strokeWidth="1.8"
            strokeLinejoin="round"
          />
          <path d="M5 19c1.7 1.2 4.1 1.9 7 1.9s5.3-.7 7-1.9" stroke="currentColor" strokeWidth="1.8" strokeLinecap="round" />
        </svg>
      );
    case "mysql":
    default:
      return (
        <svg viewBox="0 0 24 24" fill="none" className="h-full w-full">
          <ellipse cx="12" cy="6.1" rx="6.2" ry="2.6" stroke="currentColor" strokeWidth="1.8" />
          <path
            d="M5.8 6.1v7.8c0 1.4 2.8 2.6 6.2 2.6s6.2-1.2 6.2-2.6V6.1"
            stroke="currentColor"
            strokeWidth="1.8"
            strokeLinecap="round"
            strokeLinejoin="round"
          />
          <path d="M5.8 10c0 1.4 2.8 2.6 6.2 2.6s6.2-1.2 6.2-2.6" stroke="currentColor" strokeWidth="1.8" />
        </svg>
      );
  }
}

interface DriverIconProps {
  driver?: string;
  className?: string;
  glyphClassName?: string;
  tile?: boolean;
}

export function DriverIcon({
  driver = "mysql",
  className,
  glyphClassName,
  tile = false,
}: DriverIconProps) {
  const normalized = (driver as DatabaseDriver) || "mysql";
  const visual = getDriverVisual(normalized);

  if (tile) {
    return (
      <div
        className={cn(
          "flex items-center justify-center rounded-[14px] border shadow-[0_8px_20px_-14px_rgba(15,23,42,0.45)]",
          className
        )}
        style={{ backgroundColor: visual.surface, borderColor: visual.border, color: visual.accent }}
      >
        <div className={cn("h-[58%] w-[58%]", glyphClassName)}>
          <DriverGlyph driver={normalized} />
        </div>
      </div>
    );
  }

  return (
    <div className={cn("inline-flex items-center justify-center", className)} style={{ color: visual.accent }}>
      <div className={cn("h-full w-full", glyphClassName)}>
        <DriverGlyph driver={normalized} />
      </div>
    </div>
  );
}
