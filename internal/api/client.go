package api

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"

	agenterrors "github.com/shhac/agent-dd/internal/errors"
)

func siteToBaseURL(site string) string {
	if site == "" {
		site = "datadoghq.com"
	}
	return fmt.Sprintf("https://api.%s/api", site)
}

type Client struct {
	baseURL string
	apiKey  string
	appKey  string
	http    *http.Client
}

func NewClient(apiKey, appKey, site string) *Client {
	return &Client{
		baseURL: siteToBaseURL(site),
		apiKey:  apiKey,
		appKey:  appKey,
		http:    &http.Client{},
	}
}

func NewTestClient(baseURL, apiKey, appKey string) *Client {
	return &Client{
		baseURL: baseURL,
		apiKey:  apiKey,
		appKey:  appKey,
		http:    &http.Client{},
	}
}

func (c *Client) do(ctx context.Context, method, path string, body any) (json.RawMessage, error) {
	reqURL := c.baseURL + path

	var bodyReader io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return nil, agenterrors.Wrap(err, agenterrors.FixableByAgent)
		}
		bodyReader = bytes.NewReader(b)
	}

	req, err := http.NewRequestWithContext(ctx, method, reqURL, bodyReader)
	if err != nil {
		return nil, agenterrors.Wrap(err, agenterrors.FixableByAgent)
	}

	req.Header.Set("DD-API-KEY", c.apiKey)
	req.Header.Set("DD-APPLICATION-KEY", c.appKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, agenterrors.Wrap(err, agenterrors.FixableByRetry).WithHint("Network error — check connectivity")
	}
	defer func() { _ = resp.Body.Close() }()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, agenterrors.Wrap(err, agenterrors.FixableByRetry)
	}

	if resp.StatusCode >= 400 {
		return nil, classifyHTTPError(resp.StatusCode, respBody)
	}

	return json.RawMessage(respBody), nil
}

func doAndDecode[T any](c *Client, ctx context.Context, method, path string, body any) (*T, error) {
	raw, err := c.do(ctx, method, path, body)
	if err != nil {
		return nil, err
	}
	var result T
	if err := json.Unmarshal(raw, &result); err != nil {
		return nil, agenterrors.Wrap(err, agenterrors.FixableByAgent)
	}
	return &result, nil
}

// doAndDecodeField decodes a JSON response and extracts a nested field.
// Used for APIs that wrap results in envelopes like {"data": ...} or {"event": ...}.
func doAndDecodeField[W any, T any](c *Client, ctx context.Context, method, path string, body any, extract func(*W) *T) (*T, error) {
	wrapper, err := doAndDecode[W](c, ctx, method, path, body)
	if err != nil {
		return nil, err
	}
	return extract(wrapper), nil
}

// SearchMeta is the shared pagination metadata for v2 search APIs (logs, traces).
type SearchMeta struct {
	Page *SearchMetaPage `json:"page,omitempty"`
}

type SearchMetaPage struct {
	After string `json:"after,omitempty"`
}

// CursorFrom extracts the pagination cursor, returning empty if not present.
func CursorFrom(meta *SearchMeta) string {
	if meta != nil && meta.Page != nil {
		return meta.Page.After
	}
	return ""
}

// buildPath appends query parameters to a base path if any are set.
func buildPath(base string, params url.Values) string {
	if encoded := params.Encode(); encoded != "" {
		return base + "?" + encoded
	}
	return base
}

// extractErrorMessage tries multiple Datadog error response formats and falls
// back to the raw body or status code so the caller always gets a useful message.
func extractErrorMessage(status int, body []byte) string {
	var parsed struct {
		Errors  []string `json:"errors"`
		Error   string   `json:"error"`
		Message string   `json:"message"`
	}
	if json.Unmarshal(body, &parsed) == nil {
		switch {
		case len(parsed.Errors) > 0 && parsed.Errors[0] != "":
			return parsed.Errors[0]
		case parsed.Error != "":
			return parsed.Error
		case parsed.Message != "":
			return parsed.Message
		}
	} else if len(body) > 0 && len(body) <= 200 {
		return fmt.Sprintf("HTTP %d: %s", status, string(body))
	}
	return fmt.Sprintf("HTTP %d", status)
}

func classifyHTTPError(status int, body []byte) *agenterrors.APIError {
	msg := extractErrorMessage(status, body)

	switch {
	case status == 401:
		return agenterrors.New("Authentication failed: "+msg, agenterrors.FixableByHuman).
			WithHint("Check your API/app keys with 'agent-dd org test'")
	case status == 403:
		return agenterrors.New("Permission denied: "+msg, agenterrors.FixableByHuman).
			WithHint("Your API key may not have sufficient permissions")
	case status == 404:
		return agenterrors.New("Not found: "+msg, agenterrors.FixableByAgent).
			WithHint("Check the ID or name — use 'list' to see available items")
	case status == 429:
		return agenterrors.New("Rate limited", agenterrors.FixableByRetry).
			WithHint("Datadog rate limit hit — wait and retry")
	case status >= 500:
		return agenterrors.New("Datadog API error: "+msg, agenterrors.FixableByRetry).
			WithHint("Datadog server error — retry in a few seconds")
	default:
		return agenterrors.New(msg, agenterrors.FixableByAgent)
	}
}

// Validate checks that the API key is valid by hitting the /v1/validate endpoint.
func (c *Client) Validate(ctx context.Context) error {
	_, err := c.do(ctx, http.MethodGet, "/v1/validate", nil)
	return err
}
