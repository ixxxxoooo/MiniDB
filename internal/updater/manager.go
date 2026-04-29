package updater

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"

	"tableplus-ai/internal/logger"
	appversion "tableplus-ai/internal/version"

	"github.com/wailsapp/wails/v3/pkg/application"
)

const checkInterval = 20 * time.Minute

type State string

const (
	StateIdle        State = "idle"
	StateChecking    State = "checking"
	StateDownloading State = "downloading"
	StateReady       State = "ready"
	StateInstalling  State = "installing"
	StateError       State = "error"
)

var errNoSupportedAsset = errors.New("no supported update asset for current platform")

type manifest struct {
	Version      string                      `json:"version"`
	ReleaseDate  string                      `json:"release_date"`
	ReleaseNotes string                      `json:"release_notes"`
	Platforms    map[string]manifestPlatform `json:"platforms"`
	Mandatory    bool                        `json:"mandatory"`
}

type manifestPlatform struct {
	URL      string `json:"url"`
	Size     int64  `json:"size"`
	Checksum string `json:"checksum"`
}

type UpdateInfo struct {
	Version      string `json:"version"`
	ReleaseDate  string `json:"releaseDate"`
	ReleaseNotes string `json:"releaseNotes"`
	Mandatory    bool   `json:"mandatory"`
	PlatformKey  string `json:"platformKey"`
	ReleaseURL   string `json:"releaseUrl"`
	Asset        manifestPlatform
}

type Manager struct {
	app *application.App

	client *http.Client
	ctx    context.Context
	cancel context.CancelFunc

	mu             sync.Mutex
	state          State
	currentInfo    *UpdateInfo
	readyInfo      *UpdateInfo
	downloadedPath string
}

func NewManager(app *application.App) *Manager {
	ctx, cancel := context.WithCancel(context.Background())
	return &Manager{
		app:    app,
		client: &http.Client{Timeout: 5 * time.Minute},
		ctx:    ctx,
		cancel: cancel,
		state:  StateIdle,
	}
}

func (m *Manager) Start() {
	m.emitState(StateIdle, nil, "", "", false, "")
	go m.loop()
}

func (m *Manager) Shutdown() {
	if m == nil || m.cancel == nil {
		return
	}
	m.cancel()
}

func (m *Manager) CheckNow(manual bool) {
	if m == nil {
		return
	}
	go m.checkNow(manual)
}

func (m *Manager) InstallReadyUpdate() error {
	if m == nil {
		return errors.New("更新管理器未初始化")
	}
	return m.installReadyUpdate()
}

func (m *Manager) Snapshot() StatePayload {
	if m == nil {
		return StatePayload{State: string(StateIdle)}
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	payload := StatePayload{State: string(m.state)}
	info := m.currentInfo
	if m.state == StateReady && m.readyInfo != nil {
		info = m.readyInfo
	}
	if info != nil {
		payload.Version = info.Version
		payload.ReleaseDate = info.ReleaseDate
		payload.ReleaseNotes = info.ReleaseNotes
		payload.ReleaseURL = info.ReleaseURL
	}
	return payload
}

func (m *Manager) loop() {
	m.checkNow(false)

	ticker := time.NewTicker(checkInterval)
	defer ticker.Stop()
	for {
		select {
		case <-m.ctx.Done():
			return
		case <-ticker.C:
			m.checkNow(false)
		}
	}
}

func (m *Manager) checkNow(manual bool) {
	if err := m.ctx.Err(); err != nil {
		return
	}

	m.mu.Lock()
	switch m.state {
	case StateReady:
		info := m.readyInfo
		m.mu.Unlock()
		m.emitReady(info, true)
		return
	case StateChecking, StateDownloading, StateInstalling:
		state := m.state
		info := m.currentInfo
		m.mu.Unlock()
		if manual {
			m.emitState(state, info, "", fmt.Sprintf("当前正在%s，请稍后再试。", stateLabel(state)), true, "idle")
		}
		return
	default:
		m.state = StateChecking
		m.currentInfo = nil
		m.readyInfo = nil
		m.downloadedPath = ""
		m.mu.Unlock()
	}

	m.emitState(StateChecking, nil, "", "", manual, "idle")

	ctx, cancel := context.WithTimeout(m.ctx, 90*time.Second)
	defer cancel()

	info, err := m.fetchUpdateInfo(ctx)
	if err != nil {
		logger.Warn("[Updater] 检查更新失败: %v", err)
		m.setState(StateError, nil, "")
		m.emitError(nil, err.Error(), manual)
		return
	}
	if info == nil {
		m.setState(StateIdle, nil, "")
		m.emitState(StateIdle, nil, "", fmt.Sprintf("当前已是最新版本（v%s）。", appversion.CurrentVersion()), manual, "idle")
		return
	}

	logger.Info("[Updater] 发现新版本: current=%s latest=%s platform=%s", appversion.CurrentVersion(), info.Version, info.PlatformKey)
	m.setState(StateDownloading, info, "")
	m.emitState(StateDownloading, info, "", "", false, "")

	archivePath, err := m.downloadUpdate(ctx, info)
	if err != nil {
		logger.Warn("[Updater] 下载更新失败: %v", err)
		m.setState(StateError, info, "")
		m.emitError(info, err.Error(), manual)
		return
	}

	m.setState(StateReady, info, archivePath)
	m.emitState(StateReady, info, "", "", false, "")
	m.emitReady(info, true)
}

func (m *Manager) fetchUpdateInfo(ctx context.Context) (*UpdateInfo, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, appversion.UpdateManifestURL(), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "TablePlus-AI/"+appversion.CurrentVersion())

	resp, err := m.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return nil, fmt.Errorf("未找到更新清单 update.json")
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("更新清单请求失败: %s", resp.Status)
	}

	var data manifest
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return nil, err
	}
	version := strings.TrimSpace(strings.TrimPrefix(data.Version, "v"))
	if compareVersions(version, appversion.CurrentVersion()) <= 0 {
		return nil, nil
	}

	platformKey, err := currentPlatformKey()
	if err != nil {
		return nil, err
	}
	asset, ok := data.Platforms[platformKey]
	if !ok {
		return nil, errNoSupportedAsset
	}

	return &UpdateInfo{
		Version:      version,
		ReleaseDate:  strings.TrimSpace(data.ReleaseDate),
		ReleaseNotes: strings.TrimSpace(data.ReleaseNotes),
		Mandatory:    data.Mandatory,
		PlatformKey:  platformKey,
		ReleaseURL:   appversion.ReleasePageURL(),
		Asset:        asset,
	}, nil
}

