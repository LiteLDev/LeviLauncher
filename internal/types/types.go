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
	Folder  string `json:"folder"`
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

type InstanceBackupScope struct {
	Key         string                    `json:"key"`
	Label       string                    `json:"label"`
	Path        string                    `json:"path"`
	Size        int64                     `json:"size"`
	Selectable  bool                      `json:"selectable"`
	Exists      bool                      `json:"exists"`
	Shared      bool                      `json:"shared"`
	Modes       []InstanceBackupScopeMode `json:"modes,omitempty"`
	DefaultMode string                    `json:"defaultMode"`
	Warnings    []string                  `json:"warnings,omitempty"`
}

type InstanceBackupScopeMode struct {
	Key        string `json:"key"`
	Path       string `json:"path"`
	Size       int64  `json:"size"`
	Selectable bool   `json:"selectable"`
	Warning    string `json:"warning,omitempty"`
}

type InstanceBackupModsLIPPackage struct {
	Identifier        string   `json:"identifier"`
	Version           string   `json:"version"`
	ExplicitInstalled bool     `json:"explicitInstalled"`
	Folders           []string `json:"folders"`
}

type InstanceBackupRequest struct {
	Scopes          []string                       `json:"scopes"`
	ScopeModes      map[string]string              `json:"scopeModes"`
	ModsLIPPackages []InstanceBackupModsLIPPackage `json:"modsLipPackages"`
}

type InstanceBackupInfo struct {
	Name      string                `json:"name"`
	BackupDir string                `json:"backupDir"`
	Scopes    []InstanceBackupScope `json:"scopes"`
	ErrorCode string                `json:"errorCode"`
}

type InstanceBackupResult struct {
	ArchivePath    string   `json:"archivePath"`
	BackupDir      string   `json:"backupDir"`
	IncludedScopes []string `json:"includedScopes"`
	ErrorCode      string   `json:"errorCode"`
}

type InstanceBackupArchiveScope struct {
	Key      string   `json:"key"`
	Label    string   `json:"label"`
	Mode     string   `json:"mode"`
	Warnings []string `json:"warnings,omitempty"`
}

type InstanceBackupArchiveInfo struct {
	FormatVersion         int                            `json:"formatVersion"`
	Name                  string                         `json:"name"`
	ArchivePath           string                         `json:"archivePath"`
	ArchiveName           string                         `json:"archiveName"`
	GameVersion           string                         `json:"gameVersion"`
	Type                  string                         `json:"type"`
	EnableIsolation       bool                           `json:"enableIsolation"`
	CreatedAt             string                         `json:"createdAt"`
	IncludedScopes        []string                       `json:"includedScopes"`
	ScopeModes            map[string]string              `json:"scopeModes"`
	Scopes                []InstanceBackupArchiveScope   `json:"scopes"`
	ModsLIPPackages       []InstanceBackupModsLIPPackage `json:"modsLipPackages"`
	RawModFolders         []string                       `json:"rawModFolders"`
	BedrockWhitelistRoots []string                       `json:"bedrockWhitelistRoots"`
	ErrorCode             string                         `json:"errorCode"`
}

type InstanceBackupRestoreRequest struct {
	ArchivePath         string                            `json:"archivePath"`
	Scopes              []string                          `json:"scopes"`
	ConflictResolutions []InstanceBackupRestoreResolution `json:"conflictResolutions,omitempty"`
}

type InstanceBackupRestoreResolution struct {
	ConflictID string `json:"conflictId"`
	Choice     string `json:"choice"`
}

type InstanceBackupRestoreConflictDiffField struct {
	Key          string `json:"key"`
	Label        string `json:"label"`
	BackupValue  string `json:"backupValue"`
	CurrentValue string `json:"currentValue"`
}

type InstanceBackupRestoreConflict struct {
	ID             string                                   `json:"id"`
	ScopeKey       string                                   `json:"scopeKey"`
	ScopeLabel     string                                   `json:"scopeLabel"`
	Path           string                                   `json:"path"`
	SourceType     string                                   `json:"sourceType"`
	TargetType     string                                   `json:"targetType"`
	IdentityKind   string                                   `json:"identityKind"`
	IdentityKey    string                                   `json:"identityKey"`
	BackupPath     string                                   `json:"backupPath"`
	CurrentPath    string                                   `json:"currentPath"`
	BackupSummary  string                                   `json:"backupSummary"`
	CurrentSummary string                                   `json:"currentSummary"`
	DiffFields     []InstanceBackupRestoreConflictDiffField `json:"diffFields,omitempty"`
}

type InstanceBackupRestoreConflictInfo struct {
	ArchivePath    string                          `json:"archivePath"`
	IncludedScopes []string                        `json:"includedScopes"`
	Conflicts      []InstanceBackupRestoreConflict `json:"conflicts"`
	ErrorCode      string                          `json:"errorCode"`
}

type InstanceBackupRestoreScopeResult struct {
	Key       string   `json:"key"`
	Label     string   `json:"label"`
	Mode      string   `json:"mode"`
	Status    string   `json:"status"`
	ErrorCode string   `json:"errorCode"`
	Warnings  []string `json:"warnings,omitempty"`
	Details   []string `json:"details,omitempty"`
}

type InstanceBackupRestoreResult struct {
	ArchivePath    string                             `json:"archivePath"`
	Status         string                             `json:"status"`
	IncludedScopes []string                           `json:"includedScopes"`
	ScopeResults   []InstanceBackupRestoreScopeResult `json:"scopeResults"`
	ErrorCode      string                             `json:"errorCode"`
}

type InstanceBackupRestoreProgress struct {
	Phase       string `json:"phase"`
	CurrentStep int    `json:"currentStep"`
	TotalSteps  int    `json:"totalSteps"`
	ScopeKey    string `json:"scopeKey,omitempty"`
	ScopeLabel  string `json:"scopeLabel,omitempty"`
	Ts          int64  `json:"ts"`
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

type ProcessInfo struct {
	Pid         int    `json:"pid"`
	ExePath     string `json:"exePath"`
	IsLauncher  bool   `json:"isLauncher"`
	VersionName string `json:"versionName"`
}

type ExtractProgress struct {
	Dir           string `json:"dir"`
	Files         int64  `json:"files"`
	Bytes         int64  `json:"bytes"`
	TotalBytes    int64  `json:"totalBytes"`
	GlobalCurrent int64  `json:"global_current"`
	GlobalTotal   int64  `json:"global_total"`
	CurrentFile   string `json:"currentFile"`
	Ts            int64  `json:"ts"`
}

type Server struct {
	Index     string `json:"index"`
	Name      string `json:"name"`
	IP        string `json:"ip"`
	Port      string `json:"port"`
	Timestamp int64  `json:"timestamp"`
}

type FilesDroppedEvent struct {
	Files  []string `json:"files"`
	Target string   `json:"target"`
}

type LIPPackageInstallState struct {
	Identifier        string `json:"identifier"`
	PackageRef        string `json:"packageRef"`
	Installed         bool   `json:"installed"`
	ExplicitInstalled bool   `json:"explicitInstalled"`
	InstalledVersion  string `json:"installedVersion"`
	Error             string `json:"error"`
}

type LIPPackageInstallStateEntry struct {
	IdentifierKey string                 `json:"identifierKey"`
	State         LIPPackageInstallState `json:"state"`
}
