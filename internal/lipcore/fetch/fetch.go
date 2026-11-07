// Package fetch provides lip 0.38.0's download plumbing: the userspace
// cache of protocol/userspace.md §3 (Services/CacheService.cs,
// Services/CacheKeys.cs) at lipdirs.CacheDir() (httpcache.go), the
// single-GET file downloader (Infrastructure/FileDownloader.cs,
// filedownloader.go), the GitHub proxy rewrite (Git/GithubProxy.cs,
// proxy.go), the download of ZIP archives into the cache
// (Mcmod/Archives/ArchiveProvider.cs) and the errors they return
// (errors.go).
package fetch
