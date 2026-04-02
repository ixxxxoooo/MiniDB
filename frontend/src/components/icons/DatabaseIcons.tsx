import React from "react";
import { cn } from "@/lib/utils";
import type { DatabaseDriver } from "@/types/connection";

interface DbIconProps {
  className?: string;
}

/** MySQL 图标 — 海豚，品牌色 #00546B */
export function MySQLIcon({ className }: DbIconProps) {
  return (
    <svg viewBox="0 0 128 128" className={cn("shrink-0", className)}>
      <path
        fill="#00546B"
        d="M99 93.7c-3.8-.3-6.7-.5-9.2.3-.7.2-1.8.2-1.9 1.1.4.4.4 1 .7 1.5.5.9 1.5 2.2 2.3 2.8l2.8 2c1.7 1 3.6 1.6 5.2 2.6 1 .6 1.9 1.3 2.8 2 .5.3.8.9 1.4 1.1v-.1c-.3-.4-.4-.9-.7-1.3l-1.3-1.3c-1.3-1.7-2.9-3.2-4.6-4.5-1.4-1-4.4-2.3-5-3.8l-.1-.1c1-.1 2.1-.4 3.1-.7 1.5-.4 2.9-.3 4.5-.7l2.1-.7v-.4c-.8-.8-1.4-1.9-2.3-2.6-2.4-1.9-5-3.8-7.7-5.3-1.5-.8-3.3-1.4-4.9-2.1-.6-.2-1.5-.3-1.9-.8-.8-1-1.3-2.3-1.9-3.5-1.3-2.5-2.6-5.3-3.7-8-.8-1.8-1.3-3.6-2.3-5.3-4.7-7.8-9.8-12.5-17.6-17.1-1.7-.9-3.7-1.4-5.8-1.9l-3.5-.2c-.7-.3-1.5-1.2-2.1-1.6-2.6-1.7-9.4-5.2-11.3-.5-1.2 3 1.8 6 2.8 7.5.8 1.1 1.8 2.3 2.3 3.5.3.8.4 1.7.7 2.6.7 2 1.3 4.2 2.2 6 .5.9 1 1.9 1.6 2.7.4.5 1 .7 1.1 1.5-.6.9-.7 2.2-1 3.3-1.5 5-.9 11.2 1.2 14.9.6 1.1 2.2 3.6 4.3 2.6 1.8-.8 1.4-3.3 2-5.5.1-.5 0-.9.3-1.2v.1l1.8 3.6c1.3 2.2 3.7 4.4 5.7 5.9 1 .8 1.9 2.2 3.2 2.7v-.1h-.1c-.3-.4-.6-.6-1-.9-.8-.8-1.6-1.7-2.2-2.6-1.8-2.5-3.4-5.2-4.8-8-.7-1.3-1.3-2.8-1.8-4.2-.2-.5-.2-1.3-.7-1.5-.6 1-1.5 1.8-1.9 3-.7 1.9-.8 4.2-1 6.5l-.2.1c-1.3-.3-1.7-1.7-2.2-2.8-1.2-2.9-1.4-7.6-.4-11 .3-.9 1.5-3.6 1-4.4-.2-.8-1-1.2-1.4-1.8-.5-.7-1-1.7-1.3-2.6-.9-2.3-1.3-4.9-2.3-7.2-.5-1.1-1.3-2.2-2-3.2-.7-1.1-1.5-1.9-2.1-3.2-.2-.4-.5-1.2-.2-1.7.1-.3.3-.5.5-.5.5-.4 1.7.1 2.2.3 1.4.6 2.6 1.1 3.7 1.9.5.4 1.1 1.1 1.7 1.3h.8c1.2.3 2.5.1 3.6.4 2 .7 3.8 1.7 5.5 2.8 5 3.4 9.2 8.2 12 14 .5 1 .7 1.9 1.1 2.9.9 2.1 2 4.2 2.9 6.2.9 2 1.7 4 3 5.6.6.8 3.1 1.3 4.3 1.7.8.3 2.1.7 2.8 1.1 1.4.8 2.8 1.7 4.1 2.6.6.5 2.6 1.5 2.8 2.2z"
      />
      <path
        fill="#F29111"
        d="M38.7 46.2c-.8 0-1.3.1-1.9.3v.1h.1c.4.7.9 1.3 1.3 2l1 2.1.1-.1c.6-.4.9-1.1.9-2.1-.2-.2-.3-.5-.5-.7-.2-.4-.7-.6-1-1z"
      />
    </svg>
  );
}

