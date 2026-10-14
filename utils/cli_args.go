package utils

import "strings"

type Arg struct {
	Key string
	Val string
}

// ParseArgs splits argv into flags (- and -- prefixed) and positional args.
//
// Single-dash args expand into one Arg per character, each with an empty
// Val (e.g. "-asd" -> a, s, d).
//
// Double-dash args become Key/Val pairs. If "--key=val" is given, Val is
// taken directly from after the "=". Otherwise the following arg decides
// Val: if it starts with "-", Val stays empty; if it starts with a quote
// character, Val is everything up to the next unescaped matching quote
// (which may span multiple argv entries, rejoined with spaces); otherwise
// the following arg is consumed whole as Val.
func ParseArgs(argv []string) (flags []Arg, positional []string) {
	for i := 0; i < len(argv); i++ {
		arg := argv[i]
		switch {
		case strings.HasPrefix(arg, "--"):
			key := strings.TrimPrefix(arg, "--")
			val := ""
			hasEquals := false
			if idx := strings.Index(key, "="); idx != -1 {
				val = key[idx+1:]
				key = key[:idx]
				hasEquals = true
			}
			leftover := ""
			if !hasEquals && i+1 < len(argv) {
				next := argv[i+1]
				switch {
				case strings.HasPrefix(next, "-"):
				case len(next) > 0 && (next[0] == '\'' || next[0] == '"'):
					quoted, consumed, rest := consumeQuoted(argv, i+1, next[0])
					val = quoted
					i += consumed
					leftover = rest
				default:
					val = next
					i++
				}
			}
			flags = append(flags, Arg{Key: key, Val: val})
			if leftover != "" {
				positional = append(positional, leftover)
			}
		case strings.HasPrefix(arg, "-"):
			for _, c := range strings.TrimPrefix(arg, "-") {
				flags = append(flags, Arg{Key: string(c), Val: ""})
			}
		default:
			positional = append(positional, arg)
		}
	}
	return flags, positional
}

// consumeQuoted reads a quoted value starting at argv[start] (which begins
// with the opening quote char), spanning as many subsequent argv entries as
// needed until an unescaped matching quote is found. It returns the
// unquoted value, how many argv entries (starting at start) were consumed,
// and any trailing text that followed the closing quote within that last
// entry (which the caller should treat as a separate positional arg).
func consumeQuoted(argv []string, start int, quote byte) (value string, consumed int, leftover string) {
	var sb strings.Builder
	for i := start; i < len(argv); i++ {
		tok := argv[i]
		if i == start {
			tok = tok[1:]
		} else {
			sb.WriteByte(' ')
		}
		j := 0
		for j < len(tok) {
			if tok[j] == '\\' && j+1 < len(tok) && tok[j+1] == quote {
				sb.WriteByte(quote)
				j += 2
				continue
			}
			if tok[j] == quote {
				return sb.String(), i - start + 1, tok[j+1:]
			}
			sb.WriteByte(tok[j])
			j++
		}
	}
	return sb.String(), len(argv) - start, ""
}
