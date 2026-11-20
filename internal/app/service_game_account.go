package app

import "github.com/liteldev/LeviLauncher/internal/gameauth"

// GetGameSignInState returns only the game's user-facing challenge and status.
// It does not sign the launcher account in or expose the game's credentials.
func (s *UserService) GetGameSignInState() gameauth.State { return gameauth.CurrentState() }
