package app

import (
	"bytes"

	"context"

	"fmt"
	"io"

	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"

	"minidb/internal/database"
	"time"
)

// ErrToolMissing 表示没找到外部工具。
type ErrToolMissing struct {
	Tool string
	Hint string
}

func (e *ErrToolMissing) Error() string {
	return fmt.Sprintf("没有找到 %s。%s", e.Tool, e.Hint)
}

// toolDirs 是图形程序的 PATH 里常常缺少、但装了客户端工具后会出现的目录。
func toolDirs() []string {
	dirs := []string{
		"/opt/homebrew/bin", "/usr/local/bin",
		"/opt/homebrew/opt/mysql-client/bin", "/usr/local/opt/mysql-client/bin",
		"/opt/homebrew/opt/libpq/bin", "/usr/local/opt/libpq/bin",
		"/usr/local/mysql/bin", "/Applications/Postgres.app/Contents/Versions/latest/bin",
		"/usr/lib/postgresql/16/bin", "/usr/lib/postgresql/15/bin", "/usr/lib/postgresql/14/bin",
	}
	if runtime.GOOS == "windows" {
		dirs = nil
	}
	return dirs
}

// findTool 找到工具的完整路径。override 可以是文件，也可以是所在目录。
func findTool(names []string, override string) (string, error) {
	override = strings.TrimSpace(override)
	if override != "" {
		if st, err := os.Stat(override); err == nil {
			if !st.IsDir() {
				return override, nil
			}
			for _, n := range names {
				if p := filepath.Join(override, n); fileExists(p) {
					return p, nil
				}
			}
		}
		return "", &ErrToolMissing{Tool: names[0], Hint: "请检查设置里指定的路径：" + override}
	}
	for _, n := range names {
		if p, err := exec.LookPath(n); err == nil {
			return p, nil
		}
	}
	for _, d := range toolDirs() {
		for _, n := range names {
			if p := filepath.Join(d, n); fileExists(p) {
				return p, nil
			}
		}
	}
	return "", &ErrToolMissing{Tool: names[0], Hint: toolInstallHint(names[0])}
}

func fileExists(p string) bool {
	st, err := os.Stat(p)
	return err == nil && !st.IsDir()
}

func toolInstallHint(tool string) string {
	switch tool {
	case "mysqldump", "mysql":
		return "请安装 MySQL 客户端（macOS：brew install mysql-client），或在备份选项里指定工具的位置。"
	case "pg_dump", "psql":
		return "请安装 PostgreSQL 客户端（macOS：brew install libpq），或在备份选项里指定工具的位置。"
	}
	return "请安装对应的客户端工具，或在备份选项里指定它的位置。"
}

// splitArgs 把一行参数按空白拆开，引号里的空白保留。
func splitArgs(line string) []string {
	var out []string
	var cur strings.Builder
	var quote rune
	has := false
	for _, r := range line {
		switch {
		case quote != 0:
			if r == quote {
				quote = 0
			} else {
				cur.WriteRune(r)
			}
		case r == '"' || r == '\'':
			quote, has = r, true
		case r == ' ' || r == '\t' || r == '\n':
			if has || cur.Len() > 0 {
				out = append(out, cur.String())
				cur.Reset()
				has = false
			}
		default:
			cur.WriteRune(r)
		}
	}
	if has || cur.Len() > 0 {
		out = append(out, cur.String())
	}
	return out
}

// dumpCommand 是要执行的外部命令。
type dumpCommand struct {
	names []string // 工具可能的文件名，按先后找
	args  []string
	env   []string // 追加的环境变量（密码走这里，不放命令行）
}

