# PDF Conversion SDK for Go

Go SDK for the pdf2docx.cn API v2, providing easy PDF conversion capabilities.

## Features

- Simple one-line conversion API
- Step-by-step conversion control with progress monitoring
- Full error handling with typed API errors
- Context support for cancellation and timeouts
- Comprehensive test coverage

## Installation

```bash
go get github.com/fruitbars/pdf2docxcn-go
```

## Quick Start

```go
package main

import (
    "context"
    "log"
    
    "github.com/fruitbars/pdf2docxcn-go/pdfconv"
)

func main() {
    client := pdfconv.NewClient(
        pdfconv.DefaultHost,
        "your-api-key",
        "your-api-secret",
    )
    
    outputPath, err := client.Convert(
        context.Background(),
        "input.pdf",
        "./output",
        "pdf2word",
    )
    if err != nil {
        log.Fatal(err)
    }
    
    log.Printf("Converted: %s", outputPath)
}
```

## API Reference

### Client Creation

```go
// Create client with default HTTP settings
client := pdfconv.NewClient(host, apiKey, apiSecret)

// Create client with custom HTTP client
httpClient := &http.Client{Timeout: 5 * time.Minute}
client := pdfconv.NewClientWithHTTP(host, apiKey, apiSecret, httpClient)
```

### Simple Conversion

The `Convert` method handles the entire workflow automatically:

```go
outputPath, err := client.Convert(ctx, inputPath, outputDir, convType)
```

**Parameters:**
- `ctx`: Context for cancellation
- `inputPath`: Path to input PDF file
- `outputDir`: Directory to save converted file
- `convType`: Conversion type (e.g., "pdf2docx", "pdf2xlsx")

**Returns:**
- `outputPath`: Full path to the converted file
- `err`: Error if conversion failed

### Step-by-Step Conversion

For more control over the conversion process:

#### 1. Request Upload

```go
uploadInfo, err := client.RequestUpload(ctx, filename, convType)
```

Returns `UploadInfo` with:
- `TaskID`: Unique task identifier
- `UploadURL`: Pre-signed upload URL
- `UploadHeaders`: Required headers for upload
- `CommitURL`: URL to commit conversion
- `ExpireAt`: Upload URL expiration timestamp

#### 2. Upload File

```go
err := client.UploadFile(ctx, uploadInfo.UploadURL, uploadInfo.UploadHeaders, filePath)
```

#### 3. Commit Conversion

```go
err := client.CommitConversion(ctx, uploadInfo.TaskID)
```

#### 4. Check Status

```go
status, err := client.GetStatus(ctx, taskID)
```

Returns `Status` with:
- `TaskID`: Task identifier
- `Status`: "queued", "processing", "completed", or "failed"
- `Progress`: Progress percentage (0-100)

#### 5. Get Download URL

```go
downloadInfo, err := client.GetDownloadURL(ctx, taskID)
```

Returns `DownloadInfo` with:
- `TaskID`: Task identifier
- `DownloadURL`: Pre-signed download URL
- `Filename`: Suggested output filename
- `ExpireAt`: Download URL expiration timestamp

#### 6. Download File

```go
err := client.DownloadFile(ctx, downloadInfo.DownloadURL, outputPath)
```

## Error Handling

The SDK provides typed errors for API failures:

```go
info, err := client.RequestUpload(ctx, "file.pdf", "pdf2docx")
if err != nil {
    if apiErr, ok := err.(*pdfconv.APIError); ok {
        log.Printf("API error: %s (code: %s, status: %d)", 
            apiErr.Message, apiErr.Code, apiErr.HTTPStatus)
    } else {
        log.Printf("Network error: %v", err)
    }
}
```

Common API error codes:
- `unauthorized`: Invalid API credentials
- `quota_exhausted`: Usage quota exceeded
- `invalid_params`: Invalid request parameters
- `task_not_found`: Task ID not found
- `task_not_ready`: Conversion not yet complete

## Advanced Usage

### Custom Timeout

```go
ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
defer cancel()

outputPath, err := client.Convert(ctx, inputPath, outputDir, convType)
```

### Progress Monitoring

```go
ticker := time.NewTicker(2 * time.Second)
defer ticker.Stop()

for {
    select {
    case <-ticker.C:
        status, err := client.GetStatus(ctx, taskID)
        if err != nil {
            return err
        }
        
        log.Printf("Progress: %d%%", status.Progress)
        
        if status.Status == "completed" {
            goto download
        } else if status.Status == "failed" {
            return fmt.Errorf("conversion failed")
        }
    }
}

download:
// Proceed with download...
```

### Custom HTTP Client

```go
httpClient := &http.Client{
    Timeout: 5 * time.Minute,
    Transport: &http.Transport{
        Proxy: http.ProxyFromEnvironment,
        TLSClientConfig: &tls.Config{
            MinVersion: tls.VersionTLS12,
        },
    },
}

client := pdfconv.NewClientWithHTTP(host, apiKey, apiSecret, httpClient)
```

## Examples

See the [examples](examples/) directory for complete examples:
- [simple](examples/simple/main.go): Basic usage examples

## Testing

Run the test suite:

```bash
cd sdk/pdfconv
go test -v
```

Run tests with coverage:

```bash
go test -v -cover
```

## License

See LICENSE file for details.

## Support

For API documentation and support, visit: https://pdf2docx.cn/docs
