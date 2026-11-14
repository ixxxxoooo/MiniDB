package database

import (
	"context"
	"os"
	"strings"
	"testing"
)

// 执行计划的集成测试：对真实 MySQL 跑「普通 EXPLAIN」与「ANALYZE」两条路径。
// 沿用 integration_services_test.go 的约定（INTEGRATION_TEST=1 + MINIDB_IT_MYSQL_*），CI 的 MySQL 容器会自动跑。
// 本地运行：
//
//	INTEGRATION_TEST=1 MINIDB_IT_MYSQL_HOST=127.0.0.1 MINIDB_IT_MYSQL_PORT=3306 MINIDB_IT_MYSQL_USER=root \
//	  go test ./internal/database/ -run TestIntegrationExplainMySQL -count=1 -v
//
// 不同版本返回的 JSON 格式不同（8.x 默认老格式 query_block，新版默认 v2 query_plan），
// 同一组断言要在两种格式上都成立（所以用例不用表别名：老格式把别名当 table_name，新格式才是真表名）。测试只在自己建的 minidb_explain_it 库里操作，结束后删掉。
func TestIntegrationExplainMySQL(t *testing.T) {
	skipIfNoIntegration(t)
	const envKey = "MINIDB_IT_MYSQL"
	host := os.Getenv(envKey + "_HOST")
	if host == "" {
		t.Skipf("未设置 %s_HOST", envKey)
	}
	cfg := &ConnectionConfig{
		ID: "it-explain", Name: "explain", Type: MySQL,
		Host: host, Port: integrationPort(envKey+"_PORT", 3306),
		User: os.Getenv(envKey + "_USER"), Password: os.Getenv(envKey + "_PASSWORD"),
	}
	m := NewManager()
	t.Cleanup(func() { _ = m.Disconnect(cfg.ID) })
	if err := m.Connect(cfg); err != nil {
		t.Fatalf("连接失败: %v", err)
	}
	db, err := m.GetDB(cfg.ID)
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	t.Cleanup(func() { _, _ = db.Exec("DROP DATABASE IF EXISTS minidb_explain_it") })
	for _, q := range []string{
		"DROP DATABASE IF EXISTS minidb_explain_it",
		"CREATE DATABASE minidb_explain_it",
		"CREATE TABLE minidb_explain_it.users (id INT PRIMARY KEY AUTO_INCREMENT, name VARCHAR(50), age INT, KEY idx_age (age))",
		"CREATE TABLE minidb_explain_it.orders (id INT PRIMARY KEY AUTO_INCREMENT, user_id INT, amount DECIMAL(10,2), KEY idx_user (user_id))",
		"INSERT INTO minidb_explain_it.users (name, age) WITH RECURSIVE n(i) AS (SELECT 1 UNION ALL SELECT i + 1 FROM n WHERE i < 1000) SELECT CONCAT('u', i), i % 60 FROM n",
		"INSERT INTO minidb_explain_it.orders (user_id, amount) SELECT id, id * 1.5 FROM minidb_explain_it.users",
		"ANALYZE TABLE minidb_explain_it.users, minidb_explain_it.orders",
	} {
		if _, err := db.ExecContext(ctx, q); err != nil {
			t.Fatalf("%s: %v", q, err)
		}
	}
	// 和应用里一样走 Session：独占一条连接并切到测试库
	conn, err := OpenSession(ctx, db, MySQL, "minidb_explain_it")
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()

	cases := []struct {
		name, sql string
		objects   []string // 普通 EXPLAIN 的计划树里应出现的对象（以此开头即可）
	}{
		{"全表扫描", "SELECT * FROM users", []string{"users"}},
		{"索引查找", "SELECT * FROM users WHERE age = 30", []string{"users (idx_age)"}},
		{"连接+分组+排序", "SELECT users.name, SUM(orders.amount) FROM users JOIN orders ON orders.user_id = users.id WHERE users.age > 50 GROUP BY users.name ORDER BY 2 DESC", []string{"users", "orders"}},
		{"子查询", "SELECT * FROM users WHERE id IN (SELECT user_id FROM orders WHERE amount > 1000)", []string{"users", "orders"}},
	}
	for _, analyze := range []bool{false, true} {
		for _, c := range cases {
			name := c.name
			if analyze {
				name += "/ANALYZE"
			}
			t.Run(name, func(t *testing.T) {
				res, err := ExplainQuery(ctx, conn, MySQL, c.sql, analyze)
				if err != nil {
					t.Fatal(err)
				}
				if len(res.Roots) == 0 {
					t.Fatalf("没有解析成计划树（只剩原文）: note=%q raw=%.200s", res.Note, res.Raw)
				}
				flat := res.Flat()
				if analyze {
					// ANALYZE 走树形文本：对象写的是别名（u、o）而不是表名，所以只看是否真执行并带回了耗时
					if !res.Analyzed {
						t.Error("ANALYZE 的结果应标记为已真实执行")
					}
					hasTime := false
					for _, r := range flat {
						hasTime = hasTime || r.Node.ActualMs > 0
					}
					if !hasTime {
						t.Error("ANALYZE 应带回至少一个节点的实际耗时")
					}
					return
				}
				for _, want := range c.objects {
					found := false
					for _, r := range flat {
						found = found || strings.HasPrefix(r.Node.Object, want)
					}
					if !found {
						t.Errorf("计划树里应有对象 %q，实际 %d 个节点", want, len(flat))
					}
				}
			})
		}
	}
}
