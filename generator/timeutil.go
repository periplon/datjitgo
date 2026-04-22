package generator

import "time"

// timelike is an alias so expression helpers can return a comparable date
// without exposing stdlib time in their signatures.
type timelike = time.Time

// parseYMD parses an ISO-8601 date (YYYY-MM-DD). Returns (_, false) on any
// formatting mismatch so callers can fall back gracefully.
func parseYMD(s string) (time.Time, bool) {
	t, err := time.Parse("2006-01-02", s)
	if err != nil {
		return time.Time{}, false
	}
	return t, true
}
