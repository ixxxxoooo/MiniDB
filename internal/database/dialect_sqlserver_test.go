package database

import (
	"net/url"
	"strings"
	"testing"
)

func TestSQLServerDialectBasics(t *testing.T) {
	d, err := DialectOf(SQLServer)
	if err != nil || d.Label() != "SQL Server" || d.DefaultPort() != 1433 || d.DefaultUser() != "sa" || d.Driver() != "sqlserver" {
		t.Fatalf("%v %v", d, err)
	}
	c := CapsOf(SQLServer)
	if !c.PerDatabasePool || c.SessionUseDB || !c.ForeignKeys || !c.ProcessList || c.Users || c.CreateTable || c.AlterTable || c.Triggers || c.Routines || c.Explain || c.ReadOnlyTx {
		t.Fatalf("能力不对: %+v", c)
	}
	if SQLServer.MySQLCompatible() || !SQLServer.Valid() {
		t.Fatal("类型判断")
	}
	if got := QuoteIdent(SQLServer, "a]b"); got != "[a]]b]" {
		t.Fatal(got)
	}
	if got := QuoteString(SQLServer, "it's"); got != "'it''s'" {
		t.Fatal(got)
	}
	if QuoteIdent(SQLServer, "") != "" {
		t.Fatal("空串")
	}
	if _, ok := d.(BulkForeignKeys); !ok {
		t.Fatal("应支持批量外键")
	}
	if _, ok := d.(ProcessAdmin); !ok {
		t.Fatal("应支持进程管理")
	}
}

func TestSQLServerDSN(t *testing.T) {
	d := sqlserverDialect{}
	parse := func(cfg *ConnectionConfig) (*url.URL, url.Values) {
		u, err := url.Parse(d.DSN(cfg))
		if err != nil {
			t.Fatal(err)
		}
		return u, u.Query()
	}
	u, q := parse(&ConnectionConfig{Host: "h", Port: 1433, User: "sa", Password: "p@ss/1", Database: "Sales"})
	pw, _ := u.User.Password()
	if u.Scheme != "sqlserver" || u.Host != "h:1433" || u.User.Username() != "sa" || pw != "p@ss/1" {
		t.Fatalf("%v", u)
	}
	if q.Get("database") != "Sales" || q.Get("encrypt") != "disable" || q.Get("connection timeout") == "" {
		t.Fatalf("%v", q)
	}
	if _, q = parse(&ConnectionConfig{Host: "h", Port: 1, SSLMode: SSLRequired}); q.Get("encrypt") != "true" || q.Get("TrustServerCertificate") != "true" || q.Has("database") {
		t.Fatalf("required: %v", q)
	}
	if _, q = parse(&ConnectionConfig{Host: "h", Port: 1, SSLMode: SSLVerifyFull}); q.Get("encrypt") != "true" || q.Get("TrustServerCertificate") != "false" {
		t.Fatalf("verify-full: %v", q)
	}
}

func TestSQLServerSyntax(t *testing.T) {
	s := SyntaxOf(SQLServer)
	if s.Placeholder(2) != "@p2" || Placeholder(SQLServer, 1) != "@p1" {
		t.Fatal("占位符")
	}
	cases := []struct{ in, want string }{
		{"SELECT * FROM [t]", "SELECT * FROM [t] ORDER BY (SELECT NULL) OFFSET 20 ROWS FETCH NEXT 10 ROWS ONLY"},
		{"SELECT * FROM [t] ORDER BY [id];", "SELECT * FROM [t] ORDER BY [id] OFFSET 20 ROWS FETCH NEXT 10 ROWS ONLY"},
		{"SELECT * FROM (SELECT TOP 5 * FROM t ORDER BY a) x", "SELECT * FROM (SELECT TOP 5 * FROM t ORDER BY a) x ORDER BY (SELECT NULL) OFFSET 20 ROWS FETCH NEXT 10 ROWS ONLY"},
	}
	for _, c := range cases {
		if got := s.AppendPage(c.in, 10, 20); got != c.want {
			t.Errorf("AppendPage(%q)\n got %q\nwant %q", c.in, got, c.want)
		}
	}
	if got := s.AppendPage("SELECT 1 ORDER BY 1", 5, -1); got != "SELECT 1 ORDER BY 1 OFFSET 0 ROWS FETCH NEXT 5 ROWS ONLY" {
		t.Fatal(got)
	}
	if got := s.AppendPage("SELECT 1", 0, 0); got != "SELECT 1" {
		t.Fatal("FETCH 至少 1 行，limit 为 0 时不追加:", got)
	}
	if got := s.ZeroRows("[t]"); got != "SELECT TOP 0 * FROM [t]" {
		t.Fatal(got)
	}
	if got := s.CastText("[a]"); got != "CAST([a] AS NVARCHAR(MAX))" {
		t.Fatal(got)
	}
	for sql, want := range map[string]bool{
		"SELECT TOP 10 * FROM t": true,
		"SELECT * FROM t ORDER BY a OFFSET 0 ROWS FETCH NEXT 5 ROWS ONLY": true,
		"SELECT * FROM t WHERE id IN (SELECT TOP 1 id FROM u)":            false,
		"SELECT [top] FROM t": false,
		"SELECT * FROM t":     false,
	} {
		if got := s.HasRowLimit(sql); got != want {
			t.Errorf("HasRowLimit(%q) = %v", sql, got)
		}
	}
	if got := s.CountOf("SELECT a FROM t WHERE b > 1 ORDER BY a;"); got != "SELECT COUNT(*) FROM (SELECT a FROM t WHERE b > 1) AS __auto_count__" {
		t.Fatal(got)
	}
	if got := s.CountOf("SELECT a FROM (SELECT TOP 5 a FROM t ORDER BY a) x"); !strings.Contains(got, "ORDER BY a) x") {
		t.Fatal("子查询里的 ORDER BY 要保留:", got)
	}
	if got := PageQuery(SQLServer, "SELECT TOP 3 * FROM t ORDER BY a", 10, 0); got != "SELECT * FROM (SELECT TOP 3 * FROM t ORDER BY a) AS __page__ ORDER BY (SELECT NULL) OFFSET 0 ROWS FETCH NEXT 10 ROWS ONLY" {
		t.Fatal(got)
	}
	if got := importParamLimit(SQLServer); got >= 2100 {
		t.Fatal(got)
	}
	if importParamLimit(MySQL) != maxImportParams {
		t.Fatal("其他方言用默认上限")
	}
}

