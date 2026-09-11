//go:build windows

package xbox

import "strconv"

const (
	stateSignedIn  uint32 = 0
	stateSignedOut uint32 = 2
)

func GetLocalUserId() (uint64, error) {
	s, err := ensureSession()
	if err != nil {
		return 0, err
	}
	return strconv.ParseUint(s.xuid, 10, 64)
}

func GetLocalUserGamertag() (string, error) {
	s, err := ensureSession()
	if err != nil {
		return "", err
	}
	return s.gtg, nil
}

// XUserGetState reports 0 when the local account resolves silently and a
// non-zero SignedOut otherwise; the UI re-runs sign-in on any non-zero value.
func XUserGetState() (uint32, error) {
	if _, err := ensureSession(); err != nil {
		return stateSignedOut, nil
	}
	return stateSignedIn, nil
}

// ResetSession drops the cached session so the next call re-authenticates the
// current local account.
func ResetSession() error {
	resetCache()
	return nil
}
