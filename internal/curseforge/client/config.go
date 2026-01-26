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
	CurseforgeBaseURL = "https://api.curseforge.com"

	defaultTimeout = 30 * time.Second
	xAPIKeyHeader  = "x-api-key"
)

type Config struct {
	apiKey  string
	baseURL string
	timeout time.Duration
	debug   bool
	log     Logger
}

func NewConfig(apiKey string, cfgs ...CfgFunc) *Config {
	cfg := NewDefaultConfig(apiKey)
	for _, c := range cfgs {
		c(cfg)
	}
	return cfg
}

func NewDefaultConfig(apiKey string) *Config {
	return &Config{
		apiKey:  apiKey,
		baseURL: CurseforgeBaseURL,
		timeout: defaultTimeout,
		debug:   false,
		log:     noopLogger{},
	}
}

func (cfg *Config) IsDebug() bool {
	return cfg.debug
}

func (cfg *Config) NewHTTPClient() *http.Client {
	return &http.Client{
		Timeout: cfg.timeout,
	}
}

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

type CfgFunc func(config *Config)

func WithEndpoint(endpoint string) CfgFunc {
	return func(cfg *Config) {
		cfg.baseURL = endpoint
	}
}

func WithTimeout(timeout time.Duration) CfgFunc {
	return func(cfg *Config) {
		cfg.timeout = timeout
	}
}

func EnableDebug(enabled bool) CfgFunc {
	return func(cfg *Config) {
		cfg.debug = enabled
	}
}

func WithLogger(log Logger) CfgFunc {
	return func(cfg *Config) {
		cfg.log = log
	}
}
