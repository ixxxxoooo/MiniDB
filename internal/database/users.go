package database

import (
	"context"
	"fmt"
	"minidb/internal/model"
	"strings"
)

// SupportsUsers 该方言是否支持在这里管理用户。
func SupportsUsers(dbType DBType) bool { return CapsOf(dbType).Users }

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

// PrivilegesFor 返回某个范围可选的权限；不支持用户管理的类型返回 nil。
func PrivilegesFor(dbType DBType, scope PrivScope) []string {
	ua, err := userAdmin(dbType)
	if err != nil {
		return nil
	}
	return ua.Privileges(scope)
}

// ---- 语句生成 ----

// userAdmin 返回支持用户管理的方言；不支持时返回错误。
func userAdmin(dbType DBType) (UserAdmin, error) {
	d, err := DialectOf(dbType)
	if err == nil && d.Caps().Users {
		if ua, ok := d.(UserAdmin); ok {
			return ua, nil
		}
	}
	return nil, model.Errf(model.CodeUnsupportedUsers, "%s 不支持管理用户", dbType)
}

// BuildCreateUserSQL 生成创建账号的语句。
func BuildCreateUserSQL(dbType DBType, u UserSpec) (string, error) {
	if err := u.Check(); err != nil {
		return "", err
	}
	ua, err := userAdmin(dbType)
	if err != nil {
		return "", err
	}
	return ua.CreateUserSQL(u), nil
}

// BuildAlterUserSQL 生成修改账号的语句：改密码、资源限制、角色属性。没有要改的返回空串。
func BuildAlterUserSQL(dbType DBType, u UserSpec) (string, error) {
	if err := u.Check(); err != nil {
		return "", err
	}
	ua, err := userAdmin(dbType)
	if err != nil {
		return "", err
	}
	return ua.AlterUserSQL(u), nil
}

// BuildDropUserSQL 生成删除账号的语句。
func BuildDropUserSQL(dbType DBType, name, host string) (string, error) {
	if strings.TrimSpace(name) == "" {
		return "", fmt.Errorf("用户名为空")
	}
	ua, err := userAdmin(dbType)
	if err != nil {
		return "", err
	}
	return ua.DropUserSQL(name, host), nil
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
func BuildGrantSQL(dbType DBType, privs []string, scope PrivScope, name, host string, withGrant bool) (string, error) {
	return buildPrivSQL(dbType, true, privs, scope, name, host, withGrant)
}

// BuildRevokeSQL 生成收回权限的语句。
func BuildRevokeSQL(dbType DBType, privs []string, scope PrivScope, name, host string) (string, error) {
	return buildPrivSQL(dbType, false, privs, scope, name, host, false)
}

func buildPrivSQL(dbType DBType, grant bool, privs []string, scope PrivScope, name, host string, withGrant bool) (string, error) {
	if strings.TrimSpace(name) == "" {
		return "", fmt.Errorf("用户名为空")
	}
	ua, err := userAdmin(dbType)
	if err != nil {
		return "", err
	}
	return ua.PrivSQL(grant, privs, scope, name, host, withGrant)
}

// ---- 读取 ----

// GetUsers 列出服务器上的账号。
func GetUsers(ctx context.Context, db Executor, dbType DBType) ([]UserInfo, error) {
	ua, err := userAdmin(dbType)
	if err != nil {
		return nil, err
	}
	return ua.Users(ctx, db)
}

// GetUserGrants 返回账号现有的授权，每条一行：MySQL 是 SHOW GRANTS 的结果；
// PostgreSQL 把表权限写成等价的 GRANT 语句。
func GetUserGrants(ctx context.Context, db Executor, dbType DBType, name, host string) ([]string, error) {
	ua, err := userAdmin(dbType)
	if err != nil {
		return nil, err
	}
	return ua.UserGrants(ctx, db, name, host)
}
