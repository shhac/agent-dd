package api

import (
	"encoding/json"
	"fmt"

	agenterrors "github.com/shhac/agent-dd/internal/errors"
)

// extractErrorMessage tries multiple Datadog error response formats and falls
// back to the raw body (only when JSON parsing fails) or the status code, so
// the caller always gets a useful message.
//
// Note: we deliberately do NOT use the raw-body fallback when JSON parsed
// successfully but had no recognizable field — Datadog frequently returns
// `{"errors":[]}` and we'd rather surface "HTTP 400" than "HTTP 400: {…}".
func extractErrorMessage(status int, body []byte) string {
	var parsed struct {
		Errors  []string `json:"errors"`
		Error   string   `json:"error"`
		Message string   `json:"message"`
	}
	if err := json.Unmarshal(body, &parsed); err != nil {
		if len(body) > 0 && len(body) <= 200 {
			return fmt.Sprintf("HTTP %d: %s", status, string(body))
		}
		return fmt.Sprintf("HTTP %d", status)
	}

	switch {
	case len(parsed.Errors) > 0 && parsed.Errors[0] != "":
		return parsed.Errors[0]
	case parsed.Error != "":
		return parsed.Error
	case parsed.Message != "":
		return parsed.Message
	default:
		return fmt.Sprintf("HTTP %d", status)
	}
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
