//go:build !linux

package xcurl

import "context"

func EnsureForVersion(ctx context.Context, versionDir string) bool { return false }

func EnsureEmbedded(contentDir string, embedded []byte) {}