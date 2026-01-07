package types

import (
	"slices"
	"sort"
	"strconv"
	"strings"
	"time"
)

const (
	ModStatusNew             ModStatus = 1
	ModStatusChangesRequired ModStatus = 2
	ModStatusUnderSoftReview ModStatus = 3
	ModStatusApproved        ModStatus = 4
	ModStatusRejected        ModStatus = 5
	ModStatusChangesMade     ModStatus = 6
	ModStatusInactive        ModStatus = 7
	ModStatusAbandoned       ModStatus = 8
	ModStatusDeleted         ModStatus = 9
	ModStatusUnderReview     ModStatus = 10
)

const (
	RelationTypeEmbeddedLibrary    RelationType = 1
	RelationTypeOptionalDependency RelationType = 2
	RelationTypeRequiredDependency RelationType = 3
	RelationTypeTool               RelationType = 4
	RelationTypeIncompatible       RelationType = 5
	RelationTypeInclude            RelationType = 6
)

const (
	HashAlgoSHA1 HashAlgo = 1
	HashAlgoMD5  HashAlgo = 2
)

const (
	FileReleaseTypeRelease FileReleaseType = 1
	FileReleaseTypeBeta    FileReleaseType = 2
	FileReleaseTypeAlpha   FileReleaseType = 3
)

const (
	FileStatusProcessing         = 1
	FileStatusChangesRequired    = 2
	FileStatusUnderReview        = 3
	FileStatusApproved           = 4
	FileStatusRejected           = 5
	FileStatusMalwareDetected    = 6
	FileStatusDeleted            = 7
	FileStatusArchived           = 8
	FileStatusTesting            = 9
	FileStatusReleased           = 10
	FileStatusReadyForReview     = 11
	FileStatusDeprecated         = 12
	FileStatusBaking             = 13
	FileStatusAwaitingPublishing = 14
	FileStatusFailedPublishing   = 15
)

type FileStatus int

func (f FileStatus) String() string {
	switch f {
	case FileStatusProcessing:
		return "processing"
	case FileStatusChangesRequired:
		return "changes-required"
	case FileStatusUnderReview:
		return "under-review"
	case FileStatusApproved:
		return "approved"
	case FileStatusRejected:
		return "rejected"
	case FileStatusMalwareDetected:
		return "malware-detected"
	case FileStatusDeleted:
		return "deleted"
	case FileStatusArchived:
		return "archived"
	case FileStatusTesting:
		return "testing"
	case FileStatusReleased:
		return "released"
	case FileStatusReadyForReview:
		return "ready-for-review"
	case FileStatusDeprecated:
		return "deprecated"
	case FileStatusBaking:
		return "baking"
	case FileStatusAwaitingPublishing:
		return "awaiting-publishing"
	case FileStatusFailedPublishing:
		return "failed-publishing"
	default:
		return "unknown"
	}
}

type FileReleaseType int

func (f FileReleaseType) String() string {
	switch f {
	case FileReleaseTypeRelease:
		return "Release"
	case FileReleaseTypeBeta:
		return "Beta"
	case FileReleaseTypeAlpha:
		return "Alpha"
	default:
		return "Unknown"
	}
}

type HashAlgo int

func (h HashAlgo) String() string {
	switch h {
	case HashAlgoSHA1:
		return "SHA1"
	case HashAlgoMD5:
		return "MD5"
	default:
		return "Unknown"
	}
}

type RelationType int

func (r RelationType) String() string {
	switch r {
	case RelationTypeEmbeddedLibrary:
		return "embedded library"
	case RelationTypeOptionalDependency:
		return "optional"
	case RelationTypeRequiredDependency:
		return "required"
	case RelationTypeTool:
		return "tool"
	case RelationTypeIncompatible:
		return "incompatible"
	case RelationTypeInclude:
		return "include"
	default:
		return "unknown"
	}
}

type ModStatus int

