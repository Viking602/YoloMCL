package modjar

import (
	"regexp"
	"regexp/syntax"
	"unicode/utf8"
)

// goRegexp accepts a Java pattern only when RE2 parses it under the flags it
// can express; Java-only syntax (lookaround, backreferences, possessive
// quantifiers) fails to compile and stays unbound rather than misbehaving.
func goRegexp(p javaPattern) *regexp.Regexp {
	prefix := ""
	for bit, flag := range map[int32]string{2: "i", 8: "m", 32: "s"} {
		if p.flags&bit != 0 {
			prefix += flag
		}
	}
	if p.flags&^(2|8|32) != 0 {
		fail("unbound regex flags %d", p.flags)
	}
	source := p.source
	if prefix != "" {
		source = "(?" + prefix + ")" + source
	}
	re, err := regexp.Compile(source)
	if err != nil {
		fail("unbound regex syntax %q", p.source)
	}
	return re
}

func (h *bindings) regexStreamCall(r reference, receiver any, args []any) (any, bool) {
	v := h.vm
	switch {
	case r.owner == "java/lang/String" && r.name == "replaceAll" && r.desc == "(Ljava/lang/String;Ljava/lang/String;)Ljava/lang/String;":
		s, ok := receiver.(string)
		if !ok {
			return nil, false
		}
		if args[0] == nil || args[1] == nil {
			raiseJava("java/lang/NullPointerException", "replaceAll")
		}
		return javaReplaceAll(s, javaString(args[0]), javaString(args[1])), true
	case r.owner == "java/lang/String" && r.name == "matches" && r.desc == "(Ljava/lang/String;)Z":
		s, ok := receiver.(string)
		if !ok {
			return nil, false
		}
		if args[0] == nil {
			raiseJava("java/lang/NullPointerException", "matches")
		}
		re := goRegexp(javaPattern{source: javaString(args[0])})
		if regexp.MustCompile(`^(?:` + re.String() + `)$`).MatchString(s) {
			return int32(1), true
		}
		return int32(0), true
	case r.owner == "java/util/regex/Pattern" && receiver != nil:
		p, ok := objectData(receiver).(javaPattern)
		if !ok {
			return nil, false
		}
		switch r.name + r.desc {
		case "asPredicate()Ljava/util/function/Predicate;":
			v.alloc("java/lang/invoke/Lambda", nil)
			return &lambda{target: reference{r.owner, "$find", "(Ljava/lang/Object;)Z"}, kind: 5, captured: []any{receiver}, sam: "test", iface: "java/util/function/Predicate", instantiated: "(Ljava/lang/Object;)Z"}, true
		case "$find(Ljava/lang/Object;)Z":
			if args[0] == nil {
				raiseJava("java/lang/NullPointerException", "")
			}
			if goRegexp(p).MatchString(javaString(args[0])) {
				return int32(1), true
			}
			return int32(0), true
		}
	case r.owner == "java/util/regex/Matcher" && receiver != nil && r.name+r.desc != "matches()Z":
		m, ok := objectData(receiver).(*javaMatcher)
		if !ok {
			return nil, false
		}
		return m.call(r, args)
	case r.owner == "java/util/regex/Matcher" && receiver != nil && r.name+r.desc == "matches()Z":
		m, ok := objectData(receiver).(*javaMatcher)
		if !ok {
			return nil, false
		}
		re := goRegexp(m.pattern)
		// An anchored whole-input match also defines groups for group()/start().
		anchored := regexp.MustCompile(`^(?:` + re.String() + `)$`)
		m.match = anchored.FindStringSubmatchIndex(m.input)
		m.next = len(m.input) + 1
		if m.match != nil {
			m.next = m.match[1]
			return int32(1), true
		}
		return int32(0), true
	case r.owner == "java/util/stream/StreamSupport" && r.name == "stream" && r.desc == "(Ljava/util/Spliterator;Z)Ljava/util/stream/Stream;":
		// Streams are eager lists here: drain the spliterator through a
		// Consumer bound to an ArrayList, exactly as forEachRemaining would.
		if args[0] == nil {
			raiseJava("java/lang/NullPointerException", "")
		}
		sink := v.alloc("java/util/ArrayList", nil)
		v.method(reference{"java/util/ArrayList", "<init>", "()V"}, sink, false, nil)
		v.alloc("java/lang/invoke/Lambda", nil)
		consumer := &lambda{target: reference{"java/util/ArrayList", "add", "(Ljava/lang/Object;)Z"}, kind: 5, captured: []any{sink}, sam: "accept", iface: "java/util/function/Consumer", instantiated: "(Ljava/lang/Object;)V"}
		for i := 0; ; i++ {
			if i >= 4096 {
				fail("spliterator budget exceeded")
			}
			more := v.method(reference{"java/util/Spliterator", "tryAdvance", "(Ljava/util/function/Consumer;)Z"}, args[0], true, []any{consumer})
			if integer(more) == 0 {
				break
			}
		}
		l, ok := objectData(sink).(*javaList)
		if !ok {
			fail("stream sink is not a list")
		}
		return v.alloc("java/util/stream/Stream", &javaList{values: append([]any(nil), l.values...), readonly: true}), true
	}
	return nil, false
}

