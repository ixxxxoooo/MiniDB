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
	"net/url"
	"os"
	"path"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"

	"minidb/internal/appdata"
	"minidb/internal/logger"
	appversion "minidb/internal/version"

	"github.com/wailsapp/wails/v3/pkg/application"
)

const (
	// 启动后延迟首检，避开应用启动高峰
	initialCheckDelay = 10 * time.Second
	checkInterval     = 4 * time.Hour

	manifestTimeout = 30 * time.Second
	downloadTimeout = 30 * time.Minute

	// 仅开发构建允许覆盖清单地址（本地联调），正式版本不读取该变量
	manifestOverrideEnv = "MINIDB_UPDATE_MANIFEST_URL"
)

type State string

const (
	StateIdle        State = "idle"
	StateChecking    State = "checking"
	StateAvailable   State = "available" // 已发现新版本，尚未下载（关闭自动下载时停在这里）
	StateDownloading State = "downloading"
	StateReady       State = "ready"
	StateInstalling  State = "installing"
	StateError       State = "error"
)

// Settings 是用户可调的更新偏好，由 SettingsService 持久化并通过 getter 注入。
type Settings struct {
	AutoCheck      bool
	AutoDownload   bool
	SkippedVersion string
}

// DefaultSettings 是未保存过偏好时的默认值。
func DefaultSettings() Settings {
	return Settings{AutoCheck: true, AutoDownload: true}
}

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
	Version      string
	ReleaseDate  string
	ReleaseNotes string
	Mandatory    bool
	PlatformKey  string
	ReleaseURL   string
	Asset        manifestPlatform
}

// updateError 带有面向前端的消息码，避免把中文文案固化在后端。
type updateError struct {
	code string
	err  error
}

func (e *updateError) Error() string { return e.err.Error() }
func (e *updateError) Unwrap() error { return e.err }

func codedErr(code string, format string, args ...any) error {
	return &updateError{code: code, err: fmt.Errorf(format, args...)}
}

func errorCode(err error) string {
	var ue *updateError
	if errors.As(err, &ue) {
		return ue.code
	}
	return ""
}

// config 汇集 Manager 的外部依赖，便于测试替换。
type config struct {
	emit           func(event string, payload any)
	quit           func()
	getSettings    func() Settings
	manifestURL    func() string
	platformKey    func() (string, error)
	currentVersion string
	updatesDir     string
	client         *http.Client
	spawnInstaller func(archivePath string) error
	checkInstall   func() error
	isDevBuild     bool
	releaseURL     string
}

type Manager struct {
	cfg config

	ctx    context.Context
	cancel context.CancelFunc

	mu             sync.Mutex
	state          State
	info           *UpdateInfo // 当前已发现的版本
	downloadedPath string
	progress       ProgressPayload
	lastError      string
	lastErrorCode  string
	// 同一版本的同一阶段只主动提示一次，避免每次轮询都弹窗打扰用户
	prompted map[string]bool
	started  bool
}

func NewManager(app *application.App) *Manager {
	return newManager(config{
		emit: func(event string, payload any) {
			if app != nil {
				app.Event.Emit(event, payload)
			}
		},
		quit: func() {
			if app != nil {
				app.Quit()
			}
		},
		getSettings:    DefaultSettings,
		manifestURL:    defaultManifestURL,
		platformKey:    currentPlatformKey,
		currentVersion: appversion.CurrentVersion(),
		updatesDir:     appdata.UpdatesDir(),
		client:         &http.Client{},
		spawnInstaller: spawnInstaller,
		checkInstall:   checkInstallSupported,
		isDevBuild:     appversion.IsDevBuild(),
		releaseURL:     appversion.ReleasePageURL(),
	})
}

func newManager(cfg config) *Manager {
	if cfg.getSettings == nil {
		cfg.getSettings = DefaultSettings
	}
	if cfg.emit == nil {
		cfg.emit = func(string, any) {}
	}
	if cfg.quit == nil {
		cfg.quit = func() {}
	}
	if cfg.client == nil {
		cfg.client = &http.Client{}
	}
	if cfg.checkInstall == nil {
		cfg.checkInstall = func() error { return nil }
	}
	ctx, cancel := context.WithCancel(context.Background())
	return &Manager{cfg: cfg, ctx: ctx, cancel: cancel, state: StateIdle, prompted: map[string]bool{}}
}