func (m *Manager) downloadUpdate(ctx context.Context, info *UpdateInfo) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, info.Asset.URL, nil)
	if err != nil {
		return "", err
	}

	resp, err := m.client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("更新包下载失败: %s", resp.Status)
	}

	tempFile, err := os.CreateTemp("", "tableplus-ai-update-*"+archiveSuffix(info.Asset.URL))
	if err != nil {
		return "", err
	}
	defer tempFile.Close()

	hasher := sha256.New()
	total := info.Asset.Size
	if resp.ContentLength > 0 {
		total = resp.ContentLength
	}
	progress := newProgressWriter(func(downloaded int64) {
		m.emitProgress(info, downloaded, total)
	})
	m.emitProgress(info, 0, total)
	if _, err := io.Copy(io.MultiWriter(tempFile, hasher, progress), resp.Body); err != nil {
		_ = os.Remove(tempFile.Name())
		return "", err
	}
	m.emitProgress(info, total, total)

	expectedChecksum := strings.TrimSpace(strings.TrimPrefix(info.Asset.Checksum, "sha256:"))
	actualChecksum := hex.EncodeToString(hasher.Sum(nil))
	if expectedChecksum != "" && !strings.EqualFold(expectedChecksum, actualChecksum) {
		_ = os.Remove(tempFile.Name())
		return "", fmt.Errorf("checksum mismatch: expected %s got %s", expectedChecksum, actualChecksum)
	}

	logger.Info("[Updater] 更新包下载完成: version=%s path=%s", info.Version, tempFile.Name())
	return tempFile.Name(), nil
}

func (m *Manager) installReadyUpdate() error {
	m.mu.Lock()
	if m.state != StateReady || m.readyInfo == nil || m.downloadedPath == "" {
		m.mu.Unlock()
		return errors.New("当前没有可安装的更新")
	}
	info := m.readyInfo
	archivePath := m.downloadedPath
	m.state = StateInstalling
	m.mu.Unlock()
	m.emitState(StateInstalling, info, "", "", false, "")

	logger.Info("[Updater] 开始安装更新: version=%s path=%s", info.Version, archivePath)
	if err := m.spawnInstaller(archivePath); err != nil {
		m.setState(StateReady, info, archivePath)
		m.emitState(StateReady, info, "", "", false, "")
		m.emitError(info, err.Error(), true)
		return err
	}

	m.app.Quit()
	return nil
}

func (m *Manager) spawnInstaller(archivePath string) error {
	switch runtime.GOOS {
	case "darwin":
		return spawnDarwinInstaller(archivePath)
	case "windows":
		return spawnWindowsInstaller(archivePath)
	default:
		return fmt.Errorf("unsupported updater platform: %s/%s", runtime.GOOS, runtime.GOARCH)
	}
}

