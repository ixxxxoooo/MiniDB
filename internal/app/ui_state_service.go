package app

import (
	"errors"

	"minidb/internal/logger"
	"minidb/internal/storage"
)

// 界面状态存放的桶与键。集中在这里，界面层不再自己拼桶名。
const (
	uiSettingsBucket = "settings"
	uiStateBucket    = "workspace_state"
	uiPinsBucket     = "pins"
	uiColumnsBucket  = "column_layout"
	uiERBucket       = "er_layout"

	uiLayoutKey  = "ui_layout"
	uiPrefsKey   = "ui_prefs"
	uiSessionKey = "open_session"
)

// UIStateService 保存界面自己的状态：布局、偏好、上次会话、各库的工作区状态和置顶。
// 值的类型由界面层定义，这里只管存取；失败时记日志并把错误返回。
type UIStateService struct {
	store *storage.Store
}

// NewUIStateService 创建界面状态服务。
func NewUIStateService(store *storage.Store) *UIStateService {
	return &UIStateService{store: store}
}

// IsNotFound 判断错误是否只是"还没存过"，这种情况不用当故障处理。
func IsNotFound(err error) bool { return errors.Is(err, storage.ErrKeyNotFound) }

// load 读出一个值；没存过时返回 ErrKeyNotFound 包装的错误，且不记日志。
func (s *UIStateService) load(bucket, key string, dst any) error {
	err := s.store.Get(bucket, key, dst)
	if err != nil && !IsNotFound(err) {
		logger.Warn("[UIState] 读取 %s/%s 失败: %v", bucket, key, err)
	}
	return err
}

func (s *UIStateService) save(bucket, key string, v any) error {
	err := s.store.Put(bucket, key, v)
	if err != nil {
		logger.Warn("[UIState] 写入 %s/%s 失败: %v", bucket, key, err)
	}
	return err
}

// LoadLayout 读出窗口布局（侧栏宽度、面板等）到 dst。
func (s *UIStateService) LoadLayout(dst any) error { return s.load(uiSettingsBucket, uiLayoutKey, dst) }

// SaveLayout 保存窗口布局。
func (s *UIStateService) SaveLayout(v any) error { return s.save(uiSettingsBucket, uiLayoutKey, v) }

// LoadPrefs 读出界面偏好（主题、页大小等）到 dst。
func (s *UIStateService) LoadPrefs(dst any) error { return s.load(uiSettingsBucket, uiPrefsKey, dst) }

// SavePrefs 保存界面偏好。
func (s *UIStateService) SavePrefs(v any) error { return s.save(uiSettingsBucket, uiPrefsKey, v) }

// LoadSession 读出上次退出时打开的连接列表到 dst。
func (s *UIStateService) LoadSession(dst any) error {
	return s.load(uiSettingsBucket, uiSessionKey, dst)
}

// SaveSession 保存当前打开的连接列表。
func (s *UIStateService) SaveSession(v any) error { return s.save(uiSettingsBucket, uiSessionKey, v) }

// LoadWorkspaceState 读出某个键（库键或连接 ID）下保存的工作区状态到 dst。
func (s *UIStateService) LoadWorkspaceState(key string, dst any) error {
	return s.load(uiStateBucket, key, dst)
}

// SaveWorkspaceState 把工作区状态存在 key 下。
func (s *UIStateService) SaveWorkspaceState(key string, v any) error {
	return s.save(uiStateBucket, key, v)
}

// LoadPins 读出某个库的置顶列表到 dst。
func (s *UIStateService) LoadPins(key string, dst any) error { return s.load(uiPinsBucket, key, dst) }

// SavePins 保存某个库的置顶列表。
func (s *UIStateService) SavePins(key string, v any) error { return s.save(uiPinsBucket, key, v) }

// LoadColumns 读出某张表的列布局（宽度、顺序、隐藏与固定的列）到 dst。key 由连接、库和表名拼成。
func (s *UIStateService) LoadColumns(key string, dst any) error {
	return s.load(uiColumnsBucket, key, dst)
}

// SaveColumns 保存某张表的列布局。
func (s *UIStateService) SaveColumns(key string, v any) error {
	return s.save(uiColumnsBucket, key, v)
}

// LoadER 读出某个库的 ER 图布局（拖动过的表位置）到 dst。
func (s *UIStateService) LoadER(key string, dst any) error { return s.load(uiERBucket, key, dst) }

// SaveER 保存某个库的 ER 图布局。
func (s *UIStateService) SaveER(key string, v any) error { return s.save(uiERBucket, key, v) }