// SetSettingsProvider 注入用户偏好读取函数（SettingsService 在启动时调用）。
func (m *Manager) SetSettingsProvider(fn func() Settings) {
	if m == nil || fn == nil {
		return
	}
	m.mu.Lock()
	m.cfg.getSettings = fn
	m.mu.Unlock()
}

func defaultManifestURL() string {
	if appversion.IsDevBuild() {
		if override := strings.TrimSpace(os.Getenv(manifestOverrideEnv)); override != "" {
			return override
		}
	}
	return appversion.UpdateManifestURL()
}

// Start 启动后台轮询。只应调用一次。
func (m *Manager) Start() {
	if m == nil {
		return
	}
	m.mu.Lock()
	if m.started {
		m.mu.Unlock()
		return
	}
	m.started = true
	m.mu.Unlock()
	go m.loop()
}

func (m *Manager) Shutdown() {
	if m == nil || m.cancel == nil {
		return
	}
	m.cancel()
}

func (m *Manager) loop() {
	timer := time.NewTimer(initialCheckDelay)
	defer timer.Stop()
	for {
		select {
		case <-m.ctx.Done():
			return
		case <-timer.C:
			m.autoCheck()
			timer.Reset(checkInterval)
		}
	}
}

func (m *Manager) autoCheck() {
	settings := m.settings()
	if !settings.AutoCheck {
		return
	}
	// 开发构建没有真实版本号，自动检查没有意义（手动检查仍可用）
	if m.cfg.isDevBuild {
		return
	}
	m.check(false)
}

func (m *Manager) settings() Settings {
	m.mu.Lock()
	fn := m.cfg.getSettings
	m.mu.Unlock()
	return fn()
}

// CheckNow 由用户主动触发检查。
func (m *Manager) CheckNow() {
	if m == nil {
		return
	}
	go m.check(true)
}

// DownloadUpdate 开始下载已发现的新版本（关闭自动下载时由用户触发）。
func (m *Manager) DownloadUpdate() error {
	if m == nil {
		return errors.New("updater not initialized")
	}
	m.mu.Lock()
	if m.state != StateAvailable || m.info == nil {
		m.mu.Unlock()
		return codedErr(MessageNoUpdate, "no update available to download")
	}
	info := m.info
	m.mu.Unlock()
	go m.download(info, true)
	return nil
}

// SkipVersion 的持久化由 SettingsService 完成；这里只负责把状态回到 idle 并通知前端。
func (m *Manager) SkipVersion(version string) {
	if m == nil {
		return
	}
	m.mu.Lock()
	if m.info != nil && m.info.Version == version && m.state != StateInstalling && m.state != StateDownloading {
		m.state = StateIdle
		m.info = nil
		m.downloadedPath = ""
	}
	m.mu.Unlock()
	m.emitState(false, "")
}

