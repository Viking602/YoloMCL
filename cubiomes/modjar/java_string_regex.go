package modjar

import (
	"strings"
	"unicode/utf8"
)

// javaLookbehindAny is the one lookbehind form accepted: "(?<=.)" at the start
// of a pattern, i.e. the match must not begin the input or follow a line
// terminator. RE2 has no lookbehind; the condition is checked per match.
const javaLookbehindAny = "(?<=.)"

// javaReplaceAll implements String.replaceAll(regex, replacement).
func javaReplaceAll(s, pattern, replacement string) string {
	template := javaReplacementTemplate(replacement)
	lookbehind := strings.HasPrefix(pattern, javaLookbehindAny)
	re := goRegexp(javaPattern{source: strings.TrimPrefix(pattern, javaLookbehindAny)})
	if !lookbehind {
		return re.ReplaceAllString(s, template)
	}
	if !contextFree(re) {
		fail("unbound regex syntax %q", pattern)
	}
	var out []byte
	last, pos := 0, 0
	for pos <= len(s) {
		loc := re.FindStringSubmatchIndex(s[pos:])
		if loc == nil {
			break
		}
		for i := range loc {
			if loc[i] >= 0 {
				loc[i] += pos
			}
		}
		start, end := loc[0], loc[1]
		if prev, _ := utf8.DecodeLastRuneInString(s[:start]); start == 0 || prev == '\n' || prev == '\r' || prev == '\u0085' || prev == '\u2028' || prev == '\u2029' {
			// Lookbehind fails here; retry one character later.
			_, size := utf8.DecodeRuneInString(s[start:])
			if size == 0 {
				break
			}
			pos = start + size
			continue
		}
		out = append(out, s[last:start]...)
		out = re.ExpandString(out, template, s, loc)
		last = end
		if end == start {
			_, size := utf8.DecodeRuneInString(s[end:])
			if size == 0 {
				break
			}
			pos = end + size
		} else {
			pos = end
		}
	}
	return string(append(out, s[last:]...))
}

// javaReplacementTemplate converts a Matcher replacement string ($n group
// references, backslash escapes) into a Go Expand template.
func javaReplacementTemplate(r string) string {
	var b strings.Builder
	for i := 0; i < len(r); i++ {
		switch c := r[i]; c {
		case '\\':
			i++
			if i >= len(r) {
				raiseJava("java/lang/IllegalArgumentException", "character to be escaped is missing")
			}
			if r[i] == '$' {
				b.WriteString("$$")
			} else {
				b.WriteByte(r[i])
			}
		case '$':
			i++
			if i >= len(r) {
				raiseJava("java/lang/IllegalArgumentException", "Illegal group reference: group index is missing")
			}
			if r[i] == '{' {
				j := strings.IndexByte(r[i:], '}')
				if j < 0 {
					raiseJava("java/lang/IllegalArgumentException", "named capturing group is missing trailing '}'")
				}
				b.WriteString("${" + r[i+1:i+j] + "}")
				i += j
				continue
			}
			j := i
			for j < len(r) && r[j] >= '0' && r[j] <= '9' {
				j++
			}
			if j == i {
				raiseJava("java/lang/IllegalArgumentException", "Illegal group reference")
			}
			b.WriteString("${" + r[i:j] + "}")
			i = j - 1
		default:
			b.WriteByte(c)
		}
	}
	return b.String()
}