func TestSQLServerStatements(t *testing.T) {
	if got, err := BuildRenameTableSQL(SQLServer, "old t", "new"); err != nil || got != "EXEC sp_rename 'old t', 'new'" {
		t.Fatal(got, err)
	}
	clone, err := BuildCloneTableSQL(SQLServer, "a", "b", true)
	if err != nil || len(clone) != 1 || clone[0] != "SELECT * INTO [b] FROM [a]" {
		t.Fatal(clone, err)
	}
	clone, _ = BuildCloneTableSQL(SQLServer, "a", "b", false)
	if clone[0] != "SELECT * INTO [b] FROM [a] WHERE 1 = 0" {
		t.Fatal(clone)
	}
	if got := BuildTruncateTableSQL(SQLServer, "t", TableOpOptions{}); got != "TRUNCATE TABLE [t]" {
		t.Fatal(got)
	}
	if got := importColumnType(SQLServer, "text"); got != "NVARCHAR(MAX)" {
		t.Fatal(got)
	}
	if _, _, err := importModifiers(SQLServer, nil, nil, ConflictIgnore); err == nil {
		t.Fatal("SQL Server 不支持重复键处理，应报错而不是生成别的方言的语句")
	}
	if v, _, err := importModifiers(SQLServer, nil, nil, ConflictStop); err != nil || v != "INSERT INTO" {
		t.Fatal(v, err)
	}
	q, args := buildImportInsert(SQLServer, "INSERT INTO [t] ([a], [b]) VALUES ", 2, [][]interface{}{{1, 2}, {3, 4}})
	if q != "INSERT INTO [t] ([a], [b]) VALUES (@p1, @p2), (@p3, @p4)" || len(args) != 4 {
		t.Fatal(q)
	}
}

func TestUnwrapDefault(t *testing.T) {
	for in, want := range map[string]string{
		"((0))":         "0",
		"('abc')":       "'abc'",
		"(getdate())":   "getdate()",
		"((1)+(2))":     "(1)+(2)",
		"(N'a(b')":      "N'a(b'",
		"already":       "already",
		"(('x')+('y'))": "('x')+('y')",
	} {
		if got := unwrapDefault(in); got != want {
			t.Errorf("unwrapDefault(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestGuidString(t *testing.T) {
	// SQL Server 线上的 uniqueidentifier：前三段是小端
	wire := []byte{0x78, 0x56, 0x34, 0x12, 0xBC, 0x9A, 0xF0, 0xDE, 0x01, 0x23, 0x45, 0x67, 0x89, 0xAB, 0xCD, 0xEF}
	got, ok := guidString(wire)
	if !ok || got != "12345678-9ABC-DEF0-0123-456789ABCDEF" {
		t.Fatalf("%q %v", got, ok)
	}
	if _, ok := guidString([]byte{1, 2}); ok {
		t.Fatal("长度不对")
	}
	if v := normalizeScannedValue(wire, "UNIQUEIDENTIFIER"); v != "12345678-9ABC-DEF0-0123-456789ABCDEF" {
		t.Fatal(v)
	}
}

func TestBuildSQLServerDDL(t *testing.T) {
	d := func(s string) *string { return &s }
	cols := []ColumnInfo{
		{Name: "id", Type: "int", IsPrimary: true, IsAutoIncrement: true},
		{Name: "name", Type: "nvarchar(64)", Nullable: false, DefaultValue: d("'x'")},
		{Name: "owner_id", Type: "int", Nullable: true},
	}
	idx := []IndexInfo{
		{Name: "PK_t", Columns: []string{"id"}, IsPrimary: true, IsUnique: true, Type: "CLUSTERED"},
		{Name: "ux_name", Columns: []string{"name"}, IsUnique: true, Type: "NONCLUSTERED"},
	}
	fks := []ForeignKeyInfo{{Name: "fk_owner", Columns: []string{"owner_id"}, RefTable: "owners", RefColumns: []string{"id"}, OnDelete: "CASCADE", OnUpdate: "NO ACTION"}}
	got := buildSQLServerDDL("t", cols, idx, fks, 10, 5)
	for _, want := range []string{
		"CREATE TABLE [t] (",
		"[id] int IDENTITY(10,5) NOT NULL",
		"[name] nvarchar(64) NOT NULL DEFAULT 'x'",
		"[owner_id] int",
		"CONSTRAINT [PK_t] PRIMARY KEY CLUSTERED ([id])",
		"CREATE UNIQUE NONCLUSTERED INDEX [ux_name] ON [t] ([name]);",
		"ALTER TABLE [t] ADD CONSTRAINT [fk_owner] FOREIGN KEY ([owner_id]) REFERENCES [owners] ([id]) ON DELETE CASCADE;",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("缺少 %q:\n%s", want, got)
		}
	}
	if strings.Contains(got, "ON UPDATE") {
		t.Error("NO ACTION 不用写出来")
	}
	if got := sqlserverFKRule("SET_NULL"); got != "SET NULL" {
		t.Fatal(got)
	}
}
