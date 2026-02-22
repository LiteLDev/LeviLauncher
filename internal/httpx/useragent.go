package httpx

import (
	"net/http"
	"runtime"
)

const appUserAgent = "LeviLauncher (" + runtime.GOOS + "; " + runtime.GOARCH + ")"

func UserAgent() string {
	return appUserAgent
}

func ApplyDefaultHeaders(req *http.Request) {
	if req == nil {
		return
	}
	if req.Header.Get("User-Agent") == "" {
		req.Header.Set("User-Agent", appUserAgent)
	}
}
