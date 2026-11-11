package updater

const (
	EventState    = "update:state"
	EventProgress = "update:progress"
	EventReady    = "update:ready"
	EventError    = "update:error"
)

// 面向前端的消息码，由前端 i18n 翻译；后端不再下发中文文案。
const (
	MessageUpToDate           = "up_to_date"
	MessageBusy               = "busy"
	MessageInstallUnsupported = "install_unsupported"
	MessageNoUpdate           = "no_update"
	MessageNoPlatformAsset    = "no_platform_asset"
	MessageNetwork            = "network"
	MessageChecksum           = "checksum"
	MessageDownloadFailed     = "download_failed"
	MessageInstallFailed      = "install_failed"
	MessageManifestInvalid    = "manifest_invalid"
)

type StatePayload struct {
	State        string  `json:"state"`
	Version      string  `json:"version,omitempty"`
	ReleaseDate  string  `json:"releaseDate,omitempty"`
	ReleaseNotes string  `json:"releaseNotes,omitempty"`
	Mandatory    bool    `json:"mandatory,omitempty"`
	Downloaded   int64   `json:"downloaded,omitempty"`
	Total        int64   `json:"total,omitempty"`
	Percentage   float64 `json:"percentage,omitempty"`
	Error        string  `json:"error,omitempty"`
	MessageCode  string  `json:"messageCode,omitempty"`
	Prompt       bool    `json:"prompt,omitempty"`
	PromptKind   string  `json:"promptKind,omitempty"`
	ReleaseURL   string  `json:"releaseUrl,omitempty"`
	CanInstall   bool    `json:"canInstall,omitempty"`
	// Manual 表示该事件由用户主动检查触发，前端应当给出可见反馈（即使没有新版本）。
	Manual bool `json:"manual,omitempty"`
}

type ProgressPayload struct {
	State      string  `json:"state"`
	Version    string  `json:"version,omitempty"`
	Downloaded int64   `json:"downloaded"`
	Total      int64   `json:"total"`
	Percentage float64 `json:"percentage"`
}

type ReadyPayload struct {
	State        string `json:"state"`
	Version      string `json:"version,omitempty"`
	ReleaseDate  string `json:"releaseDate,omitempty"`
	ReleaseNotes string `json:"releaseNotes,omitempty"`
	Mandatory    bool   `json:"mandatory,omitempty"`
	Prompt       bool   `json:"prompt,omitempty"`
	PromptKind   string `json:"promptKind,omitempty"`
	ReleaseURL   string `json:"releaseUrl,omitempty"`
	CanInstall   bool   `json:"canInstall,omitempty"`
}

type ErrorPayload struct {
	State       string `json:"state"`
	Version     string `json:"version,omitempty"`
	Error       string `json:"error,omitempty"`
	MessageCode string `json:"messageCode,omitempty"`
	Prompt      bool   `json:"prompt,omitempty"`
	PromptKind  string `json:"promptKind,omitempty"`
}