func (m ModStatus) String() string {
	switch m {
	case ModStatusNew:
		return "new"
	case ModStatusChangesRequired:
		return "changes-required"
	case ModStatusUnderReview:
		return "under-review"
	case ModStatusApproved:
		return "approved"
	case ModStatusRejected:
		return "rejected"
	case ModStatusChangesMade:
		return "changes-made"
	case ModStatusInactive:
		return "inactive"
	case ModStatusAbandoned:
		return "abandoned"
	case ModStatusDeleted:
		return "deleted"
	case ModStatusUnderSoftReview:
		return "under-soft-review"
	default:
		return "unknown"
	}
}

type GameVersionsResponse struct {
	RawResponse
	Data []GameVersionType `json:"data"`
}

type GameVersionType struct {
	Type     int          `json:"type"`
	Versions GameVersions `json:"versions"`
}

type GameVersions []GameVersion

func compareVersions(v1, v2 string) int {
	parts1 := strings.Split(v1, ".")
	parts2 := strings.Split(v2, ".")
	maxLen := len(parts1)
	if len(parts2) > maxLen {
		maxLen = len(parts2)
	}
	for i := 0; i < maxLen; i++ {
		var s1, s2 string
		if i < len(parts1) {
			s1 = parts1[i]
		}
		if i < len(parts2) {
			s2 = parts2[i]
		}

		n1, err1 := strconv.Atoi(s1)
		n2, err2 := strconv.Atoi(s2)

		if err1 == nil && err2 == nil {
			if n1 != n2 {
				if n1 > n2 {
					return 1
				}
				return -1
			}
		} else {
			if s1 != s2 {
				if s1 > s2 {
					return 1
				}
				return -1
			}
		}
	}
	return 0
}

func (v GameVersions) Sort() GameVersions {
	sort.Slice(v, func(i, j int) bool {
		return compareVersions(v[i].Name, v[j].Name) > 0
	})
	return v
}

type GameVersion struct {
	ID   int64  `json:"id"`
	Slug string `json:"slug"`
	Name string `json:"name"`
}

type ModsResponse struct {
	RawResponse
	Data       []ModData  `json:"data"`
	Pagination Pagination `json:"pagination"`
}

type GetModFilesResponse struct {
	RawResponse
	Data       Files      `json:"data"`
	Pagination Pagination `json:"pagination"`
}

type Links struct {
	WebsiteURL string `json:"websiteUrl"`
	WikiURL    string `json:"wikiUrl"`
	IssuesURL  string `json:"issuesUrl"`
	SourceURL  string `json:"sourceUrl"`
}

type Categories struct {
	ID               int64     `json:"id"`
	GameID           int64     `json:"gameId"`
	Name             string    `json:"name"`
	Slug             string    `json:"slug"`
	URL              string    `json:"url"`
	IconURL          string    `json:"iconUrl"`
	DateModified     time.Time `json:"dateModified"`
	IsClass          bool      `json:"isClass"`
	ClassID          int64     `json:"classId"`
	ParentCategoryID int64     `json:"parentCategoryId"`
	DisplayIndex     int64     `json:"displayIndex"`
}

type GetCategoriesResponse struct {
	RawResponse
	Data []Categories `json:"data"`
}

type Authors struct {
	ID   int64  `json:"id"`
	Name string `json:"name"`
	URL  string `json:"url"`
}

type Logo struct {
	ID           int64  `json:"id"`
	ModID        int64  `json:"modId"`
	Title        string `json:"title"`
	Description  string `json:"description"`
	ThumbnailURL string `json:"thumbnailUrl"`
	URL          string `json:"url"`
}

type Screenshots struct {
	ID           int64  `json:"id"`
	ModID        int64  `json:"modId"`
	Title        string `json:"title"`
	Description  string `json:"description"`
	ThumbnailURL string `json:"thumbnailUrl"`
	URL          string `json:"url"`
}

type Hashes struct {
	Value string   `json:"value"`
	Algo  HashAlgo `json:"algo"`
}

type SortableGameVersions struct {
	GameVersionName        string    `json:"gameVersionName"`
	GameVersionPadded      string    `json:"gameVersionPadded"`
	GameVersion            string    `json:"gameVersion"`
	GameVersionReleaseDate time.Time `json:"gameVersionReleaseDate"`
	GameVersionTypeID      int       `json:"gameVersionTypeId"`
}

type Dependencies []Dependency

