package client

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"github.com/liteldev/LeviLauncher/internal/lip/client/types"
)

const BaseURL = "https://api.bedrinth.com/v3"

type Client struct {
	httpClient *http.Client
}

func NewClient() *Client {
	return &Client{
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

func (c *Client) SearchPackages(q string, perPage int, page int, sort string, order string) (*types.SearchPackagesResponse, error) {
	u, err := url.Parse(BaseURL + "/packages")
	if err != nil {
		return nil, err
	}
	qParams := u.Query()
	if q != "" {
		qParams.Set("q", q)
	}
	if perPage > 0 {
		qParams.Set("perPage", strconv.Itoa(perPage))
	}
	if page > 0 {
		qParams.Set("page", strconv.Itoa(page))
	}
	if sort != "" {
		qParams.Set("sort", sort)
	}
	if order != "" {
		qParams.Set("order", order)
	}
	u.RawQuery = qParams.Encode()

	resp, err := c.httpClient.Get(u.String())
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("api returned status: %d", resp.StatusCode)
	}

	var result struct {
		Data types.SearchPackagesResponse `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	return &result.Data, nil
}

func (c *Client) GetPackage(identifier string) (*types.GetPackageResponse, error) {
	u, err := url.Parse(BaseURL + "/packages/" + identifier)
	if err != nil {
		return nil, err
	}

	resp, err := c.httpClient.Get(u.String())
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("api returned status: %d", resp.StatusCode)
	}

	var result struct {
		Data types.GetPackageResponse `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	return &result.Data, nil
}
