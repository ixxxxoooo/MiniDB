package database

// QueryResult 查询结果
type QueryResult struct {
	Columns     []ColumnMeta             `json:"columns"`
	Rows        []map[string]interface{} `json:"rows"`
	Total       int64                    `json:"total"`
	Duration    int64                    `json:"duration"`   // 毫秒
	DurationUs  int64                    `json:"durationUs"` // 微秒：不足 1 毫秒的查询也能看到耗时
	Error       string                   `json:"error,omitempty"`
	ErrorCode   string                   `json:"errorCode,omitempty"`   // 机器可读错误码（如 cancelled），供前端 i18n
	AutoLimited bool                     `json:"autoLimited,omitempty"` // 标记是否被自动追加了 LIMIT
	// TotalUnknown 为真表示 COUNT(*) 失败，Total 只是"至少这么多行"的下界。
	TotalUnknown bool `json:"totalUnknown,omitempty"`
	// Truncated 为真表示结果超过了内存硬上限（行数或字节），后面的行没有读进来。
	Truncated bool `json:"truncated,omitempty"`
}

// 结果集进内存前的硬上限：UI 分页之外的最后一道防线，防止大表或超大字段把进程撑爆。
var (
	maxScanRows  = 200_000
	maxScanBytes = int64(256 << 20)
)

// Filter 筛选条件
type Filter struct {
	Column   string `json:"column"`
	Operator string `json:"operator"`
	Value    string `json:"value"`
	// Or 让这个条件与它前一个条件用 OR 连接（默认 AND）。
	Or bool `json:"or,omitempty"`
}

// Sort 排序条件
type Sort struct {
	Column    string `json:"column"`
	Direction string `json:"direction"`
}

// DefaultPageSize 默认分页大小（用户未写 LIMIT 时自动追加）
const DefaultPageSize = 500
