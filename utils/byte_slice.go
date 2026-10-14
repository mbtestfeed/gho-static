package utils

import (
	"bytes"
	"strings"
)

const indentUnit = "    "

// IndentFileContent prepends a newline to the start of b, then replaces every
// newline in the result with a newline followed by indents repetitions of
// the indent unit.
func IndentFileContent(b []byte, indents int) []byte {
	if len(b) == 0 {
		return b
	}
	withLeadingNewline := append([]byte("\n"), b...)
	indent := []byte("\n" + strings.Repeat(indentUnit, indents))
	result := bytes.ReplaceAll(withLeadingNewline, []byte("\n"), indent)

	if indents > 0 && b[len(b)-1] == '\n' {
		result = result[:len(result)-len(indentUnit)]
	}

	return result
}
