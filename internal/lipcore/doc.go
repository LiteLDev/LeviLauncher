// Package lipcore is a native Go reimplementation of the parts of lip
// (https://github.com/LiteLDev/lip, ported from v0.38.0-beta.1 and kept in
// step with lip v0.40.0, whose package files and state file name the side of
// an entry "side" and its runtime identifier "platform") that LeviLauncher
// uses: installing, updating, uninstalling and listing packages inside an
// instance directory, and cleaning lip's download cache.
//
// The implementation is file-compatible with lip's format 4: it reads and
// writes the same .lip/state.json (a JSON file), reads lip.toml package files
// and lip's config.toml (TOML files), shares the same cache directory
// (lipdirs.CacheDir()) and resolves packages the same way (git refs via the
// git smart-HTTP v2 protocol). The files of lip v0.x (tooth.json,
// tooth_lock.json, liprc.json) are read as lip reads them.
//
// Sub-packages:
//
//	report     - progress/log reporting interface shared by all packages
//	lipdirs    - lip's per-user directories
//	httpclient - HTTP client defaults shared with lip's Flurl configuration
//	config     - user configuration and liprc.json migration
//	semver     - SemVer 2.0 versions and WalkerCodeRanger Semver 3 ranges
//	ref        - package sources, git refs and package arguments
//	doc        - JSON value model, strict JSON, TOML and JSON files
//	pkgmodel   - package file and state file types
//	pkgfile    - package file parser and state file store
//	compat     - tooth.json and tooth_lock.json of lip v0.x
//	manifest   - tooth.json (format 3) entities, used by compat
//	migrate    - tooth.json format 1/2 -> 3 migration, used by compat
//	glob       - DotNet.Glob 3.1.3 compatible glob matching
//	stjson     - System.Text.Json compatible JSON writer
//	pathglob   - lip path patterns (globs, place)
//	gitproto   - git smart-HTTP protocol v2 client and git tree sources
//	gitpack    - git packfile decoding and tree walking
//	fetch      - download cache, GitHub proxy rewrite, file downloader
//	archive    - ZIP and directory archives, mounted sources, lip pack
//	graph      - dependency solver and installation order
//	install    - package installer, workspace view and layout, hook runner
//	txn        - workspace transaction, journal and lock
//	engine     - Install/Update/Uninstall/List orchestration
package lipcore
