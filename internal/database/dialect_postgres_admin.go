package database

import (
	"context"
	"fmt"
	"strings"
)

// PostgreSQL 的用户（角色）管理与进程管理。

func (d postgresDialect) Users(ctx context.Context, db Executor) ([]UserInfo, error) {
	var out []UserInfo
	err := queryEach(ctx, db, `SELECT rolname, rolcanlogin, rolsuper, rolcreatedb, rolcreaterole FROM pg_roles WHERE rolname NOT LIKE 'pg\_%' ORDER BY rolname`,
		nil, func(rows rowScanner) error {
			var u UserInfo
			if err := rows.Scan(&u.Name, &u.CanLogin, &u.Superuser, &u.CreateDB, &u.CreateRole); err != nil {
				return err
			}
			out = append(out, u)
			return nil
		})
	return out, err
}

// UserGrants 把角色的表权限写成等价的 GRANT 语句。
func (d postgresDialect) UserGrants(ctx context.Context, db Executor, name, _ string) ([]string, error) {
	type key struct{ schema, table string }
	var order []key
	privs := map[key][]string{}
	err := queryEach(ctx, db, `SELECT table_schema, table_name, privilege_type FROM information_schema.role_table_grants WHERE grantee = $1 ORDER BY table_schema, table_name, privilege_type`,
		[]any{name}, func(rows rowScanner) error {
			var k key
			var p string
			if err := rows.Scan(&k.schema, &k.table, &p); err != nil {
				return err
			}
			if _, seen := privs[k]; !seen {
				order = append(order, k)
			}
			privs[k] = append(privs[k], p)
			return nil
		})
	if err != nil {
		return nil, err
	}
	var out []string
	for _, k := range order {
		out = append(out, fmt.Sprintf("GRANT %s ON TABLE %s.%s TO %s", strings.Join(privs[k], ", "),
			d.QuoteIdent(k.schema), d.QuoteIdent(k.table), d.QuoteIdent(name)))
	}
	return out, nil
}

func (d postgresDialect) CreateUserSQL(u UserSpec) string {
	words := u.Attrs.Words()
	if u.Password != "" {
		words = append(words, "PASSWORD "+d.QuoteString(u.Password))
	}
	if len(words) == 0 {
		return "CREATE ROLE " + d.QuoteIdent(u.Name)
	}
	return "CREATE ROLE " + d.QuoteIdent(u.Name) + " WITH " + strings.Join(words, " ")
}

func (d postgresDialect) AlterUserSQL(u UserSpec) string {
	words := u.Attrs.Words()
	if u.Password != "" {
		words = append(words, "PASSWORD "+d.QuoteString(u.Password))
	}
	if len(words) == 0 {
		return ""
	}
	return "ALTER ROLE " + d.QuoteIdent(u.Name) + " WITH " + strings.Join(words, " ")
}

func (d postgresDialect) DropUserSQL(name, _ string) string { return "DROP ROLE " + d.QuoteIdent(name) }

func (postgresDialect) Privileges(scope PrivScope) []string {
	switch {
	case scope.Schema == "":
		return PGDatabasePrivs
	case scope.Table == "":
		return append(append([]string{}, PGSchemaPrivs...), PGTablePrivs...)
	}
	return PGTablePrivs
}

func (d postgresDialect) PrivSQL(grant bool, privs []string, scope PrivScope, name, _ string, withGrant bool) (string, error) {
	verb, prep := "GRANT", "TO"
	if !grant {
		verb, prep = "REVOKE", "FROM"
	}
	clean, err := cleanPrivs(privs, d.Privileges(scope))
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
		target = "DATABASE " + d.QuoteIdent(scope.DB)
	case scope.Table == "":
		// 只有模式权限（USAGE/CREATE）作用在模式上，其余作用在模式里所有的表上
		schemaOnly := true
		for _, p := range clean {
			if p != "USAGE" && p != "CREATE" && p != "ALL" && p != "ALL PRIVILEGES" {
				schemaOnly = false
			}
		}
		if schemaOnly {
			target = "SCHEMA " + d.QuoteIdent(scope.Schema)
		} else {
			target = "ALL TABLES IN SCHEMA " + d.QuoteIdent(scope.Schema)
		}
	default:
		target = "TABLE " + d.QuoteIdent(scope.Schema) + "." + d.QuoteIdent(scope.Table)
	}
	stmt := fmt.Sprintf("%s %s ON %s %s %s", verb, strings.Join(clean, ", "), target, prep, d.QuoteIdent(name))
	if grant && withGrant {
		stmt += " WITH GRANT OPTION"
	}
	return stmt, nil
}

func (postgresDialect) ProcessList(ctx context.Context, db Executor) ([]ProcessInfo, error) {
	var out []ProcessInfo
	err := queryEach(ctx, db, `
		SELECT pid::text, COALESCE(usename, ''), COALESCE(client_addr::text, ''), COALESCE(datname, ''),
		       COALESCE(backend_type, ''), COALESCE(EXTRACT(EPOCH FROM (now() - query_start))::bigint, 0),
		       COALESCE(state, ''), COALESCE(query, '')
		FROM pg_stat_activity
		WHERE pid <> pg_backend_pid()
		ORDER BY query_start NULLS LAST`, nil, func(rows rowScanner) error {
		var p ProcessInfo
		if err := rows.Scan(&p.ID, &p.User, &p.Host, &p.Database, &p.Command, &p.TimeSec, &p.State, &p.Info); err != nil {
			return err
		}
		p.Info = truncateProcessInfo(p.Info)
		out = append(out, p)
		return nil
	})
	return out, err
}

func (postgresDialect) KillProcess(ctx context.Context, db Executor, id uint64, queryOnly bool) error {
	fn := "pg_terminate_backend"
	if queryOnly {
		fn = "pg_cancel_backend"
	}
	var ok bool
	if err := db.QueryRowContext(ctx, "SELECT "+fn+"($1)", int64(id)).Scan(&ok); err != nil {
		return err
	}
	if !ok {
		return fmt.Errorf("未能终止进程 %d（可能已结束，或没有权限）", id)
	}
	return nil
}
