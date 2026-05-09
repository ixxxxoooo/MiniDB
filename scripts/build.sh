#!/usr/bin/env bash
# ============================================================
#  MiniDB — macOS 构建 & DMG 打包脚本
#  用法:
#    ./scripts/build.sh              # 构建 arm64 + 打包 DMG
#    ./scripts/build.sh --arch amd64 # 构建 amd64
#    ./scripts/build.sh --universal  # 构建 universal binary
#    ./scripts/build.sh --windows    # 构建 Windows amd64 exe（需要 CGO 交叉编译环境）
# ============================================================
set -euo pipefail

# 目录
SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
DIST_DIR="$PROJECT_ROOT/dist"

set -a
# shellcheck source=/dev/null
. "$PROJECT_ROOT/project.env"
set +a

default_version() {
    if [ -n "${VERSION:-}" ]; then
        printf "%s" "${VERSION#v}"
    elif [ -n "${APP_VERSION:-}" ]; then
        printf "%s" "${APP_VERSION#v}"
    else
        printf "0.0.1"
    fi
}

retry_command() {
    local attempts="$1"
    local delay="$2"
    shift 2

    local try=1
    while true; do
        if "$@"; then
            return 0
        fi
        if [ "$try" -ge "$attempts" ]; then
            return 1
        fi
        sleep "$delay"
        try=$((try + 1))
    done
}

wait_for_path_release() {
    local path="$1"
    local attempts="${2:-12}"
    local delay="${3:-5}"

    if ! command -v lsof &>/dev/null; then
        sleep "$delay"
        return 0
    fi

    local try=1
    while true; do
        if ! lsof "$path" >/dev/null 2>&1; then
            return 0
        fi
        if [ "$try" -ge "$attempts" ]; then
            echo "⚠️  $path 仍被系统占用，继续尝试后续步骤"
            return 0
        fi
        sleep "$delay"
        try=$((try + 1))
    done
}

convert_dmg_with_retry() {
    local source_dmg="$1"
    local output_dmg="$2"
    local attempts="${3:-8}"
    local delay="${4:-8}"

    local try=1
    while true; do
        rm -f "$output_dmg" "$output_dmg.dmg"
        if hdiutil convert "$source_dmg" \
            -format UDZO \
            -imagekey zlib-level=9 \
            -o "$output_dmg"; then
            return 0
        fi

        if [ "$try" -ge "$attempts" ]; then
            return 1
        fi

        echo "⚠️  hdiutil convert 失败，等待系统释放 DMG 后重试 ($try/$attempts)..."
        wait_for_path_release "$source_dmg" 6 "$delay"
        sleep "$delay"
        try=$((try + 1))
    done
}

detach_mount_if_present() {
    local mount_dir="$1"
    if [ -z "$mount_dir" ] || [ ! -d "$mount_dir" ]; then
        return 0
    fi

    local device_id
    device_id=$(hdiutil info | awk -v mount="$mount_dir" '
        $1 ~ /^\/dev\// { device=$1 }
        $0 == mount { print device; exit }
    ')
    if [ -n "$device_id" ]; then
        hdiutil detach "$device_id" -quiet -force >/dev/null 2>&1 || true
    fi
}

cleanup_dmg_workdir() {
    if [ -n "${DEVICE_ID:-}" ]; then
        hdiutil detach "$DEVICE_ID" -quiet -force >/dev/null 2>&1 || true
    fi
    if [ -n "${MOUNT_DIR:-}" ]; then
        detach_mount_if_present "$MOUNT_DIR"
    fi
    if [ -n "${TEMP_DMG:-}" ]; then
        rm -f "$TEMP_DMG" "$TEMP_DMG.dmg"
    fi
    if [ -n "${DMG_STAGING:-}" ]; then
        rm -rf "$DMG_STAGING"
    fi
}

trap cleanup_dmg_workdir EXIT

# ============ 配置区 ============
APP_NAME="$APP_DISPLAY_NAME"
APP_BUNDLE="$APP_BINARY_NAME"
VERSION="$(default_version)"
ARCH="${ARCH:-arm64}"
ARCH_PROVIDED=false
UNIVERSAL=false
WINDOWS=false
AUTH_SCRIPT_NAME="首次打开授权.command"
DEVICE_ID=""
MOUNT_DIR=""
TEMP_DMG=""
DMG_STAGING=""

# Go 环境（兼容 homebrew go@1.23）
if [ -d "/opt/homebrew/opt/go@1.23/bin" ]; then
    export PATH="/opt/homebrew/opt/go@1.23/bin:$PATH"
    export GOROOT="/opt/homebrew/opt/go@1.23/libexec"
fi

# ============ 参数解析 ============
while [[ $# -gt 0 ]]; do
    case "$1" in
        --arch)    ARCH="$2"; ARCH_PROVIDED=true; shift 2 ;;
        --version) VERSION="${2#v}"; shift 2 ;;
        --universal) UNIVERSAL=true; shift ;;
        --windows)
            WINDOWS=true
            if [ "$ARCH_PROVIDED" = false ]; then
                ARCH="amd64"
            fi
            shift
            ;;
        -h|--help)
            echo "用法: $0 [选项]"
            echo "  --arch <arm64|amd64>   目标架构 (默认: arm64)"
            echo "  --version <x.y.z>      版本号 (默认: project.env 的 APP_VERSION)"
            echo "  --universal            构建 universal binary (arm64 + amd64)"
            echo "  --windows              构建 Windows amd64 exe（需要 CGO 交叉编译环境）"
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
check_tool pnpm     "请安装 pnpm: corepack enable pnpm"
check_tool wails3   "请安装 Wails v3 CLI: go install github.com/wailsapp/wails/v3/cmd/wails3@${WAILS_VERSION}"
if [ "$WINDOWS" = false ]; then
    check_tool hdiutil  "hdiutil 是 macOS 内置工具，请在 macOS 上运行"
