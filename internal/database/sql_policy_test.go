package database

import "testing"

func TestCheckReadOnlySQL(t *testing.T) {
	if r := CheckReadOnlySQL("SELECT 1"); !r.Allowed {
		t.Fatalf("select should be allowed: %+v", r)
	}
	if r := CheckReadOnlySQL("WITH x AS (SELECT 1) SELECT * FROM x"); !r.Allowed {
		t.Fatalf("with should be allowed: %+v", r)
	}
	if r := CheckReadOnlySQL("DELETE FROM a"); r.Allowed || r.ReasonCode != "risky_sql" {
		t.Fatalf("delete should be risky: %+v", r)
	}
	if r := CheckReadOnlySQL(""); r.Allowed || r.ReasonCode != "empty_sql" {
		t.Fatalf("empty: %+v", r)
	}
	if r := CheckReadOnlySQL("UNKNOWNOP x"); r.Allowed || r.ReasonCode != "unknown_sql" {
		t.Fatalf("unknown: %+v", r)
	}
}

func TestCheckReadOnlySQLBlocksBypasses(t *testing.T) {
	blocked := []string{
		"EXPLAIN ANALYZE DELETE FROM users",
		"EXPLAIN ANALYZE UPDATE users SET a=1",
		"WITH d AS (DELETE FROM users RETURNING *) SELECT * FROM d",
		"WITH x AS (SELECT 1) INSERT INTO t SELECT * FROM x",
		"SELECT * FROM users INTO OUTFILE '/tmp/x'",
		"SELECT id INTO newtable FROM users",
		"SELECT * FROM users FOR UPDATE",
		"SELECT * FROM users FOR SHARE",
		"SELECT * FROM users LOCK IN SHARE MODE",
		"/* hi */ DELETE FROM users",
		"-- note\nDROP TABLE users",
		"SELECT 1; DROP TABLE users",
		"SELECT 1 /* ; */ ; DELETE FROM users",
		"SELECT SLEEP(10)",
		"SELECT pg_read_file('/etc/passwd')",
		"SELECT nextval('seq')",
		"/*!50000 DELETE FROM users */",
		"SELECT 'x\\'; DELETE FROM users; --'",
		"CALL do_something()",
		"SET @a = 1",
		"COPY t TO '/tmp/x'",
	}
	for _, q := range blocked {
		if r := CheckReadOnlySQL(q); r.Allowed {
			t.Errorf("应拒绝但被放行: %q", q)
		}
	}
}

func TestCheckReadOnlySQLAllowsLegitimate(t *testing.T) {
	allowed := []string{
		"SELECT * FROM users",
		"select 'delete me' as note",
		"SELECT `update`, `into` FROM t",
		"SELECT \"insert\" FROM t",
		"SELECT REPLACE(name, 'a', 'b') FROM t;",
		"WITH t AS (SELECT 1) SELECT * FROM t",
		"(SELECT 1) UNION (SELECT 2)",
		"SHOW CREATE TABLE users",
		"DESC users",
		"EXPLAIN SELECT * FROM users",
		"SELECT 'a;b' AS x",
		"/* c */ SELECT 1 -- trailing",
		"SELECT * FROM users WHERE name = 'it''s'",
	}
	for _, q := range allowed {
		if r := CheckReadOnlySQL(q); !r.Allowed {
			t.Errorf("应放行但被拒绝: %q (%s)", q, r.ReasonCode)
		}
	}
}

// 词法统一后补充：# 注释、美元引用串、嵌套注释这些方言歧义写法都不能被拿来藏写操作。
func TestCheckReadOnlySQLBlocksDialectAmbiguities(t *testing.T) {
	blocked := []string{
		"SELECT 1 # x\n; DELETE FROM users",
		"SELECT $$; DELETE FROM users$$",
		"SELECT 1 /* a /* b */ ; DELETE FROM users */",
		"SELECT 1 /* a /* b */ */ ; DELETE FROM users",
		"SELECT 1 --;DELETE FROM users",
		"SELECT 1 /* DELETE FROM users",
	}
	for _, q := range blocked {
		if r := CheckReadOnlySQL(q); r.Allowed {
			t.Errorf("应拒绝但被放行: %q", q)
		}
	}
}
