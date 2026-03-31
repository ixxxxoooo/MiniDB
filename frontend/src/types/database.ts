export interface DatabaseInfo {
  name: string;
  tableCount: number;
  size: number;
}

export interface TableInfo {
  name: string;
  type: string;
  rowCount: number;
  size: number;
  comment: string;
}

export interface ColumnInfo {
  name: string;
  type: string;
  nullable: boolean;
  defaultValue: string | null;
  isPrimary: boolean;
  isAutoIncrement: boolean;
  comment: string;
  maxLength: number | null;
}

export interface TableStats {
  rowCount: number;
  dataSize: number;
  indexSize: number;
  totalSize: number;
  createTime: string;
  updateTime: string;
  engine: string;
  collation: string;
}

export interface QueryResult {
  columns: ColumnMeta[];
  rows: Record<string, unknown>[];
  total: number;
  duration: number;
  error?: string;
}

export interface ColumnMeta {
  name: string;
  type: string;
  nullable: boolean;
}

export interface Filter {
  column: string;
  operator: "=" | "!=" | ">" | "<" | ">=" | "<=" | "LIKE" | "IS NULL" | "IS NOT NULL" | "IN";
  value: string;
}

export interface Sort {
  column: string;
  direction: "ASC" | "DESC";
}
