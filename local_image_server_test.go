package main

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func TestLocalImageRegistryServesRegisteredFile(t *testing.T) {
	t.Parallel()

	tempDir := t.TempDir()
	imagePath := filepath.Join(tempDir, "background.png")
	imageBytes := []byte("\x89PNG\r\n\x1a\nlocal-image-test")
	if err := os.WriteFile(imagePath, imageBytes, 0o600); err != nil {
		t.Fatalf("write image: %v", err)
	}

	registry := newLocalImageRegistry()
	imageURL := registry.register(imagePath)
	if imageURL == "" {
		t.Fatal("expected a local image URL")
	}

	handler := registry.middleware(http.NotFoundHandler())
	request := httptest.NewRequest(http.MethodGet, imageURL, nil)
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusOK)
	}
	if got := response.Body.Bytes(); string(got) != string(imageBytes) {
		t.Fatalf("body = %q, want %q", got, imageBytes)
	}
	if got := response.Header().Get("Cache-Control"); got != "no-store" {
		t.Fatalf("Cache-Control = %q, want no-store", got)
	}
}

func TestLocalImageRegistryRejectsUnknownAndInvalidPaths(t *testing.T) {
	t.Parallel()

	registry := newLocalImageRegistry()
	if got := registry.register(""); got != "" {
		t.Fatalf("register empty path = %q, want empty", got)
	}
	if got := registry.register(t.TempDir()); got != "" {
		t.Fatalf("register directory = %q, want empty", got)
	}

	handler := registry.middleware(http.NotFoundHandler())
	request := httptest.NewRequest(http.MethodGet, localImageURLPrefix+"unknown", nil)
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)

	if response.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusNotFound)
	}
}

func TestLocalImageRegistryPassesOtherRequestsThrough(t *testing.T) {
	t.Parallel()

	registry := newLocalImageRegistry()
	handler := registry.middleware(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		writer.WriteHeader(http.StatusNoContent)
	}))
	request := httptest.NewRequest(http.MethodGet, "/assets/index.js", nil)
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)

	if response.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusNoContent)
	}
}
