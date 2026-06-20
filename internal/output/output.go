// Package output re-exports the shared output contract from lib-agent-output,
// keeping the internal/output import path while the wire mechanism (format
// parsing, JSON encoding, error rendering) lives in one place. YAML encoding is
// supplied by the shared lib-agent-cli/yaml encoder (blank-imported below).
// What stays local is agent-dd policy: the null-pruning Print signature, the
// convenience ResolveFormat that swallows parse errors into the default, and
// the Datadog-shaped pagination trailer. (Migration shim.)
package output

import (
	"encoding/json"
	"io"
	"os"

	out "github.com/shhac/lib-agent-output"

	// Blank-imported for its init(): registers the shared YAML encoder for
	// FormatYAML, so agent-dd gets `--format yaml` without copying the encoder.
	_ "github.com/shhac/lib-agent-cli/yaml"
)

// Format and its values come from the shared contract; ParseFormat is therefore
// the family's lenient parser (accepts "ndjson"/"yml", case-insensitive).
type Format = out.Format

const (
	FormatJSON   = out.FormatJSON
	FormatYAML   = out.FormatYAML
	FormatNDJSON = out.FormatNDJSON
)

var (
	ParseFormat = out.ParseFormat
	WriteError  = out.WriteError
)

// ResolveFormat returns the parsed flag value, or defaultFormat when the flag is
// empty or unparseable. This keeps agent-dd's single-return signature (the
// shared out.ResolveFormat also returns an error); a bad flag silently falls
// back to the default, as it always has here.
func ResolveFormat(flagFormat string, defaultFormat Format) Format {
	f, err := out.ResolveFormat(flagFormat, defaultFormat)
	if err != nil {
		return defaultFormat
	}
	return f
}

// Print cleans (optional prune) then encodes data in the given format via the
// shared encoder.
func Print(data any, format Format, prune bool) {
	cleaned, ok := toCleanAny(data, prune)
	if !ok {
		return
	}
	// Data is already cleaned, so pass a nil pruner — out.Print just encodes.
	_ = out.Print(os.Stdout, cleaned, format, nil)
}

// PrintJSON is a convenience wrapper for JSON output.
func PrintJSON(data any, prune bool) {
	Print(data, FormatJSON, prune)
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

// Pagination is Datadog-shaped (a cursor + total count), so it stays local
// rather than using out.Pagination.
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

func toCleanAny(data any, prune bool) (any, bool) {
	b, err := json.Marshal(data)
	if err != nil {
		return nil, false
	}
	var decoded any
	if err := json.Unmarshal(b, &decoded); err != nil {
		return nil, false
	}
	if prune {
		decoded = out.PruneNils(decoded)
	}
	return decoded, true
}
