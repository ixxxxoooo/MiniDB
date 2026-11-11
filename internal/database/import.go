package database

import (
	"context"
	"database/sql"
	"fmt"
	"io"
	"strings"
)

// ImportColumn 一列的映射：源行下标 → 目标列名。
type ImportColumn struct {
	Source int    `json:"source"`
	Target string `json:"target"`
}

// ImportOptions 导入选项
type ImportOptions struct {
	// Atomic 为 true 时整个导入在一个事务里，任何错误或取消都会全部回滚；
	// 为 false 时每批各自提交，失败时已提交的批次保留（适合超大文件，避免超出事务大小限制）。
	Atomic bool `json:"atomic"`
	// EmptyAsNull 把空字符串写成 NULL（CSV 中空单元格通常表示缺失）。
	EmptyAsNull bool `json:"emptyAsNull"`
	// BatchSize 每条多行 INSERT 的行数，<=0 取默认值。
	BatchSize int `json:"batchSize"`
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
func ImportRows(ctx context.Context, session *Session, dbType, dbName, table string, cols []ImportColumn, next func() ([]interface{}, error), opts ImportOptions, onProgress func(done int64)) (ImportResult, error) {
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

	prefix := fmt.Sprintf("INSERT INTO %s (%s) VALUES ", quoteTable(dbType, dbName, table), strings.Join(targets, ", "))

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
func buildImportInsert(dbType, prefix string, ncols int, rows [][]interface{}) (string, []interface{}) {
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
			if dbType == "postgres" {
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