fi

echo "  Go:    $(go version)"
echo "  Node:  $(node --version)"
echo "  pnpm:  $(pnpm --version)"
echo "  Wails: $(wails3 version 2>/dev/null | head -1 || echo 'installed')"
echo "  架构:  $ARCH"
echo "  版本:  $VERSION"
echo ""

# ============ 清理 ============
echo "🧹 清理旧构建产物..."
rm -rf "$DIST_DIR"
mkdir -p "$DIST_DIR"
rm -rf "$PROJECT_ROOT/bin"

if [ "$WINDOWS" = true ]; then
    echo "🔨 正在构建 Windows/$ARCH..."
    cd "$PROJECT_ROOT"
    CGO_ENABLED=1 wails3 task build:windows ARCH="$ARCH" CGO_ENABLED=1 VERSION="$VERSION"
    cp "$PROJECT_ROOT/bin/${APP_BUNDLE}.exe" "$DIST_DIR/${APP_BUNDLE}-windows-${ARCH}.exe"
    echo "✅ Windows 构建完成: $DIST_DIR/${APP_BUNDLE}-windows-${ARCH}.exe"
    exit 0
fi

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

    CGO_ENABLED=1 wails3 task package:darwin ARCH="$goarch" VERSION="$VERSION"

    echo "✅ $target_arch 架构构建完成"
}

if [ "$UNIVERSAL" = true ]; then
    # 构建 universal binary
    echo "🔨 构建 Universal Binary (arm64 + amd64)..."
    
    # 先构建 arm64
    build_for_arch "arm64"
    cp -R "$PROJECT_ROOT/bin/${APP_BUNDLE}.app" "$DIST_DIR/${APP_BUNDLE}-arm64.app"
    rm -rf "$PROJECT_ROOT/bin"
    
    # 再构建 amd64
    build_for_arch "amd64"
    cp -R "$PROJECT_ROOT/bin/${APP_BUNDLE}.app" "$DIST_DIR/${APP_BUNDLE}-amd64.app"
    
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
    cp -R "$PROJECT_ROOT/bin/${APP_BUNDLE}.app" "$DIST_DIR/${APP_BUNDLE}.app"
fi

echo "✅ 应用包已生成，继续打包 DMG..."

# ============ 打包 DMG ============

DMG_NAME="${APP_NAME}-${VERSION}-macOS-${ARCH}.dmg"
DMG_PATH="$DIST_DIR/$DMG_NAME"
DMG_WORK_ID="${APP_BUNDLE}-${VERSION}-${ARCH}-$$"
DMG_STAGING="$DIST_DIR/dmg-staging-$DMG_WORK_ID"
TEMP_DMG="$DIST_DIR/temp-$DMG_WORK_ID.dmg"

echo "📦 正在打包 DMG: $DMG_NAME ..."

# 创建 DMG 临时目录
rm -rf "$DMG_STAGING"
mkdir -p "$DMG_STAGING/.background"

# 复制 .app
cp -R "$DIST_DIR/${APP_BUNDLE}.app" "$DMG_STAGING/${APP_NAME}.app"

# 创建 Applications 快捷方式
ln -sf /Applications "$DMG_STAGING/Applications"

