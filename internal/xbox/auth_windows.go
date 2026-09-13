//go:build windows

// Package xbox reads Xbox Live profile and identity for the explicitly selected
// Microsoft account through WebAuthenticationCoreManager (WAM) plus plain XBL REST,
// with no GDK / GamingServices dependency.
package xbox

import (
	"context"
	"fmt"
	"time"
)

const (
	msaProvider = "https://login.microsoft.com"
	xblScope    = "service::user.auth.xboxlive.com::MBI_SSL"
	// clientID is a Microsoft account application id accepted by Xbox Live for
	// the MBI_SSL relying party.
	clientID = "00000000402b5328"

	roInitMultithreaded = 1
	iidIAsyncInfo       = "00000036-0000-0000-C000-000000000046"
	slotGetStatus       = 7
	slotGetErrorCode    = 8
	asyncStarted        = 0
	asyncCompleted      = 1

	asyncTimeout = 30 * time.Second
	sessionSkew  = 5 * time.Minute
)

type session struct {
	xsts      string
	uhs       string
	xuid      string
	gtg       string
	exp       time.Time
	accountID string
}

var (
	sessionGate           = make(chan struct{}, 1)
	cached                *session
	preferredAccountID    string
	accountSelectionError error
)

func lockSession(ctx context.Context) error {
	select {
	case sessionGate <- struct{}{}:
		if err := ctx.Err(); err != nil {
			<-sessionGate
			return err
		}
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func ensureSession() (*session, error) {
	ctx := context.Background()
	if err := lockSession(ctx); err != nil {
		return nil, err
	}
	defer func() { <-sessionGate }()
	return ensureSessionLocked(ctx)
}

func ensureSessionLocked(ctx context.Context) (*session, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if accountSelectionError != nil {
		return nil, accountSelectionError
	}
	if cached != nil && time.Now().Before(cached.exp) {
		return cached, nil
	}
	// Refreshing credentials must not silently replace an explicitly selected account.
	accountID := preferredAccountID
	if cached != nil {
		accountID = cached.accountID
	}
	if accountID == "" {
		return nil, ErrInteractionRequired
	}
	ticket, accountID, err := requestWAMTicket(ctx, xblScope, accountID)
	if err != nil {
		return nil, err
	}
	userToken, err := xblUserAuth(ctx, ticket)
	if err != nil {
		return nil, err
	}
	s, err := xblXSTS(ctx, userToken)
	if err != nil {
		return nil, err
	}
	s.accountID = accountID
	cached = s
	return s, nil
}

// ConfigureAccountSelection restores account identity only, never a credential.
// A failed restoration blocks implicit fallback to the Windows default account.
func ConfigureAccountSelection(accountID string, err error) {
	sessionGate <- struct{}{}
	defer func() { <-sessionGate }()
	configureSelectionLocked(accountID, err)
}

func configureSelectionLocked(accountID string, err error) {
	if preferredAccountID != accountID || err != nil || accountSelectionError != nil {
		cached = nil
	}
	preferredAccountID = accountID
	accountSelectionError = err
}

// RestoreAccountSelection serializes preference reads with native sign-in commits.
func RestoreAccountSelection(ctx context.Context, load func() (string, error)) error {
	if err := lockSession(ctx); err != nil {
		return err
	}
	defer func() { <-sessionGate }()
	id, err := load()
	configureSelectionLocked(id, err)
	return err
}

func resetCache() {
	sessionGate <- struct{}{}
	cached = nil
	<-sessionGate
}

// GetStoreTicket obtains a fresh Store ticket for the same account as the launcher.
// Credentials remain in the backend and are never persisted or sent over bindings.
func GetStoreTicket(ctx context.Context) (token, reference string, err error) {
	return GetStoreTicketForUser(ctx, "")
}

// GetStoreTicketForUser binds a license check to the profile shown by the UI.
// An account switch must never produce a result for a different user.
func GetStoreTicketForUser(ctx context.Context, expectedXUID string) (token, reference string, err error) {
	if err = lockSession(ctx); err != nil {
		return
	}
	defer func() { <-sessionGate }()
	s, err := ensureSessionLocked(ctx)
	if err != nil {
		return "", "", err
	}
	if expectedXUID != "" && s.xuid != expectedXUID {
		return "", "", ErrAccountChanged
	}
	return requestWAMTicket(ctx, "service::www.microsoft.com::MBI_SSL", s.accountID)
}

func xblUserAuth(ctx context.Context, rpsTicket string) (string, error) {
	body := map[string]any{
		"Properties": map[string]any{
			"AuthMethod": "RPS",
			"SiteName":   "user.auth.xboxlive.com",
			"RpsTicket":  rpsTicket,
		},
		"RelyingParty": "http://auth.xboxlive.com",
		"TokenType":    "JWT",
	}
	var out struct {
		Token string `json:"Token"`
	}
	if err := postJSONContext(ctx, "https://user.auth.xboxlive.com/user/authenticate", "1", "", body, &out); err != nil {
		return "", err
	}
	if out.Token == "" {
		return "", fmt.Errorf("ERR_XBL_USER_AUTH_EMPTY")
	}
	return out.Token, nil
}

func xblXSTS(ctx context.Context, userToken string) (*session, error) {
	body := map[string]any{
		"Properties": map[string]any{
			"SandboxId":  "RETAIL",
			"UserTokens": []string{userToken},
		},
		"RelyingParty": "http://xboxlive.com",
		"TokenType":    "JWT",
	}
	var out struct {
		Token         string `json:"Token"`
		NotAfter      string `json:"NotAfter"`
		DisplayClaims struct {
			Xui []struct {
				Uhs string `json:"uhs"`
				Xid string `json:"xid"`
				Gtg string `json:"gtg"`
			} `json:"xui"`
		} `json:"DisplayClaims"`
	}
	if err := postJSONContext(ctx, "https://xsts.auth.xboxlive.com/xsts/authorize", "1", "", body, &out); err != nil {
		return nil, err
	}
	if out.Token == "" || len(out.DisplayClaims.Xui) == 0 {
		return nil, fmt.Errorf("ERR_XBL_XSTS_EMPTY")
	}
	claim := out.DisplayClaims.Xui[0]
	exp := time.Now().Add(time.Hour)
	if t, err := time.Parse(time.RFC3339, out.NotAfter); err == nil {
		exp = t.Add(-sessionSkew)
	}
	return &session{
		xsts: out.Token,
		uhs:  claim.Uhs,
		xuid: claim.Xid,
		gtg:  claim.Gtg,
		exp:  exp,
	}, nil
}

func (s *session) authHeader() string {
	return "XBL3.0 x=" + s.uhs + ";" + s.xsts
}

// SignIn changes the shared account only after native authentication and persistence succeed.
func SignIn(ctx context.Context, hwnd uintptr, persist func(string) error, dispatch func(func())) error {
	if hwnd == 0 || persist == nil || dispatch == nil {
		return ErrAuthenticationFailed
	}
	return signInSession(ctx, func() (*session, error) {
		ticket, id, err := requestWAMToken(ctx, xblScope, "", hwnd, dispatch)
		if err != nil {
			return nil, err
		}
		userToken, err := xblUserAuth(ctx, ticket)
		if err != nil {
			return nil, err
		}
		s, err := xblXSTS(ctx, userToken)
		if err != nil {
			return nil, err
		}
		s.accountID = id
		return s, nil
	}, persist)
}

// A canceled picker, failed token exchange or failed persistence must preserve
// both the displayed profile and the account used by Store authorization.
func signInSession(ctx context.Context, acquire func() (*session, error), persist func(string) error) error {
	if err := lockSession(ctx); err != nil {
		return err
	}
	defer func() { <-sessionGate }()
	s, err := acquire()
	if err != nil {
		return err
	}
	if s == nil || s.accountID == "" {
		return ErrAuthenticationFailed
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := persist(s.accountID); err != nil {
		return err
	}
	preferredAccountID = s.accountID
	accountSelectionError = nil
	cached = s
	return nil
}
