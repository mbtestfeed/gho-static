package utils

import "testing"

func equalArgs(a, b []Arg) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func equalStrings(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func TestParseArgs(t *testing.T) {
	cases := []struct {
		name       string
		argv       []string
		wantFlags  []Arg
		wantPos    []string
	}{
		{
			name:      "single dash combined flags",
			argv:      []string{"-asd"},
			wantFlags: []Arg{{"a", ""}, {"s", ""}, {"d", ""}},
		},
		{
			name:      "double dash equals",
			argv:      []string{"--port=1234"},
			wantFlags: []Arg{{"port", "1234"}},
		},
		{
			name:      "double dash space separated value",
			argv:      []string{"--port", "1234"},
			wantFlags: []Arg{{"port", "1234"}},
		},
		{
			name:      "double dash followed by another flag has empty val",
			argv:      []string{"--verbose", "-x"},
			wantFlags: []Arg{{"verbose", ""}, {"x", ""}},
		},
		{
			name:      "double dash at end of args has empty val",
			argv:      []string{"--verbose"},
			wantFlags: []Arg{{"verbose", ""}},
		},
		{
			name:    "positional args",
			argv:    []string{"src", "dest"},
			wantPos: []string{"src", "dest"},
		},
		{
			name:      "mixed flags and positional",
			argv:      []string{"--port=1234", "src", "-v", "dest"},
			wantFlags: []Arg{{"port", "1234"}, {"v", ""}},
			wantPos:   []string{"src", "dest"},
		},
		{
			name:      "quoted value single token",
			argv:      []string{"--name", "'hello'"},
			wantFlags: []Arg{{"name", "hello"}},
		},
		{
			name:      "quoted value spanning multiple tokens",
			argv:      []string{"--name", "'hello", "world'"},
			wantFlags: []Arg{{"name", "hello world"}},
		},
		{
			name:      "escaped quote inside quoted value",
			argv:      []string{"--name", `'it\'s'`},
			wantFlags: []Arg{{"name", "it's"}},
		},
		{
			name:      "equals form keeps embedded quotes literal",
			argv:      []string{`--port="12"34`},
			wantFlags: []Arg{{"port", `"12"34`}},
		},
		{
			name:      "space form splits leftover after closing quote into positional",
			argv:      []string{"--port", `"12"34`},
			wantFlags: []Arg{{"port", "12"}},
			wantPos:   []string{"34"},
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			flags, positional := ParseArgs(c.argv)
			if !equalArgs(flags, c.wantFlags) {
				t.Errorf("flags = %#v, want %#v", flags, c.wantFlags)
			}
			if !equalStrings(positional, c.wantPos) {
				t.Errorf("positional = %#v, want %#v", positional, c.wantPos)
			}
		})
	}
}