func (d Dependencies) ModIDList() []int64 {
	var modIDs []int64
	for _, dep := range d {
		modIDs = append(modIDs, dep.ModID)
	}
	return modIDs
}

func (d Dependencies) ModIDListByDependencyType(dt RelationType) []int64 {
	var modIDs []int64
	for _, dep := range d {
		if dep.RelationType == dt {
			modIDs = append(modIDs, dep.ModID)
		}
	}
	return modIDs
}

type Dependency struct {
	ModID        int64        `json:"modId"`
	RelationType RelationType `json:"relationType"`
}

type Modules struct {
	Name        string `json:"name"`
	Fingerprint int64  `json:"fingerprint"`
}

type Files []File

type File struct {
	ID                   int                    `json:"id"`
	GameID               int                    `json:"gameId"`
	ModID                int                    `json:"modId"`
	IsAvailable          bool                   `json:"isAvailable"`
	DisplayName          string                 `json:"displayName"`
	FileName             string                 `json:"fileName"`
	ReleaseType          int                    `json:"releaseType"`
	FileStatus           int                    `json:"fileStatus"`
	Hashes               []Hashes               `json:"hashes"`
	FileDate             time.Time              `json:"fileDate"`
	FileLength           int                    `json:"fileLength"`
	DownloadCount        int                    `json:"downloadCount"`
	FileSizeOnDisk       int                    `json:"fileSizeOnDisk"`
	DownloadURL          string                 `json:"downloadUrl"`
	GameVersions         []string               `json:"gameVersions"`
	SortableGameVersions []SortableGameVersions `json:"sortableGameVersions"`
	Dependencies         Dependencies           `json:"dependencies"`
	ExposeAsAlternative  bool                   `json:"exposeAsAlternative"`
	ParentProjectFileID  int                    `json:"parentProjectFileId"`
	AlternateFileID      int                    `json:"alternateFileId"`
	IsServerPack         bool                   `json:"isServerPack"`
	ServerPackFileID     int                    `json:"serverPackFileId"`
	IsEarlyAccessContent bool                   `json:"isEarlyAccessContent"`
	EarlyAccessEndDate   time.Time              `json:"earlyAccessEndDate"`
	FileFingerprint      int                    `json:"fileFingerprint"`
	Modules              []Modules              `json:"modules"`
}

func (f *File) IsCompatible(gameVersion, modLoader string) bool {
	gameVerCompat := false
	modLoaderCompat := false
	for _, v := range f.GameVersions {
		if strings.ToLower(v) == strings.ToLower(gameVersion) {
			gameVerCompat = true
		}
		if strings.ToLower(v) == strings.ToLower(modLoader) {
			modLoaderCompat = true
		}
		if gameVerCompat && modLoaderCompat {
			return true
		}
	}

	return false
}

type LatestFilesIndexes struct {
	GameVersion       string `json:"gameVersion"`
	FileID            int64  `json:"fileId"`
	Filename          string `json:"filename"`
	ReleaseType       int    `json:"releaseType"`
	GameVersionTypeID int    `json:"gameVersionTypeId"`
	ModLoader         int    `json:"modLoader"`
}

type LatestEarlyAccessFilesIndexes struct {
	GameVersion       string `json:"gameVersion"`
	FileID            int64  `json:"fileId"`
	Filename          string `json:"filename"`
	ReleaseType       int    `json:"releaseType"`
	GameVersionTypeID int    `json:"gameVersionTypeId"`
	ModLoader         int    `json:"modLoader"`
}

