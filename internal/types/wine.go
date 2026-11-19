package types

// WineSettings is used by native macOS hosts only. Windows never reads
// or writes these settings and continues to launch the game directly.
type WineSettings struct {
	App        string `json:"app"`
	Executable string `json:"executable"`
	Prefix     string `json:"prefix"`
	Graphics   string `json:"graphics"`
	Sync       string `json:"sync"`
}

type WineInstallation struct {
	Name     string       `json:"name"`
	Settings WineSettings `json:"settings"`
	Status   string       `json:"status"` // ready, needs_prefix, unsupported
}

type WineDownloadSettings struct {
	Mirror      bool   `json:"mirror"`
	GithubProxy string `json:"githubProxy"`
}

type WineInstallProgress struct {
	Stage      string `json:"stage"`
	Status     string `json:"status"`
	Downloaded int64  `json:"downloaded"`
	Total      int64  `json:"total"`
}
