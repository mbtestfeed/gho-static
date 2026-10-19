package front_matter

import "strings"

// Split splits the leading "---" delimited front matter block off md, with
// its line endings as \n. It returns the lines between the delimiters, and
// everything from the closing delimiter on. ok is false if md has no front
// matter.
func Split(md string) (header, rest string, ok bool) {
	md = strings.ReplaceAll(md, "\r\n", "\n")
	if !strings.HasPrefix(md, "---\n") {
		return "", "", false
	}
	end := strings.Index(md[4:], "\n---")
	if end == -1 {
		return "", "", false
	}
	return md[4 : 4+end], md[4+end+1:], true
}
