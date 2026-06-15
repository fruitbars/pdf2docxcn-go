package pdfconv

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"
)

// RequestUpload requests a presigned upload URL for a new conversion task.
func (c *Client) RequestUpload(ctx context.Context, filename, convType string, size int64) (*UploadInfo, error) {
	const path = "/openapi/v2/upload"

	body, err := json.Marshal(map[string]interface{}{
		"filename": filename,
		"type":     convType,
		"size":     size,
	})
	if err != nil {
		return nil, err
	}
	bodyLen := strconv.Itoa(len(body))

	headers := c.authHeaders("POST", path, "", bodyLen)
	headers["Content-Type"] = "application/json"

	req, err := http.NewRequestWithContext(ctx, "POST", c.host+path, bytes.NewReader(body))
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
			TaskID        string            `json:"taskID"`
			UploadURL     string            `json:"uploadURL"`
			UploadHeaders map[string]string `json:"uploadHeaders"`
			CommitURL     string            `json:"commitURL"`
			ExpireAt      int64             `json:"expireAt"`
		} `json:"data"`
	}
	if err := json.Unmarshal(raw, &result); err != nil {
		return nil, fmt.Errorf("parse response: %w", err)
	}
	d := result.Data
	return &UploadInfo{
		TaskID:        d.TaskID,
		UploadURL:     d.UploadURL,
		UploadHeaders: d.UploadHeaders,
		CommitURL:     d.CommitURL,
		ExpireAt:      d.ExpireAt,
	}, nil
}

// UploadFile uploads a file to the presigned URL using the headers from RequestUpload.
// No API signing needed — auth is embedded in the upload headers.
func (c *Client) UploadFile(ctx context.Context, uploadURL string, uploadHeaders map[string]string, filePath string) error {
	f, err := os.Open(filePath)
	if err != nil {
		return err
	}
	defer f.Close()

	stat, err := f.Stat()
	if err != nil {
		return err
	}

	req, err := http.NewRequestWithContext(ctx, "PUT", uploadURL, f)
	if err != nil {
		return err
	}
	req.ContentLength = stat.Size()
	for k, v := range uploadHeaders {
		req.Header.Set(k, v)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		raw, _ := io.ReadAll(resp.Body)
		return parseAPIError(raw, resp.StatusCode)
	}
	return nil
}
