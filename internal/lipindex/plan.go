package lipindex

import (
	"sort"
	"strings"
)

// Install actions.
const (
	ActionInstall   = "install"
	ActionUpgrade   = "upgrade"
	ActionDowngrade = "downgrade"
	ActionReinstall = "reinstall"
	// ActionPromote installs a package that is present only as a dependency
	// at the same version, marking it as explicitly installed.
	ActionPromote = "promote"
)

// Reasons a plan cannot run.
const (
	BlockLoaderDBUnavailable  = "ERR_FETCH_LL_DB"
	BlockGameUnsupported      = "ERR_LIP_GAME_VERSION_UNSUPPORTED"
	BlockLoaderCheckFailed    = "ERR_LIP_PACKAGE_QUERY_FAILED"
	BlockNoCompatibleLoader   = "ERR_LL_NO_COMPATIBLE_VERSION"
	BlockLoaderVersionInvalid = "ERR_LL_VERSION_UNSUPPORTED"
	BlockVersionNotFound      = "ERR_LIP_PACKAGE_VERSION_NOT_FOUND"
)

// PlanInput is what deciding an install needs (LIPInstallDialog).
type PlanInput struct {
	Variant Variant
	// Version is the requested version; "" picks the newest version the
	// instance's game supports, else the newest.
	Version string
	// GameVersion is the instance's game version ("1.21.50.07").
	GameVersion string
	// DB is the LeviLamina version database; nil when it could not be
	// loaded.
	DB LoaderDB
	// Loader and Pkg are the workspace states of LeviLamina and the package.
	Loader InstallState
	Pkg    InstallState
	// LoaderVersion is the requested LeviLamina version ("" = default).
	LoaderVersion string
}

// Plan is the decision for one install.
type Plan struct {
	Version string
	File    File
	// RequiresLoader: the version depends on LeviLamina.
	RequiresLoader bool
	// GameCompatible: the instance's game has a LeviLamina version the
	// package accepts (or it needs none).
	GameCompatible bool
	// SupportedGames are the game versions the chosen version supports.
	SupportedGames []string
	// LoaderCompatible: the installed LeviLamina satisfies the version.
	LoaderCompatible bool
	// NeedsLoader: LeviLamina must be installed or changed first, to
	// LoaderVersion; ReplacesLoader when one is installed already.
	NeedsLoader      bool
	ReplacesLoader   bool
	LoaderDowngrade  bool
	LoaderOptions    []string
	LoaderVersion    string
	InstalledLoader  string
	Action           string
	InstalledVersion string
	// Blocker is set when the install cannot run (a Block* code).
	Blocker string
}

func (in PlanInput) supportsGame(f File, game string) bool {
	if len(f.LLRanges) == 0 {
		return true
	}
	if len(in.DB) == 0 || game == "" {
		return false
	}
	return contains(SupportedGameVersions(f.LLRanges, in.DB), game)
}

// SortedFiles returns a variant's files newest first by dotted compare, as
// the install dialog lists them.
func SortedFiles(v Variant) []File {
	files := append([]File(nil), v.Files...)
	sort.SliceStable(files, func(i, j int) bool { return CompareVersions(files[i].Version, files[j].Version) > 0 })
	return files
}

// comparePlanVersions orders two versions by semantic version precedence
// when both parse, so a pre-release ranks below its release, and by the
// window's dotted compare otherwise.
func comparePlanVersions(a, b string) int {
	if _, ok := ParseSemver(a); ok {
		if _, ok := ParseSemver(b); ok {
			return CompareSemverLike(a, b)
		}
	}
	return CompareVersions(a, b)
}

