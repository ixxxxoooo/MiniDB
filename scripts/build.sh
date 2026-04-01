#!/usr/bin/env bash
# ============================================================
#  TablePlus AI — macOS 构建 & DMG 打包脚本
#  用法:
#    ./scripts/build.sh              # 构建 arm64 + 打包 DMG
#    ./scripts/build.sh --arch amd64 # 构建 amd64
#    ./scripts/build.sh --skip-dmg   # 仅构建 .app，不打包 DMG
#    ./scripts/build.sh --universal  # 构建 universal binary
# ============================================================
set -euo pipefail

# ============ 配置区 ============
APP_NAME="TablePlus AI"
APP_BUNDLE="tableplus-ai"
VERSION="${VERSION:-1.0.0}"
ARCH="${ARCH:-arm64}"
SKIP_DMG=false
UNIVERSAL=false

# 目录
SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
DIST_DIR="$PROJECT_ROOT/dist"
DMG_STAGING="$DIST_DIR/dmg-staging"

# Go 环境（兼容 homebrew go@1.23）
if [ -d "/opt/homebrew/opt/go@1.23/bin" ]; then
    export PATH="/opt/homebrew/opt/go@1.23/bin:$PATH"
    export GOROOT="/opt/homebrew/opt/go@1.23/libexec"
fi

# ============ 参数解析 ============
while [[ $# -gt 0 ]]; do
    case "$1" in
        --arch)    ARCH="$2"; shift 2 ;;
        --version) VERSION="$2"; shift 2 ;;
        --skip-dmg) SKIP_DMG=true; shift ;;
        --universal) UNIVERSAL=true; shift ;;
        -h|--help)
            echo "用法: $0 [选项]"
            echo "  --arch <arm64|amd64>   目标架构 (默认: arm64)"
            echo "  --version <x.y.z>      版本号 (默认: 1.0.0)"
            echo "  --skip-dmg             仅构建 .app，不打包 DMG"
            echo "  --universal            构建 universal binary (arm64 + amd64)"
            echo "  -h, --help             显示帮助"
            exit 0
            ;;
        *) echo "未知参数: $1"; exit 1 ;;
    esac
done

# ============ 工具检查 ============
check_tool() {
    if ! command -v "$1" &>/dev/null; then
        echo "❌ 缺少工具: $1"
        echo "   $2"
        exit 1
    fi
}

echo "🔍 检查构建环境..."
check_tool go       "请安装 Go 1.23+: brew install go@1.23"
check_tool node     "请安装 Node.js 18+: brew install node"
check_tool npm      "请安装 npm (随 Node.js 一起安装)"
check_tool wails    "请安装 Wails CLI: go install github.com/wailsapp/wails/v2/cmd/wails@latest"
check_tool hdiutil  "hdiutil 是 macOS 内置工具，请在 macOS 上运行"

echo "  Go:    $(go version)"
echo "  Node:  $(node --version)"
echo "  Wails: $(wails version 2>/dev/null | head -1 || echo 'installed')"
echo "  架构:  $ARCH"
echo "  版本:  $VERSION"
echo ""

# ============ 清理 ============
echo "🧹 清理旧构建产物..."
rm -rf "$DIST_DIR"
mkdir -p "$DIST_DIR"

# ============ 构建 ============
build_for_arch() {
    local target_arch="$1"
    echo "🔨 正在为 $target_arch 架构构建..."

    cd "$PROJECT_ROOT"
    
    # 设置交叉编译环境变量
    local goarch="$target_arch"
    if [ "$target_arch" = "amd64" ]; then
        goarch="amd64"
    elif [ "$target_arch" = "arm64" ]; then
        goarch="arm64"
    fi

    CGO_ENABLED=1 GOOS=darwin GOARCH="$goarch" \
        wails build -clean -platform "darwin/$goarch"

    echo "✅ $target_arch 架构构建完成"
}

if [ "$UNIVERSAL" = true ]; then
    # 构建 universal binary
    echo "🔨 构建 Universal Binary (arm64 + amd64)..."
    
    # 先构建 arm64
    build_for_arch "arm64"
    cp -R "$PROJECT_ROOT/build/bin/${APP_BUNDLE}.app" "$DIST_DIR/${APP_BUNDLE}-arm64.app"
    
    # 再构建 amd64
    build_for_arch "amd64"
    cp -R "$PROJECT_ROOT/build/bin/${APP_BUNDLE}.app" "$DIST_DIR/${APP_BUNDLE}-amd64.app"
    
    # 合并为 universal binary
    echo "🔗 合并为 Universal Binary..."
    cp -R "$DIST_DIR/${APP_BUNDLE}-arm64.app" "$DIST_DIR/${APP_BUNDLE}.app"
    lipo -create \
        "$DIST_DIR/${APP_BUNDLE}-arm64.app/Contents/MacOS/${APP_BUNDLE}" \
        "$DIST_DIR/${APP_BUNDLE}-amd64.app/Contents/MacOS/${APP_BUNDLE}" \
        -output "$DIST_DIR/${APP_BUNDLE}.app/Contents/MacOS/${APP_BUNDLE}"
    
    # 清理临时文件
    rm -rf "$DIST_DIR/${APP_BUNDLE}-arm64.app" "$DIST_DIR/${APP_BUNDLE}-amd64.app"
    ARCH="universal"
