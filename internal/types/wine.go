package types

// WineSettings is used by native macOS/Linux hosts only. Windows never reads
// or writes these settings and continues to launch the game directly.
type WineSettings struct {
	App        string `json:"app"`
	Executable string `json:"executable"`
	Prefix     string `json:"prefix"`
	Graphics   string `json:"graphics"`
	Sync       string `json:"sync"`
}
