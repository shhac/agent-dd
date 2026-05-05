package shared

import (
	"os"
	"strconv"
	"strings"
	"time"

	agenterrors "github.com/shhac/agent-dd/internal/errors"
	"github.com/shhac/agent-dd/internal/output"
)

// ParseTime parses relative (now-15m), RFC3339, or unix epoch time strings.
func ParseTime(s string) (time.Time, error) {
	if s == "" {
		return time.Time{}, nil
	}

	if s == "now" {
		return time.Now(), nil
	}

	if strings.HasPrefix(s, "now") {
		return parseRelativeTime(s)
	}

	if t, err := time.Parse(time.RFC3339, s); err == nil {
		return t, nil
	}

	if epoch, err := strconv.ParseInt(s, 10, 64); err == nil {
		return time.Unix(epoch, 0), nil
	}

	return time.Time{}, agenterrors.Newf(agenterrors.FixableByAgent,
		"invalid time format %q — use relative (now-15m), RFC3339 (2024-01-15T10:00:00Z), or unix epoch", s)
}

var relativeTimeUnits = map[byte]time.Duration{
	's': time.Second,
	'm': time.Minute,
	'h': time.Hour,
	'd': 24 * time.Hour,
	'w': 7 * 24 * time.Hour,
}

func parseRelativeTime(s string) (time.Time, error) {
	now := time.Now()
	rest := s[3:] // strip "now"

	if rest == "" {
		return now, nil
	}

	var sign time.Duration = -1
	switch rest[0] {
	case '+':
		sign = 1
		rest = rest[1:]
	case '-':
		rest = rest[1:]
	default:
		return time.Time{}, agenterrors.Newf(agenterrors.FixableByAgent, "invalid relative time %q", s)
	}

	if len(rest) < 2 {
		return time.Time{}, agenterrors.Newf(agenterrors.FixableByAgent, "invalid relative time %q", s)
	}

	unit := rest[len(rest)-1]
	unitDur, ok := relativeTimeUnits[unit]
	if !ok {
		return time.Time{}, agenterrors.Newf(agenterrors.FixableByAgent,
			"invalid time unit %q in %q — use s, m, h, d, or w", string(unit), s)
	}

	num, err := strconv.Atoi(rest[:len(rest)-1])
	if err != nil {
		return time.Time{}, agenterrors.Newf(agenterrors.FixableByAgent, "invalid relative time %q", s)
	}

	return now.Add(sign * time.Duration(num) * unitDur), nil
}

// ParseTimeDefaultFrom returns the parsed --from time, defaulting to 1 hour ago.
func ParseTimeDefaultFrom(s string) (time.Time, error) {
	if s == "" {
		return time.Now().Add(-1 * time.Hour), nil
	}
	return ParseTime(s)
}

// ParseTimeDefaultTo returns the parsed --to time, defaulting to now.
func ParseTimeDefaultTo(s string) (time.Time, error) {
	if s == "" {
		return time.Now(), nil
	}
	return ParseTime(s)
}

// ParseTimeRange parses a from/to time pair with defaults (from: now-1h, to: now).
func ParseTimeRange(from, to string) (time.Time, time.Time, error) {
	fromTime, err := ParseTimeDefaultFrom(from)
	if err != nil {
		return time.Time{}, time.Time{}, err
	}
	toTime, err := ParseTimeDefaultTo(to)
	if err != nil {
		return time.Time{}, time.Time{}, err
	}
	return fromTime, toTime, nil
}

// ParseTimeRangeOrWriteErr is the cobra-RunE convenience wrapper around
// ParseTimeRange: on error it writes to stderr and returns ok=false so the
// caller can `return nil` for the standard "swallow" behaviour.
func ParseTimeRangeOrWriteErr(from, to string) (fromTime, toTime time.Time, ok bool) {
	fromTime, toTime, err := ParseTimeRange(from, to)
	if err != nil {
		output.WriteError(os.Stderr, err)
		return time.Time{}, time.Time{}, false
	}
	return fromTime, toTime, true
}
