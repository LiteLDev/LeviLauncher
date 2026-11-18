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

type WineInstallPlan struct {
	CanInstall bool   `json:"canInstall"`
	Command    string `json:"command"`
	URL        string `json:"url"`
}
