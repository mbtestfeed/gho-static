package gho_static

import (
	"strings"
	"time"
)

// FrontMatter is the YAML-style metadata block at the top of a markdown page.
type FrontMatter struct {
	Title     string
	Summary   string // the page's meta description
	Author    string
	Published string
	Edited    string
	Draft     bool
	TOC       bool     // add a table of contents to the top of the page
	Tags      []string // as [a, b], a, b or "- a" lines beneath

	Unreleased bool // Published is set and after the build's time; set by the build, not parsed
}

// parseFrontMatter splits a leading "---" delimited block of "key: value"
// lines off md, returning the parsed metadata and the remaining markdown.
// Unknown keys are ignored.
func parseFrontMatter(md []byte) (FrontMatter, []byte) {
	var fm FrontMatter
	s := strings.ReplaceAll(string(md), "\r\n", "\n")
	if !strings.HasPrefix(s, "---\n") {
		return fm, md
	}
	end := strings.Index(s[4:], "\n---")
	if end == -1 {
		return fm, md
	}
	header, body := s[4:4+end], s[4+end+len("\n---"):]

	inTags := false // whether "- tag" lines follow a tags key
	for _, line := range strings.Split(header, "\n") {
		if item, ok := strings.CutPrefix(strings.TrimSpace(line), "- "); ok && inTags {
			fm.Tags = append(fm.Tags, parseList(item)...)
			continue
		}
		inTags = false
		key, val, ok := strings.Cut(line, ":")
		if !ok {
			continue
		}
		val = strings.Trim(strings.TrimSpace(val), `"'`)
		switch strings.TrimSpace(key) {
		case "title":
			fm.Title = val
		case "summary":
			fm.Summary = val
		case "tags":
			fm.Tags, inTags = parseList(val), true
		case "author":
			fm.Author = val
		case "published":
			fm.Published = val
		case "edited":
			fm.Edited = val
		case "draft":
			fm.Draft = val == "true"
		case "toc":
			fm.TOC = val == "true"
		}
	}
	return fm, []byte(strings.TrimLeft(body, "\n"))
}

// parseList splits a YAML flow list ([a, b]) or comma separated values into
// its items, unquoted and without blanks.
func parseList(s string) []string {
	var items []string
	for _, item := range strings.Split(strings.Trim(strings.TrimSpace(s), "[]"), ",") {
		if item = strings.Trim(strings.TrimSpace(item), `"'`); item != "" {
			items = append(items, item)
		}
	}
	return items
}

// parseDate parses a YAML date or datetime, in local time unless it has an
// explicit offset.
func parseDate(s string) (time.Time, bool) {
	for _, layout := range []string{"2006-01-02", "2006-01-02 15:04", "2006-01-02 15:04:05", time.RFC3339} {
		if t, err := time.ParseInLocation(layout, s, time.Local); err == nil {
			return t, true
		}
	}
	return time.Time{}, false
}

// formatDate reformats a YAML date or datetime as yyyy/mm/dd, returning s
// unchanged if it can't be parsed.
func formatDate(s string) string {
	if t, ok := parseDate(s); ok {
		return t.Format("2006/01/02")
	}
	return s
}

// IsUnreleased reports whether Published is set and after now.
func (fm FrontMatter) IsUnreleased(now time.Time) bool {
	t, ok := parseDate(fm.Published)
	return ok && now.Before(t)
}

// ReleaseLine formats Published as yyyy/mm/dd hh:mm.
func (fm FrontMatter) ReleaseLine() string {
	if t, ok := parseDate(fm.Published); ok {
		return t.Format("2006/01/02 15:04")
	}
	return fm.Published
}

// DateLine describes when the page was published and last edited, including
// only the fields that are set.
func (fm FrontMatter) DateLine() string {
	var parts []string
	if fm.Published != "" {
		parts = append(parts, "Published on "+formatDate(fm.Published))
	}
	if fm.Edited != "" {
		parts = append(parts, "last edited "+formatDate(fm.Edited))
	}
	line := strings.Join(parts, ", ")
	if line != "" {
		line = strings.ToUpper(line[:1]) + line[1:]
	}
	return line
}
