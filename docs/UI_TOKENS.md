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

石墨：冷灰 + 靛蓝。

| 令牌 | 浅色 | 深色 |
| --- | --- | --- |
| `bg` | `#ffffff` | `#15171c` |
| `bg2` | `#f6f7f9` | `#1b1e24` |
| `bg3` | `#eceef2` | `#262a33` |
| `elev` | `#ffffff` | `#232730` |
| `seg-on` | `#ffffff` | `#373d49` |
| `fg` | `#16181d` | `#e8eaef` |
| `fg2` | `#4b5260` | `#aab0bd` |
| `fg3` | `#6b7383` | `#868d9b` |
| `line` | `#e1e4ea` | `#2c3039` |
| `line2` | `#eef0f4` | `#22262d` |
| `stripe` | `#fafbfc` | `#191c21` |
| `acc` | `#4f5bd5` | `#5e69e3` |
| `acc-t` | `#4650c8` | `#939cff` |
| `acc-fg` | `#ffffff` | `#ffffff` |
| `acc-bg` | `rgba(79,91,213,.08)` | `rgba(124,134,242,.10)` |
| `acc-bg2` | `rgba(79,91,213,.14)` | `rgba(124,134,242,.18)` |

### paper

纸墨：暖灰 + 陶土橙。

| 令牌 | 浅色 | 深色 |
| --- | --- | --- |
| `bg` | `#fdfcfa` | `#1c1a17` |
| `bg2` | `#f5f2ed` | `#23201c` |
| `bg3` | `#ebe6de` | `#2d2924` |
| `elev` | `#fffefc` | `#2a2722` |
| `seg-on` | `#fffefc` | `#3d3830` |
| `fg` | `#26221d` | `#efeae2` |
| `fg2` | `#5a5247` | `#b9b0a3` |
| `fg3` | `#74695c` | `#948b7e` |
| `line` | `#e6e0d6` | `#36322b` |
| `line2` | `#f0ebe3` | `#2a2722` |
| `stripe` | `#faf8f4` | `#201e1a` |
| `acc` | `#c1481a` | `#cf5a30` |
| `acc-t` | `#b24218` | `#f28f68` |
| `acc-fg` | `#ffffff` | `#ffffff` |
| `acc-bg` | `rgba(193,72,26,.08)` | `rgba(242,143,104,.10)` |
| `acc-bg2` | `rgba(193,72,26,.14)` | `rgba(242,143,104,.18)` |

### aurora

极光：深海蓝黑 + 青绿。

| 令牌 | 浅色 | 深色 |
| --- | --- | --- |
| `bg` | `#ffffff` | `#0f141b` |
| `bg2` | `#f2f6f8` | `#0b1016` |
| `bg3` | `#e4ebf0` | `#18202b` |
| `elev` | `#ffffff` | `#161d27` |
| `seg-on` | `#ffffff` | `#243040` |
| `fg` | `#0f1720` | `#e4eaf2` |
| `fg2` | `#455564` | `#9ba8b9` |
| `fg3` | `#62727f` | `#7b899c` |
| `line` | `#dce4ea` | `#222b38` |
| `line2` | `#ebf0f4` | `#19212b` |
| `stripe` | `#f9fbfc` | `#121820` |
| `acc` | `#0b8c80` | `#1fbfab` |
| `acc-t` | `#0a7f74` | `#35d9c4` |
| `acc-fg` | `#ffffff` | `#04201c` |
| `acc-bg` | `rgba(11,140,128,.08)` | `rgba(53,217,196,.09)` |
| `acc-bg2` | `rgba(11,140,128,.14)` | `rgba(53,217,196,.16)` |

字段名对应：`bg`=`surface`，`bg2`=`surfaceSecondary`（也是 `toolbar`），`bg3`=`surfaceTertiary`（也是 `hover`、`segTrack`），`elev`=`surfaceElev`，`seg-on`=`segThumb`，`fg` / `fg2` / `fg3`=`fg` / `fgSecondary` / `fgMuted`，`line`=`border`，`line2`=`borderSubtle`，`acc`=`accent`，`acc-t`=`accentText`，`acc-fg`=`accentFg`，`acc-bg`=`accentSoft`，`acc-bg2`=`accentSoft2`。

## 共用令牌

与配色方案无关：状态色、行状态、对象图标色、数值色、SQL 语法色、图表色、阴影与遮罩。

### shared

