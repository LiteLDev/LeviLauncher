package client

import (
	"bytes"
	"fmt"
	"io"
	"log/slog"
	"net/http"
)

// Logger is a logger definition to be used with client
type Logger interface {
	Printf(format string, v ...any)
	Println(v ...any)
	DebugRequest(res *http.Response)
}

type noopLogger struct {
	Logger
}

func (l noopLogger) Printf(_ string, _ ...any) {
}

func (l noopLogger) Println(_ ...any) {
}

func (l noopLogger) DebugRequest(_ *http.Response) {
}

type DefaultSlogClientLogger struct {
	Logger
	l *slog.Logger
}

// NewDefaultSlogClientLogger creates a default logger using log/slog
func NewDefaultSlogClientLogger(logger *slog.Logger) Logger {
	return &DefaultSlogClientLogger{l: logger}
}

func (l DefaultSlogClientLogger) Printf(format string, args ...any) {
	slog.Debug(fmt.Sprintf(format, args...))
}

func (l DefaultSlogClientLogger) Println(v ...any) {
	slog.Debug(fmt.Sprintf("%v", v))
}

func (l DefaultSlogClientLogger) DebugRequest(res *http.Response) {
	buff, _ := io.ReadAll(res.Body)
	res.Body = io.NopCloser(bytes.NewReader(buff))
	slog.With(
		"request", map[string]any{
			"url":         res.Request.URL.String(),
			"req_uri":     res.Request.RequestURI,
			"response":    string(buff),
			"status":      res.Status,
			"status_code": res.StatusCode,
			"method":      res.Request.Method,
		}).Debug("CurseforgeAPIRequest")
}
