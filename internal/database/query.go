package database

// 结果集进内存前的硬上限：UI 分页之外的最后一道防线，防止大表或超大字段把进程撑爆。
var (
	maxScanRows  = 200_000
	maxScanBytes = int64(256 << 20)
)

// DefaultPageSize 默认分页大小（用户未写 LIMIT 时自动追加）
const DefaultPageSize = 500
