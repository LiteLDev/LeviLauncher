package cli

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/liteldev/LeviLauncher/internal/apppath"
	"github.com/liteldev/LeviLauncher/internal/nativeinstall"
	"github.com/liteldev/LeviLauncher/internal/processinfo"
	"github.com/liteldev/LeviLauncher/internal/xbox"
)

// accountStatus is `account status --json` and `account login --json`.
type accountStatus struct {
	SignedIn bool             `json:"signedIn"`
	Gamertag string           `json:"gamertag"`
	XUID     string           `json:"xuid"`
	Licenses *accountLicenses `json:"licenses"` // null when signed out
	Elevated bool             `json:"elevated"`
}

// accountLicenses holds authorized, trial, not_entitled or error per edition.
type accountLicenses struct {
	Release string `json:"release"`
	Preview string `json:"preview"`
}

// accountSavedFile is the launcher's saved account selection, written by
// nativeinstall on sign-in and removed on sign-out.
const accountSavedFile = "wam-account.dpapi"

// acct is the account backend; tests replace its functions.
var acct = struct {
	restore  func(ctx context.Context) error
	hasSaved func() bool
	state    func() uint32
	reset    func()
	userID   func() (string, error)
	gamertag func() (string, error)
	licenses func(ctx context.Context, xuid string, refresh bool) nativeinstall.GameLicenses
	elevated func() bool
	signIn   func(ctx context.Context) error
	signOut  func() error
}{
	restore: RestoreAccount,
	hasSaved: func() bool {
		_, err := os.Stat(filepath.Join(apppath.AccountsDir(), accountSavedFile))
		return err == nil
	},
	state: func() uint32 { s, _ := xbox.XUserGetState(); return s },
	reset: func() { _ = xbox.ResetSession() },
	userID: func() (string, error) {
		id, err := xbox.GetLocalUserId()
		if err != nil {
			return "", err
		}
		return strconv.FormatUint(id, 10), nil
	},
	gamertag: xbox.GetLocalUserGamertag,
	licenses: func(ctx context.Context, xuid string, refresh bool) nativeinstall.GameLicenses {
		if refresh {
			return nativeinstall.RefreshGameLicenses(ctx, apppath.AccountsDir(), xuid)
		}
		return nativeinstall.CheckGameLicenses(ctx, apppath.AccountsDir(), xuid)
	},
	elevated: processinfo.IsElevated,
	signIn:   SignIn,
	signOut:  SignOut,
}

func init() {
	Register(&Command{
		Path:    "account status",
		Summary: "show the signed-in Microsoft account and which editions it owns",
		Description: `The account is the one LeviLauncher uses to download and license Minecraft.
Editions are owned, trial, not owned, or unknown when the licence check failed.
Signed out is not an error: scripts read signedIn from --json.`,
		Examples: []string{"account status", "account status --refresh", "account status --json"},
		Flags:    []*Flag{BoolFlag("refresh", "", "sign in again silently and re-check licences online")},
		Run:      runAccountStatus,
	})
	Register(&Command{
		Path:    "account login",
		Summary: "sign in to a Microsoft account, or switch accounts",
		Description: `Opens the Windows account picker. It needs an interactive desktop session (not
a service or an SSH session).`,
		Examples: []string{"account login", "account login --json"},
		Run:      runAccountLogin,
	})
	Register(&Command{
		Path:    "account logout",
		Summary: "forget the launcher's Microsoft account",
		Description: `Removes the saved account and the licence keys cached for it. The account
stays known to Windows. Signing out while signed out does nothing.`,
		Examples: []string{"account logout", "account logout --yes"},
		Run:      runAccountLogout,
	})
	RegisterBackendErrors(map[string]BackendError{
		"ERR_AUTH_ACCOUNT_SELECTION": {Exit: ExitAuth, Message: "the saved Microsoft account cannot be read",
			Hint: ProgramName + " account login"},
	})
}

// acctAwait runs fn on its own goroutine so Ctrl-C returns at once even
// while a network call that takes no context is running.
func acctAwait[T any](ctx context.Context, fn func() (T, error)) (T, error) {
	type result struct {
		v   T
		err error
	}
	ch := make(chan result, 1)
	go func() {
		v, err := fn()
		ch <- result{v, err}
	}()
	select {
	case r := <-ch:
		return r.v, r.err
	case <-ctx.Done():
		var zero T
		return zero, ctx.Err()
	}
}

