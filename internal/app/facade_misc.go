package app

import (
	"minidb/internal/ai"
	"minidb/internal/dataimport"
	"minidb/internal/export"
	"minidb/internal/schemaindex"
)

// 与 database_facade.go 同理：界面层不直接依赖这些包，需要的类型和函数从这里取。
// 只做别名与转调，不加逻辑。（agent 的事件模型见 facade_agent.go。）

type (
	// ChatMessage 一条 AI 会话消息。
	ChatMessage = ai.ChatMessage
	// SchemaStatus 与 SchemaStatusEvent 是表结构索引的状态与状态事件。
	SchemaStatus      = schemaindex.Status
	SchemaStatusEvent = schemaindex.StatusEvent
	// ExportOptions 及各格式的导出选项。
	ExportOptions     = export.Options
	ExportCSVOptions  = export.CSVOptions
	ExportJSONOptions = export.JSONOptions
	ExportSQLOptions  = export.SQLOptions
	// ImportKind、ImportFileOptions、ImportPreview 是文件导入的列类型、读取选项与预览。
	ImportKind        = dataimport.Kind
	ImportFileOptions = dataimport.Options
	ImportPreview     = dataimport.Preview
)

// ImportEncodings 是导入时可选的文字编码，第一项是默认的 UTF-8。
// ImportFormatXLSX 是 Excel 工作簿的导入格式名（预览结果的 Format 等于它时界面显示工作表选择）。
const ImportFormatXLSX = dataimport.FormatXLSX

var ImportEncodings = dataimport.Encodings

// InferImportKinds 转调 dataimport.InferKinds：按文件内容推断各列类型。
func InferImportKinds(path string, opts ImportFileOptions, limit int) ([]ImportKind, error) {
	return dataimport.InferKinds(path, opts, limit)
}
