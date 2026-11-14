package ui

// @author ixxxxoooo
//
// 界面尺寸令牌（DIP）。数值来自设计稿「组件规范」，文字版在 docs/UI_TOKENS.md。
// 控件高度只有 22 / 26 / 30 三档；行高 26，间距以 4 为单位。新代码用这里的常量，不在调用处写数字。

// 窗口骨架。
const (
	sizeToolbar = 40  // 标题栏（标签并入其中）
	sizeRail    = 44  // 左侧活动栏宽度
	sizeRailBtn = 32  // 活动栏按钮
	sizeSide    = 236 // 侧栏默认宽度
	sizeStatus  = 26  // 底部状态栏
	sizeBar     = 34  // 内容区上方的条栏（数据 / 结构标签、过滤、工具）
	sizeTab     = 32  // 标题栏里的标签
	sizeRowPane = 300 // 右侧行预览停靠栏默认宽度
	sizeSideSub = 24  // 侧栏分组标题行
)

// 控件。
const (
	sizeBtnSm   = 22 // 小号：面板、列表、工具条里的操作（actBtn）
	sizeBtn     = 26 // 标准：与输入框同排（fieldBtn）
	sizeBtnLg   = 30 // 大号：对话框底部（footBtn）
	sizeInput   = 26 // 标准输入框
	sizeInputLg = 32 // 对话框里的大输入框
	sizeRow     = 26 // 表格、列表行高
	sizeHead    = 28 // 表头高度
)

// 字号。正文 13；密集信息（侧栏、标签、表头）12；控件文字 12.5；辅助 11。
const (
	fontCaption = 11
	fontDense   = 12
	fontCtl     = 12.5
	fontBody    = 13
	fontTitle   = 15
	fontHeading = 20
	fontBase    = fontBody  // 旧名，等同 fontBody
	font2XS     = fontDense // 旧名，等同 fontDense
)

// 圆角：小按钮 5、控件 6、面板与卡片 10、对话框 12。
const (
	radiusSm     = 5
	radiusBtn    = 6
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
