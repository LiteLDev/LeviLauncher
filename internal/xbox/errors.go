package xbox

import "errors"

var (
	ErrInteractionRequired  = errors.New("ERR_AUTH_INTERACTION_REQUIRED")
	ErrAccountChanged       = errors.New("ERR_AUTH_ACCOUNT_CHANGED")
	ErrAuthenticationFailed = errors.New("ERR_AUTH_FAILED")
)
