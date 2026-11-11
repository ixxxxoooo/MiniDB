import { describe, expect, it } from "vitest";
import { splitSQLStatements } from "./sqlSplit";

describe("splitSQLStatements", () => {
  it("按分号切分多条语句", () => {
    expect(splitSQLStatements("SELECT 1; SELECT 2;")).toEqual(["SELECT 1", "SELECT 2"]);
  });

  it("没有分号时整段作为一条", () => {
    expect(splitSQLStatements("SELECT *\nFROM users\nWHERE id = 1")).toEqual([
      "SELECT *\nFROM users\nWHERE id = 1",
    ]);
  });

  it("不会把以 SQL 关键字开头的相邻行拆开", () => {
    const insertSelect = "INSERT INTO a (x)\nSELECT x FROM b";
    expect(splitSQLStatements(insertSelect)).toEqual([insertSelect]);
    const ctas = "CREATE TABLE t AS\nSELECT 1";
    expect(splitSQLStatements(ctas)).toEqual([ctas]);
    const explain = "EXPLAIN\nSELECT 1";
    expect(splitSQLStatements(explain)).toEqual([explain]);
  });

  it("忽略字符串里的分号", () => {
    expect(splitSQLStatements("SELECT 'a;b'; SELECT \"c;d\"")).toEqual(["SELECT 'a;b'", 'SELECT "c;d"']);
  });

  it("忽略反引号标识符里的分号", () => {
    expect(splitSQLStatements("SELECT `a;b` FROM t; SELECT 2")).toEqual(["SELECT `a;b` FROM t", "SELECT 2"]);
  });

  it("处理双写引号与反斜杠转义", () => {
    expect(splitSQLStatements("SELECT 'it''s;ok'; SELECT 2")).toEqual(["SELECT 'it''s;ok'", "SELECT 2"]);
    expect(splitSQLStatements("SELECT 'a\\';b'; SELECT 2")).toEqual(["SELECT 'a\\';b'", "SELECT 2"]);
  });

  it("忽略注释里的分号，并丢弃纯注释片段", () => {
    expect(splitSQLStatements("SELECT 1 -- a;b\n; SELECT 2")).toEqual(["SELECT 1 -- a;b", "SELECT 2"]);
    expect(splitSQLStatements("/* x; y */ SELECT 1")).toEqual(["/* x; y */ SELECT 1"]);
    expect(splitSQLStatements("-- just a comment\n;")).toEqual([]);
    expect(splitSQLStatements("# mysql comment; still comment\nSELECT 1")).toEqual([
      "# mysql comment; still comment\nSELECT 1",
    ]);
  });

  it("处理 PostgreSQL 美元引用", () => {
    const fn = "CREATE FUNCTION f() RETURNS int AS $$ BEGIN RETURN 1; END; $$ LANGUAGE plpgsql";
    expect(splitSQLStatements(`${fn}; SELECT 1`)).toEqual([fn, "SELECT 1"]);
    const tagged = "SELECT $tag$a;b$tag$";
    expect(splitSQLStatements(tagged)).toEqual([tagged]);
  });

  it("空输入与空白输入返回空数组", () => {
    expect(splitSQLStatements("")).toEqual([]);
    expect(splitSQLStatements("  \n ; ; ")).toEqual([]);
  });
});
