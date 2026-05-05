package api

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
