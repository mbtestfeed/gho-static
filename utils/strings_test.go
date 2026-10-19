package utils

import "testing"

func TestTitleFor(t *testing.T) {
	cases := []struct {
		name  string
		input string
		want  string
	}{
		{name: "underscores become spaces", input: "my_first_post", want: "My first post"},
		{name: "already capitalised", input: "About", want: "About"},
		{name: "non-ascii first letter", input: "élan_vital", want: "Élan vital"},
		{name: "empty", input: "", want: ""},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := TitleFor(c.input); got != c.want {
				t.Errorf("TitleFor(%q) = %q, want %q", c.input, got, c.want)
			}
		})
	}
}

func TestTruncate(t *testing.T) {
	cases := []struct {
		name  string
		input string
		n     int
		want  string
	}{
		{name: "shorter than n is unchanged", input: "short", n: 10, want: "short"},
		{name: "exactly n is unchanged", input: "abcdefgh", n: 8, want: "abcdefgh"},
		{name: "longer than n is cut with an ellipsis", input: "abcdefghij", n: 8, want: "abcde..."},
		{name: "counts characters, not bytes", input: "ééééé", n: 4, want: "é..."},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := Truncate(c.input, c.n); got != c.want {
				t.Errorf("Truncate(%q, %d) = %q, want %q", c.input, c.n, got, c.want)
			}
		})
	}
}

func TestSlug(t *testing.T) {
	cases := []struct {
		name  string
		input string
		want  string
	}{
		{name: "lowercased, spaces as dashes", input: "Example Tag", want: "example-tag"},
		{name: "punctuation as dashes, trimmed at the ends", input: "Hello, World!", want: "hello--world"},
		{name: "keeps underscores and dashes", input: "under_score-dash", want: "under_score-dash"},
		{name: "keeps non-ascii letters", input: "Ünïcode", want: "ünïcode"},
		{name: "nothing left gives the fallback", input: "!!!", want: "tag"},
		{name: "empty gives the fallback", input: "", want: "tag"},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := Slug(c.input, "tag"); got != c.want {
				t.Errorf("Slug(%q, \"tag\") = %q, want %q", c.input, got, c.want)
			}
		})
	}
}
