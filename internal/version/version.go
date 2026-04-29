package version

import "strings"

const (
	AppName     = "TablePlus AI"
	CompanyName = "Jason"
	Description = "AI enhanced database management tool"
	Repository  = "lwj1989/tableplus-ai"
	DataDirName = ".tableplus-ai"
)

var (
	Version   = "0.0.1"
	Commit    = "dev"
	BuildDate = "dev"
)

func CurrentVersion() string {
	version := strings.TrimSpace(strings.TrimPrefix(Version, "v"))
	if version == "" {
		return "0.0.0"
	}
	return version
}

func ReleaseTag() string {
	return "v" + CurrentVersion()
}

func ReleasePageURL() string {
	return "https://github.com/" + Repository + "/releases"
}

func UpdateManifestURL() string {
	return "https://github.com/" + Repository + "/releases/latest/download/update.json"
}
