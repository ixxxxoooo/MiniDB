package sqltext

import (
	"strings"
	"testing"
)

func TestFormat(t *testing.T) {
	cases := []struct{ name, in, want string }{
		{"建表：一列一行，类型与修饰词大写，约束与表选项归位",
			"create table if not exists `users` (id bigint unsigned not null auto_increment primary key, name varchar(64) not null default '' comment '姓名', balance decimal(10,2) default 0.00, created_at datetime default current_timestamp on update current_timestamp, primary key (id), unique key uk_email (email), key idx_s (status, created_at), constraint fk_x foreign key (id) references other(id) on delete cascade) engine=innodb default charset=utf8mb4 comment='用户表';",
			"CREATE TABLE IF NOT EXISTS `users` (\n" +
				"  id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT PRIMARY KEY,\n" +
				"  name VARCHAR(64) NOT NULL DEFAULT '' COMMENT '姓名',\n" +
				"  balance DECIMAL(10, 2) DEFAULT 0.00,\n" +
				"  created_at DATETIME DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,\n" +
				"  PRIMARY KEY (id),\n" +
				"  UNIQUE KEY uk_email (email),\n" +
				"  KEY idx_s (status, created_at),\n" +
				"  CONSTRAINT fk_x FOREIGN KEY (id) REFERENCES other (id) ON DELETE CASCADE\n" +
				") ENGINE=innodb DEFAULT CHARSET=utf8mb4 COMMENT='用户表';"},
		{"建表：PostgreSQL 的类型",
			"create table t (id serial primary key, name character varying(255) not null, at timestamp with time zone default now(), tags text[])",
			"CREATE TABLE t (\n  id SERIAL PRIMARY KEY,\n  name CHARACTER VARYING(255) NOT NULL,\n  at TIMESTAMP WITH TIME ZONE DEFAULT now(),\n  tags TEXT[]\n)"},
		{"建表：名叫 date、text 的列名不被当成类型改写",
			"create table t (date date, text text)",
			"CREATE TABLE t (\n  date DATE,\n  text TEXT\n)"},
		{"建表：带引号的列名",
			"create table t (`id` int, \"name\" varchar(5))",
			"CREATE TABLE t (\n  `id` INT,\n  \"name\" VARCHAR(5)\n)"},
		{"查询：多列逐项换行，条件分组、函数与分组排序",
			"select a,b,count(*) as n, sum(case when x>1 then 1 else 0 end) s from t where a=1 and (b=2 or c=3) group by a,b having count(*)>1 order by a desc, b limit 10 offset 5",
			"SELECT\n  a,\n  b,\n  count(*) AS n,\n  sum(CASE WHEN x > 1 THEN 1 ELSE 0 END) s\nFROM t\nWHERE a = 1\n  AND (b = 2 OR c = 3)\nGROUP BY a, b\nHAVING count(*) > 1\nORDER BY a DESC, b\nLIMIT 10\nOFFSET 5"},
		{"查询：单列保持在 SELECT 同一行",
			"select u.id from users u left join orders o on o.uid=u.id",
			"SELECT u.id\nFROM users u\nLEFT JOIN orders o ON o.uid = u.id"},
		{"查询：JOIN 条件与子查询缩进",
			"select u.id from users u inner join items i on i.oid=u.id and i.x=1 where u.id in (select uid from vip where level>2) and u.created between '2020-01-01' and '2020-12-31'",
			"SELECT u.id\nFROM users u\nINNER JOIN items i ON i.oid = u.id\n  AND i.x = 1\nWHERE u.id IN (\n  SELECT uid\n  FROM vip\n  WHERE level > 2\n)\n  AND u.created BETWEEN '2020-01-01' AND '2020-12-31'"},
		{"查询：CTE 与 UNION ALL",
			"with a as (select 1 x), b as (select 2 y) select * from a join b on 1=1 union all select 3",
			"WITH a AS (\n  SELECT 1 x\n),\nb AS (\n  SELECT 2 y\n)\nSELECT *\nFROM a\nJOIN b ON 1 = 1\nUNION ALL\nSELECT 3"},
		{"查询：FROM 子查询与 EXISTS",
			"select * from (select a from t) x where exists (select 1 from y where y.a = x.a)",
			"SELECT *\nFROM (\n  SELECT a\n  FROM t\n) x\nWHERE EXISTS (\n  SELECT 1\n  FROM y\n  WHERE y.a = x.a\n)"},
		{"查询：DISTINCT 多列", "select distinct a, b from t", "SELECT DISTINCT\n  a,\n  b\nFROM t"},
		{"查询：LEFT() 是函数而不是 JOIN", "select left(name, 3) from t", "SELECT LEFT(name, 3)\nFROM t"},
		{"插入：多组逐组换行", "insert into t (a,b,c) values (1,'x',now()),(2,'y',null)",
			"INSERT INTO t (a, b, c)\nVALUES\n  (1, 'x', now()),\n  (2, 'y', NULL)"},
		{"插入：单组保持一行", "insert into t(a) values(1)", "INSERT INTO t (a)\nVALUES (1)"},
		{"更新：多项 SET 逐项换行", "update t set a=1,b='x',c=c+1 where id=3 and x<>-1",
			"UPDATE t\nSET\n  a = 1,\n  b = 'x',\n  c = c + 1\nWHERE id = 3\n  AND x <> -1"},
		{"更新：单项 SET 同行", "update t set a=1 where id=3", "UPDATE t\nSET a = 1\nWHERE id = 3"},
		{"删除", "delete from t where id in (1,2,3)", "DELETE FROM t\nWHERE id IN (1, 2, 3)"},
		{"改表：每个动作一行",
			"alter table users add column age int not null default 0 comment 'x', drop column old, modify name varchar(100)",
			"ALTER TABLE users\n  ADD COLUMN age INT NOT NULL DEFAULT 0 COMMENT 'x',\n  DROP COLUMN old,\n  MODIFY name VARCHAR(100)"},
		{"符号：类型转换、参数、十六进制、科学计数、一元负号、JSON 运算符都不被拆开",
			"select a::int, :id, {{name}}, ${x}, 0x1F, 1e+5, -1, a-1, 'it''s', data->>'k' from t -- tail",
			"SELECT\n  a::int,\n  :id,\n  {{name}},\n  ${x},\n  0x1F,\n  1e+5,\n  -1,\n  a - 1,\n  'it''s',\n  data ->> 'k'\nFROM t -- tail"},
		{"注释：独占一行的保持独占一行，行尾的留在行尾",
			"-- name: top\nselect 1; select 2 /* b */ ; select 3",
			"-- name: top\nSELECT 1;\n\nSELECT 2 /* b */;\n\nSELECT 3"},
		{"多条语句之间空一行", "select 1; select 2", "SELECT 1;\n\nSELECT 2"},
		{"BETWEEN 的 AND 不换行", "select * from t where a between 1 and 5 and b in (1,2,3)",
			"SELECT *\nFROM t\nWHERE a BETWEEN 1 AND 5\n  AND b IN (1, 2, 3)"},
		{"字符串里的逗号与引号不受影响", "select 'a,b' as \"x,y\" -- c, d\nfrom t", "SELECT 'a,b' AS \"x,y\" -- c, d\nFROM t"},
	}
	for _, c := range cases {
		if got := Format(c.in); got != c.want {
			t.Errorf("%s\nFormat(%q)\n got:\n%s\nwant:\n%s", c.name, c.in, got, c.want)
		}
	}
}

