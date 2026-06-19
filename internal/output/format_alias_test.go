package output

import "testing"

// ParseFormat now comes from lib-agent-output and is intentionally more lenient
// than the pre-migration parser: it accepts "ndjson"/"yml" aliases and is
// case-insensitive. Pin that as intended contract.
func TestParseFormatAliases(t *testing.T) {
	cases := map[string]Format{
		"json":   FormatJSON,
		"JSON":   FormatJSON,
		"yaml":   FormatYAML,
		"yml":    FormatYAML,
		"YAML":   FormatYAML,
		"jsonl":  FormatNDJSON,
		"ndjson": FormatNDJSON,
	}
	for in, want := range cases {
		got, err := ParseFormat(in)
		if err != nil {
			t.Errorf("ParseFormat(%q) errored: %v", in, err)
			continue
		}
		if got != want {
			t.Errorf("ParseFormat(%q) = %q, want %q", in, got, want)
		}
	}
	if _, err := ParseFormat("toml"); err == nil {
		t.Error("ParseFormat(toml) should error")
	}
}
