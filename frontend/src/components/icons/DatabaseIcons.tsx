import React from "react";
import { cn } from "@/lib/utils";
import type { DatabaseDriver } from "@/types/connection";

interface DbIconProps {
  className?: string;
}

/**
 * MySQL 图标 - 海豚标志简化版
 */
export function MySQLIcon({ className }: DbIconProps) {
  return (
    <svg viewBox="0 0 24 24" fill="none" className={cn("shrink-0", className)}>
      <rect width="24" height="24" rx="5" fill="#00758F" />
      <text
        x="12"
        y="16.5"
        textAnchor="middle"
        fill="white"
        fontSize="9"
        fontWeight="700"
        fontFamily="-apple-system, BlinkMacSystemFont, 'SF Pro Text', sans-serif"
      >
        My
      </text>
    </svg>
  );
}

/**
 * PostgreSQL 图标 - 大象标志简化版
 */
export function PostgreSQLIcon({ className }: DbIconProps) {
  return (
    <svg viewBox="0 0 24 24" fill="none" className={cn("shrink-0", className)}>
      <rect width="24" height="24" rx="5" fill="#336791" />
      <text
        x="12"
        y="16.5"
        textAnchor="middle"
        fill="white"
        fontSize="9"
        fontWeight="700"
        fontFamily="-apple-system, BlinkMacSystemFont, 'SF Pro Text', sans-serif"
      >
        Pg
      </text>
    </svg>
  );
}

/**
 * SQLite 图标
 */
export function SQLiteIcon({ className }: DbIconProps) {
  return (
    <svg viewBox="0 0 24 24" fill="none" className={cn("shrink-0", className)}>
      <rect width="24" height="24" rx="5" fill="#44A8D6" />
      <text
        x="12"
        y="16.5"
        textAnchor="middle"
        fill="white"
        fontSize="8"
        fontWeight="700"
        fontFamily="-apple-system, BlinkMacSystemFont, 'SF Pro Text', sans-serif"
      >
        SL
      </text>
    </svg>
  );
}

/**
 * TiDB 图标
 */
export function TiDBIcon({ className }: DbIconProps) {
  return (
    <svg viewBox="0 0 24 24" fill="none" className={cn("shrink-0", className)}>
      <rect width="24" height="24" rx="5" fill="#E3262D" />
      <text
        x="12"
        y="16.5"
        textAnchor="middle"
        fill="white"
        fontSize="8"
        fontWeight="700"
        fontFamily="-apple-system, BlinkMacSystemFont, 'SF Pro Text', sans-serif"
      >
        Ti
      </text>
    </svg>
  );
}

/**
 * StarRocks 图标
 */
export function StarRocksIcon({ className }: DbIconProps) {
  return (
    <svg viewBox="0 0 24 24" fill="none" className={cn("shrink-0", className)}>
      <rect width="24" height="24" rx="5" fill="#7B3FE4" />
      <text
        x="12"
        y="16.5"
        textAnchor="middle"
        fill="white"
        fontSize="7.5"
        fontWeight="700"
        fontFamily="-apple-system, BlinkMacSystemFont, 'SF Pro Text', sans-serif"
      >
        SR
      </text>
    </svg>
  );
}

/**
 * 根据数据库驱动类型获取对应图标的映射表
 */
const DRIVER_ICON_MAP: Record<DatabaseDriver, React.ComponentType<DbIconProps>> = {
  mysql: MySQLIcon,
  postgres: PostgreSQLIcon,
  sqlite: SQLiteIcon,
  tidb: TiDBIcon,
  starrocks: StarRocksIcon,
};

/**
 * 根据数据库驱动类型获取对应图标颜色
 */
export const DRIVER_COLORS: Record<DatabaseDriver, string> = {
  mysql: "#00758F",
  postgres: "#336791",
  sqlite: "#44A8D6",
  tidb: "#E3262D",
  starrocks: "#7B3FE4",
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
