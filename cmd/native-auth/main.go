//go:build darwin

package main

import (
	"context"
	"flag"
	"fmt"
	"github.com/liteldev/LeviLauncher/internal/nativeinstall"
	"github.com/liteldev/LeviLauncher/internal/xbox"
	"os"
	"path/filepath"
	"time"
)

func main() {
	cache, _ := os.UserCacheDir()
	directory := flag.String("cache", filepath.Join(cache, "LeviLauncherNativeProbe", "Accounts"), "separate native account cache")
	device := flag.Bool("device", false, "provision and authenticate an application-owned Store device")
	signIn := flag.Bool("signin", false, "complete Microsoft device-code sign-in")
	flag.Parse()
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Minute)
	defer cancel()
	var err error
	if *signIn {
		err = nativeinstall.SignInBrowserAccount(ctx, *directory, func(c xbox.BrowserSignInCode) {
			fmt.Printf("Open %s and enter %s (expires in %ds)\n", c.VerificationURL, c.UserCode, c.ExpiresIn)
		})
		if err == nil {
			fmt.Println("Microsoft and Xbox sign-in succeeded")
		}
	}
	if err == nil && *device {
		err = nativeinstall.ProbeDevice(ctx, *directory)
		if err == nil {
			fmt.Println("Native Store device provisioning and authentication succeeded")
		}
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	if !*signIn && !*device {
		flag.Usage()
	}
}
