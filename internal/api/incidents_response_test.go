package api_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/shhac/agent-dd/internal/api"
)

// commanderHandle has several fall-through paths (nil relationships, nil
// data, no matching included entry, wrong type, fallback chain across
// handle->email->name). The doc resolver in the resp helper exercises the
// same code through ListIncidents/GetIncident; this table drives it
// directly via small synthetic responses.
func TestCommanderHandleEdgeCases(t *testing.T) {
	cases := []struct {
		name     string
		body     string
		want     string
		wantNone bool
	}{
		{
			name: "nil relationships",
			body: `{"data":{"id":"i1","type":"incidents","attributes":{}}}`,
			want: "",
		},
		{
			name: "nil commander data",
			body: `{"data":{"id":"i1","type":"incidents","relationships":{"commander_user":{"data":null}}}}`,
			want: "",
		},
		{
			name: "commander id with no matching included",
			body: `{"data":{"id":"i1","type":"incidents","relationships":{"commander_user":{"data":{"id":"missing-uuid","type":"users"}}}},"included":[{"id":"other-uuid","type":"users","attributes":{"handle":"x"}}]}`,
			want: "",
		},
		{
			name: "included entry has wrong type",
			body: `{"data":{"id":"i1","type":"incidents","relationships":{"commander_user":{"data":{"id":"u1","type":"users"}}}},"included":[{"id":"u1","type":"teams","attributes":{"handle":"x"}}]}`,
			want: "",
		},
		{
			name: "fallback to email when handle empty",
			body: `{"data":{"id":"i1","type":"incidents","relationships":{"commander_user":{"data":{"id":"u1","type":"users"}}}},"included":[{"id":"u1","type":"users","attributes":{"handle":"","email":"bob@example.com","name":"Bob"}}]}`,
			want: "bob@example.com",
		},
		{
			name: "fallback to name when handle and email empty",
			body: `{"data":{"id":"i1","type":"incidents","relationships":{"commander_user":{"data":{"id":"u1","type":"users"}}}},"included":[{"id":"u1","type":"users","attributes":{"name":"Carol"}}]}`,
			want: "Carol",
		},
		{
			name: "handle takes precedence over email and name",
			body: `{"data":{"id":"i1","type":"incidents","relationships":{"commander_user":{"data":{"id":"u1","type":"users"}}}},"included":[{"id":"u1","type":"users","attributes":{"handle":"alice","email":"alice@example.com","name":"Alice"}}]}`,
			want: "alice",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				_, _ = w.Write([]byte(tc.body))
			}))
			defer srv.Close()

			client := api.NewTestClient(srv.URL+"/api", "k", "a")
			doc, err := client.GetIncident(context.Background(), "i1")
			if err != nil {
				t.Fatalf("GetIncident: %v", err)
			}
			if got := doc.CommanderHandle(); got != tc.want {
				t.Errorf("CommanderHandle = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestIncidentListResponseHasMore(t *testing.T) {
	tests := []struct {
		name string
		resp *api.IncidentListResponse
		want bool
	}{
		{"nil Meta", &api.IncidentListResponse{Meta: nil}, false},
		{"nil Pagination", &api.IncidentListResponse{Meta: &api.IncidentListMeta{Pagination: nil}}, false},
		{"NextOffset > Offset", &api.IncidentListResponse{Meta: &api.IncidentListMeta{Pagination: &api.IncidentPagination{Offset: 0, NextOffset: 25}}}, true},
		{"NextOffset == Offset", &api.IncidentListResponse{Meta: &api.IncidentListMeta{Pagination: &api.IncidentPagination{Offset: 25, NextOffset: 25}}}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.resp.HasMore(); got != tt.want {
				t.Errorf("HasMore() = %v, want %v", got, tt.want)
			}
		})
	}
}
