package database

import (
	"context"
	"database/sql"
	"fmt"
	"strconv"
	"strings"
)

// MySQL 系的用户管理与进程管理。StarRocks 虽然继承了这些方法，但 Caps 里没有声明支持，调用方不会走到这里。

func (d mysqlDialect) account(name, host string) string {
	if host == "" {
		host = "%"
	}
	return d.QuoteString(name) + "@" + d.QuoteString(host)
}

func (d mysqlDialect) scopeSQL(s PrivScope) string {
	switch {
	case s.DB == "":
		return "*.*"
	case s.Table == "":
		return d.QuoteIdent(s.DB) + ".*"
	}
	return d.QuoteIdent(s.DB) + "." + d.QuoteIdent(s.Table)
}

func (d mysqlDialect) Users(ctx context.Context, db Executor) ([]UserInfo, error) {
	var out []UserInfo
	err := queryEach(ctx, db, "SELECT User, Host FROM mysql.user ORDER BY User, Host", nil, func(rows rowScanner) error {
		var u UserInfo
		if err := rows.Scan(&u.Name, &u.Host); err != nil {
			return err
		}
		out = append(out, u)
		return nil
	})
	return out, err
}

func (d mysqlDialect) UserGrants(ctx context.Context, db Executor, name, host string) ([]string, error) {
	var out []string
	err := queryEach(ctx, db, "SHOW GRANTS FOR "+d.account(name, host), nil, func(rows rowScanner) error {
		var line string
		if err := rows.Scan(&line); err != nil {
			return err
		}
		out = append(out, line)
		return nil
	})
	return out, err
}

func (d mysqlDialect) CreateUserSQL(u UserSpec) string {
	var b strings.Builder
	b.WriteString("CREATE USER " + d.account(u.Name, u.Host))
	if u.Password != "" {
		b.WriteString(" IDENTIFIED BY " + d.QuoteString(u.Password))
	}
	if cl := u.Limits.Clauses(); len(cl) > 0 && d.kind == MySQL {
		b.WriteString(" WITH " + strings.Join(cl, " "))
	}
	return b.String()
}

func (d mysqlDialect) AlterUserSQL(u UserSpec) string {
	var parts []string
	if u.Password != "" {
		parts = append(parts, "IDENTIFIED BY "+d.QuoteString(u.Password))
	}
	if cl := u.Limits.Clauses(); len(cl) > 0 && d.kind == MySQL {
		parts = append(parts, "WITH "+strings.Join(cl, " "))
	}
	if len(parts) == 0 {
		return ""
	}
	return "ALTER USER " + d.account(u.Name, u.Host) + " " + strings.Join(parts, " ")
}

func (d mysqlDialect) DropUserSQL(name, host string) string {
	return "DROP USER " + d.account(name, host)
}

func (mysqlDialect) Privileges(scope PrivScope) []string {
	switch {
	case scope.DB == "":
		return MySQLGlobalPrivs
	case scope.Table == "":
		return MySQLDBPrivs
	}
	return MySQLTablePrivs
}

func (d mysqlDialect) PrivSQL(grant bool, privs []string, scope PrivScope, name, host string, withGrant bool) (string, error) {
	verb, prep := "GRANT", "TO"
	if !grant {
		verb, prep = "REVOKE", "FROM"
	}
	clean, err := cleanPrivs(privs, d.Privileges(scope))
	if err != nil {
		return "", err
	}
	if scope.DB == "" && scope.Table != "" {
		return "", fmt.Errorf("指定了表就必须指定数据库")
	}
	stmt := fmt.Sprintf("%s %s ON %s %s %s", verb, strings.Join(clean, ", "), d.scopeSQL(scope), prep, d.account(name, host))
	if grant && withGrant {
		stmt += " WITH GRANT OPTION"
	}
	return stmt, nil
}

func (mysqlDialect) ProcessList(ctx context.Context, db Executor) ([]ProcessInfo, error) {
	rows, err := db.QueryContext(ctx, "SHOW FULL PROCESSLIST")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	cols, err := rows.Columns()
	if err != nil {
		return nil, err
	}
	// 不同发行版（MySQL / TiDB / StarRocks）的列数和顺序不同，按列名（忽略大小写）取值
	idx := map[string]int{}
	for i, c := range cols {
		idx[strings.ToLower(c)] = i
	}
	var out []ProcessInfo
	for rows.Next() {
		raw := make([]sql.NullString, len(cols))
		dest := make([]any, len(cols))
		for i := range raw {
			dest[i] = &raw[i]
		}
		if err := rows.Scan(dest...); err != nil {
			return nil, err
		}
		get := func(name string) string {
			if i, ok := idx[name]; ok {
				return raw[i].String
			}
			return ""
		}
		sec, _ := strconv.ParseInt(get("time"), 10, 64)
		out = append(out, ProcessInfo{
			ID: get("id"), User: get("user"), Host: get("host"), Database: get("db"),
			Command: get("command"), TimeSec: sec, State: get("state"), Info: truncateProcessInfo(get("info")),
		})
	}
	return out, rows.Err()
}

func (mysqlDialect) KillProcess(ctx context.Context, db Executor, id uint64, queryOnly bool) error {
	kind := "CONNECTION"
	if queryOnly {
		kind = "QUERY"
	}
	_, err := db.ExecContext(ctx, fmt.Sprintf("KILL %s %d", kind, id))
	return err
}
