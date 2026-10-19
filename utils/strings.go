package utils

import (
	"strings"
	"unicode"
)

// TitleFor returns name with underscores as spaces and its first letter
// capitalised.
func TitleFor(name string) string {
	title := []rune(strings.ReplaceAll(name, "_", " "))
	if len(title) > 0 {
		title[0] = unicode.ToUpper(title[0])
	}
	return string(title)
}

// Truncate returns s cut to n characters, ending in "..." if anything was
// cut.
func Truncate(s string, n int) string {
	if r := []rune(s); len(r) > n {
		return string(r[:n-3]) + "..."
	}
	return s
}

// Slug returns name lowercased, with anything other than letters, digits,
// - and _ replaced by -, for use as a folder name, or fallback if that
// leaves nothing.
func Slug(name, fallback string) string {
	slug := strings.Trim(strings.Map(func(r rune) rune {
		if unicode.IsLetter(r) || unicode.IsDigit(r) || r == '-' || r == '_' {
			return unicode.ToLower(r)
		}
		return '-'
	}, name), "-")
	if slug == "" {
		return fallback
	}
	return slug
}
