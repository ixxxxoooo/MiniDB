import React from "react";
import { cn } from "@/lib/utils";
import type { DatabaseDriver } from "@/types/connection";

interface DbIconProps {
  className?: string;
}

/**
 * MySQL 图标 - 官方海豚风格
 */
export function MySQLIcon({ className }: DbIconProps) {
  return (
    <svg viewBox="0 0 128 128" className={cn("shrink-0", className)}>
      <defs>
        <linearGradient id="mysql-grad" x1="0%" y1="0%" x2="100%" y2="100%">
          <stop offset="0%" stopColor="#00758F" />
          <stop offset="100%" stopColor="#005E73" />
        </linearGradient>
      </defs>
      <rect width="128" height="128" rx="28" fill="url(#mysql-grad)" />
      <g transform="translate(18, 24) scale(0.72)">
        <path d="M64 15c-5.3 0-9.8 1-13.2 2.5C47.2 19.2 45 21.5 45 24.5c0 3.5 2.8 6.2 6 8.2 3.5 2.2 8 3.5 12 5.2 5 2.1 9 4.5 9 8.8 0 4.5-3.8 7.5-8.5 9.2-3 1.1-6.5 1.6-10.5 1.6-4 0-7.5-.5-10.5-1.6-4.7-1.7-8.5-4.7-8.5-9.2v-3h-9v3c0 7.2 5.2 12.8 12 15.8 4.2 1.8 9.2 2.8 16 2.8 6.8 0 11.8-1 16-2.8 6.8-3 12-8.6 12-15.8 0-7-5-11.5-12-14.5-4-1.7-8.5-3-12.5-5-3.2-1.6-5.5-3.5-5.5-6 0-2.5 2-4.2 5-5.5 2.5-1 5.5-1.5 9-1.5s6.5.5 9 1.5c3 1.3 5 3 5 5.5h9c0-5-3.2-9.2-8.5-11.8C73.8 16 69.3 15 64 15z" fill="white" opacity="0.95" />
        <path d="M86 72c-1.5 0-3 .3-4.2.8l-2.8 1.2c-.8-2.5-2.5-4.5-5-6l-1-.6c3.5-2 6-5.5 6-10.4v-2h-9v2c0 4-3 7-7 7h-2v9h2c1 0 2 .2 3 .5-2 2-3.2 4.5-3.2 7.5 0 6.5 5.2 11.5 12 11.5 6.7 0 12-5 12-11.5 0-1.5-.3-3-.8-4.2l3.5-1.5c1.5-.6 2.5-2 2.5-3.8v-.2c.3-1.7-2-2.3-6-2.3zm-11 20c-3.3 0-6-2.7-6-6s2.7-6 6-6 6 2.7 6 6-2.7 6-6 6z" fill="white" opacity="0.85" />
      </g>
    </svg>
  );
}

/**
 * PostgreSQL 图标 - 官方大象风格
 */
