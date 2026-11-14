package database

import (
	"context"
	"database/sql"
	"fmt"
	"io"
	"strings"
)

// 重复键的处理方式
const (
	ConflictStop    = ""        // 报错并停止
	ConflictIgnore  = "ignore"  // 跳过重复的行
	ConflictReplace = "replace" // 用新行替换旧行（MySQL 系、SQLite）
	ConflictUpdate  = "update"  // 更新已有行的其余列
)

// importModifiers 按方言给出 INSERT 的写法：关键字（INSERT / INSERT IGNORE / REPLACE …）和语句末尾的冲突子句。
func importModifiers(dbType DBType, targets, keys []string, mode string) (verb, suffix string, err error) {
	verb = "INSERT INTO"
	if mode == ConflictStop {
		return verb, "", nil
	}
	mysql := IsMySQLCompatible(dbType)
	isKey := map[string]bool{}
	for _, k := range keys {
		isKey[strings.ToLower(k)] = true
	}
	var quotedKeys, sets []string
	for _, k := range keys {
		quotedKeys = append(quotedKeys, QuoteIdent(dbType, k))
	}
	for _, t := range targets {
		name := strings.Trim(t, "`\"")
		if isKey[strings.ToLower(name)] {
			continue
		}
		switch {
		case mysql:
			sets = append(sets, fmt.Sprintf("%s = VALUES(%s)", t, t))
		case dbType == Postgres:
			sets = append(sets, fmt.Sprintf("%s = EXCLUDED.%s", t, t))
		default:
			sets = append(sets, fmt.Sprintf("%s = excluded.%s", t, t))
		}
	}
	switch mode {
	case ConflictIgnore:
		switch {
		case mysql:
			return "INSERT IGNORE INTO", "", nil
		case dbType == Postgres:
			return verb, " ON CONFLICT DO NOTHING", nil
		default:
			return "INSERT OR IGNORE INTO", "", nil
		}
	case ConflictReplace:
		switch {
		case mysql:
			return "REPLACE INTO", "", nil
		case dbType == SQLite:
			return "INSERT OR REPLACE INTO", "", nil
		}
		return "", "", fmt.Errorf("%s 不支持“替换”，请改用“更新”", dbType)
	case ConflictUpdate:
		if mysql {
			if len(sets) == 0 {
				return "INSERT IGNORE INTO", "", nil
			}
			return verb, " ON DUPLICATE KEY UPDATE " + strings.Join(sets, ", "), nil
		}
		if len(quotedKeys) == 0 {
			return "", "", fmt.Errorf("“更新”需要表有主键或指定判断重复的列")
		}
		if len(sets) == 0 {
			return verb, " ON CONFLICT DO NOTHING", nil
		}
		return verb, fmt.Sprintf(" ON CONFLICT (%s) DO UPDATE SET %s", strings.Join(quotedKeys, ", "), strings.Join(sets, ", ")), nil
	}
	return "", "", fmt.Errorf("未知的冲突处理方式: %s", mode)
}

// ImportResult 导入结果
type ImportResult struct {
	Inserted   int64 // 已成功写入（且未回滚）的行数
	RolledBack bool  // 失败/取消时整体回滚
}

const (
	defaultImportBatchRows = 500
	maxImportBatchRows     = 1000
	// 单条语句占位符上限：MySQL/PG 为 65535，SQLite 默认 32766，取保守值
	maxImportParams = 30000
)

