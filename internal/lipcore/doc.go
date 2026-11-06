// Package lipcore is a native Go reimplementation of the parts of lip
// (https://github.com/LiteLDev/lip, v0.37.0) that LeviLauncher uses: installing,
// updating, uninstalling and listing tooth packages inside an instance
// directory, and cleaning lip's download cache.
//
// The implementation is file-compatible with lip 0.37.0: it reads and writes
// the same tooth_lock.json, shares the same cache directory
// (%LOCALAPPDATA%\lip\cache) and resolves packages the same way (git tags via
// the git smart-HTTP v2 protocol, tooth.json format 3 manifests).
//
// Sub-packages:
//
//	report     - progress/log reporting interface shared by all packages
//	semver     - SemVer 2.0 versions and WalkerCodeRanger Semver 3 ranges
//	glob       - DotNet.Glob 3.1.3 compatible glob matching
//	stjson     - System.Text.Json compatible indented JSON writer
//	manifest   - tooth.json (format 3) entities, package ids and specs
//	lockfile   - tooth_lock.json workspace state
//	migrate    - tooth.json format 1/2 -> 3 migration and template rendering
//	gitproto   - git smart-HTTP protocol v2 client (ls-refs, fetch)
//	gitpack    - git packfile decoding and tree walking
//	fetch      - download cache, GitHub proxy, HTTP downloader, archives
//	solver     - backtracking dependency solver
//	engine     - Install/Update/Uninstall/List orchestration, git registry
//	engine/workspace - per-package file placement, removal and scripts
package lipcore
