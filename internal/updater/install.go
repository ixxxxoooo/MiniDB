package updater

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"minidb/internal/appdata"
)

func spawnInstaller(archivePath string) error {
	switch runtime.GOOS {
	case "darwin":
		return spawnDarwinInstaller(archivePath)
	case "windows":
		return spawnWindowsInstaller(archivePath)
	default:
		return fmt.Errorf("unsupported updater platform: %s/%s", runtime.GOOS, runtime.GOARCH)
	}
}

// checkInstallSupported 在退出应用之前判断能否原地更新。
// 不满足时返回错误，由前端引导用户去发布页手动下载，而不是退出后静默失败。
func checkInstallSupported() error {
	exe, err := os.Executable()
	if err != nil {
		return err
	}
	exe, err = filepath.EvalSymlinks(exe)
	if err != nil {
		return err
	}
	switch runtime.GOOS {
	case "darwin":
		bundle, err := resolveMacBundlePath(exe)
		if err != nil {
			return err
		}
		// 从下载的 dmg/压缩包里直接运行时系统会把应用放到随机的只读 AppTranslocation 路径
		if strings.Contains(bundle, "/AppTranslocation/") {
			return errors.New("应用正从隔离位置运行，请先把它拖到“应用程序”文件夹后再更新")
		}
		return ensureDirWritable(filepath.Dir(bundle))
	case "windows":
		return ensureDirWritable(filepath.Dir(exe))
	default:
		return fmt.Errorf("unsupported updater platform: %s", runtime.GOOS)
	}
}

func ensureDirWritable(dir string) error {
	f, err := os.CreateTemp(dir, ".minidb-write-test-*")
	if err != nil {
		return fmt.Errorf("没有写入权限，无法原地更新（%s）: %w", dir, err)
	}
	name := f.Name()
	_ = f.Close()
	_ = os.Remove(name)
	return nil
}

func resolveMacBundlePath(executablePath string) (string, error) {
	contentsDir := filepath.Dir(filepath.Dir(executablePath))
	if filepath.Base(contentsDir) != "Contents" {
		return "", errors.New("当前不是从 .app 应用包运行，无法原地更新")
	}
	appBundlePath := filepath.Dir(contentsDir)
	if filepath.Ext(appBundlePath) != ".app" {
		return "", errors.New("无法定位 macOS 应用包路径")
	}
	return appBundlePath, nil
}

func spawnDarwinInstaller(archivePath string) error {
	executablePath, err := os.Executable()
	if err != nil {
		return err
	}
	if executablePath, err = filepath.EvalSymlinks(executablePath); err != nil {
		return err
	}
	appBundlePath, err := resolveMacBundlePath(executablePath)
	if err != nil {
		return err
	}
	scriptPath, err := writeHelperScript("minidb-update-*.sh", DarwinInstallerScript)
	if err != nil {
		return err
	}
	_ = os.MkdirAll(appdata.LogsRootPath(), 0o755)
	cmd := exec.Command("/bin/sh", scriptPath, fmt.Sprintf("%d", os.Getpid()), archivePath, appBundlePath, appdata.UpdateLogPath())
	detach(cmd)
	return cmd.Start()
}

func spawnWindowsInstaller(archivePath string) error {
	executablePath, err := os.Executable()
	if err != nil {
		return err
	}
	if executablePath, err = filepath.Abs(executablePath); err != nil {
		return err
	}
	scriptPath, err := writeHelperScript("minidb-update-*.ps1", WindowsInstallerScript)
	if err != nil {
		return err
	}
	_ = os.MkdirAll(appdata.LogsRootPath(), 0o755)
	cmd := exec.Command(
		"powershell", "-NoProfile", "-ExecutionPolicy", "Bypass", "-WindowStyle", "Hidden",
		"-File", scriptPath,
		"-PidToWait", fmt.Sprintf("%d", os.Getpid()),
		"-ArchivePath", archivePath,
		"-TargetExecutable", executablePath,
		"-LogPath", appdata.UpdateLogPath(),
	)
	detach(cmd)
	return cmd.Start()
}

func writeHelperScript(pattern, content string) (string, error) {
	file, err := os.CreateTemp("", pattern)
	if err != nil {
		return "", err
	}
	if _, err := file.WriteString(content); err != nil {
		_ = file.Close()
		_ = os.Remove(file.Name())
		return "", err
	}
	if err := file.Close(); err != nil {
		_ = os.Remove(file.Name())
		return "", err
	}
	if err := os.Chmod(file.Name(), 0o700); err != nil {
		_ = os.Remove(file.Name())
		return "", err
	}
	return file.Name(), nil
}

