# TablePlus AI — 安装说明 / Installation Guide

---

## 系统要求 / System Requirements

- **操作系统**: macOS 10.13 (High Sierra) 或更高版本
- **架构**: Apple Silicon (M1/M2/M3/M4) 或 Intel x86_64
- **磁盘空间**: ≥ 100 MB

## 安装步骤 / Installation Steps

### 方法一：从 DMG 安装（推荐）

1. 双击打开 `.dmg` 文件
2. 将 **TablePlus AI** 图标拖入 **Applications** 文件夹
3. 弹出磁盘映像（右键 → 推出）
4. 从 Launchpad 或 Applications 文件夹打开应用

### Method 1: Install from DMG (Recommended)

1. Double-click the `.dmg` file to open
2. Drag **TablePlus AI** icon to **Applications** folder
3. Eject the disk image (right-click → Eject)
4. Launch from Launchpad or Applications folder

---

## 首次打开 / First Launch

### macOS 安全提示

由于应用未经过 Apple 公证（notarization），首次打开时 macOS 可能会阻止运行。

**解决方法 A（推荐）：**
1. 在 Finder 中找到应用
2. **右键点击**（或 Control + 点击）应用图标
3. 选择「**打开**」
4. 在弹出的对话框中点击「**打开**」

**解决方法 B（终端命令）：**
```bash
# 移除隔离属性
sudo xattr -rd com.apple.quarantine /Applications/TablePlus\ AI.app
```

**解决方法 C（系统设置）：**
1. 打开「系统设置」→「隐私与安全性」
2. 找到被阻止的应用提示
3. 点击「仍要打开」

### macOS Security Notice

Since the app is not notarized by Apple, macOS may block it on first launch.

**Solution A (Recommended):**
1. Find the app in Finder
2. **Right-click** (or Control + click) the app icon
3. Select "**Open**"
4. Click "**Open**" in the confirmation dialog

**Solution B (Terminal):**
```bash
sudo xattr -rd com.apple.quarantine /Applications/TablePlus\ AI.app
```

**Solution C (System Settings):**
1. Open System Settings → Privacy & Security
2. Find the blocked app notice
3. Click "Open Anyway"

---

## 数据存储位置 / Data Storage

| 内容 | 路径 |
|------|------|
| 连接配置与数据 | `~/.tableplus-ai/data.db` |
| 运行日志 | `~/.tableplus-ai/logs/` |

## 卸载 / Uninstall

```bash
# 1. 删除应用
rm -rf /Applications/TablePlus\ AI.app

# 2. 删除用户数据（可选）
rm -rf ~/.tableplus-ai
```

---

## 快捷键速查 / Keyboard Shortcuts

| 快捷键 | 功能 |
|--------|------|
| ⌘P | 全局搜索 |
| ⌘K | 切换数据库 |
| ⌘N | 新建连接 |
| ⌘T | 新建查询 |
| ⌘W | 关闭标签页 |
| ⌘, | 打开设置 |
| ⌘↵ | 执行当前 SQL |
| ⌘⇧↵ | 执行所有 SQL |
| ⌃⌘[ / ⌃⌘] | 切换子视图 |
| Space | 预览选中行 |
| ESC | 关闭弹窗 |

---

## 常见问题 / FAQ

**Q: 应用无法启动？**
A: 请确保执行了上述安全授权步骤。如仍无法启动，请检查 `~/.tableplus-ai/logs/` 中的日志。

**Q: 连接数据库失败？**
A: 请确认数据库服务已启动，连接参数正确，且网络可达。

**Q: 如何重置所有设置？**
A: 删除 `~/.tableplus-ai/data.db` 文件后重新启动应用。

---

© 2026 TablePlus AI. MIT License.
