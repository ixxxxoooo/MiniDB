package database

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
)

// UserInfo 是服务器上的一个账号。MySQL 的账号是 用户名@主机；PostgreSQL 的是角色，没有主机。
type UserInfo struct {
	Name string `json:"name"`
	Host string `json:"host"`
	// 以下只有 PostgreSQL 填：
	CanLogin   bool `json:"canLogin"`
	Superuser  bool `json:"superuser"`
	CreateDB   bool `json:"createDb"`
	CreateRole bool `json:"createRole"`
}

// SupportsUsers 该方言是否支持在这里管理用户。
func SupportsUsers(dbType string) bool {
	return dbType == "mysql" || dbType == "tidb" || dbType == "postgres"
}

// UserLimits 是 MySQL 的账号资源限制，0 表示不限制；nil 表示不改。
type UserLimits struct {
	MaxQueries     *int // 每小时查询数
	MaxUpdates     *int // 每小时更新数
	MaxConnections *int // 每小时连接数
	MaxUserConns   *int // 同时连接数
}

func (l UserLimits) clauses() []string {
	var out []string
	add := func(name string, v *int) {
		if v != nil {
			out = append(out, fmt.Sprintf("%s %d", name, *v))
		}
	}
	add("MAX_QUERIES_PER_HOUR", l.MaxQueries)
	add("MAX_UPDATES_PER_HOUR", l.MaxUpdates)
	add("MAX_CONNECTIONS_PER_HOUR", l.MaxConnections)
	add("MAX_USER_CONNECTIONS", l.MaxUserConns)
	return out
}

// PrivScope 是授权的范围。MySQL 是 *.*、库.*、库.表；PostgreSQL 是库、模式、表。
type PrivScope struct {
	DB     string // 空为全局（MySQL）
	Schema string // 仅 PostgreSQL
	Table  string // 空为整个库（MySQL）或整个模式里的所有表（PostgreSQL）
}

// MySQL 的权限清单，按范围分。
var (
	MySQLGlobalPrivs = []string{
		"SELECT", "INSERT", "UPDATE", "DELETE", "CREATE", "DROP", "REFERENCES", "INDEX", "ALTER",
		"CREATE TEMPORARY TABLES", "LOCK TABLES", "EXECUTE", "CREATE VIEW", "SHOW VIEW", "CREATE ROUTINE",
		"ALTER ROUTINE", "EVENT", "TRIGGER", "SHOW DATABASES", "PROCESS", "RELOAD", "SHUTDOWN", "FILE",
		"SUPER", "REPLICATION SLAVE", "REPLICATION CLIENT", "CREATE USER",
	}
	MySQLDBPrivs = []string{
		"SELECT", "INSERT", "UPDATE", "DELETE", "CREATE", "DROP", "REFERENCES", "INDEX", "ALTER",
		"CREATE TEMPORARY TABLES", "LOCK TABLES", "EXECUTE", "CREATE VIEW", "SHOW VIEW", "CREATE ROUTINE",
		"ALTER ROUTINE", "EVENT", "TRIGGER",
	}
	MySQLTablePrivs = []string{
		"SELECT", "INSERT", "UPDATE", "DELETE", "CREATE", "DROP", "REFERENCES", "INDEX", "ALTER",
		"CREATE VIEW", "SHOW VIEW", "TRIGGER",
	}
	PGDatabasePrivs = []string{"CONNECT", "CREATE", "TEMPORARY"}
	PGSchemaPrivs   = []string{"USAGE", "CREATE"}
	PGTablePrivs    = []string{"SELECT", "INSERT", "UPDATE", "DELETE", "TRUNCATE", "REFERENCES", "TRIGGER"}
)

// PrivilegesFor 返回某个范围可选的权限。
func PrivilegesFor(dbType string, scope PrivScope) []string {
	if dbType == "postgres" {
		switch {
		case scope.Schema == "":
			return PGDatabasePrivs
		case scope.Table == "":
			return append(append([]string{}, PGSchemaPrivs...), PGTablePrivs...)
		}
		return PGTablePrivs
	}
	switch {
	case scope.DB == "":
		return MySQLGlobalPrivs
	case scope.Table == "":
		return MySQLDBPrivs
	}
	return MySQLTablePrivs
}

