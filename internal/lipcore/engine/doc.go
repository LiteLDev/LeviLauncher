// Package engine carries out lip's workspace operations
// (src/Lip.Core/Mcmod/McmodService.cs, Resolution/Resolver.cs,
// Archives/ArchiveProvider.cs, ArchiveReader.cs and PublicApi/LipClient.cs,
// from v0.38.0-beta.1 with the fixes of lip main up to v0.40.0;
// protocol/workspace.md
// §3-§7, protocol/package.md §4,
// protocol/cli.md §3-§5.4, protocol/compatibility.md) on top of the other
// lipcore packages:
//
//	New(dir, Options{GithubProxy, HTTPClient, Side, Platform}, reporter)
//	Install(packages, dryRun, ignoreScripts, noDependencies)
//	Update(packages, dryRun, ignoreScripts)
//	Uninstall(packages, dryRun, ignoreScripts, noDependencies)
//	List() -> explicit, implicit "<identity>@<version>"
//	Packages() -> []InstalledPackage
//	CacheClean()
//
// An operation takes the workspace lock (.lip/lock) and first finishes or
// undoes an interrupted operation (txn), reads the state file
// (.lip/state.json, or the one protocol/compatibility.md §3 derives from a
// tooth_lock.json), resolves the requests (the resolver of this package),
// solves the package set (graph), plans every uninstall and install against
// one view of the workspace (install), then applies the plans in one
// transaction and writes the state file at the commit point.
//
// The engine follows lip exactly, with one parameter: the side of the
// workspaces it manages. lip manages server workspaces only; LeviLauncher's
// workspaces are client workspaces (Options.Side defaults to "client"),
// with every other protocol rule unchanged. The one exception is opt-in and
// temporary: Options.LegacyRoleVariants, LeviLauncher's compatibility layer
// for tooth.json dependencies on "X#client" (compat_legacy_variants.go).
//
// Archives: remote repositories are read through the git smart-HTTP protocol
// (gitproto.RemoteClient, packs written to lip's cache directory), URLs and
// assets are downloaded into lip's cache (fetch.HTTPCache, validated as ZIP
// archives, through the GitHub proxy first when one is set), local ZIP files
// are read into memory and local directories are read where they are.
//
// Errors keep lip's message in Error() and the cause in Unwrap; FormatError
// renders an error as lip's CLI reports it.
//
// Measured progress: a resolve bar over the requested packages and an
// indeterminate one over the package files read while solving, a task bar
// over the packages applied, git_fetch bars over the objects of every pack
// received, download bars (fetch) and the place, remove and scripts bars of
// the installer.
package engine
