package ui

import (
	"database/sql"
	"fmt"
	"os"
	"testing"
	"time"

	"minidb/internal/model"
)

// @author ixxxxoooo
//
// 设计稿对照截图：把测试库布置成设计稿里的电商库 shop（orders 等表、两个视图），
// 连接叫 shop-dev、环境 DEV，这样 Go 界面和设计稿 A 的截图内容一致，可以逐处比位置和细节。
// 只在设置了 MINIDB_SHOTS 时运行，输出 d-xxx.png。

// newShopEnv 在 newEnv 的基础上换上设计稿的数据，返回已改名的环境（连接尚未打开）。
func newShopEnv(t *testing.T) *env {
	e := newEnv(t)
	db, err := sql.Open("sqlite", e.dbPath)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	stmts := []string{
		`DROP TABLE orders`,
		`CREATE TABLE orders (id BIGINT PRIMARY KEY, user_id BIGINT, status ENUM, total DECIMAL(10,2), created_at DATETIME, note TEXT)`,
		`CREATE TABLE categories (id INT PRIMARY KEY, name VARCHAR(60), parent_id INT)`,
		`CREATE TABLE coupons (id INT PRIMARY KEY, code VARCHAR(20))`,
		`CREATE TABLE order_items (id BIGINT PRIMARY KEY, order_id BIGINT, product_id BIGINT, qty INT, price DECIMAL(10,2))`,
		`CREATE TABLE payments (id BIGINT PRIMARY KEY, order_id BIGINT, channel VARCHAR(20), amount DECIMAL(10,2))`,
		`CREATE TABLE products (id BIGINT PRIMARY KEY, sku VARCHAR(32), name VARCHAR(120), category_id INT, price DECIMAL(10,2), stock INT, status VARCHAR(10), weight_g INT, legacy_flag TINYINT(1))`,
		`CREATE VIEW v_daily_sales AS SELECT date(created_at) d, SUM(total) s FROM orders GROUP BY d`,
		`CREATE VIEW v_order_summary AS SELECT status, COUNT(*) n FROM orders GROUP BY status`,
	}
	for _, s := range stmts {
		if _, err := db.Exec(s); err != nil {
			t.Fatalf("%s: %v", s, err)
		}
	}
	status := []string{"paid", "pending", "shipped", "paid", "refunded", "paid", "shipped", "pending"}
	notes := []any{nil, "请尽快发货", "发票抬头：星河科技", nil, "客户要求周末送达", nil, "礼品包装", nil, "重复下单，已联系客户确认"}
	base := time.Date(2026, 11, 15, 10, 24, 0, 0, time.Local)
	for i := 0; i < 60; i++ {
		ts := base.Add(-time.Duration(i*37) * time.Minute).Format("2006-01-02 15:04:05")
		total := fmt.Sprintf("%.2f", float64((i*137)%480)+12.9)
		if _, err := db.Exec(`INSERT INTO orders VALUES(?,?,?,?,?,?)`, 10231-i, 1000+((i*97+31)%900), status[i%len(status)], total, ts, notes[i%len(notes)]); err != nil {
			t.Fatal(err)
		}
	}
	e.cfg.Name, e.cfg.Tag = "shop-dev", "development"
	if err := e.sh.app.Connections.SaveConnection(e.cfg); err != nil {
		t.Fatal(err)
	}
	e.sh.loadConnections()
	return e
}

// shotD 保存一张设计稿对照截图（没设 MINIDB_SHOTS 就跳过）。
func shotD(e *env, name string) { e.shot("d-" + name) }

func TestShotsDesign(t *testing.T) {
	if os.Getenv("MINIDB_SHOTS") == "" {
		t.Skip("set MINIDB_SHOTS to render screenshots")
	}
	_ = model.ConnectionConfig{}
	e := newShopEnv(t)
	w := openWorkspace(e)
	w.openTable("orders")
	e.tt.Frame()
	e.tt.Frame()
	shotD(e, "data")
}