// accountIdentity returns the signed-in account, following the window: when
// the session does not resolve silently it is reset once and read again.
// A session that needs interaction is "signed out" (signedIn false, nil).
func accountIdentity(ctx context.Context, refresh bool) (accountStatus, error) {
	st := accountStatus{}
	if err := acct.restore(ctx); err != nil {
		if ctx.Err() != nil {
			return st, ctx.Err()
		}
		return st, Backend(nativeinstall.AuthErrorCode(err)).WithDetail(err.Error())
	}
	if !acct.hasSaved() {
		return st, nil
	}
	type ident struct{ xuid, tag string }
	id, err := acctAwait(ctx, func() (ident, error) {
		if refresh || acct.state() != 0 {
			acct.reset()
		}
		xuid, err := acct.userID()
		if err != nil {
			return ident{}, err
		}
		tag, err := acct.gamertag()
		if err != nil {
			return ident{}, err
		}
		return ident{xuid, strings.TrimSpace(tag)}, nil
	})
	if err != nil {
		if ctx.Err() != nil {
			return st, ctx.Err()
		}
		code := nativeinstall.AuthErrorCode(err)
		if code == "ERR_AUTH_INTERACTION_REQUIRED" {
			return st, nil
		}
		return st, Backend(code).WithDetail(err.Error())
	}
	st.SignedIn, st.XUID, st.Gamertag = id.xuid != "", id.xuid, id.tag
	return st, nil
}

// normalizeLicenses maps a licence result to the four documented values and
// discards a result for another account, as the window does.
func normalizeLicenses(l nativeinstall.GameLicenses, xuid string) *accountLicenses {
	norm := func(s string) string {
		switch s {
		case "authorized", "trial", "not_entitled":
			return s
		}
		return "error"
	}
	if strings.TrimSpace(l.XUID) != xuid {
		return &accountLicenses{Release: "error", Preview: "error"}
	}
	return &accountLicenses{Release: norm(l.Release), Preview: norm(l.Preview)}
}

func licenseText(s string) string {
	switch s {
	case "authorized":
		return "owned"
	case "trial":
		return "trial"
	case "not_entitled":
		return "not owned"
	}
	return "unknown (the licence check failed)"
}

func accountLicensesFor(c *Ctx, st *accountStatus, refresh bool) {
	if !st.SignedIn {
		return
	}
	prog := c.Progress()
	prog.Phase("Checking Minecraft licences")
	l, err := acctAwait(c.Context(), func() (nativeinstall.GameLicenses, error) {
		return acct.licenses(c.Context(), st.XUID, refresh), nil
	})
	prog.Done()
	if err != nil {
		l = nativeinstall.GameLicenses{}
	}
	st.Licenses = normalizeLicenses(l, st.XUID)
	if st.Licenses.Release == "error" && st.Licenses.Preview == "error" && c.Context().Err() == nil {
		c.Warn("the licence check failed; run '%s account status --refresh' to try again", ProgramName)
	}
}

func renderAccount(c *Ctx, st accountStatus, signedInVerb string) error {
	ids := []string{}
	if st.SignedIn {
		ids = append(ids, st.Gamertag)
	}
	return c.Render(st, ids, func(w io.Writer) error {
		if !st.SignedIn {
			fmt.Fprintln(w, "Not signed in")
			c.Notice("Sign in with: %s account login", ProgramName)
			return nil
		}
		fmt.Fprintf(w, "%s %s (xuid %s)\n", signedInVerb, Dash(st.Gamertag), st.XUID)
		pairs := []string{}
		if st.Licenses != nil {
			pairs = append(pairs, "Minecraft", licenseText(st.Licenses.Release),
				"Minecraft Preview", licenseText(st.Licenses.Preview))
		}
		pairs = append(pairs, "Administrator", YesNo(st.Elevated))
		return KV(w, pairs...)
	})
}

func runAccountStatus(c *Ctx) error {
	refresh := c.Bool("refresh")
	st, err := accountIdentity(c.Context(), refresh)
	if err != nil {
		return err
	}
	accountLicensesFor(c, &st, refresh)
	st.Elevated = acct.elevated()
	return renderAccount(c, st, "Signed in as")
}

func runAccountLogin(c *Ctx) error {
	c.Notice("Choose an account in the Windows account picker")
	if err := acct.signIn(c.Context()); err != nil {
		return err
	}
	st, err := accountIdentity(c.Context(), false)
	if err != nil {
		return err
	}
	if !st.SignedIn {
		return Errorf(ExitAuth, "ERR_AUTH_FAILED", "the account picker closed without signing in").
			WithHint("%s account login", ProgramName)
	}
	accountLicensesFor(c, &st, true)
	st.Elevated = acct.elevated()
	return renderAccount(c, st, "Signed in as")
}

func runAccountLogout(c *Ctx) error {
	type logoutResult struct {
		SignedOut   bool `json:"signedOut"`
		WasSignedIn bool `json:"wasSignedIn"`
	}
	if !acct.hasSaved() {
		return c.Render(logoutResult{SignedOut: true}, nil, func(w io.Writer) error {
			_, err := fmt.Fprintln(w, "Not signed in")
			return err
		})
	}
	if err := c.Confirm("Sign LeviLauncher out of its Microsoft account (downloads and licence checks then need 'account login')?"); err != nil {
		return err
	}
	if err := acct.signOut(); err != nil {
		return err
	}
	return c.Render(logoutResult{SignedOut: true, WasSignedIn: true}, nil, func(w io.Writer) error {
		_, err := fmt.Fprintln(w, "Signed out")
		return err
	})
}
