package output

import (
	"encoding/json"
	"io"
	"os"

	agenterrors "github.com/shhac/agent-dd/internal/errors"
	"gopkg.in/yaml.v3"
)

type Format string

const (
	FormatJSON   Format = "json"
	FormatYAML   Format = "yaml"
	FormatNDJSON Format = "jsonl"
)

func ParseFormat(s string) (Format, error) {
	switch s {
	case "json":
		return FormatJSON, nil
	case "yaml":
		return FormatYAML, nil
	case "jsonl", "ndjson":
		return FormatNDJSON, nil
	default:
		return "", agenterrors.Newf(agenterrors.FixableByAgent, "unknown format %q, expected: json, yaml, jsonl", s)
	}
}

func ResolveFormat(flagFormat string, defaultFormat Format) Format {
	if flagFormat == "" {
		return defaultFormat
	}
	f, err := ParseFormat(flagFormat)
	if err != nil {
		return defaultFormat
	}
	return f
}

func Print(data any, format Format, prune bool) {
	switch format {
	case FormatYAML:
		printYAML(data, prune)
	default:
		printJSON(data, prune)
	}
}

// PrintJSON is a convenience wrapper for JSON output.
func PrintJSON(data any, prune bool) {
	printJSON(data, prune)
}

func printJSON(data any, prune bool) {
	b, err := json.Marshal(data)
	if err != nil {
		return
	}
	var decoded any
	if err := json.Unmarshal(b, &decoded); err != nil {
		return
	}
	if prune {
		decoded = pruneNulls(decoded)
	}
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	enc.SetEscapeHTML(false)
	_ = enc.Encode(decoded)
}

func printYAML(data any, prune bool) {
	b, err := json.Marshal(data)
	if err != nil {
		return
	}
	var m any
	if err := json.Unmarshal(b, &m); err != nil {
		return
	}
	if prune {
		m = pruneNulls(m)
	}
	enc := yaml.NewEncoder(os.Stdout)
	enc.SetIndent(2)
	_ = enc.Encode(m)
}

func WriteError(w io.Writer, err error) {
	var aerr *agenterrors.APIError
	if !agenterrors.As(err, &aerr) {
		aerr = agenterrors.Wrap(err, agenterrors.FixableByAgent)
	}
	payload := map[string]any{
		"error":      aerr.Message,
		"fixable_by": string(aerr.FixableBy),
	}
	if aerr.Hint != "" {
		payload["hint"] = aerr.Hint
	}
	enc := json.NewEncoder(w)
	enc.SetEscapeHTML(false)
	_ = enc.Encode(payload)
}

type NDJSONWriter struct {
	enc *json.Encoder
}

func NewNDJSONWriter(w io.Writer) *NDJSONWriter {
	enc := json.NewEncoder(w)
	enc.SetEscapeHTML(false)
	return &NDJSONWriter{enc: enc}
}

func (n *NDJSONWriter) WriteItem(item any) error {
	return n.enc.Encode(item)
}

type Pagination struct {
	HasMore    bool   `json:"has_more"`
	TotalItems int    `json:"total_items,omitempty"`
	NextCursor string `json:"next_cursor,omitempty"`
}

// Meta-row keys. NDJSON output mixes data rows (full per-item objects)
// with meta rows (single-key objects keyed by one of these `@`-prefixed
// names). Consumers filter on the key to separate the two streams. New
// meta keys should be added here, not as inline string literals at call
// sites, so the convention stays codified.
const (
	MetaKeyPagination = "@pagination"
	MetaKeyCounts     = "@counts"
	MetaKeySkipped    = "@skipped"
)

func (n *NDJSONWriter) WritePagination(p *Pagination) error {
	return n.enc.Encode(map[string]any{MetaKeyPagination: p})
}

// WriteMetaLine emits a single NDJSON line keyed by `key` with the given
// value. `key` should be one of the MetaKey* constants, or at least follow
// the `@`-prefix convention so consumers can filter it from the data stream.
func (n *NDJSONWriter) WriteMetaLine(key string, value any) error {
	return n.enc.Encode(map[string]any{key: value})
}

func pruneNulls(v any) any {
	switch val := v.(type) {
	case map[string]any:
		out := make(map[string]any, len(val))
		for k, v := range val {
			if v == nil {
				continue
			}
			out[k] = pruneNulls(v)
		}
		return out
	case []any:
		out := make([]any, len(val))
		for i, v := range val {
			out[i] = pruneNulls(v)
		}
		return out
	default:
		return v
	}
}
