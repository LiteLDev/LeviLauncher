package client

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"

	"github.com/liteldev/LeviLauncher/internal/curseforge/client/types"
)

const (
	getGameVersionsPath    = "/v2/games/%s/versions"
	minecraftModSearchPath = "/v1/mods/search"
	getCategoriesPath      = "/v1/categories"
)

type CurseClient interface {
	GetGameVersions(ctx context.Context, gameID string) (versions *types.GameVersionsResponse, err error)
	GetCategories(ctx context.Context, gameID string) (*types.GetCategoriesResponse, error)
	GetMods(ctx context.Context, opts ...ModsQueryOption) (*types.ModsResponse, error)
	GetModsByIDs(ctx context.Context, req *GetModsByIdsListRequest, opts ...ModsQueryOption) (*types.ModsResponse, error)
	GetModDescription(ctx context.Context, modID int64) (*types.StringResponse, error)
	GetModFiles(ctx context.Context, modID int64) (*types.GetModFilesResponse, error)
}

type curseClient struct {
	CurseClient
	opt Config
	c   *http.Client
}

// NewCurseClient creates a new Curseforge client
func NewCurseClient(apiKey string, opts ...CfgFunc) CurseClient {
	opt := NewConfig(apiKey, opts...)
	return &curseClient{
		opt: *opt,
		c:   opt.NewHTTPClient(),
	}
}

func (c *curseClient) GetGameVersions(ctx context.Context, gameID string) (versions *types.GameVersionsResponse, err error) {
	log := c.opt.log
	url := c.buildRequestPath(fmt.Sprintf(getGameVersionsPath, gameID), ApiQueryParams{})
	if c.opt.debug {
		log.Printf("GetGameVersions.Begin id: %s (url: %s)", gameID, url)
	}
	req, err := c.opt.NewGetRequest(url)
	if err != nil {
		return nil, fmt.Errorf("creating get game versions request: %w", err)
	}

	var gv types.GameVersionsResponse
	if err := c.executeRequest(req.WithContext(ctx), "get game versions", &gv); err != nil {
		return nil, fmt.Errorf("executing get game versions request: %w", err)
	}

	return &gv, nil
}

func (c *curseClient) GetCategories(ctx context.Context, gameID string) (*types.GetCategoriesResponse, error) {
	q := ApiQueryParams{
		"gameId": gameID,
	}
	req, err := c.opt.NewGetRequest(c.buildRequestPath(getCategoriesPath, q))
	if err != nil {
		return nil, fmt.Errorf("creating get categories request: %w", err)
	}

	var resp types.GetCategoriesResponse
	if err := c.executeRequest(req.WithContext(ctx), "get categories", &resp); err != nil {
		return nil, fmt.Errorf("executing get categories request: %w", err)
	}

	return &resp, nil
}

func (c *curseClient) GetModFiles(ctx context.Context, modID int64) (*types.GetModFilesResponse, error) {
	path := fmt.Sprintf("/v1/mods/%d/files", modID)
	req, err := c.opt.NewGetRequest(path)
	if err != nil {
		return nil, fmt.Errorf("creating get mod files request: %w", err)
	}

	var resp types.GetModFilesResponse
	if err := c.executeRequest(req.WithContext(ctx), "get mod files", &resp); err != nil {
		return nil, fmt.Errorf("executing get mod files request: %w", err)
	}

	return &resp, nil
}

// GetMods lists mods for a game from API
func (c *curseClient) GetMods(ctx context.Context, opts ...ModsQueryOption) (*types.ModsResponse, error) {
	q := ApiQueryParams{}
	for _, o := range opts {
		o(q)
	}
	var result types.ModsResponse
	req, err := c.opt.NewGetRequest(c.buildRequestPath(minecraftModSearchPath, q))
	if err != nil {
		err = fmt.Errorf("creating get minecraft mods request: %w", err)
		return &result, types.Wrap(err, "failed to create request object", -1)
	}

	var mv types.ModsResponse
	if err := c.executeRequest(req.WithContext(ctx), "get mods", &mv); err != nil {
		err = fmt.Errorf("executing get minecraft mods request: %w", err)
		return &result, types.Wrap(err, "failed to execute request", -1)
	}

	return &mv, nil
}