export function PostgreSQLIcon({ className }: DbIconProps) {
  return (
    <svg viewBox="0 0 128 128" className={cn("shrink-0", className)}>
      <defs>
        <linearGradient id="pg-grad" x1="0%" y1="0%" x2="100%" y2="100%">
          <stop offset="0%" stopColor="#336791" />
          <stop offset="100%" stopColor="#264F70" />
        </linearGradient>
      </defs>
      <rect width="128" height="128" rx="28" fill="url(#pg-grad)" />
      <g transform="translate(24, 18) scale(0.64)">
        <path d="M93.5 53.5c-1.5-4-4.2-7-7.5-9.2 1-3.2 1.5-6.8 1.5-10.8 0-8.5-2.8-15.5-8-20.5C74 8 67 5.5 58.5 5.5c-6 0-11.2 1.5-15.5 4-3.2-1-6.5-1.5-10-1.5-7 0-12.5 2.5-16.5 7-4 4.3-6 10-6 17v3c0 9 3 16.8 8.5 22.5-1 3.2-1.5 6.5-1.5 10 0 8 2.5 14.5 7.5 19 4.8 4.3 11 6.5 18.5 6.5 4 0 7.5-.7 10.5-2 3 2.5 6.5 3.8 10.5 3.8 5 0 9-1.8 12-5 1.8 1 4 1.5 6.5 1.5 5 0 9-2 12-5.5 3-3.5 4.5-8 4.5-13.5v-5c1.8-1 3.2-2.5 4.2-4.2 1.3-2.2 1.8-4.5 1.8-7.2-.2-3.5-1-6-2.5-8.4z" fill="white" opacity="0.15" />
        <path d="M84.5 46c-3-3.5-7.2-5.5-12-5.5h-1c1.5-4 2.2-8.2 2.2-12.5 0-7.5-2.2-13.5-6.5-17.5-4.5-4-10.5-6-17.5-6-5 0-9.5 1.2-13 3.5-3.5 2.2-6 5.5-7.5 9.5-2-1-4.5-1.5-7-1.5-5.5 0-10 2-13 5.5-3.2 3.5-5 8.5-5 14.5v2.5c0 8 2.5 15 7.5 20.5l.5.5c-.5 2.5-.8 5-.8 7.5 0 7 2 12.5 6 16.5 4 4 9.5 6 16 6 3.5 0 6.5-.5 9.5-1.8 2.8 2.5 6 3.8 10 3.8 4.5 0 8-1.5 10.5-4.5 1.5 1 3.5 1.5 5.5 1.5 4.5 0 8-1.8 10.5-5 2.5-3 3.8-7 3.8-12v-4.5c1.5-1 2.8-2.2 3.5-3.8 1-2 1.5-4 1.5-6.2 0-3.5-1-6.5-3-9z" fill="white" opacity="0.25" />
        <path d="M49.5 30c-3.5 0-6.5 1.2-8.5 3.5-2 2.3-3 5.5-3 9.5 0 4 1 7.2 3 9.5 2 2.3 5 3.5 8.5 3.5s6.5-1.2 8.5-3.5c2-2.3 3-5.5 3-9.5 0-4-1-7.2-3-9.5-2-2.3-5-3.5-8.5-3.5zm0 20c-2 0-3.5-.8-4.5-2.2-1-1.5-1.5-3.5-1.5-5.8 0-2.3.5-4.3 1.5-5.8 1-1.4 2.5-2.2 4.5-2.2s3.5.8 4.5 2.2c1 1.5 1.5 3.5 1.5 5.8 0 2.3-.5 4.3-1.5 5.8-1 1.4-2.5 2.2-4.5 2.2z" fill="white" opacity="0.9" />
        <ellipse cx="49.5" cy="42" rx="3" ry="4" fill="white" opacity="0.9" />
        <path d="M35 65c0 3 1 5.5 3 7.2 2 1.8 4.5 2.8 8 2.8 2 0 4-.3 5.5-1v-6c-1.5.5-3 .8-4.5.8-2 0-3.5-.5-4.5-1.5-1-1-1.5-2.5-1.5-4.3h14v-3c0-4-1-7-3-9-2-2-5-3-8-3-3.5 0-6 1.2-8 3.5-2 2.2-3 5.5-3 9.5h2zm7-9c.8-1 2-1.5 3.5-1.5 1.3 0 2.3.5 3 1.5.8 1 1.2 2.5 1.2 4H41c.2-1.7.5-3 1-4z" fill="white" opacity="0.7" />
      </g>
    </svg>
  );
}

/**
 * SQLite 图标 - 官方羽毛风格
 */
export function SQLiteIcon({ className }: DbIconProps) {
  return (
    <svg viewBox="0 0 128 128" className={cn("shrink-0", className)}>
      <defs>
        <linearGradient id="sqlite-grad" x1="0%" y1="0%" x2="100%" y2="100%">
          <stop offset="0%" stopColor="#0F80CC" />
          <stop offset="100%" stopColor="#044A6E" />
        </linearGradient>
      </defs>
      <rect width="128" height="128" rx="28" fill="url(#sqlite-grad)" />
      <g transform="translate(30, 16) scale(0.55)">
        <path d="M62 8L30 65c-3 5.3-4 11-3.5 17 1 12 8 22 18 28.5 6 4 13 6 20 6.2 7.5.2 14.5-1.5 20.5-5 10-6 16-16 17.5-28 .5-5-.2-10-2.2-14.5L62 8z" fill="white" opacity="0.2" />
        <path d="M62 16L34 66c-2.5 4.5-3.5 9.5-3 14.5.8 10 6.5 18.5 15 24 5 3.2 10.5 5 16.5 5.2 6.5.2 12.5-1.2 17.5-4.2 8.5-5 13.5-13.5 14.8-23.5.5-4.5-.2-8.5-2-12.5L62 16z" fill="white" opacity="0.3" />
        <path d="M62 28L40 67c-2 3.5-3 7.5-2.5 11.5.7 8 5.5 15 12.5 19.2 4 2.5 8.5 4 13.5 4.2 5.2.2 10-1 14-3.5 7-4 11-11 12-19 .5-3.5-.2-7-1.5-10L62 28z" fill="white" opacity="0.85" />
        <path d="M55 55v32" stroke="white" strokeWidth="3.5" strokeLinecap="round" opacity="0.4" />
        <path d="M48 60l7-5 7 5" stroke="white" strokeWidth="3" strokeLinecap="round" strokeLinejoin="round" opacity="0.35" />
      </g>
    </svg>
  );
}

