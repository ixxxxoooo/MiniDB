# MiniDB 界面令牌

这是界面令牌的**唯一来源**，来自设计稿「MiniDB 界面重设计」。代码里的实现是 `internal/ui/theme_tokens.go`（结构）、`theme_palettes.go`（色值）、`theme_metrics.go`（尺寸）。

两边由 `TestTokensMatchDocs` 逐项对账：**改色值或尺寸，文档和代码要一起改**，只改一边测试会失败。用法和组件规则见 [UI_GUIDE.md](UI_GUIDE.md)。

## 使用规则

- 业务代码只引用令牌（`p.accent`、`sizeBtn` 这类），**不写 `ui.Hex(...)`、不在调用处写字号和控件高度的数字**。`TestUINoHardcodedStyle` 守着：`ui.Hex(` 只许出现在 `theme*.go`，字号和尺寸的数字字面量只许减不许增。
- 颜色只表达三件事：**选中**（`accentSoft2`）、**状态**（`success` / `warning` / `danger` / `info`）、**对象类型**（`icon*`）。数据值只在必要处着色：数字蓝、时间青、NULL 灰斜体。
- 强调色**固定**，不跟随连接颜色。连接颜色只出现在连接头像和生产环境的顶部色带上。
- 层级靠明度（`surface` → `surfaceSecondary` → `surfaceTertiary` → `surfaceElev`），不靠加线；边框优先于阴影，阴影只给弹窗、菜单、浮层。
- 悬停色由代码推导：`accentHover` 在浅色下是 `accent` 加深 8%，深色下是提亮 10%，不单独登记。

## 配色方案

三套方案，默认石墨。只改变表面、文字、线条和强调色。下表每行是「令牌 | 浅色 | 深色」，括号里是代码里的字段名。

### graphite

石墨：冷灰 + 靛蓝（强调色渐变的另一端是紫 `#8b5cf6`，只用在 Logo 上）。数值同设计稿 A（`docs/design/v2/styles/tokens.css` 的 light / dark）。

| 令牌 | 浅色 | 深色 |
| --- | --- | --- |
| `bg` | `#ffffff` | `#13161c` |
| `bg2` | `#f6f7fa` | `#181c24` |
| `bg3` | `#eceef3` | `#1f242e` |
| `elev` | `#ffffff` | `#13161c` |
| `seg-on` | `#ffffff` | `#13161c` |
| `fg` | `#14161c` | `#eceef4` |
| `fg2` | `#4b5160` | `#a6acbb` |
| `fg3` | `#8a90a1` | `#6f7686` |
| `fg4` | `#b3b8c5` | `#474d5b` |
| `line` | `#e3e5ec` | `#262b36` |
| `line2` | `#eceef3` | `#1e232c` |
| `stripe` | `#f9fafc` | `#161a21` |
| `hover` | `rgba(20,22,28,0.05)` | `rgba(255,255,255,0.053)` |
| `acc` | `#5b5df0` | `#7476ff` |
| `acc-t` | `#4e50ca` | `#a4a6ff` |
| `acc-fg` | `#ffffff` | `#ffffff` |
| `acc-bg` | `rgba(91,93,240,0.12)` | `rgba(116,118,255,0.12)` |
| `acc-bg2` | `rgba(91,93,240,0.2)` | `rgba(116,118,255,0.2)` |

### paper

纸墨：暖灰 + 陶土橙。

| 令牌 | 浅色 | 深色 |
| --- | --- | --- |
| `bg` | `#fbf8f1` | `#1c1a17` |
| `bg2` | `#f5f0e4` | `#23201c` |
| `bg3` | `#ebe4d3` | `#2d2924` |
| `elev` | `#fbf8f1` | `#2a2722` |
| `seg-on` | `#fbf8f1` | `#3d3830` |
| `fg` | `#2b2620` | `#efeae2` |
| `fg2` | `#5d5547` | `#b9b0a3` |
| `fg3` | `#968c79` | `#948b7e` |
| `fg4` | `#bdb39f` | `#5e564b` |
| `line` | `#ded5c1` | `#36322b` |
| `line2` | `#e8e1d0` | `#2a2722` |
| `stripe` | `#f8f4ea` | `#201e1a` |
| `hover` | `rgba(60,40,10,0.053)` | `rgba(255,240,220,0.053)` |
| `acc` | `#c2552d` | `#cf5a30` |
| `acc-t` | `#a74d2b` | `#f28f68` |
| `acc-fg` | `#ffffff` | `#ffffff` |
| `acc-bg` | `rgba(194,85,45,0.12)` | `rgba(242,143,104,0.12)` |
| `acc-bg2` | `rgba(194,85,45,0.2)` | `rgba(242,143,104,0.2)` |

### aurora

极光：深海蓝黑 + 青绿。