else
    build_for_arch "$ARCH"
    cp -R "$PROJECT_ROOT/build/bin/${APP_BUNDLE}.app" "$DIST_DIR/${APP_BUNDLE}.app"
fi

echo "✅ 应用构建完成: $DIST_DIR/${APP_BUNDLE}.app"

# ============ 签名提示 ============
echo ""
echo "💡 代码签名提示:"
echo "   当前构建未签名。如需分发，请执行:"
echo "   codesign --force --deep --sign \"Developer ID Application: YOUR_NAME (TEAM_ID)\" \\"
echo "       \"$DIST_DIR/${APP_BUNDLE}.app\""
echo ""

# ============ 打包 DMG ============
if [ "$SKIP_DMG" = true ]; then
    echo "⏭️  跳过 DMG 打包 (--skip-dmg)"
    echo ""
    echo "📦 构建产物:"
    echo "   $DIST_DIR/${APP_BUNDLE}.app"
    exit 0
fi

DMG_NAME="${APP_NAME}-${VERSION}-macOS-${ARCH}.dmg"
DMG_PATH="$DIST_DIR/$DMG_NAME"

echo "📦 正在打包 DMG: $DMG_NAME ..."

# 创建 DMG 临时目录
rm -rf "$DMG_STAGING"
mkdir -p "$DMG_STAGING/.background"

# 复制 .app
cp -R "$DIST_DIR/${APP_BUNDLE}.app" "$DMG_STAGING/${APP_NAME}.app"

# 创建 Applications 快捷方式
ln -sf /Applications "$DMG_STAGING/Applications"

# 复制安装说明
if [ -f "$PROJECT_ROOT/docs/INSTALL.md" ]; then
    cp "$PROJECT_ROOT/docs/INSTALL.md" "$DMG_STAGING/安装说明.md"
fi

# 复制许可证
if [ -f "$PROJECT_ROOT/LICENSE" ]; then
    cp "$PROJECT_ROOT/LICENSE" "$DMG_STAGING/LICENSE"
fi

# 生成 DMG 背景图
echo "🎨 生成 DMG 背景图..."
python3 "$SCRIPT_DIR/create_dmg_background.py" "$DMG_STAGING/.background/background.png"

# 创建临时 DMG
TEMP_DMG="$DIST_DIR/temp.dmg"
rm -f "$TEMP_DMG" "$DMG_PATH"

echo "   创建临时磁盘映像..."
hdiutil create \
    -srcfolder "$DMG_STAGING" \
    -volname "$APP_NAME" \
    -fs HFS+ \
    -fsargs "-c c=64,a=16,e=16" \
    -format UDRW \
    -size 300m \
    "$TEMP_DMG"

# 挂载并配置 DMG 外观
echo "   配置 DMG 外观..."
MOUNT_DIR=$(hdiutil attach -readwrite -noverify -noautoopen "$TEMP_DMG" | grep "/Volumes/" | sed 's/.*\/Volumes\//\/Volumes\//')

# 使用 AppleScript 配置 Finder 窗口
osascript <<APPLESCRIPT
tell application "Finder"
    tell disk "$APP_NAME"
        open
        set current view of container window to icon view
        set toolbar visible of container window to false
        set statusbar visible of container window to false
        set bounds of container window to {200, 120, 820, 520}
        set theViewOptions to the icon view options of container window
        set arrangement of theViewOptions to not arranged
        set icon size of theViewOptions to 80
        set background picture of theViewOptions to file ".background:background.png"
        -- 设置图标位置
        set position of item "${APP_NAME}.app" of container window to {155, 200}
        set position of item "Applications" of container window to {465, 200}
        -- 隐藏其他文件
        try
            set position of item ".background" of container window to {900, 900}
        end try
        try
            set position of item "安装说明.md" of container window to {310, 340}
        end try
        try
            set position of item "LICENSE" of container window to {310, 340}
        end try
        close
        open
        update without registering applications
        delay 2
        close
    end tell
end tell
APPLESCRIPT

# 设置权限
chmod -Rf go-w "$MOUNT_DIR" 2>/dev/null || true
sync
sync

# 卸载
hdiutil detach "$MOUNT_DIR" -quiet -force 2>/dev/null || true

# 压缩为最终 DMG
echo "   压缩生成最终 DMG..."
hdiutil convert "$TEMP_DMG" \
    -format UDZO \
    -imagekey zlib-level=9 \
    -o "$DMG_PATH"

rm -f "$TEMP_DMG"
rm -rf "$DMG_STAGING"

# 计算文件信息
DMG_SIZE=$(du -h "$DMG_PATH" | cut -f1)
DMG_SHA256=$(shasum -a 256 "$DMG_PATH" | cut -d' ' -f1)

echo ""
echo "============================================================"
echo "✅ 构建完成！"
echo "============================================================"
echo ""
echo "📦 DMG 文件:  $DMG_PATH"
echo "   大小:      $DMG_SIZE"
echo "   SHA-256:   $DMG_SHA256"
echo ""
echo "🚀 安装方式:"
echo "   1. 双击打开 .dmg 文件"
echo "   2. 将 '${APP_NAME}' 拖入 Applications 文件夹"
echo "   3. 首次打开需要：右键点击应用 → 打开 → 确认打开"
echo ""
echo "📋 如果出现「无法打开」提示，请执行:"
echo "   sudo xattr -rd com.apple.quarantine /Applications/${APP_NAME}.app"
echo ""