func spawnDarwinInstaller(archivePath string) error {
	executablePath, err := os.Executable()
	if err != nil {
		return err
	}
	executablePath, err = filepath.Abs(executablePath)
	if err != nil {
		return err
	}
	appBundlePath, err := resolveMacBundlePath(executablePath)
	if err != nil {
		return err
	}
	scriptPath, err := writeHelperScript("tableplus-ai-update-*.sh", darwinInstallerScript)
	if err != nil {
		return err
	}
	cmd := exec.Command("/bin/sh", scriptPath, fmt.Sprintf("%d", os.Getpid()), archivePath, appBundlePath)
	return cmd.Start()
}

func spawnWindowsInstaller(archivePath string) error {
	executablePath, err := os.Executable()
	if err != nil {
		return err
	}
	executablePath, err = filepath.Abs(executablePath)
	if err != nil {
		return err
	}
	scriptPath, err := writeHelperScript("tableplus-ai-update-*.ps1", windowsInstallerScript)
	if err != nil {
		return err
	}
	cmd := exec.Command(
		"powershell",
		"-NoProfile",
		"-ExecutionPolicy",
		"Bypass",
		"-WindowStyle",
		"Hidden",
		"-File",
		scriptPath,
		"-PidToWait",
		fmt.Sprintf("%d", os.Getpid()),
		"-ArchivePath",
		archivePath,
		"-TargetExecutable",
		executablePath,
	)
	return cmd.Start()
}

func (m *Manager) emitState(state State, info *UpdateInfo, errMsg, message string, prompt bool, promptKind string) {
	if m.app == nil {
		return
	}
	payload := StatePayload{
		State:      string(state),
		Error:      strings.TrimSpace(errMsg),
		Message:    strings.TrimSpace(message),
		Prompt:     prompt,
		PromptKind: strings.TrimSpace(promptKind),
	}
	if info != nil {
		payload.Version = info.Version
		payload.ReleaseDate = info.ReleaseDate
		payload.ReleaseNotes = info.ReleaseNotes
		payload.ReleaseURL = info.ReleaseURL
	}
	m.app.Event.Emit(EventState, payload)
}

func (m *Manager) emitProgress(info *UpdateInfo, downloaded, total int64) {
	if m.app == nil {
		return
	}
	if total < 0 {
		total = 0
	}
	percentage := 0.0
	if total > 0 {
		percentage = (float64(downloaded) / float64(total)) * 100
		if percentage > 100 {
			percentage = 100
		}
	}
	payload := ProgressPayload{
		State:      string(StateDownloading),
		Downloaded: downloaded,
		Total:      total,
		Percentage: percentage,
	}
	if info != nil {
		payload.Version = info.Version
	}
	m.app.Event.Emit(EventProgress, payload)
}

func (m *Manager) emitReady(info *UpdateInfo, prompt bool) {
	if m.app == nil || info == nil {
		return
	}
	m.app.Event.Emit(EventReady, ReadyPayload{
		State:        string(StateReady),
		Version:      info.Version,
		ReleaseDate:  info.ReleaseDate,
		ReleaseNotes: info.ReleaseNotes,
		Prompt:       prompt,
		PromptKind:   "ready",
		ReleaseURL:   info.ReleaseURL,
	})
}

func (m *Manager) emitError(info *UpdateInfo, errMsg string, prompt bool) {
	if m.app == nil {
		return
	}
	payload := ErrorPayload{
		State:      string(StateError),
		Error:      strings.TrimSpace(errMsg),
		Prompt:     prompt,
		PromptKind: "error",
	}
	if info != nil {
		payload.Version = info.Version
	}
	m.app.Event.Emit(EventError, payload)
}

func (m *Manager) setState(state State, info *UpdateInfo, archivePath string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.state = state
	m.currentInfo = info
	m.readyInfo = info
	m.downloadedPath = archivePath
}

func currentPlatformKey() (string, error) {
	switch runtime.GOOS {
	case "darwin":
		switch runtime.GOARCH {
		case "arm64":
			return "macos-arm64", nil
		case "amd64":
			return "macos-amd64", nil
		}
	case "windows":
		if runtime.GOARCH == "amd64" {
			return "windows-amd64", nil
		}
	}
	return "", fmt.Errorf("unsupported platform for updater: %s/%s", runtime.GOOS, runtime.GOARCH)
}

func archiveSuffix(downloadURL string) string {
	if strings.HasSuffix(downloadURL, ".tar.gz") {
		return ".tar.gz"
	}
	return filepath.Ext(downloadURL)
}

