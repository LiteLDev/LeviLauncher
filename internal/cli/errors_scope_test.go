package cli

import "testing"

// Backend codes several command groups get mean different things there:
// each group's wording applies only while one of its commands runs.
func TestBackendErrorScopes(t *testing.T) {
	in := func(path, code string) *Error {
		t.Helper()
		restore := setBackendScope(path)
		defer restore()
		return Backend(code)
	}
	for _, tc := range []struct {
		path, code string
		exit       int
		message    string
	}{
		{"config set", "ERR_INVALID_PATH", ExitUsage, "invalid folder"},
		{"world remove", "ERR_INVALID_PATH", ExitFailure, "the item is not inside this instance's content folders"},
		{"pack import", "ERR_CREATE_TARGET_DIR", ExitFailure, "cannot create the target folder"},
		{"instance copy-data", "ERR_CREATE_TARGET_DIR", ExitFailure, "cannot create the instance's data folder"},
		// The exact path wins over the noun.
		{"instance create", "ERR_NAME_EXISTS", ExitConflict, "an instance with this name already exists"},
		{"instance rename", "ERR_NAME_EXISTS", ExitConflict, "an instance with that name already exists"},
		// The noun's table serves verbs the exact path's table does not name.
		{"instance create", "ERR_DEV_MODE_REQUIRED", ExitSystem, "registering a GDK instance needs Windows Developer Mode"},
		{"backup create", "ERR_UWP_UNSUPPORTED_FEATURE", ExitConflict, "UWP instances have no mods"},
		{"loader install", "ERR_UWP_UNSUPPORTED_FEATURE", ExitConflict, "LeviLamina needs a GDK instance; UWP instances are not supported"},
		// Outside every scope: the neutral global wording.
		{"", "ERR_UWP_UNSUPPORTED_FEATURE", ExitConflict, "this is not supported on UWP instances"},
		{"curseforge install", "ERR_CREATE_TARGET_DIR", ExitFailure, "cannot create the target folder"},
		// A code only one scoped table names keeps its message elsewhere.
		{"curseforge install", "ERR_DUPLICATE_UUID", ExitConflict, "a pack with the same UUID is already installed"},
	} {
		e := in(tc.path, tc.code)
		if e.Exit != tc.exit || e.Message != tc.message {
			t.Errorf("%q %s = exit %d %q, want exit %d %q", tc.path, tc.code, e.Exit, e.Message, tc.exit, tc.message)
		}
	}
	if activeScope != "" {
		t.Errorf("scope not restored: %q", activeScope)
	}
}
