package ui

import "github.com/egoist/mygo/ui"

// 对象类型 → 图标 / 颜色的唯一出处：侧栏、标签栏、概览、命令面板、@ 提及都从这里取，
// 同一种东西在任何地方都是同一个图标、同一个颜色（参考 TablePlus：表是网格、视图是眼睛、
// 函数是 ƒ、存储过程是「执行」三角、查询是带 </> 的文档）。
//
// @author ixxxxoooo
var (
	icQuery     = svg(`<path d="M14 2H6a2 2 0 0 0-2 2v16a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2V8z"/><path d="M14 2v6h6"/><path d="m10 13-2 2 2 2"/><path d="m14 13 2 2-2 2"/>`)
	icFunction  = svg(`<rect width="18" height="18" x="3" y="3" rx="2"/><path d="M9 17c2 0 2.8-1 2.8-2.8V10c0-2 1-3.3 3.2-3"/><path d="M9 11.2h5.7"/>`)
	icProcedure = svg(`<rect width="18" height="18" x="3" y="3" rx="2"/><path d="M10 8l5 4-5 4z"/>`)
)

// ObjectKind 是标签页、侧栏条目所代表的对象类型。
type ObjectKind string

// 对象类型。取值同时是存取时用的字符串，改动要兼容已存的数据。
const (
	KindTable     ObjectKind = "table"
	KindView      ObjectKind = "view"
	KindQuery     ObjectKind = "query"
	KindFunction  ObjectKind = "function"
	KindProcedure ObjectKind = "procedure"
	KindOverview  ObjectKind = "overview" // 库概览标签（只出现在会话存档里）
	KindER        ObjectKind = "er"       // ER 图标签（只出现在会话存档里）
)

// objectIcon 返回一类对象的图标。
// 参数 kind：表、视图、查询、函数、存储过程；其余按表处理。
// 返回值：该类对象统一使用的图标。
func objectIcon(kind ObjectKind) *ui.SVG {
	switch kind {
	case KindView:
		return icView
	case KindQuery:
		return icQuery
	case KindFunction:
		return icFunction
	case KindProcedure:
		return icProcedure
	}
	return icTable
}
