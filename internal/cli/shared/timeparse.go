package shared

import (
	"strconv"
	"strings"
	"time"

	agenterrors "github.com/shhac/agent-dd/internal/errors"
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
	numStr := rest[:len(rest)-1]
	num, err := strconv.Atoi(numStr)
	if err != nil {
		return time.Time{}, agenterrors.Newf(agenterrors.FixableByAgent, "invalid relative time %q", s)
	}

	var duration time.Duration
	switch unit {
	case 's':
		duration = time.Duration(num) * time.Second
	case 'm':
		duration = time.Duration(num) * time.Minute
	case 'h':
		duration = time.Duration(num) * time.Hour
	case 'd':
		duration = time.Duration(num) * 24 * time.Hour
	case 'w':
		duration = time.Duration(num) * 7 * 24 * time.Hour
	default:
		return time.Time{}, agenterrors.Newf(agenterrors.FixableByAgent,
			"invalid time unit %q in %q — use s, m, h, d, or w", string(unit), s)
	}

	return now.Add(sign * duration), nil
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