// Snapshot 返回当前状态，供前端启动时同步。
func (m *Manager) Snapshot() StatePayload {
	if m == nil {
		return StatePayload{State: string(StateIdle)}
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.payloadLocked(false, "")
}

func (m *Manager) payloadLocked(prompt bool, promptKind string) StatePayload {
	p := StatePayload{
		State:       string(m.state),
		Error:       m.lastError,
		MessageCode: m.lastErrorCode,
		Prompt:      prompt,
		PromptKind:  promptKind,
		ReleaseURL:  m.cfg.releaseURL,
	}
	if m.state != StateError {
		p.Error, p.MessageCode = "", ""
	}
	if m.info != nil {
		p.Version = m.info.Version
		p.ReleaseDate = m.info.ReleaseDate
		p.ReleaseNotes = m.info.ReleaseNotes
		p.Mandatory = m.info.Mandatory
	}
	if m.state == StateDownloading {
		p.Downloaded = m.progress.Downloaded
		p.Total = m.progress.Total
		p.Percentage = m.progress.Percentage
	}
	if m.state == StateReady {
		p.CanInstall = m.cfg.checkInstall() == nil
	}
	return p
}

// emitState 广播当前状态。prompt=true 时前端应主动提示用户（弹卡片）。
func (m *Manager) emitState(prompt bool, promptKind string) {
	m.mu.Lock()
	p := m.payloadLocked(prompt, promptKind)
	m.mu.Unlock()
	m.cfg.emit(EventState, p)
}

func (m *Manager) emitMessage(code string, manual bool) {
	m.mu.Lock()
	p := m.payloadLocked(false, "")
	m.mu.Unlock()
	p.MessageCode = code
	p.Manual = manual
	p.Prompt = manual
	p.PromptKind = "message"
	m.cfg.emit(EventState, p)
}

// shouldPrompt 同一版本同一阶段只主动提示一次；手动操作总是提示。
func (m *Manager) shouldPrompt(kind, version string, manual bool) bool {
	if manual {
		return true
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	key := kind + ":" + version
	if m.prompted[key] {
		return false
	}
	m.prompted[key] = true
	return true
}

func (m *Manager) setError(code string, err error, info *UpdateInfo) {
	m.mu.Lock()
	m.state = StateError
	m.lastError = err.Error()
	m.lastErrorCode = code
	if info != nil {
		m.info = info
	}
	m.mu.Unlock()
}

func (m *Manager) check(manual bool) {
	if m.ctx.Err() != nil {
		return
	}

	m.mu.Lock()
	switch m.state {
	case StateChecking, StateDownloading, StateInstalling:
		m.mu.Unlock()
		if manual {
			m.emitMessage(MessageBusy, true)
		}
		return
	}
	prev := m.state
	// 已有可用/就绪的版本时不回退到 checking，避免界面上的"重启安装"按钮闪烁消失
	if prev != StateReady && prev != StateAvailable {
		m.state = StateChecking
	}
	m.mu.Unlock()
	if prev != StateReady && prev != StateAvailable {
		m.emitState(false, "")
	}

	ctx, cancel := context.WithTimeout(m.ctx, manifestTimeout)
	defer cancel()
	info, err := m.fetchUpdateInfo(ctx)
	if err != nil {
		logger.Warn("[Updater] 检查更新失败: %v", err)
		if prev == StateReady || prev == StateAvailable {
			if manual {
				m.emitMessage(errorCodeOr(err, MessageNetwork), true)
			}
			return
		}
		if manual {
			m.setError(errorCodeOr(err, MessageNetwork), err, nil)
			m.cfg.emit(EventError, ErrorPayload{State: string(StateError), Error: err.Error(), MessageCode: errorCodeOr(err, MessageNetwork), Prompt: true, PromptKind: "error"})
			m.emitState(false, "")
		} else {
			// 自动检查失败静默：只记日志，状态回到 idle，不打扰用户
			m.mu.Lock()
			m.state = StateIdle
			m.mu.Unlock()
			m.emitState(false, "")
		}
		return
	}

	if info == nil {
		m.mu.Lock()
		if prev != StateReady && prev != StateAvailable {
			m.state = StateIdle
			m.info = nil
		}
		m.mu.Unlock()
		m.emitState(false, "")
		if manual {
			m.emitMessage(MessageUpToDate, true)
		}
		return
	}

	// 用户选择跳过的版本：自动检查不再提示（手动检查仍然展示）
	if !manual && m.settings().SkippedVersion == info.Version {
		m.mu.Lock()
		if prev != StateReady && prev != StateAvailable {
			m.state = StateIdle
		}
		m.mu.Unlock()
		m.emitState(false, "")
		return
	}

	m.mu.Lock()
	sameAsCurrent := m.info != nil && m.info.Version == info.Version
	m.mu.Unlock()

	if (prev == StateReady || prev == StateAvailable) && sameAsCurrent {
		kind := "available"
		if prev == StateReady {
			kind = "ready"
		}
		m.emitState(m.shouldPrompt(kind, info.Version, manual), kind)
		return
	}

	// 发现了比已就绪/已发现版本更新的版本：丢弃旧状态，走完整流程
	m.mu.Lock()
	m.info = info
	m.downloadedPath = ""
	m.state = StateAvailable
	m.mu.Unlock()
	logger.Info("[Updater] 发现新版本: current=%s latest=%s platform=%s", m.cfg.currentVersion, info.Version, info.PlatformKey)

	// 已下载并校验通过的包（例如上次启动时下载好的）直接进入 ready
	if cached := m.findCachedPackage(info); cached != "" {
		m.markReady(info, cached, manual)
		return
	}

	if m.settings().AutoDownload {
		m.download(info, manual)
		return
	}
	m.emitState(m.shouldPrompt("available", info.Version, manual), "available")
}

func errorCodeOr(err error, fallback string) string {
	if code := errorCode(err); code != "" {
		return code
	}
	return fallback
}

func (m *Manager) fetchUpdateInfo(ctx context.Context) (*UpdateInfo, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, m.cfg.manifestURL(), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "MiniDB/"+m.cfg.currentVersion)

	resp, err := m.cfg.client.Do(req)
	if err != nil {
		return nil, codedErr(MessageNetwork, "%v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return nil, codedErr(MessageManifestInvalid, "update manifest not found (update.json)")
	}
	if resp.StatusCode != http.StatusOK {
		return nil, codedErr(MessageNetwork, "update manifest request failed: %s", resp.Status)
	}

	var data manifest
	if err := json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(&data); err != nil {
		return nil, codedErr(MessageManifestInvalid, "invalid update manifest: %v", err)
	}
	version := strings.TrimSpace(strings.TrimPrefix(data.Version, "v"))
	if version == "" {
		return nil, codedErr(MessageManifestInvalid, "update manifest has no version")
	}
	if compareVersions(version, m.cfg.currentVersion) <= 0 {
		return nil, nil
	}

	platformKey, err := m.cfg.platformKey()
	if err != nil {
		return nil, codedErr(MessageNoPlatformAsset, "%v", err)
	}
	asset, ok := data.Platforms[platformKey]
	if !ok || strings.TrimSpace(asset.URL) == "" {
		return nil, codedErr(MessageNoPlatformAsset, "no update asset for platform %s", platformKey)
	}
	if err := validateAssetURL(asset.URL); err != nil {
		return nil, err
	}
	if normalizeChecksum(asset.Checksum) == "" {
		// 没有校验和就无法确认下载内容完整，宁可不更新
		return nil, codedErr(MessageManifestInvalid, "update manifest is missing sha256 checksum for %s", platformKey)
	}

	return &UpdateInfo{
		Version:      version,
		ReleaseDate:  strings.TrimSpace(data.ReleaseDate),
		ReleaseNotes: strings.TrimSpace(data.ReleaseNotes),
		Mandatory:    data.Mandatory,
		PlatformKey:  platformKey,
		ReleaseURL:   m.cfg.releaseURL,
		Asset:        asset,
	}, nil
}

// validateAssetURL 仅接受 https（本地回环地址的 http 允许，供联调使用）。
func validateAssetURL(raw string) error {
	u, err := url.Parse(raw)
	if err != nil {
		return codedErr(MessageManifestInvalid, "invalid asset url: %v", err)
	}
	if u.Scheme == "https" {
		return nil
	}
	host := u.Hostname()
	if u.Scheme == "http" && (host == "127.0.0.1" || host == "localhost" || host == "::1") {
		return nil
	}
	return codedErr(MessageManifestInvalid, "asset url must use https: %s", raw)
}

func normalizeChecksum(v string) string {
	v = strings.TrimSpace(v)
	v = strings.TrimPrefix(strings.ToLower(v), "sha256:")
	return strings.TrimSpace(v)
}

// packagePath 返回更新包在更新目录中的最终路径；文件名只取 URL 的 base，防止路径穿越。
func (m *Manager) packagePath(info *UpdateInfo) (string, error) {
	u, err := url.Parse(info.Asset.URL)
	if err != nil {
		return "", err
	}
	name := path.Base(u.Path)
	if name == "" || name == "." || name == "/" || strings.ContainsAny(name, `\/`) {
		return "", codedErr(MessageManifestInvalid, "invalid asset file name in %s", info.Asset.URL)
	}
	return filepath.Join(m.cfg.updatesDir, name), nil
}

// findCachedPackage 返回已经下载且校验通过的更新包路径，没有则返回空串。
func (m *Manager) findCachedPackage(info *UpdateInfo) string {
	dest, err := m.packagePath(info)
	if err != nil {
		return ""
	}
	st, err := os.Stat(dest)
	if err != nil || st.IsDir() {
		return ""
	}
	if info.Asset.Size > 0 && st.Size() != info.Asset.Size {
		return ""
	}
	sum, err := fileSHA256(dest)
	if err != nil || !strings.EqualFold(sum, normalizeChecksum(info.Asset.Checksum)) {
		_ = os.Remove(dest)
		return ""
	}
	return dest
}

func fileSHA256(p string) (string, error) {
	f, err := os.Open(p)
	if err != nil {
		return "", err
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

func (m *Manager) markReady(info *UpdateInfo, archivePath string, manual bool) {
	m.mu.Lock()
	m.state = StateReady
	m.info = info
	m.downloadedPath = archivePath
	m.mu.Unlock()
	m.cleanupOtherPackages(archivePath)
	prompt := m.shouldPrompt("ready", info.Version, manual)
	m.emitState(prompt, "ready")
	m.mu.Lock()
	canInstall := m.cfg.checkInstall() == nil
	m.mu.Unlock()
	m.cfg.emit(EventReady, ReadyPayload{
		State: string(StateReady), Version: info.Version, ReleaseDate: info.ReleaseDate, ReleaseNotes: info.ReleaseNotes,
		Mandatory: info.Mandatory, Prompt: prompt, PromptKind: "ready", ReleaseURL: info.ReleaseURL, CanInstall: canInstall,
	})
}

// cleanupOtherPackages 删除更新目录里除 keep 之外的旧包与残留的 .part 文件。
func (m *Manager) cleanupOtherPackages(keep string) {
	entries, err := os.ReadDir(m.cfg.updatesDir)
	if err != nil {
		return
	}
	for _, e := range entries {
		full := filepath.Join(m.cfg.updatesDir, e.Name())
		if e.IsDir() || full == keep {
			continue
		}
		_ = os.Remove(full)
	}
}

// download 下载并校验更新包。下载使用独立的长超时 context，不与检查清单共用。
func (m *Manager) download(info *UpdateInfo, manual bool) {
	m.mu.Lock()
	if m.state == StateDownloading || m.state == StateInstalling {
		m.mu.Unlock()
		return
	}
	m.state = StateDownloading
	m.info = info
	m.progress = ProgressPayload{State: string(StateDownloading), Version: info.Version, Total: info.Asset.Size}
	m.lastError, m.lastErrorCode = "", ""
	m.mu.Unlock()
	m.emitState(m.shouldPrompt("available", info.Version, manual), "available")

	ctx, cancel := context.WithTimeout(m.ctx, downloadTimeout)
	defer cancel()

	archivePath, err := m.downloadPackage(ctx, info)
	if err != nil {
		if m.ctx.Err() != nil {
			return
		}
		logger.Warn("[Updater] 下载更新失败: %v", err)
		code := errorCodeOr(err, MessageDownloadFailed)
		m.setError(code, err, info)
		m.cfg.emit(EventError, ErrorPayload{State: string(StateError), Version: info.Version, Error: err.Error(), MessageCode: code, Prompt: manual, PromptKind: "error"})
		m.emitState(false, "")
		return
	}
	m.markReady(info, archivePath, manual)
}

func (m *Manager) downloadPackage(ctx context.Context, info *UpdateInfo) (string, error) {
	dest, err := m.packagePath(info)
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(m.cfg.updatesDir, 0o755); err != nil {
		return "", codedErr(MessageDownloadFailed, "cannot create updates dir: %v", err)
	}
	part := dest + ".part"

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, info.Asset.URL, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("User-Agent", "MiniDB/"+m.cfg.currentVersion)
	resp, err := m.cfg.client.Do(req)
	if err != nil {
		return "", codedErr(MessageDownloadFailed, "%v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", codedErr(MessageDownloadFailed, "download failed: %s", resp.Status)
	}

	f, err := os.Create(part)
	if err != nil {
		return "", codedErr(MessageDownloadFailed, "%v", err)
	}
	cleanup := func() { _ = f.Close(); _ = os.Remove(part) }

	total := info.Asset.Size
	if resp.ContentLength > 0 {
		total = resp.ContentLength
	}
	hasher := sha256.New()
	pw := &progressWriter{onProgress: func(done int64) { m.setProgress(info, done, total) }, lastEmit: time.Now()}
	written, err := io.Copy(io.MultiWriter(f, hasher, pw), resp.Body)
	if err != nil {
		cleanup()
		return "", codedErr(MessageDownloadFailed, "%v", err)
	}
	if err := f.Close(); err != nil {
		_ = os.Remove(part)
		return "", codedErr(MessageDownloadFailed, "%v", err)
	}
	m.setProgress(info, written, total)

	if info.Asset.Size > 0 && written != info.Asset.Size {
		_ = os.Remove(part)
		return "", codedErr(MessageChecksum, "size mismatch: expected %d got %d", info.Asset.Size, written)
	}
	expected := normalizeChecksum(info.Asset.Checksum)
	actual := hex.EncodeToString(hasher.Sum(nil))
	if !strings.EqualFold(expected, actual) {
		_ = os.Remove(part)
		return "", codedErr(MessageChecksum, "checksum mismatch: expected %s got %s", expected, actual)
	}
	if err := os.Rename(part, dest); err != nil {
		_ = os.Remove(part)
		return "", codedErr(MessageDownloadFailed, "%v", err)
	}
	logger.Info("[Updater] 更新包下载完成: version=%s path=%s", info.Version, dest)
	return dest, nil
}

func (m *Manager) setProgress(info *UpdateInfo, downloaded, total int64) {
	pct := 0.0
	if total > 0 {
		pct = float64(downloaded) / float64(total) * 100
		if pct > 100 {
			pct = 100
		}
	}
	p := ProgressPayload{State: string(StateDownloading), Version: info.Version, Downloaded: downloaded, Total: total, Percentage: pct}
	m.mu.Lock()
	m.progress = p
	m.mu.Unlock()
	m.cfg.emit(EventProgress, p)
}

// InstallReadyUpdate 退出应用并由外部脚本替换后重启。
func (m *Manager) InstallReadyUpdate() error {
	if m == nil {
		return errors.New("updater not initialized")
	}
	m.mu.Lock()
	if m.state != StateReady || m.info == nil || m.downloadedPath == "" {
		m.mu.Unlock()
		return codedErr(MessageNoUpdate, "no update ready to install")
	}
	info := m.info
	archivePath := m.downloadedPath
	m.mu.Unlock()

	// 退出前预检：不能原地更新就不要退出应用，引导用户手动安装
	if err := m.cfg.checkInstall(); err != nil {
		logger.Warn("[Updater] 无法原地更新: %v", err)
		m.cfg.emit(EventError, ErrorPayload{State: string(StateReady), Version: info.Version, Error: err.Error(), MessageCode: MessageInstallUnsupported, Prompt: true, PromptKind: "error"})
		return codedErr(MessageInstallUnsupported, "%v", err)
	}

	m.mu.Lock()
	m.state = StateInstalling
	m.mu.Unlock()
	m.emitState(false, "")

	logger.Info("[Updater] 开始安装更新: version=%s path=%s", info.Version, archivePath)
	if err := m.cfg.spawnInstaller(archivePath); err != nil {
		m.mu.Lock()
		m.state = StateReady
		m.mu.Unlock()
		m.emitState(false, "")
		m.cfg.emit(EventError, ErrorPayload{State: string(StateReady), Version: info.Version, Error: err.Error(), MessageCode: MessageInstallFailed, Prompt: true, PromptKind: "error"})
		return codedErr(MessageInstallFailed, "%v", err)
	}
	m.cfg.quit()
	return nil
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

type progressWriter struct {
	onProgress func(downloaded int64)
	downloaded int64
	lastEmit   time.Time
}

func (w *progressWriter) Write(p []byte) (int, error) {
	n := len(p)
	w.downloaded += int64(n)
	if w.onProgress != nil && time.Since(w.lastEmit) >= 150*time.Millisecond {
		w.lastEmit = time.Now()
		w.onProgress(w.downloaded)
	}
	return n, nil
}
