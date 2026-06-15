package pdfconv

import (
	"net/http"
	"time"
)

// DefaultHost is the production API endpoint.
const DefaultHost = "https://api.pdf2docx.cn"

// Client is a client for the pdf2docx.cn API v2.
type Client struct {
	host       string
	apiKey     string
	apiSecret  string
	httpClient *http.Client
}

// NewClient creates a new Client with default settings.
func NewClient(host, apiKey, apiSecret string) *Client {
	return &Client{
		host:      host,
		apiKey:    apiKey,
		apiSecret: apiSecret,
		httpClient: &http.Client{
			Timeout: 120 * time.Second,
		},
	}
}

// NewClientWithHTTP creates a new Client with a custom http.Client.
// Use this when you need custom timeout, proxy, or TLS settings.
func NewClientWithHTTP(host, apiKey, apiSecret string, httpClient *http.Client) *Client {
	return &Client{
		host:       host,
		apiKey:     apiKey,
		apiSecret:  apiSecret,
		httpClient: httpClient,
	}
}
