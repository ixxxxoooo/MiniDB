export type DatabaseDriver = "mysql" | "postgres" | "sqlite" | "tidb" | "starrocks";

// 连接环境标签类型：本地、测试、生产
export type ConnectionTag = "local" | "test" | "production";

export interface ConnectionConfig {
  id: string;
  name: string;
  type: DatabaseDriver;
  host: string;
  port: number;
  user: string;
  password: string;
  database: string;
  sslMode: string;
  color: string;
  group: string;
  tag: ConnectionTag;
  createdAt?: string;
  updatedAt?: string;
}

// Tag 显示标签
export const TAG_LABELS: Record<ConnectionTag, string> = {
  local: "Local",
  test: "Test",
  production: "Prod",
};

// Tag 主题色配置
export const TAG_COLORS: Record<ConnectionTag, { bg: string; text: string; border: string }> = {
  local: { bg: "rgba(52, 199, 89, 0.12)", text: "#34c759", border: "rgba(52, 199, 89, 0.3)" },
  test: { bg: "rgba(255, 149, 0, 0.12)", text: "#ff9500", border: "rgba(255, 149, 0, 0.3)" },
  production: { bg: "rgba(255, 59, 48, 0.12)", text: "#ff3b30", border: "rgba(255, 59, 48, 0.3)" },
};

export interface ConnectionState {
  id: string;
  status: "connected" | "disconnected" | "connecting" | "error";
  error?: string;
  databases: string[];
  currentDatabase: string;
  serverVersion?: string;
}

export interface Workspace {
  id: string; // 通常构成为 "connectionId:databaseName"
  connectionId: string;
  database: string;
}

export const DEFAULT_PORTS: Record<DatabaseDriver, number> = {
  mysql: 3306,
  postgres: 5432,
  sqlite: 0,
  tidb: 4000,
  starrocks: 9030,
};

export const DRIVER_LABELS: Record<DatabaseDriver, string> = {
  mysql: "MySQL",
  postgres: "PostgreSQL",
  sqlite: "SQLite",
  tidb: "TiDB",
  starrocks: "StarRocks",
};

export const CONNECTION_COLORS = [
  "#007aff",
  "#34c759",
  "#ff9500",
  "#ff3b30",
  "#af52de",
  "#5856d6",
  "#ff2d55",
  "#00c7be",
];
