/**
 * 把编辑器里的一段文本切成多条 SQL 语句。
 *
 * 只按"不在字符串/注释/引用标识符里的分号"切分；不做按行切分的猜测——
 * `INSERT INTO a⏎SELECT ...`、`CREATE TABLE x AS⏎SELECT ...` 这类跨行语句必须保持完整。
 * 注释会保留在语句内（服务端会忽略），只有"纯注释"的片段会被丢弃。
 */
export function splitSQLStatements(sql: string): string[] {
  const statements: string[] = [];
  let current = "";
  let hasCode = false; // 当前片段里是否出现过非注释、非空白内容
  const n = sql.length;
  let i = 0;

  const flush = () => {
    const trimmed = current.trim();
    if (trimmed && hasCode) statements.push(trimmed);
    current = "";
    hasCode = false;
  };

  while (i < n) {
    const c = sql[i];
    const next = sql[i + 1];

    // -- 行注释（MySQL 要求 -- 后跟空白；这里与 PG 一并宽松处理为注释）和 # 行注释
    if ((c === "-" && next === "-") || c === "#") {
      let end = sql.indexOf("\n", i);
      if (end === -1) end = n;
      current += sql.slice(i, end);
      i = end;
      continue;
    }

    // /* 块注释 */
    if (c === "/" && next === "*") {
      let end = sql.indexOf("*/", i + 2);
      end = end === -1 ? n : end + 2;
      current += sql.slice(i, end);
      i = end;
      continue;
    }

    // 引号：'...' "..." `...`，双写表示转义；单/双引号内反斜杠转义下一个字符
    if (c === "'" || c === '"' || c === "`") {
      const quote = c;
      let j = i + 1;
      while (j < n) {
        if (quote !== "`" && sql[j] === "\\" && j + 1 < n) {
          j += 2;
          continue;
        }
        if (sql[j] === quote) {
          if (sql[j + 1] === quote) {
            j += 2;
            continue;
          }
          j += 1;
          break;
        }
        j += 1;
      }
      current += sql.slice(i, j);
      hasCode = true;
      i = j;
      continue;
    }

    // PostgreSQL 美元引用 $$...$$ / $tag$...$tag$
    if (c === "$") {
      const m = /^\$[A-Za-z_][A-Za-z0-9_]*\$|^\$\$/.exec(sql.slice(i, i + 64));
      if (m) {
        const tag = m[0];
        const end = sql.indexOf(tag, i + tag.length);
        const stop = end === -1 ? n : end + tag.length;
        current += sql.slice(i, stop);
        hasCode = true;
        i = stop;
        continue;
      }
    }

    if (c === ";") {
      flush();
      i += 1;
      continue;
    }

    current += c;
    if (!/\s/.test(c)) hasCode = true;
    i += 1;
  }
  flush();
  return statements;
}
