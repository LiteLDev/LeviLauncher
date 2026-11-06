// Package fetch provides lip 0.37.0's download plumbing: the on-disk cache
// (Services/CacheService.cs), GitHub proxy rewriting with fallback
// (Services/SourceService.cs, Git/GitRemoteClient.cs), the HTTP file
// downloader with progress (Infrastructure/FileDownloader.cs) and asset
// sources (Sources/ArchiveSource.cs, SingleFileSource.cs, CompositeSource.cs,
// DirectorySource.cs).
//
// The implementation lives in cache.go, proxy.go, download.go and source.go.
package fetch
