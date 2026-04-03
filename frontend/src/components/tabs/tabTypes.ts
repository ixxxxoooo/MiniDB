import type { ColumnInfo } from "@/types/database";
import type { DatabaseDriver } from "@/types/connection";

export type TableSubView = "data" | "structure" | "info" | "doc";

export interface EditingStructureCol extends ColumnInfo {
  __status?: "new" | "modified" | "deleted";
  __uid: string;
}

export interface EditingIndexRow {
  __uid: string;
  __status?: "new" | "deleted";
  name: string;
  type: string;
  isUnique: boolean;
  columns: string[];
  isPrimary?: boolean;
}

export interface StructureColDef {
  key: string;
  label: string;
  editable: boolean;
  minWidth: number;
  isTypeSelect?: boolean;
  isCheckbox?: boolean;
}

// 各数据库引擎支持的全部字段类型列表
const DATA_TYPES_MAP: Record<DatabaseDriver, string[]> = {
  mysql: [
    "tinyint", "smallint", "mediumint", "int", "bigint",
    "decimal", "numeric", "float", "double", "bit", "boolean",
    "date", "datetime", "timestamp", "time", "year",
    "char", "varchar", "tinytext", "text", "mediumtext", "longtext",
    "binary", "varbinary", "tinyblob", "blob", "mediumblob", "longblob",
    "enum", "set",
    "json",
    "geometry", "point", "linestring", "polygon",
    "multipoint", "multilinestring", "multipolygon", "geometrycollection",
  ],
  postgres: [
    "smallint", "integer", "bigint", "decimal", "numeric",
    "real", "double precision", "smallserial", "serial", "bigserial",
    "money",
    "character varying", "varchar", "character", "char", "text",
    "bytea",
    "date", "time", "time with time zone",
    "timestamp", "timestamp with time zone", "interval",
    "boolean",
    "enum",
    "bit", "bit varying",
    "cidr", "inet", "macaddr", "macaddr8",
    "box", "circle", "line", "lseg", "path", "point", "polygon",
    "json", "jsonb",
    "uuid",
    "xml",
    "tsquery", "tsvector",
    "int4range", "int8range", "numrange", "tsrange", "tstzrange", "daterange",
    "integer[]", "text[]", "boolean[]", "jsonb[]",
  ],
  sqlite: [
    "INTEGER", "REAL", "TEXT", "BLOB", "NUMERIC",
    "INT", "TINYINT", "SMALLINT", "MEDIUMINT", "BIGINT",
    "UNSIGNED BIG INT", "INT2", "INT8",
    "CHARACTER(20)", "VARCHAR(255)", "VARYING CHARACTER(255)",
    "NCHAR(55)", "NATIVE CHARACTER(70)", "NVARCHAR(100)", "CLOB",
    "DOUBLE", "DOUBLE PRECISION", "FLOAT",
    "DECIMAL(10,5)", "BOOLEAN", "DATE", "DATETIME",
  ],
  tidb: [
    "tinyint", "smallint", "mediumint", "int", "bigint",
    "decimal", "numeric", "float", "double", "bit", "boolean",
    "date", "datetime", "timestamp", "time", "year",
    "char", "varchar", "tinytext", "text", "mediumtext", "longtext",
    "binary", "varbinary", "tinyblob", "blob", "mediumblob", "longblob",
    "enum", "set",
    "json",
  ],
  starrocks: [
    "BOOLEAN", "TINYINT", "SMALLINT", "INT", "BIGINT", "LARGEINT",
    "FLOAT", "DOUBLE", "DECIMAL",
    "CHAR", "VARCHAR", "STRING", "BINARY", "VARBINARY",
    "DATE", "DATETIME",
    "JSON", "ARRAY", "MAP", "STRUCT",
    "BITMAP", "HLL", "PERCENTILE",
  ],
};

export function getDataTypes(driver: DatabaseDriver | undefined): string[] {
  return DATA_TYPES_MAP[driver || "mysql"] || DATA_TYPES_MAP.mysql;
}

export const STRUCTURE_COL_DEFS: StructureColDef[] = [
  { key: "name", label: "column_name", editable: true, minWidth: 140 },
  { key: "type", label: "data_type", editable: true, minWidth: 120, isTypeSelect: true },
  { key: "characterSet", label: "character_set", editable: false, minWidth: 100 },
  { key: "collation", label: "collation", editable: false, minWidth: 130 },
  { key: "nullable", label: "is_nullable", editable: true, minWidth: 80, isCheckbox: true },
  { key: "defaultValue", label: "column_default", editable: true, minWidth: 110 },
  { key: "extra", label: "extra", editable: false, minWidth: 100 },
  { key: "foreignKey", label: "foreign_key", editable: false, minWidth: 110 },
  { key: "comment", label: "comment", editable: true, minWidth: 140 },
];

export const INDEX_COL_DEFS = [
  { key: "name", label: "index_name", minWidth: 160 },
  { key: "type", label: "index_algorithm", minWidth: 120 },
  { key: "isUnique", label: "is_unique", minWidth: 90 },
  { key: "columns", label: "column_name", minWidth: 220 },
] as const;
