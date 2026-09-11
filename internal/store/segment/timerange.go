package segment

import (
	"fmt"
	"strings"
	"time"
)

// timeFormats accepted for --from/--to and the HTTP API's from/to params,
// tried in order. RFC3339 is preferred; the others are convenience shorthands
// interpreted in the machine's local timezone.
var timeFormats = []string{
	time.RFC3339,
	"2006-01-02T15:04:05",
	"2006-01-02 15:04:05",
	"2006-01-02 15:04",
	"2006-01-02",
}

// ResolveRange turns the user-facing time selectors into a concrete [from, to]
// window. Precedence:
//
//   - since (a duration like "2h" or "30m"): window is [now-since, now].
//   - otherwise from/to are parsed individually; an empty bound stays zero,
//     which Extract treats as open-ended (beginning-of-time / now).
//
// It is shared by the `loog extract` command and the recorder's HTTP API so
// both accept identical inputs.
func ResolveRange(fromStr, toStr, sinceStr string) (from, to time.Time, err error) {
	if s := strings.TrimSpace(sinceStr); s != "" {
		d, derr := time.ParseDuration(s)
		if derr != nil {
			return time.Time{}, time.Time{}, fmt.Errorf("invalid --since %q: %w", s, derr)
		}
		if d <= 0 {
			return time.Time{}, time.Time{}, fmt.Errorf("--since must be positive, got %q", s)
		}
		now := time.Now()
		return now.Add(-d), now, nil
	}

	if s := strings.TrimSpace(fromStr); s != "" {
		from, err = parseTime(s)
		if err != nil {
			return time.Time{}, time.Time{}, fmt.Errorf("invalid --from %q: %w", s, err)
		}
	}
	if s := strings.TrimSpace(toStr); s != "" {
		to, err = parseTime(s)
		if err != nil {
			return time.Time{}, time.Time{}, fmt.Errorf("invalid --to %q: %w", s, err)
		}
	}
	if !from.IsZero() && !to.IsZero() && to.Before(from) {
		return time.Time{}, time.Time{}, fmt.Errorf("--to (%s) is before --from (%s)", to, from)
	}
	return from, to, nil
}

func parseTime(s string) (time.Time, error) {
	for _, layout := range timeFormats {
		if t, err := time.ParseInLocation(layout, s, time.Local); err == nil {
			return t, nil
		}
	}
	return time.Time{}, fmt.Errorf("unrecognized time format (try RFC3339, e.g. 2006-01-02T15:04:05Z)")
}
