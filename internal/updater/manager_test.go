package updater

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

type recorder struct {
	mu     sync.Mutex
	events []recordedEvent
}

type recordedEvent struct {
	name    string
	payload any
}

func (r *recorder) emit(name string, payload any) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.events = append(r.events, recordedEvent{name, payload})
}

func (r *recorder) states() []StatePayload {
	r.mu.Lock()
	defer r.mu.Unlock()
	var out []StatePayload
	for _, e := range r.events {
		if p, ok := e.payload.(StatePayload); ok && e.name == EventState {
			out = append(out, p)
		}
	}
	return out
}

func (r *recorder) countPrompts(kind string) int {
	n := 0
	for _, p := range r.states() {
		if p.Prompt && p.PromptKind == kind {
			n++
		}
	}
	return n
}

type fixture struct {
	srv       *httptest.Server
	rec       *recorder
	mgr       *Manager
	payload   []byte
	version   string
	checksum  string
	size      int64
	downloads atomic.Int32
	settings  Settings
	dir       string
	spawned   atomic.Int32
	quit      atomic.Int32
	installOK error
}

func newFixture(t *testing.T, current, latest string) *fixture {
	t.Helper()
	f := &fixture{rec: &recorder{}, payload: []byte("fake-package-contents"), version: latest, settings: DefaultSettings(), dir: t.TempDir()}
	sum := sha256.Sum256(f.payload)
	f.checksum = "sha256:" + hex.EncodeToString(sum[:])
	f.size = int64(len(f.payload))

	mux := http.NewServeMux()
	mux.HandleFunc("/update.json", func(w http.ResponseWriter, r *http.Request) {
		m := manifest{
			Version: f.version, ReleaseDate: "2026-01-01T00:00:00Z", ReleaseNotes: "notes",
			Platforms: map[string]manifestPlatform{"test-platform": {URL: f.srv.URL + "/pkg-" + f.version + ".tar.gz", Size: f.size, Checksum: f.checksum}},
		}
		_ = json.NewEncoder(w).Encode(m)
	})
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		f.downloads.Add(1)
		_, _ = w.Write(f.payload)
	})
	f.srv = httptest.NewServer(mux)
	t.Cleanup(f.srv.Close)

	f.mgr = newManager(config{
		emit:           f.rec.emit,
		quit:           func() { f.quit.Add(1) },
		getSettings:    func() Settings { return f.settings },
		manifestURL:    func() string { return f.srv.URL + "/update.json" },
		platformKey:    func() (string, error) { return "test-platform", nil },
		currentVersion: current,
		updatesDir:     filepath.Join(f.dir, "updates"),
		client:         f.srv.Client(),
		spawnInstaller: func(string) error { f.spawned.Add(1); return nil },
		checkInstall:   func() error { return f.installOK },
		releaseURL:     "https://example.com/releases",
	})
	t.Cleanup(f.mgr.Shutdown)
	return f
}

func TestCheckNoNewVersion(t *testing.T) {
	f := newFixture(t, "1.0.0", "1.0.0")
	f.mgr.check(true)
	if got := f.mgr.Snapshot().State; got != "idle" {
		t.Fatalf("state=%s want idle", got)
	}
	var sawUpToDate bool
	for _, p := range f.rec.states() {
		if p.MessageCode == MessageUpToDate && p.Manual {
			sawUpToDate = true
		}
	}
	if !sawUpToDate {
		t.Fatal("手动检查没有新版本时应给出 up_to_date 反馈")
	}
	if f.downloads.Load() != 0 {
		t.Fatal("没有新版本不应下载")
	}
}

