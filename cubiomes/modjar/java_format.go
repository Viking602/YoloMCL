package modjar

import (
	"strconv"
	"strings"
)

// Locale-independent Formatter conversions. Unsupported flags/conversions
// cannot fall through to Go fmt, whose rounding and indexing semantics differ.
func (h *bindings) formatString(format string, args []any) string {
	var out strings.Builder
	next := 0
	for i := 0; i < len(format); i++ {
		if format[i] != '%' {
			out.WriteByte(format[i])
			continue
		}
		i++
		if i >= len(format) {
			fail("incomplete Java format conversion")
		}
		if format[i] == '%' {
			out.WriteByte('%')
			continue
		}
		index := next
		explicit := false
		start := i
		for i < len(format) && format[i] >= '0' && format[i] <= '9' {
			i++
		}
		if i > start {
			if i >= len(format) || format[i] != '$' {
				fail("unbound Java format width")
			}
			n, err := strconv.Atoi(format[start:i])
			if err != nil || n <= 0 {
				fail("invalid Java format index")
			}
			index = n - 1
			explicit = true
			i++
		}
		if i >= len(format) || index >= len(args) {
			fail("Java format argument missing")
		}
		if !explicit {
			next++
		}
		x := args[index]
		switch format[i] {
		case 's':
			out.WriteString(h.formatValue(x))
		case 'b':
			truth := x != nil
			if o, ok := x.(*object); ok && o.class == "java/lang/Boolean" {
				truth = integer(o.data) != 0
			}
			out.WriteString(strconv.FormatBool(truth))
		case 'd':
			if x == nil {
				out.WriteString("null")
				break
			}
			o, ok := x.(*object)
			if !ok || o.class != "java/lang/Byte" && o.class != "java/lang/Short" && o.class != "java/lang/Integer" && o.class != "java/lang/Long" {
				fail("unbound Java decimal format type")
			}
			switch n := o.data.(type) {
			case int32:
				out.WriteString(strconv.FormatInt(int64(n), 10))
			case int64:
				out.WriteString(strconv.FormatInt(n, 10))
			default:
				fail("unbound Java decimal format type")
			}
		default:
			fail("unbound Java format conversion %c", format[i])
		}
		if out.Len() > 65536 {
			fail("Java formatted string budget")
		}
	}
	if out.Len() > 65536 {
		fail("Java formatted string budget")
	}
	return out.String()
}
func (h *bindings) formatValue(x any) string {
	if x == nil {
		return "null"
	}
	if s, ok := x.(string); ok {
		return s
	}
	if o, ok := x.(*object); ok {
		switch o.class {
		case "java/lang/Integer", "java/lang/Byte", "java/lang/Short":
			return concatValue("I", o.data)
		case "java/lang/Long":
			return concatValue("J", o.data)
		case "java/lang/Boolean":
			return concatValue("Z", o.data)
		case "java/lang/Character":
			return concatValue("C", o.data)
		}
		return javaString(h.vm.method(reference{o.class, "toString", "()Ljava/lang/String;"}, o, true, nil))
	}
	fail("unbound formatted value %T", x)
	return ""
}
