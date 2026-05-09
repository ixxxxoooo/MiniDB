# Build Directory

`build/` 保存 Wails v3 构建配置、平台资源和打包任务。应用业务代码不放在这里；这里的文件主要服务于本地构建、CI 发布和平台安装包。

前端依赖统一使用 pnpm 和 `frontend/pnpm-lock.yaml`。Wails 构建任务、GitHub CI 和 release workflow 都按 pnpm 设计，不混用 npm / Yarn。

## 目录结构

- `config.yml`：Wails v3 主构建配置，包含应用信息、前端构建命令、绑定输出目录和开发命令。
- `appicon.png`：应用主图标，macOS 和 Windows 构建都会从这里派生平台图标。
- `darwin/`：macOS 专用文件，包括 `Info.plist`、`Info.dev.plist`、DMG 背景、授权脚本和 `Taskfile.yml`。
- `windows/`：Windows 专用文件，包括 `info.json`、`wails.exe.manifest`、NSIS 安装包模板和 `Taskfile.yml`。

## 常用构建命令

构建变量默认读取仓库根目录的 `project.env` 文件。需要调整版本时运行：

```bash
./scripts/set-version.sh 0.0.1
```

这个脚本会同步平台 metadata、前端 package 和 Go 运行时常量，避免各平台产物版本不一致。

```bash
# 开发模式
wails3 dev -config ./build/config.yml

# 标准生产构建
wails3 build

# 生成 Wails v3 TypeScript 绑定
wails3 generate bindings -clean=true -ts

# macOS 本地打包，默认读取 project.env
wails3 task package:darwin ARCH=arm64

# Windows 打包（需要 CGO/MinGW 环境），默认读取 project.env
wails3 task package:windows ARCH=amd64
```

也可以使用仓库根目录的脚本：

```bash
./scripts/build.sh --arch arm64
```

## Release 与自动更新资产

GitHub Release 由 `.github/workflows/release.yml` 生成，除了给用户手动下载的 DMG / Windows installer，还会生成应用内自动更新使用的压缩包和 manifest：

- macOS DMG：`MiniDB-<version>-macOS-<arch>.dmg`
- Windows 安装包：`MiniDB-<version>-Windows-amd64-Setup.exe`
- macOS 更新包：`minidb-<version>-macos-<arch>.tar.gz`，内部包含 `MiniDB.app`
- Windows 更新包：`minidb-<version>-windows-amd64.zip`，内部包含更新后的 `.exe`
- 更新 manifest：`update.json`
- 校验文件：`SHA256SUMS.txt`

`update.json` 由 `scripts/release/main.go` 生成。运行中的应用会从 GitHub Releases latest 下载这个文件，根据当前平台选择更新资产，下载后校验 SHA-256，再提示用户重启安装。

## 平台注意事项

- macOS 当前最低版本与构建配置保持一致：`10.15`。
- Windows 构建依赖 `github.com/mattn/go-sqlite3`，必须具备 CGO 编译环境。推荐在原生 Windows + MSYS2/MinGW 或 CI/Docker 交叉编译环境中验证。
- 修改 Go 服务公开方法后，需要重新运行 `wails3 generate bindings -clean=true -ts` 并检查 `frontend/bindings/`。
