//go:build !windows

package cli

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strings"

	"github.com/liteldev/LeviLauncher/internal/apppath"
	"github.com/liteldev/LeviLauncher/internal/nativeinstall"
	"github.com/liteldev/LeviLauncher/internal/xbox"
)

// SignIn runs the Microsoft browser device-code sign-in flow and saves the
// chosen account as the launcher's account.
func SignIn(ctx context.Context) error {
	_ = RestoreAccount(ctx)
	notice := signInNoticeFrom(ctx)
	err := nativeinstall.SignInBrowserAccount(ctx, apppath.AccountsDir(), func(code xbox.BrowserSignInCode) {
		if notice != nil {
			notice("Open %s and enter code %s", code.VerificationURL, code.UserCode)
		} else {
			fmt.Fprintf(os.Stderr, "Open %s and enter code %s\n", code.VerificationURL, code.UserCode)
		}
		if runtime.GOOS == "darwin" && strings.HasPrefix(code.VerificationURL, "https://") {
			_ = exec.Command("/usr/bin/open", code.VerificationURL).Start()
		}
	})
	if err != nil {
		return Backend(nativeinstall.AuthErrorCode(err)).WithDetail(err.Error())
	}
	return nil
}
