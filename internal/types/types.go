package types

type VersionJson struct {
	Name        string `json:"name"`
	Uuid        string `json:"uuid"`
	Version     string `json:"version"`
	IsPreview   bool   `json:"isPreview"`
	IsPreLoader bool   `json:"isPreLoader"`
}

type MinecraftVersion struct {
	Version   string `json:"version"`
	Uuid      string `json:"uuid"`
	Type      int    `json:"type"`
	SupportPL bool   `json:"supportPL"`
}

type LocalVersion struct {
	Name        string `json:"name"`
	Uuid        string `json:"uuid"`
	Path        string `json:"path"`
	Version     string `json:"version"`
	IsLaunched  bool   `json:"isLaunched"`
	IsPreview   bool   `json:"isPreview"`
	IsPreLoader bool   `json:"isPreLoader"`
}

type PreloaderJson struct {
	ColorLog bool   `json:"colorLog"`
	LogLevel int    `json:"logLevel"`
	LogPath  string `json:"logPath"`
	ModsPath string `json:"modsPath"`
	Version  int    `json:"version"`
}

type ModManifestJson struct {
	Name    string `json:"name"`
	Entry   string `json:"entry"`
	Version string `json:"version"`
	Type    string `json:"type"`
	Author  string `json:"author,omitempty"`
}

type ModInfo struct {
	Name    string `json:"name"`
	Entry   string `json:"entry"`
	Version string `json:"version"`
	Type    string `json:"type"`
	Author  string `json:"author,omitempty"`
}

type LanguageJson struct {
	Code     string `json:"code"`
	Language string `json:"language"`
}

type CheckUpdate struct {
	IsUpdate bool   `json:"isUpdate"`
	Version  string `json:"version"`
	Body     string `json:"body"`
}

type FileEntry struct {
	Name  string `json:"name"`
	Path  string `json:"path"`
	IsDir bool   `json:"isDir"`
	Size  int64  `json:"size"`
}

type MirrorTestResult struct {
	URL       string `json:"url"`
	LatencyMs int64  `json:"latencyMs"`
	Ok        bool   `json:"ok"`
	Status    int    `json:"status"`
	Error     string `json:"error,omitempty"`
}

type ContentRoots struct {
	Base          string `json:"base"`
	UsersRoot     string `json:"usersRoot"`
	ResourcePacks string `json:"resourcePacks"`
	BehaviorPacks string `json:"behaviorPacks"`
	IsIsolation   bool   `json:"isIsolation"`
	IsPreview     bool   `json:"isPreview"`
}

type PackInfo struct {
	Name             string `json:"name"`
	Description      string `json:"description"`
	Version          string `json:"version"`
	MinEngineVersion string `json:"minEngineVersion"`
	IconDataUrl      string `json:"iconDataUrl"`
	Path             string `json:"path"`
}

type LevelDatField struct {
	Name        string   `json:"name"`
	Tag         string   `json:"tag"`
	ValueString string   `json:"valueString"`
	ValueJSON   string   `json:"valueJSON"`
	IsBoolLike  bool     `json:"isBoolLike"`
	InData      bool     `json:"inData"`
	Path        []string `json:"path,omitempty"`
}

type ExtractProgress struct {
	Dir   string `json:"dir"`
	Files int64  `json:"files"`
	Bytes int64  `json:"bytes"`
	Ts    int64  `json:"ts"`
}