| 令牌 | 浅色 | 深色 |
| --- | --- | --- |
| `ok` | `#16794c` | `#4cc38a` |
| `ok-bg` | `rgba(22,121,76,.10)` | `rgba(76,195,138,.14)` |
| `warn` | `#a45f00` | `#eab04a` |
| `warn-bg` | `rgba(217,130,0,.13)` | `rgba(234,176,74,.14)` |
| `err` | `#c9302c` | `#f27575` |
| `err-bg` | `rgba(201,48,44,.09)` | `rgba(242,117,117,.14)` |
| `info` | `#1f64b8` | `#74aef7` |
| `info-bg` | `rgba(31,100,184,.09)` | `rgba(116,174,247,.14)` |
| `new` | `#e7f4e8` | `#1c3326` |
| `del` | `#fce8e8` | `#3b2227` |
| `edit` | `#fff3c4` | `#3b3520` |
| `ic-t` | `#3d78d4` | `#74a6f7` |
| `ic-v` | `#8656cc` | `#b896f4` |
| `ic-q` | `#1d9363` | `#4cc38a` |
| `ic-f` | `#c26f10` | `#eab04a` |
| `v-num` | `#2b5cc4` | `#83abff` |
| `v-time` | `#0c7f80` | `#4fd1c5` |
| `sx-kw` | `#6a43d6` | `#b59cff` |
| `sx-fn` | `#1f66c2` | `#7fb5ff` |
| `sx-str` | `#16794c` | `#7fd8a3` |
| `sx-num` | `#c0560a` | `#f2a76d` |
| `sx-cm` | `#7c8494` | `#6e7688` |
| `sx-op` | `#5a6270` | `#a9afbc` |
| `c1` | `#4f5bd5` | `#8e98ff` |
| `c2` | `#1b9a89` | `#4fd1c5` |
| `c3` | `#e08a2e` | `#f2a76d` |
| `c4` | `#d4538a` | `#f27aa8` |
| `c5` | `#7b8aa3` | `#9aa7b8` |
| `sh` | `rgba(16,24,40,.16)` | `rgba(0,0,0,.5)` |
| `scrim` | `rgba(16,20,30,.34)` | `rgba(0,0,0,.5)` |

字段名对应：`ok` / `warn` / `err` / `info`=`success` / `warning` / `danger` / `info`（`-bg` 后缀是 `successBg` 等），`new` / `del` / `edit`=`rowNew` / `rowDelete` / `cellEdit`，`ic-t` / `ic-v` / `ic-q` / `ic-f`=`iconTable` / `iconView` / `iconQuery` / `iconRoutine`，`v-num` / `v-time`=`valNum` / `valTime`，`sx-*`=`sxKeyword` / `sxFunc` / `sxString` / `sxNumber` / `sxComment` / `sxOp`，`c1`~`c5`=`chart[0]`~`chart[4]`，`sh`=`shadow`。

用途：`ok` 已连接、成功、安全模式「静默」；`info` 只读、提醒 1；`warn` 待提交、提醒 2；`err` 生产环境、危险、失败、「安全」档。`ic-t` 表、`ic-v` 视图、`ic-q` 查询、`ic-f` 主键与函数。

## 尺寸（DIP）

下表每行是「常量 | 值 | 用途」，值就是代码里的数字。

### metrics

| 常量 | 值 | 用途 |
| --- | --- | --- |
| `sizeToolbar` | `40` | 标题栏，标签并入其中 |
| `sizeRail` | `44` | 左侧活动栏宽度 |
| `sizeRailBtn` | `32` | 活动栏按钮 |
| `sizeRailIcon` | `18` | 活动栏图标 |
| `sizeSide` | `236` | 侧栏默认宽度 |
| `sizeStatus` | `26` | 底部状态栏 |
| `sizeBar` | `34` | 内容区上方的条栏 |
| `sizeTab` | `32` | 标题栏里的标签 |
| `sizeRowPane` | `300` | 右侧行预览停靠栏默认宽度 |
| `sizeSideSub` | `24` | 侧栏分组标题行 |
| `sizeSearch` | `176` | 标题栏搜索按钮宽度 |
| `sizeAvatar` | `28` | 连接卡头像 |
| `sizeSideSel` | `24` | 连接卡里的库选择、安全模式芯片高度 |
| `sizeBtnSm` | `22` | 小号按钮（`actBtn`） |
| `sizeBtn` | `26` | 标准按钮（`fieldBtn`） |
| `sizeBtnLg` | `30` | 大号按钮（`footBtn`，对话框底部） |
| `sizeInput` | `26` | 标准输入框 |
| `sizeInputLg` | `32` | 对话框里的大输入框 |
| `sizeRow` | `26` | 表格、列表行高 |
| `sizeHead` | `28` | 表头高度 |
| `sizeChip` | `20` | 圆角芯片 |
| `sizeChipSq` | `18` | 方角芯片（环境标记） |
| `sizeUnder` | `28` | 下划线标签条 |
| `sizeDot` | `6` | 状态小圆点 |
| `fontCaption` | `11` | 辅助：计数、分组名、提示 |
| `fontDense` | `12` | 密集信息：侧栏、标签、表头、表格 |
| `fontCtl` | `12.5` | 控件文字：按钮、输入框、菜单 |
| `fontBody` | `13` | 正文：对话框 |
| `fontTitle` | `15` | 区块标题 |
| `fontHeading` | `20` | 页面标题 |
| `radiusSm` | `5` | 小按钮 |
| `radiusBtn` | `6` | 控件 |
| `radiusSeg` | `7` | 分段开关的轨道 |
| `radiusRail` | `8` | 活动栏按钮 |
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
