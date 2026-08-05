package main

import (
	"crypto/rand"
	"encoding/base64"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

const (
	localImageURLPrefix  = "/__levilauncher/local-image/"
	localImageTokenLimit = 64
)

type localImageRegistry struct {
	mu     sync.RWMutex
	paths  map[string]string
	tokens []string
}

func newLocalImageRegistry() *localImageRegistry {
	return &localImageRegistry{
		paths: make(map[string]string),
	}
}

func (registry *localImageRegistry) register(path string) string {
	if registry == nil {
		return ""
	}

	cleanPath, err := filepath.Abs(strings.TrimSpace(path))
	if err != nil || cleanPath == "" {
		return ""
	}

	info, err := os.Stat(cleanPath)
	if err != nil || !info.Mode().IsRegular() {
		return ""
	}

	tokenBytes := make([]byte, 24)
	if _, err := rand.Read(tokenBytes); err != nil {
		return ""
	}
	token := base64.RawURLEncoding.EncodeToString(tokenBytes)

	registry.mu.Lock()
	defer registry.mu.Unlock()

	registry.paths[token] = cleanPath
	registry.tokens = append(registry.tokens, token)
	if len(registry.tokens) > localImageTokenLimit {
		expiredToken := registry.tokens[0]
		registry.tokens = registry.tokens[1:]
		delete(registry.paths, expiredToken)
	}

	return localImageURLPrefix + token
}

func (registry *localImageRegistry) middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if !strings.HasPrefix(request.URL.Path, localImageURLPrefix) {
			next.ServeHTTP(writer, request)
			return
		}

		if request.Method != http.MethodGet && request.Method != http.MethodHead {
			writer.Header().Set("Allow", http.MethodGet+", "+http.MethodHead)
			http.Error(writer, http.StatusText(http.StatusMethodNotAllowed), http.StatusMethodNotAllowed)
			return
		}

		token := strings.TrimPrefix(request.URL.Path, localImageURLPrefix)
		if token == "" || strings.Contains(token, "/") {
			http.NotFound(writer, request)
			return
		}

		registry.mu.RLock()
		path, ok := registry.paths[token]
		registry.mu.RUnlock()
		if !ok {
			http.NotFound(writer, request)
			return
		}

		file, err := os.Open(path)
		if err != nil {
			http.NotFound(writer, request)
			return
		}
		defer file.Close()

		info, err := file.Stat()
		if err != nil || !info.Mode().IsRegular() {
			http.NotFound(writer, request)
			return
		}

		writer.Header().Set("Cache-Control", "no-store")
		writer.Header().Set("X-Content-Type-Options", "nosniff")
		http.ServeContent(writer, request, filepath.Base(path), info.ModTime(), file)
	})
}