func (c *curseClient) GetModsByIDs(ctx context.Context, filter *GetModsByIdsListRequest, opts ...ModsQueryOption) (*types.ModsResponse, error) {
	if filter == nil || len(filter.ModIds) < 1 {
		return nil, fmt.Errorf("%w: invalid filter, mod ids required (%#v)", types.ErrInvalidRequestParams, filter)
	}
	q := ApiQueryParams{}
	for _, o := range opts {
		o(q)
	}
	var result types.ModsResponse
	req, err := c.opt.NewPostRequest("/v1/mods", filter)
	if err != nil {
		err = fmt.Errorf("creating get minecraft mods request: %w", err)
		return &result, types.Wrap(err, "failed to create request object", -1)
	}

	var mv types.ModsResponse
	if err := c.executeRequest(req.WithContext(ctx), "get mods by ids", &mv); err != nil {
		return &result, types.Wrap(err, "failed to execute request", -1)
	}

	return &mv, nil
}

func (c *curseClient) GetModDescription(ctx context.Context, modID int64) (*types.StringResponse, error) {
	path := fmt.Sprintf("/v1/mods/%d/description", modID)
	req, err := c.opt.NewGetRequest(path)
	if err != nil {
		return nil, fmt.Errorf("creating get mod description request: %w", err)
	}

	var resp types.StringResponse
	if err := c.executeRequest(req.WithContext(ctx), "get mod description", &resp); err != nil {
		return nil, fmt.Errorf("executing get mod description request: %w", err)
	}

	return &resp, nil
}

func (c *curseClient) buildRequestPath(path string, q ApiQueryParams) string {
	return fmt.Sprintf("%s?%s", path, q.QueryString())
}

func (c *curseClient) parseResponse(res *http.Response, step string, debug bool, out types.CurseforgeAPIResponse) error {
	if res.StatusCode/100 != 2 {
		b, _ := io.ReadAll(res.Body)
		return types.Wrap(
			fmt.Errorf("%s request failed with status code %d", step, res.StatusCode),
			string(b),
			res.StatusCode,
		)
	}

	b, err := io.ReadAll(res.Body)
	if err != nil {
		return fmt.Errorf("reading get game versions response: %w", err)
	}

	bodyAsString := string(b)

	if err := json.Unmarshal(b, out); err != nil {
		return types.Wrap(
			fmt.Errorf("decoding get game versions response: %w", err),
			string(b),
			res.StatusCode,
		)
	}
	out.SetRawResponseBody(bodyAsString)

	return nil
}

func (c *curseClient) executeRequest(req *http.Request, step string, out types.CurseforgeAPIResponse) error {
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Content-Type", "application/json")

	reqData := map[string]any{
		"step":   step,
		"url":    req.URL.String(),
		"method": req.Method,
	}

	if req.Body != nil {
		b, err := io.ReadAll(req.Body)
		if err != nil {
			return fmt.Errorf("reading api request body: %w", err)
		}

		reqData["body"] = string(b)

		req.Body = io.NopCloser(bytes.NewReader(b))
	}

	res, err := c.c.Do(req)
	if err != nil {
		err = fmt.Errorf("executing get minecraft mods request: %w", err)
		slog.With("request", reqData, "error", err).Error("APIRequest")
		return types.Wrap(err, "failed to execute request", -1)
	}

	reqData["status_code"] = res.StatusCode

	resBody, err := io.ReadAll(res.Body)
	if err != nil {
		return fmt.Errorf("reading api response body: %w", err)
	}

	reqData["response_body"] = string(resBody)
	res.Body = io.NopCloser(bytes.NewReader(resBody))

	slog.With("request", reqData).Info("APIRequest")

	if err := c.parseResponse(res, step, c.opt.debug, out); err != nil {
		return fmt.Errorf("parsing api response response: %w", err)
	}

	return nil
}
