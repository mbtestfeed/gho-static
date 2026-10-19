package front_matter

import "testing"

func TestSplit(t *testing.T) {
	cases := []struct {
		name       string
		md         string
		wantHeader string
		wantRest   string
		wantOK     bool
	}{
		{
			name:       "front matter and body",
			md:         "---\ntitle: About\n---\n\n# About\n",
			wantHeader: "title: About",
			wantRest:   "---\n\n# About\n",
			wantOK:     true,
		},
		{
			name:       "several lines",
			md:         "---\ntitle: About\ndraft: true\n---\nbody",
			wantHeader: "title: About\ndraft: true",
			wantRest:   "---\nbody",
			wantOK:     true,
		},
		{
			name:       "windows line endings become \\n",
			md:         "---\r\ntitle: About\r\n---\r\nbody\r\n",
			wantHeader: "title: About",
			wantRest:   "---\nbody\n",
			wantOK:     true,
		},
		{
			name:       "only the first closing delimiter ends it",
			md:         "---\ntitle: About\n---\ntext\n---\nmore",
			wantHeader: "title: About",
			wantRest:   "---\ntext\n---\nmore",
			wantOK:     true,
		},
		{
			name: "no front matter",
			md:   "# About\n",
		},
		{
			name: "unclosed front matter",
			md:   "---\ntitle: About\n",
		},
		{
			name: "empty",
			md:   "",
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			header, rest, ok := Split(c.md)
			if header != c.wantHeader || rest != c.wantRest || ok != c.wantOK {
				t.Errorf("Split(%q) = (%q, %q, %v), want (%q, %q, %v)", c.md, header, rest, ok, c.wantHeader, c.wantRest, c.wantOK)
			}
		})
	}
}