// Decide works out what installing a package version means for an
// instance.
func Decide(in PlanInput) Plan {
	game := NormalizeSemverInput(in.GameVersion)
	files := SortedFiles(in.Variant)
	var p Plan
	var file *File
	want := strings.TrimSpace(in.Version)
	if want != "" {
		for i := range files {
			if strings.EqualFold(NormalizeSemverInput(files[i].Version), NormalizeSemverInput(want)) {
				file = &files[i]
				break
			}
		}
		if file == nil {
			p.Version = want
			p.Blocker = BlockVersionNotFound
			return p
		}
	} else {
		// Prefer a stable release, so a pre-release the dotted order ranks
		// above its own release (1.0.0-rc.1 > 1.0.0) is not picked.
		for i := range files {
			if IsStable(files[i].Version) && in.supportsGame(files[i], game) {
				file = &files[i]
				break
			}
		}
		for i := 0; file == nil && i < len(files); i++ {
			if in.supportsGame(files[i], game) {
				file = &files[i]
			}
		}
		if file == nil && len(files) > 0 {
			file = &files[0]
		}
		if file == nil {
			p.Blocker = BlockVersionNotFound
			return p
		}
	}
	p.File, p.Version = *file, file.Version
	p.RequiresLoader = len(file.LLRanges) > 0
	p.GameCompatible = in.supportsGame(*file, game)
	if p.RequiresLoader && len(in.DB) > 0 {
		p.SupportedGames = SupportedGameVersions(file.LLRanges, in.DB)
	}
	p.InstalledLoader = strings.TrimSpace(in.Loader.InstalledVersion)
	llInstalled := in.Loader.Installed
	p.LoaderCompatible = llInstalled && p.InstalledLoader != "" && Compatible(p.InstalledLoader, file.LLRanges)
	checkFailed := p.RequiresLoader && !llInstalled && strings.TrimSpace(in.Loader.Error) != ""
	p.NeedsLoader = p.RequiresLoader && !checkFailed && !p.LoaderCompatible
	p.ReplacesLoader = p.NeedsLoader && llInstalled
	if p.RequiresLoader && game != "" && len(in.DB) > 0 {
		for _, v := range in.DB.Versions(game) {
			if Compatible(v, file.LLRanges) {
				p.LoaderOptions = append(p.LoaderOptions, v)
			}
		}
	}
	if p.NeedsLoader {
		if want := strings.TrimSpace(in.LoaderVersion); want != "" {
			for _, v := range p.LoaderOptions {
				if strings.EqualFold(NormalizeSemverInput(v), NormalizeSemverInput(want)) {
					p.LoaderVersion = v
				}
			}
		} else {
			for _, v := range p.LoaderOptions {
				if !IsReleaseCandidate(v) {
					p.LoaderVersion = v
					break
				}
			}
			if p.LoaderVersion == "" && len(p.LoaderOptions) > 0 {
				p.LoaderVersion = p.LoaderOptions[0]
			}
		}
		p.LoaderDowngrade = p.ReplacesLoader && p.InstalledLoader != "" && p.LoaderVersion != "" &&
			comparePlanVersions(p.LoaderVersion, p.InstalledLoader) < 0
	}
	p.InstalledVersion = strings.TrimSpace(in.Pkg.InstalledVersion)
	switch {
	case !in.Pkg.Installed:
		p.Action = ActionInstall
	case p.InstalledVersion == "":
		p.Action = ActionReinstall
	default:
		switch d := comparePlanVersions(p.Version, p.InstalledVersion); {
		case d > 0:
			p.Action = ActionUpgrade
		case d < 0:
			p.Action = ActionDowngrade
		case !in.Pkg.Explicit:
			p.Action = ActionPromote
		default:
			p.Action = ActionReinstall
		}
	}
	switch {
	case p.RequiresLoader && len(in.DB) == 0:
		p.Blocker = BlockLoaderDBUnavailable
	case p.RequiresLoader && !p.GameCompatible:
		p.Blocker = BlockGameUnsupported
	case checkFailed:
		p.Blocker = BlockLoaderCheckFailed
	case p.NeedsLoader && len(p.LoaderOptions) == 0:
		p.Blocker = BlockNoCompatibleLoader
	case p.NeedsLoader && p.LoaderVersion == "":
		p.Blocker = BlockLoaderVersionInvalid
	}
	return p
}
