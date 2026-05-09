# MiniDB — 安装说明 / Installation Guide

---

## 系统要求 / System Requirements

- **操作系统**: macOS 10.15 (Catalina) 或更高版本；Windows 10/11 amd64
- **架构**: Apple Silicon (M1/M2/M3/M4) 或 Intel x86_64
- **磁盘空间**: ≥ 100 MB

## 安装步骤 / Installation Steps

### macOS：从 DMG 安装（推荐）

1. 双击打开 `.dmg` 文件
2. 将 **MiniDB** 图标拖入 **Applications** 文件夹
3. 弹出磁盘映像（右键 → 推出）
4. 从 Launchpad 或 Applications 文件夹打开应用

### macOS: Install from DMG (Recommended)

1. Double-click the `.dmg` file to open
2. Drag **MiniDB** icon to **Applications** folder
3. Eject the disk image (right-click → Eject)
4. Launch from Launchpad or Applications folder

### Windows：从安装包安装

1. 下载 `MiniDB-<version>-Windows-amd64-Setup.exe`
2. 双击运行安装包
3. 按安装向导完成安装
4. 从开始菜单或桌面快捷方式启动应用

### Windows: Install from Setup

1. Download `MiniDB-<version>-Windows-amd64-Setup.exe`
2. Double-click the installer
3. Follow the setup wizard
4. Launch from Start Menu or desktop shortcut

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
双击 DMG 中附带的 **`首次打开授权.command`**，或手动执行：

```bash
# 移除隔离属性
sudo xattr -rd com.apple.quarantine /Applications/MiniDB.app
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
Double-click **`首次打开授权.command`** included in the DMG, or run:

```bash
sudo xattr -rd com.apple.quarantine /Applications/MiniDB.app
```

**Solution C (System Settings):**
1. Open System Settings → Privacy & Security
2. Find the blocked app notice
3. Click "Open Anyway"

---

## 自动更新 / Auto Update

应用会在「设置 → 关于」中提供更新入口：

1. 点击「检查更新」
2. 如发现新版本，应用会下载当前平台对应的更新包
3. 下载完成后会校验 SHA-256
4. 校验通过后点击「重启安装」完成更新

The app provides update controls in Settings → About:

1. Click "Check for Updates"
2. If a new version is available, the app downloads the matching package for your platform
3. The downloaded package is verified with SHA-256
4. Click "Restart to Install" when the update is ready

自动更新需要能访问 GitHub Releases。若更新失败，可在关于页打开发布页，手动下载最新 DMG 或 Windows 安装包覆盖安装。

Auto update requires access to GitHub Releases. If it fails, open the release page from About and manually install the latest DMG or Windows installer.

---

## 数据存储位置 / Data Storage

| 内容 | 路径 |
|------|------|
| 连接配置与数据 | `~/.minidb/data.db` |
| 运行日志 | `~/.minidb/logs/` |

Windows 版本使用系统用户目录下的应用数据目录保存同类数据；应用内日志页可直接查看运行日志。

## 卸载 / Uninstall

```bash
# 1. 删除应用
rm -rf /Applications/MiniDB.app

# 2. 删除用户数据（可选）
rm -rf ~/.minidb
```

---

## 快捷键速查 / Keyboard Shortcuts

| 快捷键 | 功能 |
|--------|------|
| ⌘K | 全局搜索 |
| ⌘T | 新建查询标签页 |
| ⌘W | 关闭当前标签页 |
| ⌘, | 打开设置 |
| ⌘↵ | 执行当前 SQL 语句或选中 SQL |
| ⌘⇧↵ | 执行所有 SQL 语句 |
| ⌘⇧F | 格式化 SQL |
| ⌘S | 保存 SQL |
| Space | 预览选中行 |
| ESC | 关闭弹窗 |

---

## 常见问题 / FAQ

**Q: 应用无法启动？**
A: 请确保执行了上述安全授权步骤。如仍无法启动，请检查 `~/.minidb/logs/` 中的日志。

**Q: 连接数据库失败？**
A: 请确认数据库服务已启动，连接参数正确，且网络可达。

**Q: 如何重置所有设置？**
A: 删除 `~/.minidb/data.db` 文件后重新启动应用。

**Q: 自动更新没有生效？**
A: 请确认网络可以访问 GitHub Releases，并检查「关于」页显示的错误信息。也可以从发布页手动下载最新安装包覆盖安装。

---

© 2026 MiniDB. MIT License.
