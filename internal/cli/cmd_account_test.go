package cli

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/liteldev/LeviLauncher/internal/nativeinstall"
	"github.com/liteldev/LeviLauncher/internal/xbox"
)

// fakeAccount replaces the account backend: no Windows account, network or
// saved selection is touched.
type fakeAccount struct {
	saved      bool
	state      uint32
	resets     int
	xuid, tag  string
	idErr      error
	lic        nativeinstall.GameLicenses
	refreshed  bool
	signInErr  error
	signedOut  bool
	elevated   bool
	restoreErr error
}

func installFakeAccount(t *testing.T) *fakeAccount {
	t.Helper()
	f := &fakeAccount{xuid: "2535400000000001", tag: "Steve123"}
	saved := acct
	t.Cleanup(func() { acct = saved })
	acct.restore = func(context.Context) error { return f.restoreErr }
	acct.hasSaved = func() bool { return f.saved }
	acct.state = func() uint32 { return f.state }
	acct.reset = func() { f.resets++; f.state = 0 }
	acct.userID = func() (string, error) {
		if f.idErr != nil {
			return "", f.idErr
		}
		return f.xuid, nil
	}
	acct.gamertag = func() (string, error) { return f.tag, nil }
	acct.licenses = func(_ context.Context, xuid string, refresh bool) nativeinstall.GameLicenses {
		f.refreshed = refresh
		l := f.lic
		if l.XUID == "" {
			l.XUID = xuid
		}
		return l
	}
	acct.elevated = func() bool { return f.elevated }
	acct.signIn = func(context.Context) error {
		if f.signInErr != nil {
			return f.signInErr
		}
		f.saved = true
		return nil
	}
	acct.signOut = func() error { f.signedOut = true; f.saved = false; return nil }
	return f
}

func TestAccountStatusSignedOut(t *testing.T) {
	h := newHarness(t)
	installFakeAccount(t)
	code, out, errOut := h.run("account", "status")
	if code != 0 || out != "Not signed in\n" || !strings.Contains(errOut, "account login") {
		t.Fatalf("signed out: %d %q %q", code, out, errOut)
	}
	code, out, _ = h.run("account", "status", "--json")
	var st accountStatus
	if code != 0 || json.Unmarshal([]byte(out), &st) != nil || st.SignedIn || st.Licenses != nil {
		t.Fatalf("--json: %d %q", code, out)
	}
	if !strings.Contains(out, `"licenses": null`) {
		t.Fatalf("licenses must be null when signed out: %q", out)
	}
}

func TestAccountStatusSignedIn(t *testing.T) {
	h := newHarness(t)
	f := installFakeAccount(t)
	f.saved, f.state = true, 2 // the session needs a reset first, as in the window
	f.lic = nativeinstall.GameLicenses{Release: "authorized", Preview: "not_entitled"}
	code, out, errOut := h.run("account", "status")
	if code != 0 || f.resets != 1 || f.refreshed {
		t.Fatalf("status: %d resets=%d refreshed=%v %q", code, f.resets, f.refreshed, errOut)
	}
	for _, want := range []string{"Signed in as Steve123 (xuid 2535400000000001)", "Minecraft:", "owned", "not owned", "Administrator:"} {
		if !strings.Contains(out, want) {
			t.Errorf("status output lacks %q: %q", want, out)
		}
	}
	code, out, _ = h.run("account", "status", "--refresh", "--json")
	var st accountStatus
	if code != 0 || json.Unmarshal([]byte(out), &st) != nil || !st.SignedIn || st.Licenses.Release != "authorized" ||
		st.Licenses.Preview != "not_entitled" || !f.refreshed {
		t.Fatalf("--refresh --json: %d %q", code, out)
	}
	if code, out, _ := h.run("account", "status", "-q"); code != 0 || out != "Steve123\n" {
		t.Fatalf("-q: %d %q", code, out)
	}

	// A session that needs interaction is signed out, not an error.
	f.idErr = xbox.ErrInteractionRequired
	if code, out, _ := h.run("account", "status"); code != 0 || out != "Not signed in\n" {
		t.Fatalf("expired session: %d %q", code, out)
	}
	// Other failures are reported.
	f.idErr = errors.New("network down")
	if code, _, errOut := h.run("account", "status"); code != ExitAuth || !strings.Contains(errOut, "[ERR_AUTH_FAILED]") {
		t.Fatalf("failure: %d %q", code, errOut)
	}
}

func TestNormalizeLicenses(t *testing.T) {
	got := normalizeLicenses(nativeinstall.GameLicenses{XUID: "1", Release: "trial", Preview: "weird"}, "1")
	if got.Release != "trial" || got.Preview != "error" {
		t.Fatalf("%+v", got)
	}
	// A result for another account is discarded.
	got = normalizeLicenses(nativeinstall.GameLicenses{XUID: "2", Release: "authorized", Preview: "authorized"}, "1")
	if got.Release != "error" || got.Preview != "error" {
		t.Fatalf("other account: %+v", got)
	}
}

func TestAccountLoginLogout(t *testing.T) {
	h := newHarness(t)
	f := installFakeAccount(t)
	f.signInErr = Backend("ERR_CANCELED")
	if code, _, _ := h.run("account", "login"); code != ExitCancelled {
		t.Fatalf("cancelled picker: %d", code)
	}
	f.signInErr = Backend("ERR_NO_INTERACTIVE_DESKTOP")
	if code, _, _ := h.run("account", "login"); code != ExitSystem {
		t.Fatalf("no desktop: %d", code)
	}
	f.signInErr = nil
	f.lic = nativeinstall.GameLicenses{Release: "authorized", Preview: "authorized"}
	code, out, errOut := h.run("account", "login")
	if code != 0 || !strings.Contains(out, "Signed in as Steve123") || !f.refreshed {
		t.Fatalf("login: %d %q %q", code, out, errOut)
	}

	if code, _, errOut := h.run("account", "logout"); code != ExitUsage || !strings.Contains(errOut, "--yes") || f.signedOut {
		t.Fatalf("logout without --yes: %d %q", code, errOut)
	}
	if code, out, _ := h.run("account", "logout", "--yes"); code != 0 || out != "Signed out\n" || !f.signedOut {
		t.Fatalf("logout: %d %q", code, out)
	}
	// Idempotent: nothing to confirm when signed out.
	f.signedOut = false
	if code, out, _ := h.run("account", "logout"); code != 0 || out != "Not signed in\n" || f.signedOut {
		t.Fatalf("logout twice: %d %q", code, out)
	}
}
