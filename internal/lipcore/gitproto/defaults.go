package gitproto

import "github.com/liteldev/LeviLauncher/internal/lipcore/httpclient"

// defaultHTTPClient is used when RemoteClient.HTTP is nil. Like lip's Flurl
// client it refuses https -> http redirects, so a downgraded discovery
// fails instead of re-basing later POSTs on a plaintext endpoint.
var defaultHTTPClient = httpclient.Default
