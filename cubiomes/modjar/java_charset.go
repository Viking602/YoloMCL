package modjar

import "unicode/utf16"

// encodeCharset encodes a Java string with one of the standard charsets.
// Unmappable characters become '?', matching String.getBytes.
func encodeCharset(s, name string) ([]byte, bool) {
	switch name {
	case "UTF-8":
		return []byte(s), true
	case "US-ASCII", "ISO-8859-1":
		limit := rune(0x7f)
		if name == "ISO-8859-1" {
			limit = 0xff
		}
		var out []byte
		for _, r := range s {
			if r > limit {
				r = '?'
			}
			out = append(out, byte(r))
		}
		return out, true
	case "UTF-16", "UTF-16BE", "UTF-16LE":
		units := utf16.Encode([]rune(s))
		var out []byte
		if name == "UTF-16" {
			out = append(out, 0xfe, 0xff) // Java writes a big-endian BOM
		}
		for _, u := range units {
			if name == "UTF-16LE" {
				out = append(out, byte(u), byte(u>>8))
			} else {
				out = append(out, byte(u>>8), byte(u))
			}
		}
		return out, true
	}
	return nil, false
}

func (h *bindings) charsetCall(r reference, receiver any, args []any) (any, bool) {
	if r.owner != "java/lang/String" || r.name != "getBytes" {
		return nil, false
	}
	var name string
	switch r.desc {
	case "(Ljava/nio/charset/Charset;)[B":
		if args[0] == nil {
			raiseJava("java/lang/NullPointerException", "null charset")
		}
		name, _ = objectData(args[0]).(string)
	case "(Ljava/lang/String;)[B":
		if args[0] == nil {
			raiseJava("java/lang/NullPointerException", "null charset name")
		}
		var ok bool
		if name, ok = standardCharset(javaString(args[0])); !ok {
			raiseJava("java/io/UnsupportedEncodingException", javaString(args[0]))
		}
	default:
		return nil, false // the platform default charset is not fixed here
	}
	if receiver == nil {
		raiseJava("java/lang/NullPointerException", "null string")
	}
	b, ok := encodeCharset(javaString(receiver), name)
	if !ok {
		fail("unbound string charset %s", name)
	}
	if len(b) > 16<<20 {
		fail("string byte budget")
	}
	h.vm.alloc("[B", nil) // shared object budget
	return javaByteArray(b), true
}
