# MiniDB 安装说明 / Installation Guide

## 系统要求 / Requirements

- macOS 12 (Monterey) 或更高版本；Apple Silicon 与 Intel 均可（发布包是 universal）
- 磁盘空间 ≥ 100 MB

macOS 12 or later, Apple Silicon or Intel (the release is a universal build). At least 100 MB of disk space.

## 安装 / Install

1. 打开下载的 `.dmg`，把 **MiniDB** 拖进 **Applications**。
2. 在 Launchpad 或 Applications 里打开它。

Open the `.dmg`, drag **MiniDB** into **Applications**, then launch it.

## 首次打开 / First launch

应用使用 ad-hoc 签名，没有经过 Apple 公证，所以首次打开 macOS 可能拦截。任选一种方式放行：

The app is signed ad hoc and not notarized, so macOS may block the first launch. Use any of these:

- **右键打开（推荐）/ Right-click → Open**：在 Finder 里右键（或 Control + 点击）MiniDB，选「打开」，再在对话框里点「打开」。
- **系统设置 / System Settings**：「隐私与安全性」里找到被拦截的提示，点「仍要打开」。
- **终端 / Terminal**：

  ```bash
  sudo xattr -rd com.apple.quarantine /Applications/MiniDB.app
  ```

## 自动更新 / Auto update

在「设置 → 关于」里点「检查更新」。发现新版本后应用会下载更新包，用发布时内置的公钥校验 Ed25519 签名，通过后点「重启安装」即可。更新需要能访问 GitHub Releases；失败时可以在发布页手动下载最新 `.dmg` 覆盖安装。

Open Settings → About and click "Check for updates". A new version is downloaded and its Ed25519 signature is verified against the public key built into the app; then click "Restart to install". Updates need access to GitHub Releases; if they fail, download the latest `.dmg` from the release page and install over the old app.

## 数据位置 / Where data lives

| 内容 / What | 位置 / Where |
| --- | --- |
| 连接、历史、收藏、AI 会话 / Connections, history, favorites, AI chats | `~/Library/Application Support/MiniDB/minidb.sqlite` |
| 日志 / Logs | `~/Library/Application Support/MiniDB/logs/` |
| 本地加密密钥 / Local encryption key | 登录钥匙串；没有钥匙串时是数据目录里的 `secret.key` |

数据库密码、AI 密钥和 SSH 口令都用 AES-GCM 加密后才落库。首次启动时，旧版（Wails 版）留在 `~/.minidb` 的数据会被自动导入，旧目录原样保留。

Database passwords, AI keys and SSH secrets are encrypted with AES-GCM before they are stored. On first launch, data of the old Wails-based version in `~/.minidb` is imported automatically and left untouched.

## 卸载 / Uninstall

```bash
rm -rf /Applications/MiniDB.app                      # 应用 / the app
rm -rf "$HOME/Library/Application Support/MiniDB"    # 用户数据（可选）/ user data (optional)
```

## 快捷键 / Shortcuts

常用的默认快捷键如下，全部可以在「设置 → 快捷键」里改。

The common defaults below can all be changed in Settings → Shortcuts.

| 快捷键 / Shortcut | 功能 / Action |
| --- | --- |
| ⌘N / ⌘T / ⌘W | 新建连接 / 新建查询 / 关闭标签页 — New connection / new query / close tab |
| ⌘P / ⌘K / ⌘I | 命令面板 / 切换数据库 / AI 助手 — Command palette / switch database / AI assistant |
| ⌘↵ / ⇧⌘↵ | 执行当前语句或选中内容 / 全部执行 — Run statement or selection / run all |
| ⇧⌘F | 格式化 SQL（编辑器内）— Format SQL (editor) |
| ⌘J / ⌘D | 行内 AI / 收藏查询（编辑器内）— Inline AI / favorite (editor) |
| ⌘S / ⇧⌘P / ⇧⌘⌫ | 提交 / 预览 SQL / 丢弃修改 — Commit / preview SQL / discard |
| ⌘R | 刷新 — Refresh |
| ⌘, | 设置 — Settings |
| Space | 预览选中行 — Preview the chosen row |

## 常见问题 / FAQ

**应用无法启动？/ The app will not start?**
按上面的步骤放行；仍然不行就看 `~/Library/Application Support/MiniDB/logs/` 里的日志。 Allow it as described above; if it still fails, check the logs.

**连不上数据库？/ Cannot connect?**
确认服务已启动、参数正确、网络可达；走 SSH 隧道的连接再检查隧道配置。 Check that the server is up, the settings are right and the network is reachable; for SSH tunnels check the tunnel settings too.

**怎么重置所有设置？/ How do I reset everything?**
退出应用，删掉 `~/Library/Application Support/MiniDB/minidb.sqlite` 后重新打开。 Quit the app, delete that file, and start it again.

---

© 2026 MiniDB · MIT License
