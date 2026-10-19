package dates

import (
	"strings"
	"time"
)

// Parse parses a YAML date or datetime, in local time unless it has an
// explicit offset.
func Parse(s string) (time.Time, bool) {
	for _, layout := range []string{"2006-01-02", "2006-01-02 15:04", "2006-01-02 15:04:05", time.RFC3339} {
		if t, err := time.ParseInLocation(layout, s, time.Local); err == nil {
			return t, true
		}
	}
	return time.Time{}, false
}

// Format reformats a YAML date or datetime as yyyy/mm/dd, returning s
// unchanged if it can't be parsed.
func Format(s string) string {
	if t, ok := Parse(s); ok {
		return t.Format("2006/01/02")
	}
	return s
}

// PostFolder returns the folder name for post name published on date.
func PostFolder(name string, date time.Time) string {
	return date.Format("20060102") + "_" + name
}

// PostName returns folder without its leading yyyymmdd date and the
// underscore after it, if it has them. It undoes PostFolder.
func PostName(folder string) string {
	if len(folder) >= 8 && (len(folder) == 8 || folder[8] == '_') {
		if _, err := time.Parse("20060102", folder[:8]); err == nil {
			return strings.TrimPrefix(folder[8:], "_")
		}
	}
	return folder
}