type ModData struct {
	ID                            int64                           `json:"id"`
	GameID                        int                             `json:"gameId"`
	Name                          string                          `json:"name"`
	Slug                          string                          `json:"slug"`
	Links                         Links                           `json:"links"`
	Summary                       string                          `json:"summary"`
	Status                        ModStatus                       `json:"status"`
	DownloadCount                 int                             `json:"downloadCount"`
	IsFeatured                    bool                            `json:"isFeatured"`
	PrimaryCategoryID             int                             `json:"primaryCategoryId"`
	Categories                    []Categories                    `json:"categories"`
	ClassID                       int                             `json:"classId"`
	Authors                       []Authors                       `json:"authors"`
	Logo                          Logo                            `json:"logo"`
	Screenshots                   []Screenshots                   `json:"screenshots"`
	MainFileID                    int                             `json:"mainFileId"`
	LatestFiles                   Files                           `json:"latestFiles"`
	LatestFilesIndexes            []LatestFilesIndexes            `json:"latestFilesIndexes"`
	LatestEarlyAccessFilesIndexes []LatestEarlyAccessFilesIndexes `json:"latestEarlyAccessFilesIndexes"`
	DateCreated                   time.Time                       `json:"dateCreated"`
	DateModified                  time.Time                       `json:"dateModified"`
	DateReleased                  time.Time                       `json:"dateReleased"`
	AllowModDistribution          bool                            `json:"allowModDistribution"`
	GamePopularityRank            int64                           `json:"gamePopularityRank"`
	IsAvailable                   bool                            `json:"isAvailable"`
	ThumbsUpCount                 int64                           `json:"thumbsUpCount"`
	Rating                        int64                           `json:"rating"`
}

type Pagination struct {
	Index       int64 `json:"index"`
	PageSize    int64 `json:"pageSize"`
	ResultCount int64 `json:"resultCount"`
	TotalCount  int64 `json:"totalCount"`
}

func (m *ModData) GetLatestFile() *File {
	if len(m.LatestFiles) < 1 {
		return nil
	}
	return &m.LatestFiles[0]
}

func (m *ModData) GetLatestFileGameVersions() []string {
	f := m.GetLatestFile()
	if f == nil {
		return nil
	}

	return f.GameVersions
}

func (m *ModData) GetLatestFileByGameVersion(gv string) *File {

	for _, f := range m.LatestFiles {
		if slices.Contains(f.GameVersions, gv) {
			return &f
		}
	}

	return nil
}

func (m *ModData) GetLatestFileByGameVersionsAndModloader(gv, ml string) *File {
	for _, f := range m.LatestFiles {
		if f.IsCompatible(gv, ml) {
			return &f
		}
	}
	return nil
}

func (f *File) GetDependencyModIDList() []Dependency {
	var modIDs []Dependency
	for _, d := range f.Dependencies {
		modIDs = append(modIDs, d)
	}
	return modIDs
}

type ModLoaderDetailsResponse struct {
	RawResponse
	Data ModloaderDetails `json:"data"`
}

type ModloaderDetails struct {
	ID                             int       `json:"id"`
	GameVersionID                  int       `json:"gameVersionId"`
	MinecraftGameVersionID         int       `json:"minecraftGameVersionId"`
	ForgeVersion                   string    `json:"forgeVersion"`
	Name                           string    `json:"name"`
	Type                           int       `json:"type"`
	DownloadURL                    string    `json:"downloadUrl"`
	Filename                       string    `json:"filename"`
	InstallMethod                  int       `json:"installMethod"`
	Latest                         bool      `json:"latest"`
	Recommended                    bool      `json:"recommended"`
	Approved                       bool      `json:"approved"`
	DateModified                   time.Time `json:"dateModified"`
	MavenVersionString             string    `json:"mavenVersionString"`
	VersionJSON                    string    `json:"versionJson"`
	LibrariesInstallLocation       string    `json:"librariesInstallLocation"`
	MinecraftVersion               string    `json:"minecraftVersion"`
	AdditionalFilesJSON            string    `json:"additionalFilesJson"`
	ModLoaderGameVersionID         int       `json:"modLoaderGameVersionId"`
	ModLoaderGameVersionTypeID     int       `json:"modLoaderGameVersionTypeId"`
	ModLoaderGameVersionStatus     int       `json:"modLoaderGameVersionStatus"`
	ModLoaderGameVersionTypeStatus int       `json:"modLoaderGameVersionTypeStatus"`
	McGameVersionID                int       `json:"mcGameVersionId"`
	McGameVersionTypeID            int       `json:"mcGameVersionTypeId"`
	McGameVersionStatus            int       `json:"mcGameVersionStatus"`
	McGameVersionTypeStatus        int       `json:"mcGameVersionTypeStatus"`
	InstallProfileJSON             string    `json:"installProfileJson"`
}
