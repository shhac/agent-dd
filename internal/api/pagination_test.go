package api_test

import (
	"testing"

	"github.com/shhac/agent-dd/internal/api"
)

func TestCursorFrom(t *testing.T) {
	tests := []struct {
		name string
		meta *api.SearchMeta
		want string
	}{
		{"nil meta", nil, ""},
		{"meta with nil page", &api.SearchMeta{}, ""},
		{"page with empty After", &api.SearchMeta{Page: &api.SearchMetaPage{}}, ""},
		{"page with cursor", &api.SearchMeta{Page: &api.SearchMetaPage{After: "next-cursor"}}, "next-cursor"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := api.CursorFrom(tt.meta); got != tt.want {
				t.Errorf("CursorFrom() = %q, want %q", got, tt.want)
			}
		})
	}
}