// buildDumpCommand 拼出备份命令。
func buildDumpCommand(cfg *database.ConnectionConfig, host string, port int, dbName string, o DumpOptions) (*dumpCommand, error) {
	extra := splitArgs(o.ExtraArgs)
	switch {
	case database.IsMySQLCompatible(cfg.Type) && cfg.Type != database.StarRocks:
		args := []string{"--host", host, "--port", strconv.Itoa(port), "--user", cfg.User,
			"--single-transaction", "--routines", "--triggers", "--default-character-set=utf8mb4"}
		if o.SchemaOnly {
			args = append(args, "--no-data")
		}
		args = append(append(args, extra...), dbName)
		return &dumpCommand{names: []string{"mysqldump", "mariadb-dump"}, args: args, env: []string{"MYSQL_PWD=" + cfg.Password}}, nil
	case cfg.Type == database.Postgres:
		args := []string{"--host", host, "--port", strconv.Itoa(port), "--username", cfg.User, "--no-password", "--format=plain"}
		if o.SchemaOnly {
			args = append(args, "--schema-only")
		}
		args = append(append(args, extra...), "--dbname", dbName)
		env := []string{"PGPASSWORD=" + cfg.Password}
		if cfg.SSLMode != "" {
			env = append(env, "PGSSLMODE="+cfg.SSLMode)
		}
		return &dumpCommand{names: []string{"pg_dump"}, args: args, env: env}, nil
	}
	return nil, fmt.Errorf("%s 不支持用外部工具备份", cfg.Type)
}

// buildRestoreCommand 拼出还原命令；SQL 从标准输入读入。
func buildRestoreCommand(cfg *database.ConnectionConfig, host string, port int, dbName, extraArgs string) (*dumpCommand, error) {
	extra := splitArgs(extraArgs)
	switch {
	case database.IsMySQLCompatible(cfg.Type) && cfg.Type != database.StarRocks:
		args := []string{"--host", host, "--port", strconv.Itoa(port), "--user", cfg.User, "--default-character-set=utf8mb4"}
		args = append(append(args, extra...), dbName)
		return &dumpCommand{names: []string{"mysql", "mariadb"}, args: args, env: []string{"MYSQL_PWD=" + cfg.Password}}, nil
	case cfg.Type == database.Postgres:
		args := []string{"--host", host, "--port", strconv.Itoa(port), "--username", cfg.User, "--no-password",
			"--quiet", "-v", "ON_ERROR_STOP=1"}
		args = append(append(args, extra...), "--dbname", dbName)
		env := []string{"PGPASSWORD=" + cfg.Password}
		if cfg.SSLMode != "" {
			env = append(env, "PGSSLMODE="+cfg.SSLMode)
		}
		return &dumpCommand{names: []string{"psql"}, args: args, env: env}, nil
	}
	return nil, fmt.Errorf("%s 不支持还原", cfg.Type)
}

// runTool 运行外部工具。stdin 非空时作为标准输入；stdout 写到 out；失败时返回带 stderr 的错误。
func runTool(ctx context.Context, tool string, c *dumpCommand, stdin io.Reader, out io.Writer) error {
	cmd := exec.CommandContext(ctx, tool, c.args...)
	cmd.Env = append(os.Environ(), c.env...)
	cmd.Stdin = stdin
	cmd.Stdout = out
	// 取消后外部工具若还有子进程攥着管道，也只等这么久就放手
	cmd.WaitDelay = 500 * time.Millisecond
	var stderr bytes.Buffer
	cmd.Stderr = &limitedBuffer{buf: &stderr, max: 8 << 10}
	if err := cmd.Run(); err != nil {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		msg := strings.TrimSpace(stderr.String())
		// 工具对命令行密码的提醒不是错误原因
		var lines []string
		for _, l := range strings.Split(msg, "\n") {
			if !strings.Contains(l, "Using a password on the command line") {
				lines = append(lines, l)
			}
		}
		msg = strings.TrimSpace(strings.Join(lines, "\n"))
		if msg == "" {
			msg = err.Error()
		}
		return fmt.Errorf("%s 失败: %s", filepath.Base(tool), msg)
	}
	return nil
}

// limitedBuffer 只留开头的 max 字节，多的丢掉。
type limitedBuffer struct {
	buf *bytes.Buffer
	max int
}

func (l *limitedBuffer) Write(p []byte) (int, error) {
	if room := l.max - l.buf.Len(); room > 0 {
		if len(p) > room {
			l.buf.Write(p[:room])
		} else {
			l.buf.Write(p)
		}
	}
	return len(p), nil
}
