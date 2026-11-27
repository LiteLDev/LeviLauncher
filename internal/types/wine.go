package types

// WineSettings is used by native macOS hosts only. Windows never reads
// or writes these settings and continues to launch the game directly.
type WineSettings struct {
	App               string   `json:"app"`
	Executable        string   `json:"executable"`
	Prefix            string   `json:"prefix"`
	Graphics          string   `json:"graphics"`
	D3DMetalVersion   string   `json:"d3dMetalVersion,omitempty"`   // auto | gptk4 | gptk3
	EnableMetal4      bool     `json:"enableMetal4,omitempty"`      // false (default) forces Metal 3 compatibility on macOS 27+
	Sync              string   `json:"sync"`
	ThreadQoS         string   `json:"threadQos,omitempty"`         // balanced | aggressive | default
	DisableRosettaAVX bool     `json:"disableRosettaAvx,omitempty"` // false (default) enables ROSETTA_ADVERTISE_AVX=1
	LeftCommandIsCtrl bool     `json:"leftCommandIsCtrl,omitempty"`
	RetinaMode        bool     `json:"retinaMode,omitempty"`
	MetalHUD          bool     `json:"metalHud,omitempty"`
	VSync             string   `json:"vsync,omitempty"`             // auto | on | off (native CAMetalLayer displaySyncEnabled)
	MaxFPS            int      `json:"maxFps,omitempty"`
	EnableMetalFX     bool     `json:"enableMetalFx,omitempty"`     // kept for backwards compatibility; derived from MetalFXScale
	MetalFXScale      string   `json:"metalFxScale,omitempty"`      // off | quality | balanced | performance | ultra_performance | extreme | max
	CompatDebugLog    bool     `json:"compatDebugLog,omitempty"`    // enables XGR_TRACE=1 & MCWINRT_TRACE=1
	Runtime           string   `json:"runtime,omitempty"`           // derived from the selected executable
	D3DMetalVersions  []string `json:"d3dMetalVersions,omitempty"`  // derived available D3DMetal selector options
}

type WineShaderCacheInfo struct {
	Path      string `json:"path"`
	SizeBytes int64  `json:"sizeBytes"`
	FileCount int    `json:"fileCount"`
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

type LegacyWineInfo struct {
	Active    bool   `json:"active"`
	Present   bool   `json:"present"`
	Path      string `json:"path"`
	SizeBytes int64  `json:"sizeBytes"`
}