| 令牌 | 浅色 | 深色 |
| --- | --- | --- |
| `bg` | `#ffffff` | `#0e1226` |
| `bg2` | `#f2f6f8` | `#131833` |
| `bg3` | `#e4ebf0` | `#1a2042` |
| `elev` | `#ffffff` | `#0e1226` |
| `seg-on` | `#ffffff` | `#0e1226` |
| `fg` | `#0f1720` | `#e9ecff` |
| `fg2` | `#455564` | `#a3a9d6` |
| `fg3` | `#62727f` | `#6a71a3` |
| `fg4` | `#a7b3bd` | `#444b7a` |
| `line` | `#dce4ea` | `#232a52` |
| `line2` | `#ebf0f4` | `#1a2042` |
| `stripe` | `#f9fbfc` | `#111630` |
| `hover` | `rgba(15,23,32,0.05)` | `rgba(160,170,255,0.08)` |
| `acc` | `#0b8c80` | `#7476ff` |
| `acc-t` | `#0a7f74` | `#898bff` |
| `acc-fg` | `#ffffff` | `#ffffff` |
| `acc-bg` | `rgba(11,140,128,0.1)` | `rgba(116,118,255,0.12)` |
| `acc-bg2` | `rgba(11,140,128,0.18)` | `rgba(116,118,255,0.2)` |

字段名对应：`bg`=`surface`，`bg2`=`surfaceSecondary`（也是 `toolbar`），`bg3`=`surfaceTertiary`（标题栏、活动栏、分段轨道也用它），`elev`=`surfaceElev`，`seg-on`=`segThumb`，`fg` / `fg2` / `fg3` / `fg4`=`fg` / `fgSecondary` / `fgMuted` / `fgFaint`，`hover`=`hover`（半透明的悬停底），`line`=`border`，`line2`=`borderSubtle`，`acc`=`accent`，`acc-t`=`accentText`，`acc-fg`=`accentFg`，`acc-bg`=`accentSoft`，`acc-bg2`=`accentSoft2`。

## 共用令牌

与配色方案无关：状态色、行状态、对象图标色、数值色、SQL 语法色、图表色、阴影与遮罩。

### shared

| 令牌 | 浅色 | 深色 |
| --- | --- | --- |
| `ok` | `#17a56a` | `#17a56a` |
| `ok-bg` | `rgba(23,165,106,0.14)` | `rgba(23,165,106,0.14)` |
| `warn` | `#d98a0b` | `#d98a0b` |
| `warn-bg` | `rgba(217,138,11,0.16)` | `rgba(217,138,11,0.16)` |
| `err` | `#e5484d` | `#e5484d` |
| `err-bg` | `rgba(229,72,77,0.14)` | `rgba(229,72,77,0.14)` |
| `info` | `#3b82f6` | `#3b82f6` |
| `info-bg` | `rgba(59,130,246,0.14)` | `rgba(59,130,246,0.14)` |
| `new` | `#e7f6ee` | `#14281f` |
| `del` | `#fdeaea` | `#2e1c20` |
| `edit` | `#fdf0d4` | `#33290f` |
| `ic-t` | `#3b82f6` | `#3b82f6` |
| `ic-v` | `#8b5cf6` | `#8b5cf6` |
| `ic-q` | `#d98a0b` | `#d98a0b` |
| `ic-f` | `#d98a0b` | `#d98a0b` |
| `v-num` | `#3b82f6` | `#3b82f6` |
| `v-time` | `#0f8a5f` | `#5fd6a0` |
| `sx-kw` | `#7c3aed` | `#b794f6` |
| `sx-fn` | `#2563eb` | `#7db2ff` |
| `sx-str` | `#0f8a5f` | `#5fd6a0` |
| `sx-num` | `#c2410c` | `#f2a672` |
| `sx-cm` | `#9aa0af` | `#5b6272` |
| `sx-op` | `#4b5160` | `#a6acbb` |
| `c1` | `#5b5df0` | `#7476ff` |
| `c2` | `#14b8a6` | `#14b8a6` |
| `c3` | `#e08a2e` | `#f2a672` |
| `c4` | `#ec4899` | `#ec4899` |
| `c5` | `#7b8aa3` | `#9aa7b8` |
| `sh` | `rgba(16,20,40,0.18)` | `rgba(0,0,0,0.65)` |
| `scrim` | `rgba(18,20,30,0.32)` | `rgba(0,0,0,0.55)` |

字段名对应：`ok` / `warn` / `err` / `info`=`success` / `warning` / `danger` / `info`（`-bg` 后缀是 `successBg` 等），`new` / `del` / `edit`=`rowNew` / `rowDelete` / `cellEdit`，`ic-t` / `ic-v` / `ic-q` / `ic-f`=`iconTable` / `iconView` / `iconQuery` / `iconRoutine`，`v-num` / `v-time`=`valNum` / `valTime`，`sx-*`=`sxKeyword` / `sxFunc` / `sxString` / `sxNumber` / `sxComment` / `sxOp`，`c1`~`c5`=`chart[0]`~`chart[4]`，`sh`=`shadow`。

