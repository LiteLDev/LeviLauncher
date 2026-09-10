package app

import (
	"context"
	"errors"
	"path/filepath"
	"sync"
	"time"

	"github.com/liteldev/LeviLauncher/internal/apppath"
	"github.com/liteldev/LeviLauncher/internal/msaccount"
	"github.com/liteldev/LeviLauncher/internal/nativeinstall"
	"github.com/wailsapp/wails/v3/pkg/application"
)

// EventMicrosoftAccountChanged carries a MicrosoftAccountStatus to the frontend
// whenever sign-in state changes.
const EventMicrosoftAccountChanged = "microsoft-account-changed"

type MicrosoftAccountStatus struct {
	Phase       string `json:"phase"`
	AccountName string `json:"accountName"`
	ErrorCode   string `json:"errorCode"`
	Revision    uint64 `json:"revision"`
}

// MicrosoftAccountService only exposes display state and user actions. Login
// payloads, device keys, user STS and Store tickets remain inside native packages.
type MicrosoftAccountService struct {
	mu                    sync.Mutex
	status                MicrosoftAccountStatus
	cacheDir, browserPath string
	window                *application.WebviewWindow
	cancel                context.CancelFunc
	host                  *msaccount.Host
}

func NewMicrosoftAccountService(browserPath string) *MicrosoftAccountService {
	return &MicrosoftAccountService{cacheDir: filepath.Join(apppath.ConfigDir(), "microsoft-account"), browserPath: browserPath, status: MicrosoftAccountStatus{Phase: "restoring"}}
}

// Attach binds the service to the window it emits status events on and starts
// restoring the previously signed-in account.
//
//wails:ignore
func (s *MicrosoftAccountService) Attach(window *application.WebviewWindow) {
	s.window = window
	application.Get().OnShutdown(func() { s.CancelLogin() })
	go func() {
		msaccount.CleanProfiles(s.cacheDir)
		account, err := nativeinstall.RestoreAccount(context.Background(), s.cacheDir)
		s.mu.Lock()
		s.setAccountLocked(account, accountError(err))
		s.mu.Unlock()
		s.publish()
	}()
}

func (s *MicrosoftAccountService) GetStatus() MicrosoftAccountStatus {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.status
}

func (s *MicrosoftAccountService) StartLogin() MicrosoftAccountStatus {
	s.mu.Lock()
	if s.status.Phase == "signing_in" {
		host := s.host
		state := s.status
		s.mu.Unlock()
		if host != nil {
			host.Focus()
		}
		return state
	}
	if s.status.Phase == "restoring" || s.status.Phase == "signing_out" {
		state := s.status
		s.mu.Unlock()
		return state
	}
	previous := nativeinstall.Account{Name: s.status.AccountName, SignedIn: s.status.AccountName != ""}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	s.cancel = cancel
	s.status.Phase = "signing_in"
	s.status.ErrorCode = ""
	s.status.Revision++
	state := s.status
	s.mu.Unlock()
	s.publish()
	go func() {
		defer cancel()
		var host *msaccount.Host
		account, err := nativeinstall.Authenticate(ctx, s.cacheDir, func(ctx context.Context, next string) (map[string]string, error) {
			if host == nil {
				var err error
				host, err = msaccount.NewHost(s.cacheDir, s.browserPath, cancel)
				if err != nil {
					return nil, err
				}
				s.mu.Lock()
				s.host = host
				s.mu.Unlock()
			}
			return host.Prompt(ctx, next)
		})
		if host != nil {
			host.Close()
		}
		s.mu.Lock()
		s.host = nil
		s.cancel = nil
		if err != nil {
			account = previous
		}
		s.setAccountLocked(account, accountError(err))
		s.mu.Unlock()
		s.publish()
	}()
	return state
}

func (s *MicrosoftAccountService) CancelLogin() {
	s.mu.Lock()
	cancel := s.cancel
	s.mu.Unlock()
	if cancel != nil {
		cancel()
	}
}

func (s *MicrosoftAccountService) SignOut() MicrosoftAccountStatus {
	s.mu.Lock()
	if s.status.Phase == "restoring" || s.status.Phase == "signing_in" || s.status.Phase == "signing_out" {
		state := s.status
		s.mu.Unlock()
		return state
	}
	previous := nativeinstall.Account{Name: s.status.AccountName, SignedIn: s.status.AccountName != ""}
	s.status.Phase = "signing_out"
	s.status.ErrorCode = ""
	s.status.Revision++
	state := s.status
	s.mu.Unlock()
	s.publish()
	go func() {
		err := nativeinstall.SignOut(context.Background(), s.cacheDir)
		s.mu.Lock()
		account := nativeinstall.Account{}
		if err != nil {
			account = previous
		}
		s.setAccountLocked(account, accountError(err))
		s.mu.Unlock()
		s.publish()
	}()
	return state
}

func (s *MicrosoftAccountService) setAccountLocked(account nativeinstall.Account, code string) {
	s.status.Phase = "signed_out"
	s.status.AccountName = ""
	if account.SignedIn {
		s.status.Phase = "signed_in"
		s.status.AccountName = account.Name
	}
	s.status.ErrorCode = code
	s.status.Revision++
}
func (s *MicrosoftAccountService) publish() {
	if s.window != nil {
		s.window.EmitEvent(EventMicrosoftAccountChanged, s.GetStatus())
	}
}
func accountError(err error) string {
	if err == nil {
		return ""
	}
	if errors.Is(err, context.Canceled) || errors.Is(err, msaccount.ErrClosed) {
		return "ERR_AUTH_CANCELED"
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return "ERR_AUTH_TIMEOUT"
	}
	var failure *nativeinstall.Error
	if errors.As(err, &failure) {
		if failure.Code == "ERR_NATIVE_MSIXVC" {
			return "ERR_AUTH_FAILED"
		}
		return failure.Code
	}
	return "ERR_AUTH_FAILED"
}
