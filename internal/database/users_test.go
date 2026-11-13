package database

import (
	"strings"
	"testing"
)

func ip(n int) *int   { return &n }
func bp(b bool) *bool { return &b }

func TestUserSQLMySQL(t *testing.T) {
	got, err := BuildCreateUserSQL("mysql", UserSpec{Name: "o'brien", Host: "10.0.%", Password: `pa\ss'w`, Limits: UserLimits{MaxQueries: ip(100), MaxUserConns: ip(0)}})
	if err != nil {
		t.Fatal(err)
	}
	want := `CREATE USER 'o\'brien'@'10.0.%' IDENTIFIED BY 'pa\\ss\'w' WITH MAX_QUERIES_PER_HOUR 100 MAX_USER_CONNECTIONS 0`
	if got != want {
		t.Fatalf("\n got %s\nwant %s", got, want)
	}
	// 主机留空是 %
	if got, _ := BuildCreateUserSQL("mysql", UserSpec{Name: "a"}); got != "CREATE USER 'a'@'%'" {
		t.Fatal(got)
	}
	if got, _ := BuildAlterUserSQL("mysql", UserSpec{Name: "a", Host: "h"}); got != "" {
		t.Fatalf("没有要改的应返回空: %q", got)
	}
	got, _ = BuildAlterUserSQL("mysql", UserSpec{Name: "a", Host: "h", Password: "x"})
	if got != `ALTER USER 'a'@'h' IDENTIFIED BY 'x'` {
		t.Fatal(got)
	}
	if got, _ := BuildDropUserSQL("mysql", "a", "h"); got != `DROP USER 'a'@'h'` {
		t.Fatal(got)
	}
}

func TestGrantSQLMySQL(t *testing.T) {
	cases := []struct {
		scope  PrivScope
		privs  []string
		revoke bool
		want   string
	}{
		{PrivScope{}, []string{"select", "process"}, false, "GRANT SELECT, PROCESS ON *.* TO 'u'@'h'"},
		{PrivScope{DB: "shop"}, []string{"ALL PRIVILEGES"}, false, "GRANT ALL PRIVILEGES ON `shop`.* TO 'u'@'h' WITH GRANT OPTION"},
		{PrivScope{DB: "shop", Table: "orders"}, []string{"SELECT"}, true, "REVOKE SELECT ON `shop`.`orders` FROM 'u'@'h'"},
	}
	for i, c := range cases {
		var got string
		var err error
		if c.revoke {
			got, err = BuildRevokeSQL("mysql", c.privs, c.scope, "u", "h")
		} else {
			got, err = BuildGrantSQL("mysql", c.privs, c.scope, "u", "h", i == 1)
		}
		if err != nil || got != c.want {
			t.Errorf("%d: %q %v\nwant %q", i, got, err, c.want)
		}
	}
	// 全局权限不能用在库级，防止拼出不合法的语句
	if _, err := BuildGrantSQL("mysql", []string{"SHUTDOWN"}, PrivScope{DB: "shop"}, "u", "h", false); err == nil {
		t.Error("库级不能授 SHUTDOWN")
	}
	if _, err := BuildGrantSQL("mysql", []string{"SELECT; DROP"}, PrivScope{}, "u", "h", false); err == nil {
		t.Error("非法权限名必须拒绝")
	}
	if _, err := BuildGrantSQL("mysql", nil, PrivScope{}, "u", "h", false); err == nil {
		t.Error("没选权限必须报错")
	}
	if _, err := BuildGrantSQL("mysql", []string{"SELECT"}, PrivScope{Table: "t"}, "u", "h", false); err == nil {
		t.Error("有表没库必须报错")
	}
}

func TestUserSQLPostgres(t *testing.T) {
	got, _ := BuildCreateUserSQL("postgres", UserSpec{Name: "App", Password: "p'w", Attrs: PGRoleAttrs{Login: bp(true), CreateDB: bp(false)}})
	if got != `CREATE ROLE "App" WITH LOGIN NOCREATEDB PASSWORD 'p''w'` {
		t.Fatal(got)
	}
	if got, _ := BuildCreateUserSQL("postgres", UserSpec{Name: "r"}); got != `CREATE ROLE "r"` {
		t.Fatal(got)
	}
	if got, _ := BuildAlterUserSQL("postgres", UserSpec{Name: "r", Attrs: PGRoleAttrs{Superuser: bp(true)}}); got != `ALTER ROLE "r" WITH SUPERUSER` {
		t.Fatal(got)
	}
	cases := []struct {
		scope PrivScope
		privs []string
		want  string
	}{
		{PrivScope{DB: "app"}, []string{"CONNECT"}, `GRANT CONNECT ON DATABASE "app" TO "r"`},
		{PrivScope{DB: "app", Schema: "public"}, []string{"USAGE"}, `GRANT USAGE ON SCHEMA "public" TO "r"`},
		{PrivScope{DB: "app", Schema: "public"}, []string{"SELECT", "INSERT"}, `GRANT SELECT, INSERT ON ALL TABLES IN SCHEMA "public" TO "r"`},
		{PrivScope{DB: "app", Schema: "public", Table: "t"}, []string{"SELECT"}, `GRANT SELECT ON TABLE "public"."t" TO "r"`},
	}
	for _, c := range cases {
		got, err := BuildGrantSQL("postgres", c.privs, c.scope, "r", "", false)
		if err != nil || got != c.want {
			t.Errorf("%q %v\nwant %q", got, err, c.want)
		}
	}
	if got, _ := BuildRevokeSQL("postgres", []string{"SELECT"}, PrivScope{Schema: "s", Table: "t"}, "r", ""); !strings.HasPrefix(got, "REVOKE SELECT ON TABLE") || !strings.HasSuffix(got, `FROM "r"`) {
		t.Fatal(got)
	}
	if _, err := BuildGrantSQL("postgres", []string{"SELECT"}, PrivScope{Table: "t"}, "r", "", false); err == nil {
		t.Error("有表没模式必须报错")
	}
	if _, err := BuildCreateUserSQL("sqlite", UserSpec{Name: "x"}); err == nil || SupportsUsers("sqlite") {
		t.Error("sqlite 不支持")
	}
}
