package api

import (
	"io"
	"log"
	"net/http"
)

// Option configures a Client.
type Option func(*Client)

// WithAccessToken sets the access token for API authentication.
// The token is injected as a query parameter via a custom Transport.
func WithAccessToken(token string) Option {
	return func(c *Client) {
		c.accessToken = token
	}
}

// WithHTTPClient sets a custom HTTP client.
func WithHTTPClient(hc *http.Client) Option {
	return func(c *Client) {
		c.httpClient = hc
	}
}

// WithBaseURL sets a custom base URL for the API.
func WithBaseURL(url string) Option {
	return func(c *Client) {
		c.rawBaseURL = url
	}
}

// WithPCSBaseURL sets a custom PCS base URL for upload/download operations.
// Default: https://d.pcs.baidu.com
func WithPCSBaseURL(url string) Option {
	return func(c *Client) {
		c.rawPCSURL = url
	}
}

// WithDebug enables debug logging of requests and responses.
func WithDebug(debug bool) Option {
	return func(c *Client) {
		c.debug = debug
	}
}

// WithLogger sets a custom logger for debug output.
// If not set, the standard log package is used.
func WithLogger(w io.Writer) Option {
	return func(c *Client) {
		c.logger = log.New(w, "[baidupan] ", log.LstdFlags)
	}
}

// WithAPIKey sets the API key for authentication.
// The key is injected as a query parameter "api_key" in every request.
func WithAPIKey(key string) Option {
	return func(c *Client) {
		c.apiKey = key
	}
}

