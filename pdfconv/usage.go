package pdfconv

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

// GetUsage returns the current API quota status.
func (c *Client) GetUsage(ctx context.Context) (*Usage, error) {
	const path = "/openapi/v2/usage"

	headers := c.authHeaders("GET", path, "", "0")
	req, err := http.NewRequestWithContext(ctx, "GET", c.host+path, nil)
	if err != nil {
		return nil, err
	}
	for k, v := range headers {
		req.Header.Set(k, v)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	raw, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return nil, parseAPIError(raw, resp.StatusCode)
	}

	var result struct {
		Data struct {
			Enabled     bool   `json:"enabled"`
			Description string `json:"description"`
			CountMode   string `json:"countMode"`
			Calls       struct {
				Total     int `json:"total"`
				Used      int `json:"used"`
				Remaining int `json:"remaining"`
			} `json:"calls"`
			Pages struct {
				Total     int `json:"total"`
				Used      int `json:"used"`
				Remaining int `json:"remaining"`
			} `json:"pages"`
		} `json:"data"`
	}
	if err := json.Unmarshal(raw, &result); err != nil {
		return nil, fmt.Errorf("parse response: %w", err)
	}
	d := result.Data
	return &Usage{
		Enabled:     d.Enabled,
		Description: d.Description,
		CountMode:   d.CountMode,
		Calls:       UsageQuota{Total: d.Calls.Total, Used: d.Calls.Used, Remaining: d.Calls.Remaining},
		Pages:       UsageQuota{Total: d.Pages.Total, Used: d.Pages.Used, Remaining: d.Pages.Remaining},
	}, nil
}