// DarwinInstallerScript 在应用退出后替换 .app：
// 解压到与目标同一文件系统的临时目录（保证 mv 是原子重命名）→ 旧包改名为 .bak →
// 新包就位 → 失败则还原 .bak。任何一步失败都会重新打开旧版本并写日志，而不是让应用消失。
const DarwinInstallerScript = `#!/bin/sh
PID_TO_WAIT="$1"
ARCHIVE_PATH="$2"
TARGET_APP="$3"
LOG_FILE="$4"
BACKUP_APP="${TARGET_APP}.bak"
TMP_DIR=""

log() {
  printf '%s [update] %s\n' "$(date '+%Y-%m-%d %H:%M:%S')" "$*" >> "$LOG_FILE" 2>/dev/null
}

cleanup() {
  [ -n "$TMP_DIR" ] && rm -rf "$TMP_DIR"
  rm -f "$0"
}
trap cleanup EXIT

fail() {
  log "FAILED: $*"
  open "$TARGET_APP" 2>/dev/null
  exit 1
}

log "waiting for pid $PID_TO_WAIT to exit"
WAITED=0
while kill -0 "$PID_TO_WAIT" 2>/dev/null; do
  sleep 1
  WAITED=$((WAITED + 1))
  [ "$WAITED" -ge 120 ] && fail "old process did not exit within 120s"
done

TMP_DIR="$(mktemp -d "$(dirname "$TARGET_APP")/.minidb-update.XXXXXX")" || fail "cannot create temp dir next to $TARGET_APP"

log "extracting $ARCHIVE_PATH"
tar -xzf "$ARCHIVE_PATH" -C "$TMP_DIR" || fail "extract failed"

NEW_APP="$(find "$TMP_DIR" -maxdepth 1 -type d -name '*.app' | head -n 1)"
[ -n "$NEW_APP" ] || fail "no .app bundle in archive"

rm -rf "$BACKUP_APP"
mv "$TARGET_APP" "$BACKUP_APP" || fail "cannot move old app aside"
if ! mv "$NEW_APP" "$TARGET_APP"; then
  mv "$BACKUP_APP" "$TARGET_APP"
  fail "cannot move new app into place (old version restored)"
fi

xattr -dr com.apple.quarantine "$TARGET_APP" 2>/dev/null
rm -rf "$BACKUP_APP"
rm -f "$ARCHIVE_PATH"
log "update installed, relaunching"
open "$TARGET_APP"
`

// WindowsInstallerScript 同样先备份旧 exe，复制失败则还原。
const WindowsInstallerScript = `param(
  [int]$PidToWait,
  [string]$ArchivePath,
  [string]$TargetExecutable,
  [string]$LogPath
)

$ErrorActionPreference = "Stop"
$TargetDir = Split-Path -Parent $TargetExecutable
$BackupExecutable = "$TargetExecutable.bak"
$TempDir = Join-Path ([System.IO.Path]::GetTempPath()) ("minidb-update-" + [System.Guid]::NewGuid().ToString("N"))
$Installed = $false

function Write-Log([string]$Message) {
  try { Add-Content -LiteralPath $LogPath -Value ("{0} [update] {1}" -f (Get-Date -Format "yyyy-MM-dd HH:mm:ss"), $Message) } catch {}
}

try {
  Write-Log "waiting for pid $PidToWait to exit"
  $waited = 0
  while (Get-Process -Id $PidToWait -ErrorAction SilentlyContinue) {
    Start-Sleep -Seconds 1
    $waited++
    if ($waited -ge 120) { throw "old process did not exit within 120s" }
  }

  New-Item -ItemType Directory -Path $TempDir | Out-Null
  Expand-Archive -LiteralPath $ArchivePath -DestinationPath $TempDir -Force
  $ExtractedExecutables = @(Get-ChildItem -LiteralPath $TempDir -File | Where-Object { $_.Extension -ieq ".exe" })
  if ($ExtractedExecutables.Count -eq 0) { throw "No executable found in Windows update archive" }

  $TargetExecutableName = [System.IO.Path]::GetFileName($TargetExecutable)
  $PayloadExecutable = $ExtractedExecutables | Where-Object { $_.Name -ieq $TargetExecutableName } | Select-Object -First 1
  if ($null -eq $PayloadExecutable) { $PayloadExecutable = $ExtractedExecutables[0] }

  if (Test-Path -LiteralPath $BackupExecutable) { Remove-Item -LiteralPath $BackupExecutable -Force }
  Move-Item -LiteralPath $TargetExecutable -Destination $BackupExecutable -Force
  try {
    Copy-Item -LiteralPath $PayloadExecutable.FullName -Destination $TargetExecutable -Force
    Get-ChildItem -LiteralPath $TempDir -Force | ForEach-Object {
      if ($_.PSIsContainer) {
        Copy-Item -LiteralPath $_.FullName -Destination (Join-Path $TargetDir $_.Name) -Recurse -Force
      } elseif ($_.Extension -ine ".exe") {
        Copy-Item -LiteralPath $_.FullName -Destination (Join-Path $TargetDir $_.Name) -Force
      }
    }
    $Installed = $true
  } catch {
    Write-Log ("copy failed, restoring old version: " + $_.Exception.Message)
    if (Test-Path -LiteralPath $TargetExecutable) { Remove-Item -LiteralPath $TargetExecutable -Force -ErrorAction SilentlyContinue }
    Move-Item -LiteralPath $BackupExecutable -Destination $TargetExecutable -Force
    throw
  }

  Remove-Item -LiteralPath $BackupExecutable -Force -ErrorAction SilentlyContinue
  Remove-Item -LiteralPath $ArchivePath -Force -ErrorAction SilentlyContinue
  Write-Log "update installed, relaunching"
} catch {
  Write-Log ("FAILED: " + $_.Exception.Message)
} finally {
  if (Test-Path -LiteralPath $TempDir) { Remove-Item -LiteralPath $TempDir -Recurse -Force -ErrorAction SilentlyContinue }
  Remove-Item -LiteralPath $MyInvocation.MyCommand.Path -Force -ErrorAction SilentlyContinue
  if (Test-Path -LiteralPath $TargetExecutable) { Start-Process -FilePath $TargetExecutable }
}
`
