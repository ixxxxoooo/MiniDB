export type DatabaseDriver = "mysql" | "postgres" | "sqlite" | "tidb" | "starrocks";

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
  createdAt?: string;
  updatedAt?: string;
}

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