// ImportRows 把 next 产出的行批量写入 table。next 返回 io.EOF 表示结束。
// onProgress 在每批写入后回调累计行数（可为 nil）。
// ctx 取消时返回 ctx.Err()；Atomic 模式下已写入的行会被回滚。
func ImportRows(ctx context.Context, session *Session, dbType DBType, dbName, table string, cols []ImportColumn, next func() ([]interface{}, error), opts ImportOptions, onProgress func(done int64)) (ImportResult, error) {
	var res ImportResult
	if len(cols) == 0 {
		return res, fmt.Errorf("没有可导入的列映射")
	}
	seen := map[string]bool{}
	targets := make([]string, len(cols))
	for i, c := range cols {
		key := strings.ToLower(c.Target)
		if c.Target == "" || seen[key] {
			return res, fmt.Errorf("目标列无效或重复: %q", c.Target)
		}
		seen[key] = true
		targets[i] = QuoteIdent(dbType, c.Target)
	}

	batchRows := opts.BatchSize
	if batchRows <= 0 {
		batchRows = defaultImportBatchRows
	}
	if batchRows > maxImportBatchRows {
		batchRows = maxImportBatchRows
	}
	if limit := maxImportParams / len(cols); batchRows > limit {
		batchRows = limit
	}
	if batchRows < 1 {
		batchRows = 1
	}

	verb, suffix, err := importModifiers(dbType, targets, opts.Keys, opts.Conflict)
	if err != nil {
		return res, err
	}
	prefix := fmt.Sprintf("%s %s (%s) VALUES ", verb, quoteTable(dbType, dbName, table), strings.Join(targets, ", "))

	var tx *sql.Tx
	begin := func() error {
		t, err := session.Begin()
		if err != nil {
			return err
		}
		tx = t
		return nil
	}
	if err := begin(); err != nil {
		return res, fmt.Errorf("开启事务失败: %w", err)
	}
	// 任何提前返回都回滚当前未提交的事务
	defer func() {
		if tx != nil {
			_ = tx.Rollback()
		}
	}()

	var committed, pending int64 // 已提交 / 当前事务内未提交的行数
	batch := make([][]interface{}, 0, batchRows)
	firstRowOfBatch := int64(1) // 当前批第一行的行号（从 1 起），用于错误定位

	flush := func() error {
		if len(batch) == 0 {
			return nil
		}
		query, args := buildImportInsert(dbType, prefix, len(cols), batch)
		query += suffix
		if _, err := tx.ExecContext(ctx, query, args...); err != nil {
			if ctx.Err() != nil {
				return ctx.Err()
			}
			return fmt.Errorf("第 %d–%d 行写入失败: %w", firstRowOfBatch, firstRowOfBatch+int64(len(batch))-1, err)
		}
		pending += int64(len(batch))
		firstRowOfBatch += int64(len(batch))
		batch = batch[:0]
		if !opts.Atomic {
			if err := tx.Commit(); err != nil {
				tx = nil
				return fmt.Errorf("提交失败: %w", err)
			}
			tx = nil
			committed += pending
			pending = 0
			if err := begin(); err != nil {
				return fmt.Errorf("开启事务失败: %w", err)
			}
		}
		if onProgress != nil {
			onProgress(committed + pending)
		}
		return nil
	}

	fail := func(err error) (ImportResult, error) {
		// 非原子模式下已提交的批次保留；当前未提交部分由 defer 回滚。
		res.Inserted = committed
		res.RolledBack = opts.Atomic
		if opts.Atomic {
			res.Inserted = 0
		}
		return res, err
	}

	for rowNo := int64(1); ; rowNo++ {
		if err := ctx.Err(); err != nil {
			return fail(err)
		}
		row, err := next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return fail(fmt.Errorf("第 %d 行解析失败: %w", rowNo, err))
		}
		values := make([]interface{}, len(cols))
		for i, c := range cols {
			var v interface{}
			if c.Source >= 0 && c.Source < len(row) {
				v = row[c.Source]
			}
			if s, ok := v.(string); ok && opts.EmptyAsNull && s == "" {
				v = nil
			}
			values[i] = v
		}
		batch = append(batch, values)
		if len(batch) >= batchRows {
			if err := flush(); err != nil {
				return fail(err)
			}
		}
	}
	if err := flush(); err != nil {
		return fail(err)
	}
	if err := tx.Commit(); err != nil {
		tx = nil
		return fail(fmt.Errorf("提交失败: %w", err))
	}
	tx = nil
	res.Inserted = committed + pending
	return res, nil
}

// buildImportInsert 生成多行 INSERT 的 SQL 与参数。PG 使用 $n，其余使用 ?。
func buildImportInsert(dbType DBType, prefix string, ncols int, rows [][]interface{}) (string, []interface{}) {
	var sb strings.Builder
	sb.WriteString(prefix)
	args := make([]interface{}, 0, len(rows)*ncols)
	n := 0
	for r, row := range rows {
		if r > 0 {
			sb.WriteString(", ")
		}
		sb.WriteByte('(')
		for c := 0; c < ncols; c++ {
			if c > 0 {
				sb.WriteString(", ")
			}
			if dbType == Postgres {
				n++
				fmt.Fprintf(&sb, "$%d", n)
			} else {
				sb.WriteByte('?')
			}
			args = append(args, row[c])
		}
		sb.WriteByte(')')
	}
	return sb.String(), args
}

// NewTableColumn 是为导入新建的表的一列：Kind 为 int、float 或 text。
type NewTableColumn struct {
	Name string `json:"name"`
	Kind string `json:"kind"`
}

// importColumnType 把推断出的类型写成各方言的列类型。
func importColumnType(dbType DBType, kind string) string {
	switch {
	case dbType == SQLite:
		switch kind {
		case "int":
			return "INTEGER"
		case "float":
			return "REAL"
		}
		return "TEXT"
	case dbType == Postgres:
		switch kind {
		case "int":
			return "BIGINT"
		case "float":
			return "DOUBLE PRECISION"
		}
		return "TEXT"
	}
	switch kind { // MySQL 系
	case "int":
		return "BIGINT"
	case "float":
		return "DOUBLE"
	}
	return "TEXT"
}

// BuildCreateImportTableSQL 生成为导入建表的语句。
func BuildCreateImportTableSQL(dbType DBType, dbName, table string, cols []NewTableColumn) (string, error) {
	if strings.TrimSpace(table) == "" {
		return "", fmt.Errorf("表名为空")
	}
	if len(cols) == 0 {
		return "", fmt.Errorf("没有要建的列")
	}
	seen := map[string]bool{}
	defs := make([]string, len(cols))
	for i, c := range cols {
		name := strings.TrimSpace(c.Name)
		if name == "" || seen[strings.ToLower(name)] {
			return "", fmt.Errorf("列名为空或重复: %q", c.Name)
		}
		seen[strings.ToLower(name)] = true
		defs[i] = fmt.Sprintf("%s %s", QuoteIdent(dbType, name), importColumnType(dbType, c.Kind))
	}
	return fmt.Sprintf("CREATE TABLE %s (\n  %s\n)", quoteTable(dbType, dbName, table), strings.Join(defs, ",\n  ")), nil
}