func TestAutoDownloadReachesReadyAndPromptsOnce(t *testing.T) {
	f := newFixture(t, "1.0.0", "1.1.0")
	f.mgr.check(false)
	snap := f.mgr.Snapshot()
	if snap.State != "ready" || snap.Version != "1.1.0" {
		t.Fatalf("snapshot=%+v want ready 1.1.0", snap)
	}
	if _, err := os.Stat(filepath.Join(f.dir, "updates", "pkg-1.1.0.tar.gz")); err != nil {
		t.Fatalf("更新包应落在更新目录: %v", err)
	}
	// 再次自动检查：同一版本不应重复弹提示，也不应重复下载
	f.mgr.check(false)
	if n := f.rec.countPrompts("ready"); n != 1 {
		t.Fatalf("ready 提示次数=%d want 1", n)
	}
	if n := f.downloads.Load(); n != 1 {
		t.Fatalf("下载次数=%d want 1", n)
	}
	// 手动检查总是提示
	f.mgr.check(true)
	if n := f.rec.countPrompts("ready"); n != 2 {
		t.Fatalf("手动检查应再次提示，实际 %d", n)
	}
}

func TestAutoDownloadDisabledStopsAtAvailable(t *testing.T) {
	f := newFixture(t, "1.0.0", "1.1.0")
	f.settings.AutoDownload = false
	f.mgr.check(false)
	if got := f.mgr.Snapshot().State; got != "available" {
		t.Fatalf("state=%s want available", got)
	}
	if f.downloads.Load() != 0 {
		t.Fatal("关闭自动下载时不应下载")
	}
	if err := f.mgr.DownloadUpdate(); err != nil {
		t.Fatal(err)
	}
	waitForState(t, f.mgr, "ready")
	if f.downloads.Load() != 1 {
		t.Fatalf("下载次数=%d want 1", f.downloads.Load())
	}
}

func TestCachedPackageIsReusedAfterRestart(t *testing.T) {
	f := newFixture(t, "1.0.0", "1.1.0")
	f.mgr.check(false)
	if f.downloads.Load() != 1 {
		t.Fatal("首次应下载")
	}
	// 模拟应用重启：新 Manager，同一个更新目录
	second := newManager(f.mgr.cfg)
	t.Cleanup(second.Shutdown)
	second.check(false)
	if got := second.Snapshot().State; got != "ready" {
		t.Fatalf("重启后 state=%s want ready", got)
	}
	if f.downloads.Load() != 1 {
		t.Fatalf("重启后不应重复下载，下载次数=%d", f.downloads.Load())
	}
}

func TestChecksumMismatchRejectsPackage(t *testing.T) {
	f := newFixture(t, "1.0.0", "1.1.0")
	f.checksum = "sha256:" + hex.EncodeToString(make([]byte, 32))
	f.mgr.check(false)
	snap := f.mgr.Snapshot()
	if snap.State != "error" || snap.MessageCode != MessageChecksum {
		t.Fatalf("snapshot=%+v want error/checksum", snap)
	}
	entries, _ := os.ReadDir(filepath.Join(f.dir, "updates"))
	if len(entries) != 0 {
		t.Fatalf("校验失败后不应留下文件: %v", entries)
	}
	if err := f.mgr.InstallReadyUpdate(); err == nil {
		t.Fatal("校验失败的包不应可安装")
	}
}

func TestMissingChecksumIsRejected(t *testing.T) {
	f := newFixture(t, "1.0.0", "1.1.0")
	f.checksum = ""
	f.mgr.check(true)
	if got := f.mgr.Snapshot().MessageCode; got != MessageManifestInvalid {
		t.Fatalf("messageCode=%q want %s", got, MessageManifestInvalid)
	}
	if f.downloads.Load() != 0 {
		t.Fatal("缺少校验和时不应下载")
	}
}

func TestSkippedVersionSilentForAutoButVisibleForManual(t *testing.T) {
	f := newFixture(t, "1.0.0", "1.1.0")
	f.settings.SkippedVersion = "1.1.0"
	f.mgr.check(false)
	if got := f.mgr.Snapshot().State; got != "idle" {
		t.Fatalf("自动检查应忽略被跳过的版本，state=%s", got)
	}
	if f.downloads.Load() != 0 {
		t.Fatal("被跳过的版本不应自动下载")
	}
	f.mgr.check(true)
	if got := f.mgr.Snapshot().State; got != "ready" {
		t.Fatalf("手动检查仍应展示被跳过的版本，state=%s", got)
	}
}

