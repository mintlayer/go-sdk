// Copyright (c) 2026 Mintlayer Institutional FZCO
// Contact: hello@mintlayer.org
//
// Use of this source code is governed by an MIT license
// that can be found in the LICENSE file.

// Package indexer provides an HTTP REST client for the Mintlayer indexer
// (api-web-server, default mainnet port 3000). All endpoints are under /api/v2/.
package indexer

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// HTTPError is returned when the server responds with a non-2xx status code.
type HTTPError struct {
	StatusCode int
	Body       string
}

func (e *HTTPError) Error() string {
	return fmt.Sprintf("HTTP %d: %s", e.StatusCode, e.Body)
}

// Client is an HTTP REST client for the Mintlayer indexer (api-web-server).
type Client struct {
	apiBase    string
	httpClient *http.Client
}

// Option configures a Client.
type Option func(*Client)

// WithHTTPClient replaces the default HTTP client.
func WithHTTPClient(hc *http.Client) Option {
	return func(c *Client) {
		c.httpClient = hc
	}
}

// WithTimeout sets a per-request timeout on the default HTTP client.
// Cannot be combined with WithHTTPClient — the last option applied wins.
func WithTimeout(d time.Duration) Option {
	return func(c *Client) {
		c.httpClient = &http.Client{Timeout: d}
	}
}

// New creates a Client targeting baseURL (e.g. "http://127.0.0.1:3000").
func New(baseURL string, opts ...Option) *Client {
	c := &Client{
		apiBase:    strings.TrimRight(baseURL, "/") + "/api/v2",
		httpClient: &http.Client{Timeout: 30 * time.Second},
	}
	for _, o := range opts {
		o(c)
	}
	return c
}

// get performs a GET request to apiBase+path with optional query parameters.
// The JSON response body is decoded into out (pass nil to discard the body).
func (c *Client) get(ctx context.Context, path string, query url.Values, out any) error {
	rawURL := c.apiBase + path
	if len(query) > 0 {
		rawURL += "?" + query.Encode()
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Accept", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("http request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(resp.Body)
		return &HTTPError{StatusCode: resp.StatusCode, Body: strings.TrimSpace(string(body))}
	}

	if out != nil {
		if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
			return fmt.Errorf("decode response: %w", err)
		}
	}
	return nil
}

// post performs a POST request to apiBase+path with a plain-text body.
// The JSON response body is decoded into out (pass nil to discard the body).
func (c *Client) post(ctx context.Context, path string, body string, out any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.apiBase+path, strings.NewReader(body))
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", "text/plain")
	req.Header.Set("Accept", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("http request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		respBody, _ := io.ReadAll(resp.Body)
		return &HTTPError{StatusCode: resp.StatusCode, Body: strings.TrimSpace(string(respBody))}
	}

	if out != nil {
		if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
			return fmt.Errorf("decode response: %w", err)
		}
	}
	return nil
}
