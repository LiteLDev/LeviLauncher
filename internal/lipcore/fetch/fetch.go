// Package fetch provides lip's download plumbing.
//
// lip 0.38.0 (httpcache.go, filedownloader.go, RewriteGithub in proxy.go,
// errors.go): the userspace cache of protocol/userspace.md §3
// (Services/CacheService.cs, Services/CacheKeys.cs) at lipdirs.CacheDir(),
// the single-GET file downloader (Infrastructure/FileDownloader.cs), the
// GitHub proxy rewrite (Git/GithubProxy.cs) and the download of ZIP
// archives into the cache (Mcmod/Archives/ArchiveProvider.cs).
//
// lip 0.37.0 (cache.go: Cache, download.go: Downloader, proxy.go:
// ProxyRewriter, source.go), still used by the 0.37 engine: the git blob
// cache under %LOCALAPPDATA%\lip\cache and \temp (Services/CacheService.cs),
// GitHub proxy rewriting with fallback (Services/SourceService.cs,
// Git/GitRemoteClient.cs), the HTTP file downloader with progress and asset
// sources (Sources/ArchiveSource.cs, SingleFileSource.cs, CompositeSource.cs,
// DirectorySource.cs).
package fetch