// ---- MySQL ----

// mysqlStr 是 MySQL 字符串字面量。
func mysqlStr(s string) string {
	return "'" + strings.NewReplacer(`\`, `\\`, `'`, `\'`).Replace(s) + "'"
}

func mysqlAccount(name, host string) string {
	if host == "" {
		host = "%"
	}
	return mysqlStr(name) + "@" + mysqlStr(host)
}

func mysqlScope(s PrivScope) string {
	switch {
	case s.DB == "":
		return "*.*"
	case s.Table == "":
		return QuoteIdent("mysql", s.DB) + ".*"
	}
	return QuoteIdent("mysql", s.DB) + "." + QuoteIdent("mysql", s.Table)
}

// ---- PostgreSQL ----

func pgStr(s string) string { return "'" + strings.ReplaceAll(s, "'", "''") + "'" }

func pgRole(name string) string { return QuoteIdent("postgres", name) }

// PGRoleAttrs 是 PostgreSQL 角色的属性；nil 表示不改。
type PGRoleAttrs struct {
	Login      *bool
	Superuser  *bool
	CreateDB   *bool
	CreateRole *bool
}

func (a PGRoleAttrs) words() []string {
	var out []string
	add := func(on, off string, v *bool) {
		if v == nil {
			return
		}
		if *v {
			out = append(out, on)
		} else {
			out = append(out, off)
		}
	}
	add("LOGIN", "NOLOGIN", a.Login)
	add("SUPERUSER", "NOSUPERUSER", a.Superuser)
	add("CREATEDB", "NOCREATEDB", a.CreateDB)
	add("CREATEROLE", "NOCREATEROLE", a.CreateRole)
	return out
}

// ---- 语句生成 ----

// UserSpec 是创建或修改账号要用的信息。
type UserSpec struct {
	Name     string
	Host     string // 仅 MySQL
	Password string // 空表示不设置（创建时为无密码）
	Limits   UserLimits
	Attrs    PGRoleAttrs
}

func (u UserSpec) check() error {
	if strings.TrimSpace(u.Name) == "" {
		return fmt.Errorf("用户名为空")
	}
	return nil
}

// BuildCreateUserSQL 生成创建账号的语句。
func BuildCreateUserSQL(dbType string, u UserSpec) (string, error) {
	if err := u.check(); err != nil {
		return "", err
	}
	switch {
	case dbType == "mysql" || dbType == "tidb":
		var b strings.Builder
		b.WriteString("CREATE USER " + mysqlAccount(u.Name, u.Host))
		if u.Password != "" {
			b.WriteString(" IDENTIFIED BY " + mysqlStr(u.Password))
		}
		if cl := u.Limits.clauses(); len(cl) > 0 && dbType == "mysql" {
			b.WriteString(" WITH " + strings.Join(cl, " "))
		}
		return b.String(), nil
	case dbType == "postgres":
		words := u.Attrs.words()
		if u.Password != "" {
			words = append(words, "PASSWORD "+pgStr(u.Password))
		}
		if len(words) == 0 {
			return "CREATE ROLE " + pgRole(u.Name), nil
		}
		return "CREATE ROLE " + pgRole(u.Name) + " WITH " + strings.Join(words, " "), nil
	}
	return "", fmt.Errorf("%s 不支持管理用户", dbType)
}

// BuildAlterUserSQL 生成修改账号的语句：改密码、资源限制、角色属性。没有要改的返回空串。
func BuildAlterUserSQL(dbType string, u UserSpec) (string, error) {
	if err := u.check(); err != nil {
		return "", err
	}
	switch {
	case dbType == "mysql" || dbType == "tidb":
		var parts []string
		if u.Password != "" {
			parts = append(parts, "IDENTIFIED BY "+mysqlStr(u.Password))
		}
		if cl := u.Limits.clauses(); len(cl) > 0 && dbType == "mysql" {
			parts = append(parts, "WITH "+strings.Join(cl, " "))
		}
		if len(parts) == 0 {
			return "", nil
		}
		return "ALTER USER " + mysqlAccount(u.Name, u.Host) + " " + strings.Join(parts, " "), nil
	case dbType == "postgres":
		words := u.Attrs.words()
		if u.Password != "" {
			words = append(words, "PASSWORD "+pgStr(u.Password))
		}
		if len(words) == 0 {
			return "", nil
		}
		return "ALTER ROLE " + pgRole(u.Name) + " WITH " + strings.Join(words, " "), nil
	}
	return "", fmt.Errorf("%s 不支持管理用户", dbType)
}

// BuildDropUserSQL 生成删除账号的语句。
func BuildDropUserSQL(dbType, name, host string) (string, error) {
	if strings.TrimSpace(name) == "" {
		return "", fmt.Errorf("用户名为空")
	}
	switch {
	case dbType == "mysql" || dbType == "tidb":
		return "DROP USER " + mysqlAccount(name, host), nil
	case dbType == "postgres":
		return "DROP ROLE " + pgRole(name), nil
	}
	return "", fmt.Errorf("%s 不支持管理用户", dbType)
}

func cleanPrivs(privs []string, allowed []string) ([]string, error) {
	ok := map[string]bool{"ALL PRIVILEGES": true, "ALL": true}
	for _, a := range allowed {
		ok[a] = true
	}
	var out []string
	for _, p := range privs {
		p = strings.ToUpper(strings.TrimSpace(p))
		if p == "" {
			continue
		}
		if !ok[p] {
			return nil, fmt.Errorf("不支持的权限: %s", p)
		}
		out = append(out, p)
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("没有选择权限")
	}
	return out, nil
}

// BuildGrantSQL 生成授权语句；一次授权只针对一个范围。
func BuildGrantSQL(dbType string, privs []string, scope PrivScope, name, host string, withGrant bool) (string, error) {
	return buildPrivSQL(dbType, true, privs, scope, name, host, withGrant)
}

// BuildRevokeSQL 生成收回权限的语句。
func BuildRevokeSQL(dbType string, privs []string, scope PrivScope, name, host string) (string, error) {
	return buildPrivSQL(dbType, false, privs, scope, name, host, false)
}

func buildPrivSQL(dbType string, grant bool, privs []string, scope PrivScope, name, host string, withGrant bool) (string, error) {
	if strings.TrimSpace(name) == "" {
		return "", fmt.Errorf("用户名为空")
	}
	verb, prep := "GRANT", "TO"
	if !grant {
		verb, prep = "REVOKE", "FROM"
	}
	switch {
	case dbType == "mysql" || dbType == "tidb":
		clean, err := cleanPrivs(privs, PrivilegesFor(dbType, scope))
		if err != nil {
			return "", err
		}
		if scope.DB == "" && scope.Table != "" {
			return "", fmt.Errorf("指定了表就必须指定数据库")
		}
		stmt := fmt.Sprintf("%s %s ON %s %s %s", verb, strings.Join(clean, ", "), mysqlScope(scope), prep, mysqlAccount(name, host))
		if grant && withGrant {
			stmt += " WITH GRANT OPTION"
		}
		return stmt, nil
	case dbType == "postgres":
		clean, err := cleanPrivs(privs, PrivilegesFor(dbType, scope))
		if err != nil {
			return "", err
		}
		var target string
		switch {
		case scope.Schema == "" && scope.Table != "":
			return "", fmt.Errorf("指定了表就必须指定模式")
		case scope.Schema == "" && scope.DB == "":
			return "", fmt.Errorf("没有指定数据库")
		case scope.Schema == "":
			target = "DATABASE " + QuoteIdent("postgres", scope.DB)
		case scope.Table == "":
			// 只有模式权限（USAGE/CREATE）作用在模式上，其余作用在模式里所有的表上
			schemaOnly := true
			for _, p := range clean {
				if p != "USAGE" && p != "CREATE" && p != "ALL" && p != "ALL PRIVILEGES" {
					schemaOnly = false
				}
			}
			if schemaOnly {
				target = "SCHEMA " + QuoteIdent("postgres", scope.Schema)
			} else {
				target = "ALL TABLES IN SCHEMA " + QuoteIdent("postgres", scope.Schema)
			}
		default:
			target = "TABLE " + QuoteIdent("postgres", scope.Schema) + "." + QuoteIdent("postgres", scope.Table)
		}
		stmt := fmt.Sprintf("%s %s ON %s %s %s", verb, strings.Join(clean, ", "), target, prep, pgRole(name))
		if grant && withGrant {
			stmt += " WITH GRANT OPTION"
		}
		return stmt, nil
	}
	return "", fmt.Errorf("%s 不支持管理用户", dbType)
}

// ---- 读取 ----

// GetUsers 列出服务器上的账号。
func GetUsers(ctx context.Context, db *sql.DB, dbType string) ([]UserInfo, error) {
	var out []UserInfo
	switch {
	case dbType == "mysql" || dbType == "tidb":
		rows, err := db.QueryContext(ctx, "SELECT User, Host FROM mysql.user ORDER BY User, Host")
		if err != nil {
			return nil, err
		}
		defer rows.Close()
		for rows.Next() {
			var u UserInfo
			if err := rows.Scan(&u.Name, &u.Host); err != nil {
				return nil, err
			}
			out = append(out, u)
		}
		return out, rows.Err()
	case dbType == "postgres":
		rows, err := db.QueryContext(ctx, `SELECT rolname, rolcanlogin, rolsuper, rolcreatedb, rolcreaterole FROM pg_roles WHERE rolname NOT LIKE 'pg\_%' ORDER BY rolname`)
		if err != nil {
			return nil, err
		}
		defer rows.Close()
		for rows.Next() {
			var u UserInfo
			if err := rows.Scan(&u.Name, &u.CanLogin, &u.Superuser, &u.CreateDB, &u.CreateRole); err != nil {
				return nil, err
			}
			out = append(out, u)
		}
		return out, rows.Err()
	}
	return nil, fmt.Errorf("%s 不支持管理用户", dbType)
}

// GetUserGrants 返回账号现有的授权，每条一行：MySQL 是 SHOW GRANTS 的结果；
// PostgreSQL 把角色属性和表权限写成等价的 GRANT 语句。
func GetUserGrants(ctx context.Context, db *sql.DB, dbType, name, host string) ([]string, error) {
	var out []string
	switch {
	case dbType == "mysql" || dbType == "tidb":
		rows, err := db.QueryContext(ctx, "SHOW GRANTS FOR "+mysqlAccount(name, host))
		if err != nil {
			return nil, err
		}
		defer rows.Close()
		for rows.Next() {
			var line string
			if err := rows.Scan(&line); err != nil {
				return nil, err
			}
			out = append(out, line)
		}
		return out, rows.Err()
	case dbType == "postgres":
		rows, err := db.QueryContext(ctx, `SELECT table_schema, table_name, privilege_type FROM information_schema.role_table_grants WHERE grantee = $1 ORDER BY table_schema, table_name, privilege_type`, name)
		if err != nil {
			return nil, err
		}
		defer rows.Close()
		type key struct{ schema, table string }
		var order []key
		privs := map[key][]string{}
		for rows.Next() {
			var k key
			var p string
			if err := rows.Scan(&k.schema, &k.table, &p); err != nil {
				return nil, err
			}
			if _, seen := privs[k]; !seen {
				order = append(order, k)
			}
			privs[k] = append(privs[k], p)
		}
		if err := rows.Err(); err != nil {
			return nil, err
		}
		for _, k := range order {
			out = append(out, fmt.Sprintf("GRANT %s ON TABLE %s.%s TO %s", strings.Join(privs[k], ", "),
				QuoteIdent("postgres", k.schema), QuoteIdent("postgres", k.table), pgRole(name)))
		}
		return out, nil
	}
	return nil, fmt.Errorf("%s 不支持管理用户", dbType)
}
