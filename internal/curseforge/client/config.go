package client

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/liteldev/LeviLauncher/internal/curseforge/client/types"
)

const (
	// CurseforgeBaseURL default API base path
	CurseforgeBaseURL = "https://api.curseforge.com"

	defaultTimeout = 30 * time.Second
	xAPIKeyHeader  = "x-api-key"
)

// Config is the client config data
type Config struct {
	apiKey  string
	baseURL string
	timeout time.Duration
	debug   bool
	log     Logger
}

// NewConfig creates a new client Config with default values
func NewConfig(apiKey string, cfgs ...CfgFunc) *Config {
	cfg := NewDefaultConfig(apiKey)
	for _, c := range cfgs {
		c(cfg)
	}
	return cfg
}

// NewDefaultConfig creates a new client Config with default values
func NewDefaultConfig(apiKey string) *Config {
	return &Config{
		apiKey:  apiKey,
		baseURL: CurseforgeBaseURL,
		timeout: defaultTimeout,
		debug:   false,
		log:     noopLogger{},
	}
}

// IsDebug is debug enabled
func (cfg *Config) IsDebug() bool {
	return cfg.debug
}

// NewHTTPClient creates a new HTTP client
func (cfg *Config) NewHTTPClient() *http.Client {
	return &http.Client{
		Timeout: cfg.timeout,
	}
}

// NewGetRequest creates a new GET request object to be used with client
func (cfg *Config) NewGetRequest(path string) (*http.Request, error) {
	req, err := http.NewRequest(http.MethodGet, fmt.Sprintf("%s%s", cfg.baseURL, path), nil)
	if err != nil {
		log.Printf("Failed to create request object: %s", err.Error())
		return req, err
	}
	req.Header.Add(xAPIKeyHeader, cfg.apiKey)
	req.Header.Add("Accept", "application/json")

	return req, nil
}

// NewPostRequest creates a new POST request object to be used with client
func (cfg *Config) NewPostRequest(path string, body any) (*http.Request, error) {
	jsonBody, err := json.Marshal(body)
	if err != nil {
		return nil, &types.CurseforgeAPIError{
			Status:  -1,
			Message: "marshalling request body",
			Err:     fmt.Errorf("marshalling request body: %w", err),
		}

	}

	url := fmt.Sprintf("%s%s", cfg.baseURL, path)

	req, err := http.NewRequest(http.MethodPost, url, bytes.NewBuffer(jsonBody))
	if err != nil {
		log.Printf("Failed to create request object: %s", err.Error())
		return req, err
	}
	req.Header.Add(xAPIKeyHeader, cfg.apiKey)

	req.Header.Add("Content-Type", "application/json")
	req.Header.Add("Accept", "application/json")

	return req, nil
}

/*
CfgFunc is configuration function
*/
type CfgFunc func(config *Config)

/*
WithEndpoint to define the API endpoint to be used
*/
func WithEndpoint(endpoint string) CfgFunc {
	return func(cfg *Config) {
		cfg.baseURL = endpoint
	}
}

/*
WithTimeout to define the API client timeout to be used
*/
func WithTimeout(timeout time.Duration) CfgFunc {
	return func(cfg *Config) {
		cfg.timeout = timeout
	}
}

/*
EnableDebug to define the API endpoint to be used
*/
func EnableDebug(enabled bool) CfgFunc {
	return func(cfg *Config) {
		cfg.debug = enabled
	}
}

/*
WithLogger defines the logger to be used
*/
func WithLogger(log Logger) CfgFunc {
	return func(cfg *Config) {
		cfg.log = log
	}
}
