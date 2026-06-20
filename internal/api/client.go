package api

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"

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

	// Debug, when set, logs one line per request (method + redacted URL) to
	// stderr before it is sent. Wired from the global --debug flag.
	Debug bool
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

// RawRequest is the escape hatch behind `agent-dd api`: it issues an arbitrary
// request through the same credential/site resolution and error classification
// as every typed command, returning the response body verbatim. `path` is
// relative to the site's `/api` base (e.g. "/v2/spans/analytics/aggregate").
// `body` may be nil, a json.RawMessage, or any JSON-marshalable value.
func (c *Client) RawRequest(ctx context.Context, method, path string, body any) (json.RawMessage, error) {
	return c.do(ctx, method, path, body)
}

// RequestPreview describes the request RawRequest would send, with credentials
// redacted. It backs `agent-dd api --print-request` so callers can confirm
// exactly what would hit Datadog (and tell a CLI construction bug apart from a
// genuine API rejection) without sending anything or ever seeing the keys.
type RequestPreview struct {
	Method  string            `json:"method"`
	URL     string            `json:"url"`
	Headers map[string]string `json:"headers"`
	Body    json.RawMessage   `json:"body,omitempty"`
}

// PreviewRequest builds a redacted preview of the request for the given inputs
// without sending it. Header values for the credential headers are masked.
func (c *Client) PreviewRequest(method, path string, body json.RawMessage) RequestPreview {
	var requestBody any
	if len(body) > 0 {
		requestBody = body
	}
	req, err := c.buildRequest(context.Background(), method, path, requestBody)
	if err != nil {
		return RequestPreview{
			Method: method,
			URL:    c.baseURL + path,
			Headers: map[string]string{
				"DD-API-KEY":         redactSecret(c.apiKey),
				"DD-APPLICATION-KEY": redactSecret(c.appKey),
			},
			Body: body,
		}
	}

	headers := map[string]string{}
	for key, vals := range req.Header {
		if key == "Dd-Api-Key" || key == "Dd-Application-Key" {
			continue
		}
		if len(vals) == 0 {
			continue
		}
		headers[key] = vals[0]
	}
	headers["DD-API-KEY"] = redactSecret(c.apiKey)
	headers["DD-APPLICATION-KEY"] = redactSecret(c.appKey)

	var previewBody json.RawMessage
	if req.Body != nil {
		previewBody, _ = io.ReadAll(req.Body)
	}

	return RequestPreview{
		Method:  method,
		URL:     req.URL.String(),
		Headers: headers,
		Body:    previewBody,
	}
}

// redactSecret masks a credential, revealing nothing but its presence and
// length so a preview can confirm a key is set without leaking it.
func redactSecret(s string) string {
	if s == "" {
		return "(unset)"
	}
	return fmt.Sprintf("(set, %d chars)", len(s))
}

func (c *Client) do(ctx context.Context, method, path string, body any) (json.RawMessage, error) {
	req, err := c.buildRequest(ctx, method, path, body)
	if err != nil {
		return nil, err
	}

	if c.Debug {
		fmt.Fprintf(os.Stderr, "[debug] %s %s\n", method, c.baseURL+path)
	}

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

func (c *Client) buildRequest(ctx context.Context, method, path string, body any) (*http.Request, error) {
	var bodyReader io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return nil, agenterrors.Wrap(err, agenterrors.FixableByAgent)
		}
		bodyReader = bytes.NewReader(b)
	}

	req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, bodyReader)
	if err != nil {
		return nil, agenterrors.Wrap(err, agenterrors.FixableByAgent)
	}

	req.Header.Set("DD-API-KEY", c.apiKey)
	req.Header.Set("DD-APPLICATION-KEY", c.appKey)
	req.Header.Set("Content-Type", "application/json")
	return req, nil
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
