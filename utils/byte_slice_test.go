package utils

import (
	"bytes"
	"testing"
)

func TestIndentFileContent(t *testing.T) {
	cases := []struct {
		name    string
		input   []byte
		indents int
		want    []byte
	}{
		{
			name:    "multiple lines, two indents",
			input:   []byte("line1\nline2\nline3"),
			indents: 2,
			want:    []byte("\n        line1\n        line2\n        line3"),
		},
		{
			name:    "no newlines, two indents",
			input:   []byte("single line"),
			indents: 2,
			want:    []byte("\n        single line"),
		},
		{
			name:    "empty input, two indents",
			input:   []byte(""),
			indents: 2,
			want:    []byte(""),
		},
		{
			name:    "one indent",
			input:   []byte("line1\nline2"),
			indents: 1,
			want:    []byte("\n    line1\n    line2"),
		},
		{
			name:    "zero indents",
			input:   []byte("line1\nline2"),
			indents: 0,
			want:    []byte("\nline1\nline2"),
		},
		{
			name:    "trailing newline reduces final indent by one",
			input:   []byte("line1\nline2\n"),
			indents: 2,
			want:    []byte("\n        line1\n        line2\n    "),
		},
		{
			name:    "trailing newline with zero indents is unchanged",
			input:   []byte("line1\n"),
			indents: 0,
			want:    []byte("\nline1\n"),
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := IndentFileContent(c.input, c.indents)
			if !bytes.Equal(got, c.want) {
				t.Errorf("IndentFileContent(%q, %d) = %q, want %q", c.input, c.indents, got, c.want)
			}
		})
	}
}
