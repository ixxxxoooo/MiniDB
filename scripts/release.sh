#!/bin/sh
# 本地发版：测试 → 本地构建并签名 → 上传到 GitHub 草稿 release → 打 tag 并推送 → 发布。
# 不走线上 CI 构建：安装包、更新包、增量包和 update-*.json 都在本机生成并直接上传，发布页只存放结果。
#
# 用法：先改 mygo.json 的 version、在 CHANGELOG.md 加对应的「## x.y.z」小节，提交后运行  scripts/release.sh
#   SKIP_TESTS=1      跳过 go vet / go test（只在刚跑过时用）
#   DRAFT_ONLY=1      只上传草稿，不打 tag、不发布（先去页面检查再手动发布）
# 更新签名私钥取自 MYGO_UPDATER_PRIVATE_KEY，缺省读 ~/Library/Application Support/mygo/update-keys/mygo-update.key。
set -eu
cd "$(dirname "$0")/.."

VERSION=$(go run ./scripts/version)
TAG="v$VERSION"
REPO=$(python3 -c 'import json; print(json.load(open("mygo.json"))["updates"]["github"])')

die() { echo "错误：$*" >&2; exit 1; }

[ -z "$(git status --porcelain)" ] || die "工作区有未提交的改动，请先提交"
grep -q "^## $VERSION\$" CHANGELOG.md || die "CHANGELOG.md 里没有「## $VERSION」小节（更新窗口和 release 说明都取自这里）"
git rev-parse "$TAG" >/dev/null 2>&1 && die "tag $TAG 已存在，请先在 mygo.json 里提升版本号"
command -v gh >/dev/null || die "需要 GitHub CLI（gh）并已登录"
gh auth status >/dev/null 2>&1 || die "gh 未登录，先运行 gh auth login"
gh release view "$TAG" --repo "$REPO" >/dev/null 2>&1 && die "GitHub 上已经有 $TAG 的 release"

if [ -z "${MYGO_UPDATER_PRIVATE_KEY:-}" ]; then
  KEY="$HOME/Library/Application Support/mygo/update-keys/mygo-update.key"
  [ -f "$KEY" ] || die "找不到更新签名私钥：设置 MYGO_UPDATER_PRIVATE_KEY，或把 mygo-update.key 放到 $KEY"
  MYGO_UPDATER_PRIVATE_KEY=$(cat "$KEY")
fi
export MYGO_UPDATER_PRIVATE_KEY

if [ -z "${SKIP_TESTS:-}" ]; then
  go vet ./... && go tool mygo vet .
  CGO_ENABLED=0 go test ./...
fi

echo "==> 本地构建并上传 $TAG（草稿）"
rm -rf dist
go tool mygo build -platform darwin/universal -upload

if [ -n "${DRAFT_ONLY:-}" ]; then
  echo "已上传草稿：https://github.com/$REPO/releases 。检查后运行：git tag $TAG && git push origin $TAG && gh release edit $TAG --repo $REPO --draft=false --latest"
  exit 0
fi

echo "==> 推送代码与 tag"
git push origin HEAD
git tag -a "$TAG" -m "MiniDB $VERSION"
git push origin "$TAG"

echo "==> 发布"
gh release edit "$TAG" --repo "$REPO" --draft=false --latest
echo "已发布：https://github.com/$REPO/releases/tag/$TAG"
