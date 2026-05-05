package api

import (
	"context"
	"net/http"
)

// Validate checks that the API key is valid by hitting the /v1/validate endpoint.
func (c *Client) Validate(ctx context.Context) error {
	_, err := c.do(ctx, http.MethodGet, "/v1/validate", nil)
	return err
}
