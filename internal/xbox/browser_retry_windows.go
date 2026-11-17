package xbox

// Preserve the existing Windows authentication behaviour.
func browserPollMayRetry(status int, err error) bool { return false }
