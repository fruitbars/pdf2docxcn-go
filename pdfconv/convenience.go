package pdfconv

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// Convert is a convenience method that runs the full 6-step conversion workflow:
// RequestUpload → UploadFile → CommitConversion → poll GetStatus → GetDownloadURL → DownloadFile.
// It blocks until the conversion completes or ctx is cancelled.
// DefaultConvertTimeout is the max time Convert() will wait for a conversion
// when the caller's ctx has no deadline. 15 minutes covers the server-side cap.
const DefaultConvertTimeout = 15 * time.Minute

func (c *Client) Convert(ctx context.Context, inputPath, outputDir, convType string) (string, error) {
	// Apply a default deadline so a stalled server task doesn't block forever.
	if _, hasDeadline := ctx.Deadline(); !hasDeadline {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, DefaultConvertTimeout)
		defer cancel()
	}

	stat, err := os.Stat(inputPath)
	if err != nil {
		return "", err
	}

	uploadInfo, err := c.RequestUpload(ctx, filepath.Base(inputPath), convType, stat.Size())
	if err != nil {
		return "", fmt.Errorf("request upload: %w", err)
	}

	if err := c.UploadFile(ctx, uploadInfo.UploadURL, uploadInfo.UploadHeaders, inputPath); err != nil {
		return "", fmt.Errorf("upload: %w", err)
	}

	if err := c.CommitConversion(ctx, uploadInfo.TaskID); err != nil {
		return "", fmt.Errorf("commit: %w", err)
	}

	for {
		select {
		case <-ctx.Done():
			return "", ctx.Err()
		case <-time.After(5 * time.Second):
			status, err := c.GetStatus(ctx, uploadInfo.TaskID)
			if err != nil {
				return "", fmt.Errorf("status: %w", err)
			}
			switch status.Status {
			case "completed":
				// exit switch, proceed to download below
			case "failed":
				return "", fmt.Errorf("conversion failed for task %s", uploadInfo.TaskID)
			default:
				continue
			}

			dlInfo, err := c.GetDownloadURL(ctx, uploadInfo.TaskID)
			if err != nil {
				return "", fmt.Errorf("download url: %w", err)
			}
			outputPath := filepath.Join(outputDir, dlInfo.Filename)
			if err := c.DownloadFile(ctx, dlInfo.DownloadURL, outputPath); err != nil {
				return "", fmt.Errorf("download: %w", err)
			}
			return outputPath, nil
		}
	}
}