func TestPrereleaseCurrentVersionUpdatesToRelease(t *testing.T) {
	f := newFixture(t, "1.2.0-beta.1", "1.2.0")
	f.mgr.check(false)
	if got := f.mgr.Snapshot().State; got != "ready" {
		t.Fatalf("预发布版本应能升级到同号正式版，state=%s", got)
	}
}

func TestNewerVersionReplacesReadyPackage(t *testing.T) {
	f := newFixture(t, "1.0.0", "1.1.0")
	f.mgr.check(false)
	f.version = "1.2.0"
	f.mgr.check(false)
	snap := f.mgr.Snapshot()
	if snap.State != "ready" || snap.Version != "1.2.0" {
		t.Fatalf("snapshot=%+v want ready 1.2.0", snap)
	}
	entries, _ := os.ReadDir(filepath.Join(f.dir, "updates"))
	if len(entries) != 1 || entries[0].Name() != "pkg-1.2.0.tar.gz" {
		t.Fatalf("旧版本包应被清理，目录内容: %v", entries)
	}
}

func TestInstallPrecheckBlocksQuit(t *testing.T) {
	f := newFixture(t, "1.0.0", "1.1.0")
	f.mgr.check(false)
	f.installOK = os.ErrPermission
	if err := f.mgr.InstallReadyUpdate(); err == nil {
		t.Fatal("预检失败应返回错误")
	}
	if f.quit.Load() != 0 || f.spawned.Load() != 0 {
		t.Fatal("预检失败时不能退出应用，也不能启动安装脚本")
	}
	if got := f.mgr.Snapshot().State; got != "ready" {
		t.Fatalf("预检失败后应保持 ready，state=%s", got)
	}
}

func TestInstallSpawnsAndQuits(t *testing.T) {
	f := newFixture(t, "1.0.0", "1.1.0")
	f.mgr.check(false)
	if err := f.mgr.InstallReadyUpdate(); err != nil {
		t.Fatal(err)
	}
	if f.spawned.Load() != 1 || f.quit.Load() != 1 {
		t.Fatalf("spawned=%d quit=%d want 1/1", f.spawned.Load(), f.quit.Load())
	}
}

func TestManifestFailureAutoIsSilent(t *testing.T) {
	f := newFixture(t, "1.0.0", "1.1.0")
	f.srv.Close()
	f.mgr.check(false)
	if got := f.mgr.Snapshot().State; got != "idle" {
		t.Fatalf("自动检查失败应静默回到 idle，state=%s", got)
	}
	f.mgr.check(true)
	if got := f.mgr.Snapshot().State; got != "error" {
		t.Fatalf("手动检查失败应进入 error，state=%s", got)
	}
}

func TestAssetURLMustBeHTTPS(t *testing.T) {
	if err := validateAssetURL("http://evil.example.com/a.tar.gz"); err == nil {
		t.Fatal("非本地 http 地址应被拒绝")
	}
	for _, ok := range []string{"https://github.com/a/b.tar.gz", "http://127.0.0.1:8080/a.tar.gz", "http://localhost/a"} {
		if err := validateAssetURL(ok); err != nil {
			t.Fatalf("%s 应被接受: %v", ok, err)
		}
	}
}

func TestPackagePathCannotEscapeUpdatesDir(t *testing.T) {
	f := newFixture(t, "1.0.0", "1.1.0")
	info := &UpdateInfo{Asset: manifestPlatform{URL: "https://example.com/a/..%2F..%2Fevil.tar.gz"}}
	p, err := f.mgr.packagePath(info)
	if err == nil && filepath.Dir(p) != f.mgr.cfg.updatesDir {
		t.Fatalf("包路径逃出了更新目录: %s", p)
	}
}

func waitForState(t *testing.T, m *Manager, want string) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if m.Snapshot().State == want {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatalf("等待状态 %s 超时，当前 %s", want, m.Snapshot().State)
}