/** PostgreSQL 图标 — 大象，品牌色 #336791 */
export function PostgreSQLIcon({ className }: DbIconProps) {
  return (
    <svg viewBox="0 0 128 128" className={cn("shrink-0", className)}>
      <g transform="translate(20, 16) scale(0.68)">
        <path
          fill="#336791"
          d="M90.5 79.5c0 7-5.5 12.7-12.2 12.7-2.1 0-4-.5-5.7-1.5l-1.6 1.8c-3 3.6-7.3 5.7-12.2 5.7-2.5 0-4.8-.6-6.9-1.7l-1.3.8c-2 1.2-4.3 1.8-6.8 1.8-6.4 0-10.6-4.3-10.6-9.8 0-2.6.9-5 2.6-6.9l1-1.1-1.3-.9c-5.1-3.5-8.5-10.3-8.5-18 0-2.7.3-5.3.9-7.7l.4-1.5-1.3-.9c-3.6-2.5-6.4-7.7-6.4-13.3 0-9.2 6-17.2 14.4-19.2l1.3-.3-.3-1.3c-1-3.7-.4-7.5 1.6-10.6 2.3-3.5 6.1-5.5 10.2-5.5 3.3 0 6.4 1.3 8.7 3.6l1 1 1-1c1.4-1.4 3.2-2 5-2 4.2 0 8 2.4 9.9 6.2 1.6 3.2 1.5 6.7-.3 9.5l-.9 1.3 1.5.5c8 2.7 13.6 10.6 13.6 20 0 3.5-.7 6.8-1.9 9.6l-.5 1.1h1.2c4.4 0 7.3 2 8.7 5.1 1 2.2 1.4 4.6 1.1 6.9-.2 2.3-1 4.4-2.2 6.3l-.7 1.1.9.8c.2.2.5.5.7.8.9 2 1.3 4.2 1.3 6.5z"
        />
        <circle cx="52" cy="42" r="5" fill="#6BA5C9" />
        <circle cx="76" cy="42" r="5" fill="#6BA5C9" />
        <path d="M52 60c0 0 4 8 12 8s12-8 12-8" stroke="#6BA5C9" strokeWidth="3.5" strokeLinecap="round" fill="none" />
        <path d="M80 72c4 6 4 16 0 22" stroke="#336791" strokeWidth="3" strokeLinecap="round" fill="none" opacity="0.4" />
      </g>
    </svg>
  );
}

/** SQLite 图标 — 菱形 + S，品牌色 */
export function SQLiteIcon({ className }: DbIconProps) {
  return (
    <svg viewBox="0 0 128 128" className={cn("shrink-0", className)}>
      <g transform="translate(28, 14)">
        <path d="M36 10 L60 50 L50 54 L36 90 L22 54 L12 50 Z" fill="#0EA5E9" opacity="0.9" />
        <path d="M36 10 L60 50 L50 54 L36 90 Z" fill="#0284C7" opacity="0.35" />
        <path d="M36 10 L36 90" stroke="#075985" strokeWidth="2" opacity="0.3" />
        <text x="36" y="80" textAnchor="middle" fill="white" fontSize="22" fontWeight="bold" fontFamily="system-ui">S</text>
      </g>
    </svg>
  );
}

/** TiDB 图标 — 六边形网格，品牌色 #CC2C36 */
export function TiDBIcon({ className }: DbIconProps) {
  return (
    <svg viewBox="0 0 128 128" className={cn("shrink-0", className)}>
      <g transform="translate(24, 28)">
        <path d="M40 0 L0 23 L0 69 L40 92 L80 69 L80 23 Z" fill="none" stroke="#CC2C36" strokeWidth="4" strokeLinejoin="round" />
        <path d="M40 0 L40 92" stroke="#CC2C36" strokeWidth="2" opacity="0.3" />
        <path d="M0 23 L40 46 L80 23" stroke="#CC2C36" strokeWidth="2" opacity="0.3" />
        <circle cx="40" cy="30" r="7" fill="#CC2C36" />
        <circle cx="22" cy="42" r="5" fill="#CC2C36" opacity="0.6" />
        <circle cx="58" cy="42" r="5" fill="#CC2C36" opacity="0.6" />
        <circle cx="40" cy="58" r="6" fill="#CC2C36" opacity="0.8" />
      </g>
    </svg>
  );
}

/** StarRocks 图标 — 星形，品牌色 #5B2D8E */
export function StarRocksIcon({ className }: DbIconProps) {
  return (
    <svg viewBox="0 0 128 128" className={cn("shrink-0", className)}>
      <g transform="translate(22, 18)">
        <path d="M42 6 L50 30 L76 32 L56 48 L62 74 L42 58 L22 74 L28 48 L8 32 L34 30 Z" fill="#5B2D8E" />
        <circle cx="42" cy="40" r="8" fill="white" opacity="0.25" />
        <circle cx="42" cy="40" r="3.5" fill="white" opacity="0.8" />
        <path d="M14 82 c8 6 18 10 28 10 s20-4 28-10" stroke="#5B2D8E" strokeWidth="3.5" strokeLinecap="round" fill="none" opacity="0.4" />
      </g>
    </svg>
  );
}

// 驱动图标颜色映射
export const DRIVER_COLORS: Record<DatabaseDriver, string> = {
  mysql: "#00546B",
  postgres: "#336791",
  sqlite: "#003B57",
  tidb: "#CC2C36",
  starrocks: "#5B2D8E",
};

// 驱动图标组件映射
const DRIVER_ICON_MAP: Record<DatabaseDriver, React.ComponentType<DbIconProps>> = {
  mysql: MySQLIcon,
  postgres: PostgreSQLIcon,
  sqlite: SQLiteIcon,
  tidb: TiDBIcon,
  starrocks: StarRocksIcon,
};

/** 通用数据库图标组件 — 根据 driver 类型渲染对应图标 */
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
