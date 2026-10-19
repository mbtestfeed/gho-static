package dates

import (
	"testing"
	"time"
)

func TestParse(t *testing.T) {
	cases := []struct {
		name   string
		input  string
		want   time.Time
		wantOK bool
	}{
		{name: "date", input: "2026-01-31", want: time.Date(2026, 1, 31, 0, 0, 0, 0, time.Local), wantOK: true},
		{name: "date and time", input: "2026-01-31 14:05", want: time.Date(2026, 1, 31, 14, 5, 0, 0, time.Local), wantOK: true},
		{name: "date and time with seconds", input: "2026-01-31 14:05:09", want: time.Date(2026, 1, 31, 14, 5, 9, 0, time.Local), wantOK: true},
		{name: "RFC 3339 keeps its offset", input: "2026-01-31T14:05:09Z", want: time.Date(2026, 1, 31, 14, 5, 9, 0, time.UTC), wantOK: true},
		{name: "other format", input: "31/01/2026"},
		{name: "invalid date", input: "2026-02-30"},
		{name: "empty", input: ""},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, ok := Parse(c.input)
			if ok != c.wantOK || !got.Equal(c.want) {
				t.Errorf("Parse(%q) = (%v, %v), want (%v, %v)", c.input, got, ok, c.want, c.wantOK)
			}
		})
	}
}

func TestFormat(t *testing.T) {
	cases := []struct {
		name  string
		input string
		want  string
	}{
		{name: "date", input: "2026-01-31", want: "2026/01/31"},
		{name: "datetime drops the time", input: "2026-01-31 14:05", want: "2026/01/31"},
		{name: "unparseable is unchanged", input: "last tuesday", want: "last tuesday"},
		{name: "empty", input: "", want: ""},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := Format(c.input); got != c.want {
				t.Errorf("Format(%q) = %q, want %q", c.input, got, c.want)
			}
		})
	}
}

func TestPostFolder(t *testing.T) {
	date := time.Date(2026, 10, 19, 15, 30, 0, 0, time.Local)
	if got, want := PostFolder("my_post", date), "20261019_my_post"; got != want {
		t.Errorf("PostFolder(\"my_post\", %v) = %q, want %q", date, got, want)
	}
}

func TestPostName(t *testing.T) {
	cases := []struct {
		name   string
		folder string
		want   string
	}{
		{name: "dated folder", folder: "20261019_my_post", want: "my_post"},
		{name: "only a date", folder: "20261019", want: ""},
		{name: "undated folder", folder: "my_post", want: "my_post"},
		{name: "invalid date is kept", folder: "20261399_my_post", want: "20261399_my_post"},
		{name: "no underscore after the date", folder: "20261019my_post", want: "20261019my_post"},
		{name: "shorter than a date", folder: "2026", want: "2026"},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := PostName(c.folder); got != c.want {
				t.Errorf("PostName(%q) = %q, want %q", c.folder, got, c.want)
			}
		})
	}

	t.Run("undoes PostFolder", func(t *testing.T) {
		if got := PostName(PostFolder("my_post", time.Now())); got != "my_post" {
			t.Errorf("PostName(PostFolder(\"my_post\", now)) = %q, want \"my_post\"", got)
		}
	})
}
