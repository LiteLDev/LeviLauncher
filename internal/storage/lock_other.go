//go:build !windows

package storage

import "time"

// lockMigration is a no-op off Windows, where the launcher does not run.
func lockMigration(time.Duration) (unlock func(), ok bool) { return func() {}, true }