用途：`ok` 已连接、成功、安全模式「静默」；`info` 只读、提醒 1；`warn` 待提交、提醒 2；`err` 生产环境、危险、失败、「安全」档。`ic-t` 表、`ic-v` 视图、`ic-q` 查询、`ic-f` 主键与函数。

## 尺寸（DIP）

下表每行是「常量 | 值 | 用途」，值就是代码里的数字。

### metrics

| 常量 | 值 | 用途 |
| --- | --- | --- |
| `sizeToolbar` | `36` | 标题栏，标签并入其中 |
| `sizeRail` | `44` | 左侧活动栏宽度 |
| `sizeRailBtn` | `32` | 活动栏按钮 |
| `sizeRailIcon` | `17` | 活动栏图标 |
| `sizeSide` | `231` | 侧栏默认宽度（不含右边 1px 分隔线） |
| `sizeStatus` | `24` | 底部状态栏 |
| `sizeBar` | `36` | 内容区上方的条栏 |
| `sizeTab` | `28` | 标题栏里的标签 |
| `sizeRowPane` | `300` | 右侧行预览停靠栏默认宽度 |
| `sizeSideSub` | `24` | 侧栏分组标题行 |
| `sizeSearch` | `190` | 标题栏搜索按钮宽度 |
| `sizeAvatar` | `22` | 连接头像 |
| `sizeSideSel` | `26` | 侧栏库选择框高度 |
| `sizeBtnSm` | `22` | 小号按钮（`actBtn`） |
| `sizeBtn` | `26` | 标准按钮（`fieldBtn`） |
| `sizeBtnLg` | `30` | 大号按钮（`footBtn`，对话框底部） |
| `sizeInput` | `26` | 标准输入框 |
| `sizeInputLg` | `32` | 对话框里的大输入框 |
| `sizeRow` | `26` | 表格、列表行高 |
| `sizeRowComfy` | `30` | 设置里「界面密度：标准」时数据表的行高 |
| `sizeHead` | `28` | 表头高度 |
| `sizeRowNo` | `44` | 数据表行号列宽度 |
| `sizeSidePane` | `186` | 图表页右侧配置栏宽度 |
| `sizeChip` | `18` | 圆角芯片 |
| `sizeChipSq` | `16` | 方角芯片（环境标记） |
| `sizeUnder` | `28` | 下划线标签条 |
| `sizeDot` | `6` | 状态小圆点 |
| `sizeKbd` | `17` | 快捷键键帽高度 |
| `fontMicro` | `10` | 最小：键帽、类型标签、计数角标 |
| `fontCaption` | `11` | 辅助：计数、分组名、提示 |
| `fontDense` | `12` | 密集信息：侧栏、标签、表头、表格 |
| `fontCtl` | `12.5` | 控件文字：按钮、输入框、菜单 |
| `fontBody` | `13` | 正文：对话框 |
| `fontTitle` | `15` | 区块标题 |
| `fontHeading` | `20` | 页面标题 |
| `radiusXs` | `4` | 环境标记、键帽 |
| `radiusSm` | `5` | 小按钮 |
| `radiusBtn` | `6` | 控件 |
| `radiusSeg` | `7` | 分段开关的轨道 |
| `radiusRail` | `9` | 活动栏按钮 |
| `radiusMenu` | `9` | 菜单、提示卡 |
| `radiusPanel` | `10` | 面板、卡片 |
| `radiusDialog` | `12` | 对话框 |
| `spaceXS` | `4` | 间距阶梯 |
| `spaceSM` | `8` | 间距阶梯 |
| `spaceMD` | `12` | 间距阶梯 |
| `spaceLG` | `16` | 间距阶梯 |
| `spaceXL` | `24` | 间距阶梯 |
| `envBandH` | `3` | 生产环境顶部色带高度 |

字体：界面用系统字体栈（中文走系统自带字体）；表格、SQL、标识符用等宽字体 `mono`（JetBrains Mono，没有安装时依次退到 SF Mono、Menlo）。字重：正文 400，控件与标签 500，标题与强调 600。

浮层阴影两层：主层 `0 10px 30px`、近身层 `0 2px 6px`，颜色取 `sh`。深色下阴影几乎看不见，所以对话框、菜单、提示都再描一圈 `line`。

## 线条对比度

设计稿的线更淡：浅色 `line` 对白底约 1.27:1，`line2` 约 1.1:1。`theme_contrast_test.go` 按这个新区间守着三套配色的明暗两面，不能把某一套的线单独调重。
