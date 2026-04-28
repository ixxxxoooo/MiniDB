import * as Generated from "../../../../bindings/tableplus-ai/services/queryservice";
import type { QueryResult } from "../../../../bindings/tableplus-ai/internal/database/models";

export * from "../../../../bindings/tableplus-ai/services/queryservice";

function requireQueryResult(result: QueryResult | null): QueryResult {
  if (!result) {
    throw new Error("Query returned no result");
  }
  return result;
}

export async function ExecuteSQL(connID: string, dbName: string, sqlStr: string): Promise<QueryResult> {
  return requireQueryResult(await Generated.ExecuteSQL(connID, dbName, sqlStr));
}

export async function ExecuteSQLPaged(connID: string, dbName: string, sqlStr: string, page: number, pageSize: number): Promise<QueryResult> {
  return requireQueryResult(await Generated.ExecuteSQLPaged(connID, dbName, sqlStr, page, pageSize));
}

export async function QueryTableData(connID: string, dbName: string, table: string, page: number, pageSize: number, filters: Parameters<typeof Generated.QueryTableData>[5], sorts: Parameters<typeof Generated.QueryTableData>[6]): Promise<QueryResult> {
  return requireQueryResult(await Generated.QueryTableData(connID, dbName, table, page, pageSize, filters, sorts));
}

export async function QueryTableDataWithRawInput(connID: string, dbName: string, table: string, page: number, pageSize: number, filters: Parameters<typeof Generated.QueryTableDataWithRawInput>[5], sorts: Parameters<typeof Generated.QueryTableDataWithRawInput>[6], rawInput: string): Promise<QueryResult> {
  return requireQueryResult(await Generated.QueryTableDataWithRawInput(connID, dbName, table, page, pageSize, filters, sorts, rawInput));
}
