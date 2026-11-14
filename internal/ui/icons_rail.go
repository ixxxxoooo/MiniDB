package ui

// @author ixxxxoooo
//
// 活动栏与标题栏用到的图标，路径取自设计稿（24×24，描边）。

var (
	icGrid    = svg(`<rect x="3" y="3" width="7" height="7" rx="1.5"/><rect x="14" y="3" width="7" height="7" rx="1.5"/><rect x="3" y="14" width="7" height="7" rx="1.5"/><rect x="14" y="14" width="7" height="7" rx="1.5"/>`)
	icER      = svg(`<rect x="3" y="3" width="8" height="6" rx="1.5"/><rect x="13" y="15" width="8" height="6" rx="1.5"/><path d="M7 9v3a3 3 0 0 0 3 3h3"/>`)
	icUsers   = svg(`<circle cx="9" cy="8" r="3.5"/><path d="M2.5 20c0-3.6 2.9-6 6.5-6s6.5 2.4 6.5 6M16 4.8a3.5 3.5 0 0 1 0 6.4M18 14.4c2 .8 3.5 2.6 3.5 5.6"/>`)
	icFileSQL = svg(`<path d="M14 3H7a2 2 0 0 0-2 2v14a2 2 0 0 0 2 2h10a2 2 0 0 0 2-2V8l-5-5zM14 3v5h5M10 13l-2 2 2 2M14 13l2 2-2 2"/>`)
	icImpExp  = svg(`<path d="M12 4v11M7 11l5 5 5-5M4 20h16"/>`)
	icChevUD  = svg(`<path d="M8 9l4-4 4 4M8 15l4 4 4-4"/>`)
)