func stateLabel(state State) string {
	switch state {
	case StateChecking:
		return "检查更新"
	case StateDownloading:
		return "下载更新"
	case StateInstalling:
		return "安装更新"
	case StateError:
		return "更新失败"
	default:
		return string(state)
	}
}

func resolveMacBundlePath(executablePath string) (string, error) {
	contentsDir := filepath.Dir(filepath.Dir(executablePath))
	if filepath.Base(contentsDir) != "Contents" {
		return "", errors.New("当前 macOS 运行环境不是 .app 包，无法执行原地更新")
	}
	appBundlePath := filepath.Dir(contentsDir)
	if filepath.Ext(appBundlePath) != ".app" {
		return "", errors.New("无法定位 macOS 应用包路径")
	}
	return appBundlePath, nil
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

type progressWriter struct {
	onProgress func(downloaded int64)
	downloaded int64
	lastEmit   time.Time
}

func newProgressWriter(onProgress func(downloaded int64)) *progressWriter {
	return &progressWriter{
		onProgress: onProgress,
		lastEmit:   time.Now(),
	}
}

func (w *progressWriter) Write(p []byte) (int, error) {
	n := len(p)
	w.downloaded += int64(n)
	if w.onProgress != nil && time.Since(w.lastEmit) >= 120*time.Millisecond {
		w.lastEmit = time.Now()
		w.onProgress(w.downloaded)
	}
	return n, nil
}

const darwinInstallerScript = `#!/bin/sh
set -eu

PID_TO_WAIT="$1"
ARCHIVE_PATH="$2"
TARGET_APP="$3"
TMP_DIR="$(mktemp -d)"

cleanup() {
  rm -rf "$TMP_DIR"
  rm -f "$ARCHIVE_PATH"
  rm -f "$0"
}

trap cleanup EXIT

while kill -0 "$PID_TO_WAIT" 2>/dev/null; do
  sleep 1
done

tar -xzf "$ARCHIVE_PATH" -C "$TMP_DIR"

EXTRACTED_APP="$(find "$TMP_DIR" -maxdepth 1 -type d -name '*.app' | head -n 1)"
if [ -z "$EXTRACTED_APP" ]; then
  echo "No .app bundle found in archive" >&2
  exit 1
fi

rm -rf "$TARGET_APP"
mv "$EXTRACTED_APP" "$TARGET_APP"
open "$TARGET_APP"
`

const windowsInstallerScript = `param(
  [int]$PidToWait,
  [string]$ArchivePath,
  [string]$TargetExecutable
)

$ErrorActionPreference = "Stop"
$TargetDir = Split-Path -Parent $TargetExecutable
$TempDir = Join-Path ([System.IO.Path]::GetTempPath()) ("tableplus-ai-update-" + [System.Guid]::NewGuid().ToString("N"))

New-Item -ItemType Directory -Path $TempDir | Out-Null

try {
  while (Get-Process -Id $PidToWait -ErrorAction SilentlyContinue) {
    Start-Sleep -Seconds 1
  }

  Expand-Archive -LiteralPath $ArchivePath -DestinationPath $TempDir -Force
  $ExtractedExecutables = @(Get-ChildItem -LiteralPath $TempDir -File | Where-Object { $_.Extension -ieq ".exe" })
  if ($ExtractedExecutables.Count -eq 0) {
    throw "No executable found in Windows update archive"
  }

  $TargetExecutableName = [System.IO.Path]::GetFileName($TargetExecutable)
  $PayloadExecutable = $ExtractedExecutables | Where-Object { $_.Name -ieq $TargetExecutableName } | Select-Object -First 1
  if ($null -eq $PayloadExecutable) {
    $PayloadExecutable = $ExtractedExecutables[0]
  }

  Get-ChildItem -LiteralPath $TempDir -Force | ForEach-Object {
    if ($_.PSIsContainer) {
      Copy-Item -LiteralPath $_.FullName -Destination (Join-Path $TargetDir $_.Name) -Recurse -Force
    } elseif ($_.Extension -ine ".exe") {
      Copy-Item -LiteralPath $_.FullName -Destination (Join-Path $TargetDir $_.Name) -Force
    }
  }
  Copy-Item -LiteralPath $PayloadExecutable.FullName -Destination $TargetExecutable -Force
  Start-Process -FilePath $TargetExecutable
} finally {
  Remove-Item -LiteralPath $ArchivePath -Force -ErrorAction SilentlyContinue
  Remove-Item -LiteralPath $TempDir -Recurse -Force -ErrorAction SilentlyContinue
  Remove-Item -LiteralPath $MyInvocation.MyCommand.Path -Force -ErrorAction SilentlyContinue
}
`
