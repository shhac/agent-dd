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

// doAndDecodeData is a specialization of doAndDecodeField for the JSON:API
// {"data": T} envelope used by every v2 endpoint. Saves each call site from
// declaring a one-off wrapper struct and identity extractor.
func doAndDecodeData[T any](c *Client, ctx context.Context, method, path string, body any) (*T, error) {
	type wrapper struct {
		Data T `json:"data"`
	}
	w, err := doAndDecode[wrapper](c, ctx, method, path, body)
	if err != nil {
		return nil, err
	}
	return &w.Data, nil
}

// buildPath appends query parameters to a base path if any are set.
func buildPath(base string, params url.Values) string {
	if encoded := params.Encode(); encoded != "" {
		return base + "?" + encoded
	}
	return base
}