/**
 * TiDB 图标 - 官方红色风格
 */
export function TiDBIcon({ className }: DbIconProps) {
  return (
    <svg viewBox="0 0 128 128" className={cn("shrink-0", className)}>
      <defs>
        <linearGradient id="tidb-grad" x1="0%" y1="0%" x2="100%" y2="100%">
          <stop offset="0%" stopColor="#E3262D" />
          <stop offset="100%" stopColor="#B91C22" />
        </linearGradient>
      </defs>
      <rect width="128" height="128" rx="28" fill="url(#tidb-grad)" />
      <g transform="translate(20, 26) scale(0.7)">
        <path d="M63 8L18 34v52l45 26 45-26V34L63 8z" fill="none" stroke="white" strokeWidth="5" opacity="0.9" strokeLinejoin="round" />
        <path d="M63 8v78" stroke="white" strokeWidth="4" opacity="0.7" />
        <path d="M18 34l45 26" stroke="white" strokeWidth="4" opacity="0.5" />
        <path d="M108 34l-45 26" stroke="white" strokeWidth="4" opacity="0.5" />
        <circle cx="63" cy="34" r="8" fill="white" opacity="0.9" />
        <circle cx="38" cy="48" r="6" fill="white" opacity="0.6" />
        <circle cx="88" cy="48" r="6" fill="white" opacity="0.6" />
        <circle cx="63" cy="64" r="7" fill="white" opacity="0.8" />
      </g>
    </svg>
  );
}

/**
 * StarRocks 图标 - 星辰紫色风格
 */
export function StarRocksIcon({ className }: DbIconProps) {
  return (
    <svg viewBox="0 0 128 128" className={cn("shrink-0", className)}>
      <defs>
        <linearGradient id="sr-grad" x1="0%" y1="0%" x2="100%" y2="100%">
          <stop offset="0%" stopColor="#7B3FE4" />
          <stop offset="100%" stopColor="#5A2DB5" />
        </linearGradient>
      </defs>
      <rect width="128" height="128" rx="28" fill="url(#sr-grad)" />
      <g transform="translate(22, 18) scale(0.67)">
        <path d="M63 6l15.5 31.5L112 43l-24.5 24 5.8 33.5L63 84.5l-30.3 16L38.5 67 14 43l33.5-5.5L63 6z" fill="white" opacity="0.9" strokeLinejoin="round" />
        <path d="M63 6l15.5 31.5L112 43l-24.5 24 5.8 33.5L63 84.5l-30.3 16L38.5 67 14 43l33.5-5.5L63 6z" fill="none" stroke="white" strokeWidth="2" opacity="0.3" />
        <circle cx="63" cy="52" r="12" fill="url(#sr-grad)" opacity="0.7" />
        <circle cx="63" cy="52" r="5" fill="white" opacity="0.8" />
        <path d="M25 105c10 8 23.5 13 38 13s28-5 38-13" stroke="white" strokeWidth="4.5" strokeLinecap="round" opacity="0.5" fill="none" />
      </g>
    </svg>
  );
}

// 驱动图标颜色映射
export const DRIVER_COLORS: Record<DatabaseDriver, string> = {
  mysql: "#00758F",
  postgres: "#336791",
  sqlite: "#0F80CC",
  tidb: "#E3262D",
  starrocks: "#7B3FE4",
};

// 驱动图标组件映射
const DRIVER_ICON_MAP: Record<DatabaseDriver, React.ComponentType<DbIconProps>> = {
  mysql: MySQLIcon,
  postgres: PostgreSQLIcon,
  sqlite: SQLiteIcon,
  tidb: TiDBIcon,
  starrocks: StarRocksIcon,
};

/**
 * 通用数据库图标组件 - 根据 driver 类型渲染对应图标
 */
export function DriverIcon({
  driver,
  className,
}: {
  driver: DatabaseDriver;
  className?: string;
}) {
  const Icon = DRIVER_ICON_MAP[driver] || MySQLIcon;
  return <Icon className={className} />;
}
