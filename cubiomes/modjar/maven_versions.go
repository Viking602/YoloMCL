// Version-ordering algorithm adapted from Apache Maven ComparableVersion 3.8.5.
// Copyright 2001-2022 The Apache Software Foundation. Licensed under Apache-2.0;
// see testdata/LICENSE.Maven.txt and testdata/NOTICE.Maven.txt.
// This Go implementation uses bounded input strings and native value lists.

package modjar

import (
	"cmp"
	"fmt"
	"slices"
	"strings"
)

// Maven's ordering includes arbitrary-length numbers, qualifier aliases, and
// nested hyphen/number transitions; semantic-version ordering is not equivalent.
// Algorithm follows Apache Maven ComparableVersion 3.8.5 (Apache-2.0).
type mavenItem struct {
	kind  byte
	value string
	items []*mavenItem
}

func mavenVersion(s string) *mavenItem {
	s = strings.ToLower(s)
	root := &mavenItem{kind: 'l'}
	list := root
	stack := []*mavenItem{root}
	start := 0
	digit := false
	add := func(value string, numeric, followed bool) {
		x := &mavenItem{kind: 's', value: value}
		if numeric {
			x.kind = 'n'
			x.value = strings.TrimLeft(value, "0")
		} else {
			if followed {
				switch value {
				case "a":
					x.value = "alpha"
				case "b":
					x.value = "beta"
				case "m":
					x.value = "milestone"
				}
			}
			switch x.value {
			case "ga", "final", "release":
				x.value = ""
			case "cr":
				x.value = "rc"
			}
		}
		list.items = append(list.items, x)
	}
	nest := func() {
		next := &mavenItem{kind: 'l'}
		list.items = append(list.items, next)
		list = next
		stack = append(stack, next)
	}
	for i := 0; i < len(s); i++ {
		ch := s[i]
		switch {
		case ch == '.' || ch == '-':
			if i == start {
				add("0", true, false)
			} else {
				add(s[start:i], digit, false)
			}
			start = i + 1
			if ch == '-' {
				nest()
			}
		case ch >= '0' && ch <= '9':
			if !digit && i > start {
				add(s[start:i], false, true)
				start = i
				nest()
			}
			digit = true
		default:
			if digit && i > start {
				add(s[start:i], true, false)
				start = i
				nest()
			}
			digit = false
		}
	}
	if start < len(s) {
		add(s[start:], digit, false)
	}
	for i := len(stack) - 1; i >= 0; i-- {
		x := stack[i]
		for j := len(x.items) - 1; j >= 0; j-- {
			item := x.items[j]
			zero := item.kind == 'n' && item.value == "" || item.kind == 's' && item.value == "" || item.kind == 'l' && len(item.items) == 0
			if zero {
				x.items = slices.Delete(x.items, j, j+1)
			} else if item.kind != 'l' {
				break
			}
		}
	}
	return root
}
func mavenQualifier(s string) string {
	for i, q := range []string{"alpha", "beta", "milestone", "rc", "snapshot", "", "sp"} {
		if q == s {
			return string(rune('0' + i))
		}
	}
	return "7-" + s
}
func compareMaven(a, b *mavenItem) int {
	if a == nil {
		if b == nil {
			return 0
		}
		return -compareMaven(b, nil)
	}
	if b == nil {
		switch a.kind {
		case 'n':
			return cmp.Compare(len(a.value), 0)
		case 's':
			return strings.Compare(mavenQualifier(a.value), "5")
		case 'l':
			for _, x := range a.items {
				if c := compareMaven(x, nil); c != 0 {
					return c
				}
			}
		}
		return 0
	}
	if a.kind != b.kind {
		rank := func(k byte) int {
			switch k {
			case 's':
				return 0
			case 'l':
				return 1
			default:
				return 2
			}
		}
		return cmp.Compare(rank(a.kind), rank(b.kind))
	}
	switch a.kind {
	case 'n':
		if c := cmp.Compare(len(a.value), len(b.value)); c != 0 {
			return c
		}
		return strings.Compare(a.value, b.value)
	case 's':
		return strings.Compare(mavenQualifier(a.value), mavenQualifier(b.value))
	default:
		for i := 0; i < max(len(a.items), len(b.items)); i++ {
			var x, y *mavenItem
			if i < len(a.items) {
				x = a.items[i]
			}
			if i < len(b.items) {
				y = b.items[i]
			}
			if c := compareMaven(x, y); c != 0 {
				return c
			}
		}
		return 0
	}
}

type mavenInterval struct {
	low, high               *mavenItem
	includeLow, includeHigh bool
}
type mavenRange struct {
	intervals   []mavenInterval
	recommended *mavenItem
}

func parseMavenRange(s string) (mavenRange, error) {
	var out mavenRange
	if len(s) > 1024 {
		return out, fmt.Errorf("version range length")
	}
	s = strings.TrimSpace(s)
	if s == "" {
		return out, fmt.Errorf("empty version range")
	}
	for _, r := range s {
		if r > 127 {
			return out, fmt.Errorf("non-ASCII Maven version")
		}
	}
	if s[0] != '[' && s[0] != '(' {
		out.recommended = mavenVersion(s)
		return out, nil
	}
	for s != "" {
		end := strings.IndexAny(s, "])")
		if end < 0 {
			return out, fmt.Errorf("invalid Maven range")
		}
		text := strings.TrimSpace(s[1:end])
		low, high, paired := strings.Cut(text, ",")
		x := mavenInterval{includeLow: s[0] == '[', includeHigh: s[end] == ']'}
		if !paired {
			if !x.includeLow || !x.includeHigh || text == "" {
				return out, fmt.Errorf("invalid exact Maven range")
			}
			x.low = mavenVersion(text)
			x.high = x.low
		} else {
			low, high = strings.TrimSpace(low), strings.TrimSpace(high)
			if strings.Contains(high, ",") {
				return out, fmt.Errorf("invalid Maven bounds")
			}
			if low != "" {
				x.low = mavenVersion(low)
			}
			if high != "" {
				x.high = mavenVersion(high)
			}
			if x.low != nil && x.high != nil {
				c := compareMaven(x.low, x.high)
				if c > 0 || c == 0 && (!x.includeLow || !x.includeHigh) {
					return out, fmt.Errorf("empty Maven range")
				}
			}
		}
		out.intervals = append(out.intervals, x)
		s = strings.TrimSpace(s[end+1:])
		s = strings.TrimSpace(strings.TrimPrefix(s, ","))
		if s != "" && s[0] != '[' && s[0] != '(' {
			return out, fmt.Errorf("invalid Maven range union")
		}
	}
	return out, nil
}
func (r mavenRange) contains(v *mavenItem) bool {
	if r.recommended != nil {
		return true
	}
	for _, x := range r.intervals {
		low, high := true, true
		if x.low != nil {
			c := compareMaven(v, x.low)
			low = c > 0 || c == 0 && x.includeLow
		}
		if x.high != nil {
			c := compareMaven(v, x.high)
			high = c < 0 || c == 0 && x.includeHigh
		}
		if low && high {
			return true
		}
	}
	return false
}
