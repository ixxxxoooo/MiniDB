#!/bin/sh
# 重新生成 README 用的配图：先用无窗口界面测试出原始截图，再给每张加上背景，写到 docs/screenshots/。
# 改了界面、想更新 README 里的图时运行：scripts/readme-shots.sh
# 要加新图：在 internal/ui/shots_readme_test.go 里加一个场景，场景名就是输出的文件名。
set -eu
cd "$(dirname "$0")/.."

RAW=$(mktemp -d)
trap 'rm -rf "$RAW"' EXIT

MINIDB_SHOTS="$RAW" go test ./internal/ui/ -run TestReadmeShots -count=1
go run ./scripts/mockup "$RAW" docs/screenshots
