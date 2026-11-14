package database

// 外键引用写成 "表.列"，放在 ColumnInfo.ForeignKey 里；各方言读取时填充（见 dialect_*.go）。

// SplitForeignKey 把 "表.列" 拆开；没有列时只返回表名。
func SplitForeignKey(ref string) (table, column string) {
	for i := len(ref) - 1; i >= 0; i-- {
		if ref[i] == '.' {
			return ref[:i], ref[i+1:]
		}
	}
	return ref, ""
}