// 格式化必须幂等：排过版的结果再排一次不变。
func TestFormatIsIdempotent(t *testing.T) {
	for _, in := range []string{
		"create table t (id bigint unsigned not null auto_increment primary key, name varchar(64) default '' comment 'x', key idx (name)) engine=innodb comment='t'",
		"with a as (select 1 x) select a, b, count(*) from a left join b on a.x=b.x where a.y in (select 1) and (a.z=1 or a.z=2) group by a, b order by a",
		"insert into t (a,b) values (1,2),(3,4)",
		"update t set a=1,b=2 where id=1",
		"alter table t add column a int, drop column b",
		"select case when a then 1 when b then 2 else 3 end from t -- x\nwhere a = :id",
	} {
		once := Format(in)
		if twice := Format(once); twice != once {
			t.Errorf("not idempotent for %q\nonce:\n%s\ntwice:\n%s", in, once, twice)
		}
	}
}

// 格式化只改变空白与关键字大小写：去掉空白后的词法单元（关键字不分大小写）完全一致。
func TestFormatKeepsTokens(t *testing.T) {
	for _, in := range []string{
		"select `Name`, 'x;y' , 1.5 from  \"T\"  where a<>b -- tail",
		"create table t (id int not null default 0, name varchar(10) comment '名', primary key (id)) engine=InnoDB",
		"select a::int, :p, {{q}}, 0x1F, -1, data->>'k', t.* from t where a between 1 and 2",
		"insert into t (a) values (1),(2)",
	} {
		a, b := stripTokens(in), stripTokens(Format(in))
		if len(a) != len(b) {
			t.Fatalf("token count %d vs %d for %q: %q", len(a), len(b), in, Format(in))
		}
		for i := range a {
			if a[i] != b[i] {
				t.Errorf("token %d: %q vs %q (%q)", i, a[i], b[i], in)
			}
		}
	}
}

func stripTokens(s string) []string {
	var r []string
	for _, tk := range tokenize(s) {
		if tk.kind == tkSpace {
			continue
		}
		if tk.kind == tkWord {
			r = append(r, strings.ToUpper(tk.text))
		} else {
			r = append(r, tk.text)
		}
	}
	return r
}

// 乱写的输入不能崩溃：括号不配对、引号没闭合、只有注释。
func TestFormatToleratesBrokenInput(t *testing.T) {
	for _, in := range []string{"", ")", "((", "select 'abc", "select (1", "/* x", "-- only", "create table (", "select from where", ";;;", "select }{"} {
		_ = Format(in)
	}
}

func TestFormatWithOptions(t *testing.T) {
	in := "select a,b from t where x=1 and y=2"
	if got, want := FormatWith(in, FormatOptions{Indent: "\t"}), "SELECT\n\ta,\n\tb\nFROM t\nWHERE x = 1\n\tAND y = 2"; got != want {
		t.Errorf("制表符缩进:\n got %q\nwant %q", got, want)
	}
	if got, want := FormatWith(in, FormatOptions{Indent: "    ", LowerKeywords: true}), "select\n    a,\n    b\nfrom t\nwhere x = 1\n    and y = 2"; got != want {
		t.Errorf("四空格 + 小写:\n got %q\nwant %q", got, want)
	}
	if FormatWith(in, FormatOptions{}) != Format(in) {
		t.Error("零值选项应与 Format 一致")
	}
	// 字符串和标识符里的内容不受大小写选项影响
	if got := FormatWith("select 'FROM x' as `WHERE`", FormatOptions{LowerKeywords: true}); got != "select 'FROM x' as `WHERE`" {
		t.Errorf("引号内容被改动: %q", got)
	}
}