// contextFree reports whether a pattern has no assertions that look at text
// before the match (^, \A, \b, \B), so searching a suffix is equivalent to
// searching the whole input from an offset.
func contextFree(re *regexp.Regexp) bool {
	parsed, err := syntax.Parse(re.String(), syntax.Perl)
	if err != nil {
		return false
	}
	var walk func(*syntax.Regexp) bool
	walk = func(x *syntax.Regexp) bool {
		switch x.Op {
		case syntax.OpBeginLine, syntax.OpBeginText, syntax.OpWordBoundary, syntax.OpNoWordBoundary:
			return false
		}
		for _, sub := range x.Sub {
			if !walk(sub) {
				return false
			}
		}
		return true
	}
	return walk(parsed)
}

// charIndex converts a byte offset to this VM's code-point string index.
func charIndex(s string, b int) int32 { return int32(utf8.RuneCountInString(s[:b])) }

// byteIndex converts a code-point index to a byte offset, or -1 if out of range.
func byteIndex(s string, c int) int {
	if c < 0 {
		return -1
	}
	b := 0
	for i := 0; i < c; i++ {
		if b >= len(s) {
			return -1
		}
		_, n := utf8.DecodeRuneInString(s[b:])
		b += n
	}
	return b
}

func (m *javaMatcher) find(from int) bool {
	re := goRegexp(m.pattern)
	if from > 0 && !contextFree(re) {
		fail("unbound Matcher.find from offset with context-sensitive pattern %q", m.pattern.source)
	}
	if from > len(m.input) {
		m.match = nil
		return false
	}
	loc := re.FindStringSubmatchIndex(m.input[from:])
	if loc == nil {
		m.match = nil
		m.next = len(m.input) + 1
		return false
	}
	for i := range loc {
		if loc[i] >= 0 {
			loc[i] += from
		}
	}
	m.match = loc
	m.next = loc[1]
	if loc[1] == loc[0] {
		// Java advances past an empty match before the next find.
		if loc[1] < len(m.input) {
			_, n := utf8.DecodeRuneInString(m.input[loc[1]:])
			m.next = loc[1] + n
		} else {
			m.next = len(m.input) + 1
		}
	}
	return true
}

func (m *javaMatcher) group(i int) any {
	if m.match == nil {
		raiseJava("java/lang/IllegalStateException", "No match found")
	}
	if i < 0 || 2*i+1 >= len(m.match) {
		raiseJava("java/lang/IndexOutOfBoundsException", "No group")
	}
	if m.match[2*i] < 0 {
		return nil
	}
	return m.input[m.match[2*i]:m.match[2*i+1]]
}

func (m *javaMatcher) call(r reference, args []any) (any, bool) {
	boolean := func(b bool) any {
		if b {
			return int32(1)
		}
		return int32(0)
	}
	switch r.name + r.desc {
	case "find()Z":
		return boolean(m.find(m.next)), true
	case "find(I)Z":
		b := byteIndex(m.input, int(integer(args[0])))
		if b < 0 {
			raiseJava("java/lang/IndexOutOfBoundsException", "Illegal start index")
		}
		return boolean(m.find(b)), true
	case "group()Ljava/lang/String;":
		return m.group(0), true
	case "group(I)Ljava/lang/String;":
		return m.group(int(integer(args[0]))), true
	case "groupCount()I":
		return int32(goRegexp(m.pattern).NumSubexp()), true
	case "start()I", "end()I":
		if m.match == nil {
			raiseJava("java/lang/IllegalStateException", "No match available")
		}
		if r.name == "start" {
			return charIndex(m.input, m.match[0]), true
		}
		return charIndex(m.input, m.match[1]), true
	}
	return nil, false
}
