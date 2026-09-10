//go:build windows

// SPDX-License-Identifier: GPL-3.0-only
// msixvc-native installs a local, complete MSIXVC using online account authorization.
// No tokens, license bodies or content keys are accepted as command-line arguments.
package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"github.com/liteldev/LeviLauncher/internal/nativeinstall"
	"os"
	"os/signal"
)

func main() {
	cache := flag.String("cache", "", "LeviLauncher Microsoft account cache directory (required)")
	market := flag.String("market", "US", "Two-letter Store market")
	full := flag.Bool("require-full", true, "Reject trial licenses")
	flag.Parse()
	if flag.NArg() != 2 || *cache == "" {
		fmt.Fprintln(os.Stderr, "Usage: msixvc-native [-cache directory] [-market US] [-require-full] input.msixvc new-output-directory")
		os.Exit(2)
	}
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()
	enc := json.NewEncoder(os.Stdout)
	result, err := nativeinstall.Install(ctx, flag.Arg(0), flag.Arg(1), nativeinstall.Options{
		CacheDir: *cache, Market: *market, RequireFullLicense: *full,
		Progress: func(p nativeinstall.Progress) {
			_ = enc.Encode(map[string]any{"stage": "extract", "bytes": p.Current, "total": p.Total})
		},
		Diagnostic: func(d map[string]any) { _ = enc.Encode(d) },
	})
	if err != nil {
		code := "ERR_NATIVE_MSIXVC"
		var native *nativeinstall.Error
		if errors.As(err, &native) {
			code = native.Code
		}
		if errors.Is(err, context.Canceled) {
			code = "ERR_CANCELED"
		}
		_ = enc.Encode(map[string]any{"error": code})
		os.Exit(1)
	}
	_ = enc.Encode(result)
}
