package pdfconv

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
)

// GetDownloadURL returns the download URL for a completed conversion.
func (c *Client) GetDownloadURL(ctx context.Context, taskID string) (*DownloadInfo, error) {
	const path = "/openapi/v2/download"
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
		Data struct {
			TaskID      string `json:"taskID"`
			DownloadURL string `json:"downloadURL"`
			Filename    string `json:"filename"`
			ExpireAt    int64  `json:"expireAt"`
		} `json:"data"`
	}
	if err := json.Unmarshal(raw, &result); err != nil {
		return nil, fmt.Errorf("parse response: %w", err)
	}
	d := result.Data
	return &DownloadInfo{TaskID: d.TaskID, DownloadURL: d.DownloadURL, Filename: d.Filename, ExpireAt: d.ExpireAt}, nil
}

// DownloadFile downloads the file at downloadURL and saves it to outputPath.
// No API auth needed — the URL is already signed.
func (c *Client) DownloadFile(ctx context.Context, downloadURL, outputPath string) error {
	req, err := http.NewRequestWithContext(ctx, "GET", downloadURL, nil)
	if err != nil {
		return err
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		raw, _ := io.ReadAll(resp.Body)
		return parseAPIError(raw, resp.StatusCode)
	}

	if err := os.MkdirAll(filepath.Dir(outputPath), 0755); err != nil {
		return err
	}

	out, err := os.Create(outputPath)
	if err != nil {
		return err
	}
	defer out.Close()

	if _, err = io.Copy(out, resp.Body); err != nil {
		out.Close()
		os.Remove(outputPath)
		return err
	}
	return nil
}
