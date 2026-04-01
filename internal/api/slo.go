package api

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
)

type SLOListResponse struct {
	Data []SLO `json:"data"`
}

func (c *Client) ListSLOs(ctx context.Context, search string, tags []string) ([]SLO, error) {
	params := url.Values{}
	if search != "" {
		params.Set("query", search)
	}
	for _, tag := range tags {
		params.Add("tags_query", tag)
	}

	resp, err := doAndDecode[SLOListResponse](c, ctx, http.MethodGet, buildPath("/v1/slo", params), nil)
	if err != nil {
		return nil, err
	}
	return resp.Data, nil
}

func (c *Client) GetSLO(ctx context.Context, id string) (*SLO, error) {
	type resp struct {
		Data SLO `json:"data"`
	}
	path := fmt.Sprintf("/v1/slo/%s", url.PathEscape(id))
	return doAndDecodeField[resp, SLO](c, ctx, http.MethodGet, path, nil, func(r *resp) *SLO { return &r.Data })
}

func (c *Client) GetSLOHistory(ctx context.Context, id string, from, to int64) (*SLOHistory, error) {
	type resp struct {
		Data SLOHistory `json:"data"`
	}
	params := url.Values{
		"from_ts": {fmt.Sprintf("%d", from)},
		"to_ts":   {fmt.Sprintf("%d", to)},
	}
	path := fmt.Sprintf("/v1/slo/%s/history?%s", url.PathEscape(id), params.Encode())
	return doAndDecodeField[resp, SLOHistory](c, ctx, http.MethodGet, path, nil, func(r *resp) *SLOHistory { return &r.Data })
}
