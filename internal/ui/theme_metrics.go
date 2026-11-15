package ui

// @author ixxxxoooo
//
// 界面尺寸令牌（DIP）。数值来自设计稿「组件规范」，文字版在 docs/UI_TOKENS.md。
// 控件高度只有 22 / 26 / 30 三档；行高 26，间距以 4 为单位。新代码用这里的常量，不在调用处写数字。

// 窗口骨架。
const (
	sizeToolbar  = 36  // 标题栏（标签并入其中）
	sizeRail     = 44  // 左侧活动栏宽度
	sizeRailBtn  = 32  // 活动栏按钮
	sizeRailIcon = 17  // 活动栏图标
	sizeSide     = 231 // 侧栏默认宽度（不含右边的 1px 分隔线）
	sizeStatus   = 24  // 底部状态栏
	sizeBar      = 36  // 内容区上方的条栏（数据 / 结构标签、过滤、工具）
	sizeTab      = 28  // 标题栏里的标签（药丸）
	sizeRowPane  = 300 // 右侧行预览停靠栏默认宽度
	sizeSideSub  = 24  // 侧栏分组标题行
	sizeSearch   = 190 // 标题栏的搜索按钮宽度
	sizeAvatar   = 22  // 连接头像（侧栏连接头）
	sizeSideSel  = 26  // 侧栏库选择框高度
	sizeSidePane = 186 // 图表页右侧配置栏宽度
	sizeSpark    = 38  // 监控指标卡里的走势高度
	sizeNotes    = 320 // 通知浮层宽度
	sizePopover  = 340 // 侧栏连接卡下方的连接 / 库浮层宽度
	sizeSetW     = 640 // 设置窗口宽度
	sizeSetH     = 460 // 设置窗口高度
	sizeSetNav   = 136 // 设置窗口左边导航宽度
)

// 控件。
const (
	sizeBtnSm    = 22 // 小号：面板、列表、工具条里的操作（actBtn）
	sizeBtn      = 26 // 标准：与输入框同排（fieldBtn）
	sizeBtnLg    = 30 // 大号：对话框底部（footBtn）
	sizeInput    = 26 // 标准输入框
	sizeInputLg  = 32 // 对话框里的大输入框
	sizeRow      = 26 // 表格、列表行高
	sizeRowComfy = 30 // 设置里「界面密度：标准」时数据表的行高
	sizeHead     = 28 // 表头高度
	sizeRowNo    = 44 // 数据表的行号列宽度
	sizeChip     = 18 // 圆角芯片
	sizeChipSq   = 16 // 方角芯片（环境标记）
	sizeUnder    = 28 // 下划线标签条
	sizeDot      = 6  // 状态小圆点
	sizeKbd      = 17 // 快捷键键帽高度
)

// 字号。正文 13；密集信息（侧栏、标签、表头）12；控件文字 12.5；辅助 11。
const (
	fontMicro   = 10 // 最小：键帽、类型标签、计数角标
	fontCaption = 11
	fontDense   = 12
	fontCtl     = 12.5
	fontBody    = 13
	fontTitle   = 15
	fontHeading = 20
	fontBase    = fontBody  // 旧名，等同 fontBody
	font2XS     = fontDense // 旧名，等同 fontDense
)

// 圆角：小按钮 5、控件 6、分段轨道 7、活动栏按钮 8、菜单 9、面板与卡片 10、对话框 12。
const (
	radiusXs     = 4
	radiusSm     = 5
	radiusBtn    = 6
	radiusSeg    = 7
	radiusRail   = 9
	radiusMenu   = 9
	radiusPanel  = 10
	radiusDialog = 12
)

// 间距：以 4 为单位。
const (
	spaceXS = 4
	spaceSM = 8
	spaceMD = 12
	spaceLG = 16
	spaceXL = 24
)

// 浮层阴影：两层，主层（偏移、模糊、扩散）与近身的细层。颜色取令牌的 shadow。
const (
	shadowY, shadowBlur   = 10, 30
	shadowY2, shadowBlur2 = 2, 6
)

// 生产环境顶部色带的高度。
const envBandH = 3
