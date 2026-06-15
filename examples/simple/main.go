package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/fruitbars/pdf2docxcn-go/pdfconv"
)

func main() {
	// Initialize client
	client := pdfconv.NewClient(
		pdfconv.DefaultHost,
		"your-api-key",
		"your-api-secret",
	)

	ctx := context.Background()

	// Example 1: Simple one-step conversion
	fmt.Println("=== Example 1: Simple Conversion ===")
	outputPath, err := client.Convert(ctx, "input.pdf", "./output", "pdf2word")
	if err != nil {
		log.Fatalf("Conversion failed: %v", err)
	}
	fmt.Printf("✓ Conversion complete: %s\n\n", outputPath)

	// Example 2: Step-by-step conversion with progress monitoring
	fmt.Println("=== Example 2: Step-by-Step Conversion ===")

	// Step 1: Request upload
	stat, err := os.Stat("input.pdf")
	if err != nil {
		log.Fatalf("Stat failed: %v", err)
	}
	uploadInfo, err := client.RequestUpload(ctx, "example.pdf", "pdf2word", stat.Size())
	if err != nil {
		log.Fatalf("Request upload failed: %v", err)
	}
	fmt.Printf("✓ Upload requested, task ID: %s\n", uploadInfo.TaskID)

	// Step 2: Upload file
	err = client.UploadFile(ctx, uploadInfo.UploadURL, uploadInfo.UploadHeaders, "input.pdf")
	if err != nil {
		log.Fatalf("Upload failed: %v", err)
	}
	fmt.Println("✓ File uploaded")

	// Step 3: Commit conversion
	err = client.CommitConversion(ctx, uploadInfo.TaskID)
	if err != nil {
		log.Fatalf("Commit failed: %v", err)
	}
	fmt.Println("✓ Conversion started")

	// Step 4: Poll for completion
	fmt.Print("⏳ Converting")
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			log.Fatal("Context cancelled")
		case <-ticker.C:
			status, err := client.GetStatus(ctx, uploadInfo.TaskID)
			if err != nil {
				log.Fatalf("Get status failed: %v", err)
			}

			fmt.Print(".")

			switch status.Status {
			case "completed":
				fmt.Println(" Done!")
				goto download
			case "failed":
				log.Fatalf("\n✗ Conversion failed")
			case "queued", "processing":
				// Continue polling
				continue
			default:
				log.Fatalf("\n✗ Unknown status: %s", status.Status)
			}
		}
	}

download:
	// Step 5: Get download URL
	downloadInfo, err := client.GetDownloadURL(ctx, uploadInfo.TaskID)
	if err != nil {
		log.Fatalf("Get download URL failed: %v", err)
	}
	fmt.Printf("✓ Download URL obtained: %s\n", downloadInfo.Filename)

	// Step 6: Download file
	err = client.DownloadFile(ctx, downloadInfo.DownloadURL, "./output/"+downloadInfo.Filename)
	if err != nil {
		log.Fatalf("Download failed: %v", err)
	}
	fmt.Printf("✓ File downloaded: ./output/%s\n", downloadInfo.Filename)
}
