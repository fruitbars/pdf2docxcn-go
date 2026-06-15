package pdfconv

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
)

// CommitConversion notifies the server that upload is complete and starts conversion.
func (c *Client) CommitConversion(ctx context.Context, taskID string) error {
	const path = "/openapi/v2/commit"
	query := "taskID=" + url.QueryEscape(taskID)

	headers := c.authHeaders("POST", path, query, "0")
	req, err := http.NewRequestWithContext(ctx, "POST", c.host+path+"?"+query, nil)
	if err != nil {
		return err
	}
	for k, v := range headers {
		req.Header.Set(k, v)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	raw, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return parseAPIError(raw, resp.StatusCode)
	}
	return nil
}

// GetStatus returns the current status of a conversion task.
func (c *Client) GetStatus(ctx context.Context, taskID string) (*Status, error) {
	const path = "/openapi/v2/status"
	query := "taskID=" + url.QueryEscape(taskID)

	headers := c.authHeaders("GET", path, query, "0")
	req, err := http.NewRequestWithContext(ctx, "GET", c.host+path+"?"+query, nil)
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
		TaskID   string `json:"taskID"`
		Status   string `json:"status"`
		Progress int    `json:"progress"`
	}
	if err := json.Unmarshal(raw, &result); err != nil {
		return nil, fmt.Errorf("parse response: %w", err)
	}
	return &Status{TaskID: result.TaskID, Status: result.Status, Progress: result.Progress}, nil
}
