package modjar

import (
	"strconv"
	"strings"
)

func javaConcat(c *class, boot bootstrap, desc string, args []any, constants bool) string {
	types, _ := descriptor(desc)
	if len(types) != len(args) {
		fail("string concat argument mismatch")
	}
	values := make([]string, len(args))
	for i, arg := range args {
		values[i] = concatValue(types[i], arg)
	}
	recipe := strings.Repeat("\x01", len(args))
	start := 0
	if constants {
		if len(boot.args) == 0 {
			fail("string concat recipe missing")
		}
		recipe = javaString(c.literal(boot.args[0]))
		start = 1
	}
	var b strings.Builder
	n := 0
	for _, ch := range recipe {
		switch ch {
		case 1:
			if n >= len(values) {
				fail("string concat recipe arity")
			}
			b.WriteString(values[n])
			n++
		case 2:
			if start >= len(boot.args) {
				fail("string concat constant arity")
			}
			value := c.literal(boot.args[start])
			start++
			switch x := value.(type) {
			case string:
				b.WriteString(x)
			case int32:
				b.WriteString(strconv.FormatInt(int64(x), 10))
			case int64:
				b.WriteString(strconv.FormatInt(x, 10))
			default:
				fail("unbound string concat constant")
			}
		default:
			b.WriteRune(ch)
		}
		if b.Len() > 65536 {
			fail("string concat budget")
		}
	}
	if n != len(values) || start != len(boot.args) {
		fail("string concat recipe arity")
	}
	return b.String()
}

func concatValue(descriptor string, value any) string {
	switch descriptor {
	case "Z":
		return strconv.FormatBool(integer(value) != 0)
	case "C":
		n := integer(value)
		if n >= 0xd800 && n <= 0xdfff {
			fail("unpaired UTF-16 character in concat")
		}
		return string(rune(uint16(n)))
	case "B", "S", "I":
		return strconv.FormatInt(int64(integer(value)), 10)
	case "J":
		return strconv.FormatInt(value.(int64), 10)
	case "F", "D":
		fail("Java floating-point string formatting is not bound")
	default:
		if value == nil {
			return "null"
		}
		return javaString(value)
	}
	return ""
}