# 生成首次打开授权脚本，方便用户双击执行
AUTH_SCRIPT_PATH="$DMG_STAGING/$AUTH_SCRIPT_NAME"
cat > "$AUTH_SCRIPT_PATH" <<EOF
#!/usr/bin/env bash
set -euo pipefail

APP_NAME="$APP_NAME"
APP_PATH="/Applications/\${APP_NAME}.app"

echo "========================================"
echo "  \${APP_NAME} - macOS 首次打开授权"
echo "========================================"
echo ""

if [ ! -d "\$APP_PATH" ]; then
    echo "未在 /Applications 找到 \${APP_NAME}.app"
    echo "请先将 DMG 中的应用拖到 Applications 文件夹，再重新运行本脚本。"
    echo ""
    read -r -p "按回车键退出..."
    exit 1
fi

echo "将执行以下命令移除隔离属性："
echo "sudo xattr -rd com.apple.quarantine \"\$APP_PATH\""
echo ""
sudo xattr -rd com.apple.quarantine "\$APP_PATH"
echo ""
echo "授权完成，现在可以直接从 Applications 打开 \${APP_NAME}。"
echo ""
read -r -p "按回车键退出..."
EOF
chmod +x "$AUTH_SCRIPT_PATH"

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
PYTHON_BIN="${PYTHON_BIN:-python3}"
if "$PYTHON_BIN" -c "import PIL" >/dev/null 2>&1; then
    "$PYTHON_BIN" "$SCRIPT_DIR/create_dmg_background.py" "$DMG_STAGING/.background/background.png"
else
    echo "⚠️  检测到 Python 缺少 Pillow（PIL），跳过自定义背景图生成"
    echo "   可执行: $PYTHON_BIN -m pip install Pillow"
    echo "   将继续使用纯色背景完成 DMG 打包"
    # 生成 1x1 白色 PNG 占位图，避免 Finder 设置背景图片时报错
    printf '%s' 'iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mP8/x8AAwMCAO5m2xkAAAAASUVORK5CYII=' | base64 --decode > "$DMG_STAGING/.background/background.png"
fi

# 创建临时 DMG
detach_mount_if_present "/Volumes/$APP_NAME"
rm -f "$TEMP_DMG" "$TEMP_DMG.dmg" "$DMG_PATH"

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
ATTACH_OUTPUT=$(hdiutil attach -readwrite -noverify -noautoopen "$TEMP_DMG")
MOUNT_DIR=$(printf '%s\n' "$ATTACH_OUTPUT" | awk '/\/Volumes\// {sub(/.*\/Volumes\//, "/Volumes/"); print; exit}')
DEVICE_ID=$(printf '%s\n' "$ATTACH_OUTPUT" | awk '/Apple_HFS/ {print $1; exit}')

if [ -z "${MOUNT_DIR:-}" ] || [ -z "${DEVICE_ID:-}" ]; then
    echo "❌ 无法获取 DMG 挂载信息"
    echo "$ATTACH_OUTPUT"
    exit 1
fi

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
        try
            set position of item "${AUTH_SCRIPT_NAME}" of container window to {220, 340}
        end try
        try
            set position of item "安装说明.md" of container window to {400, 340}
        end try
        -- 隐藏其他文件
        try
            set position of item ".background" of container window to {900, 900}
        end try
        try
            set position of item "LICENSE" of container window to {900, 940}
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
retry_command 5 2 hdiutil detach "$DEVICE_ID" -quiet -force
wait_for_path_release "$TEMP_DMG" 12 5

# 压缩为最终 DMG
echo "   压缩生成最终 DMG..."
convert_dmg_with_retry "$TEMP_DMG" "$DMG_PATH"

rm -f "$TEMP_DMG" "$TEMP_DMG.dmg"
rm -f "$DIST_DIR/.DS_Store"
rm -rf "$DMG_STAGING"
TEMP_DMG=""
DMG_STAGING=""
rm -rf "$DIST_DIR/${APP_BUNDLE}.app"

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
echo "   产物目录仅保留 DMG 文件"
echo ""
echo "🚀 安装方式:"
echo "   1. 双击打开 .dmg 文件"
echo "   2. 将 '${APP_NAME}' 拖入 Applications 文件夹"
echo "   3. 如被 macOS 拦截，可双击 DMG 中的 '${AUTH_SCRIPT_NAME}'"
echo "   4. 或首次打开时：右键点击应用 → 打开 → 确认打开"
echo ""
echo "📋 DMG 内已附带授权脚本，也可手动执行:"
echo "   sudo xattr -rd com.apple.quarantine /Applications/${APP_NAME}.app"
echo ""
