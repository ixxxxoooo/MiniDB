# MiniDB 界面设计稿 v2

一套**可交互的静态设计稿**（纯 HTML / CSS / JS，无构建、无依赖），用来在改 Go 界面代码之前先选定方向。

## 打开方式

```bash
cd docs/design/v2
python3 -m http.server 8777     # 然后访问 http://localhost:8777/index.html
```

（直接双击 `index.html` 也能看大部分内容，但浏览器可能因 `file://` 限制拦截部分资源。）

## 里面有什么

| 区域 | 内容 |
| --- | --- |
| 顶部控制条 | 三套方案（A 精炼工作台 / B 轨道 / C 阿特拉斯）× 四个主题（浅色 / 深色 / 纸墨 / 极光）× 六种强调色，实时切换 |
| 应用页面 | 欢迎页（三种形态）、项目概览、数据、表结构、查询、执行计划、结果图表、ER 图、监控、导出、导入、AI 助手、行预览 |
| 设计系统 | 现状审计、三套方案对比、令牌、Logo、动效实验室、场景与状态、做选择 |
| 弹层 | 命令面板（⌘K）、右键菜单、连接 / 库切换、新建连接、设置（带即时预览）、危险确认、SQL 预览、通知、提示卡 |

## 可以试的交互

- `1` `2` `3` 切换方案，`T` 切换主题，`⌘K` 命令面板，`⌘,` 设置，`⌘I` AI，`⌘B` 收起侧栏。
- 双击数据单元格编辑，右下角会出现「待提交」条；右键行 / 表 / 连接卡片有菜单。
- ER 图里的表可以拖动；查询页的分割条可以拖。
- 「动效实验室」里的滑块会改写整个设计稿的动效速度。

## 目录

```
index.html            入口
styles/               tokens.css（令牌）· ui.css（组件）· shells.css（三套外壳）· pages.css（页面）
scripts/              icons / data / logo / shell / pages-* / overlays / main
assets/orig, cur      原版与上一轮重设计的截图（现状审计用）
assets/preview        三套方案的真实渲染预览
logo/                 导出的独立 SVG（固定品牌色）；node logo/export.js 重新生成
```

选定方案后，令牌对应关系见 `styles/tokens.css`，落地时要和 `docs/UI_TOKENS.md`、`internal/ui/theme_*.go` 一起改。
