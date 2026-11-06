package modjar

import (
	"maps"
	"path"
	"slices"
	"strings"
	"unicode/utf16"
	"unicode/utf8"
)

// javaFile is a game-directory path in the emulated server filesystem (see
// java_paths.go): existence and reads reflect the instance's real config files
// (read-only), and anything a mod writes lands in the in-memory overlay.
type javaFile struct {
	path string
}

// stringWriter backs java.io.StringWriter as UTF-16 code units.
type stringWriter struct {
	units []uint16
}

type javaBytes struct {
	data []byte
	pos  int
	low  int32 // pending UTF-16 low surrogate of a decoded supplementary char
}

func (h *bindings) ioCall(r reference, receiver any, args []any) (any, bool) {
	v := h.vm
	if x, ok := h.pathCall(r, receiver, args); ok {
		return x, true
	}
	if (r.owner == "java/io/FileWriter" || r.owner == "java/io/FileOutputStream") && r.name == "<init>" && (r.desc == "(Ljava/io/File;)V" || r.desc == "(Ljava/lang/String;)V") {
		if args[0] == nil {
			raiseJava("java/lang/NullPointerException", "file")
		}
		name := ""
		if f, ok := objectData(args[0]).(*javaFile); ok {
			name = f.path
		} else {
			name = javaString(args[0])
		}
		p := cleanGamePath(name)
		if p.foreign || !h.isGameDir(javaPath{p: parentPath(p.p)}) {
			raiseJava("java/io/FileNotFoundException", name)
		}
		h.writeGameFile(p, nil)
		receiver.(*object).data = &javaOutput{path: p.p, open: true}
		return nil, true
	}
	if r.owner == "org/apache/commons/io/FileUtils" && receiver == nil {
		switch {
		case r.name == "readFileToString" && (r.desc == "(Ljava/io/File;)Ljava/lang/String;" || r.desc == "(Ljava/io/File;Ljava/nio/charset/Charset;)Ljava/lang/String;" || r.desc == "(Ljava/io/File;Ljava/lang/String;)Ljava/lang/String;"):
			if args[0] == nil {
				raiseJava("java/lang/NullPointerException", "file")
			}
			f := objectData(args[0]).(*javaFile)
			data, ok := h.readGameFile(cleanGamePath(f.path))
			if !ok {
				raiseJava("java/io/FileNotFoundException", f.path)
			}
			return string(data), true
		case r.name == "write" && (r.desc == "(Ljava/io/File;Ljava/lang/CharSequence;)V" || r.desc == "(Ljava/io/File;Ljava/lang/CharSequence;Ljava/nio/charset/Charset;)V" || r.desc == "(Ljava/io/File;Ljava/lang/CharSequence;Ljava/lang/String;)V"):
			if args[0] == nil || args[1] == nil {
				raiseJava("java/lang/NullPointerException", "write")
			}
			f := objectData(args[0]).(*javaFile)
			gp := cleanGamePath(f.path)
			if gp.foreign {
				raiseJava("java/nio/file/AccessDeniedException", f.path)
			}
			h.writeGameFile(gp, []byte(javaString(args[1])))
			return nil, true
		case r.name == "readFileToByteArray" && r.desc == "(Ljava/io/File;)[B":
			if args[0] == nil {
				raiseJava("java/lang/NullPointerException", "file")
			}
			f := objectData(args[0]).(*javaFile)
			data, ok := h.readGameFile(cleanGamePath(f.path))
			if !ok {
				raiseJava("java/io/FileNotFoundException", f.path)
			}
			vals := make([]any, len(data))
			for i, b := range data {
				vals[i] = int32(int8(b))
			}
			return &array{values: vals, descriptor: "[B"}, true
		}
	}
	if r.owner == "java/io/File" {
		switch {
		case r.name == "<init>" && r.desc == "(Ljava/lang/String;)V":
			if args[0] == nil {
				raiseJava("java/lang/NullPointerException", "File path")
			}
			receiver.(*object).data = &javaFile{path: javaString(args[0])}
			return nil, true
		case r.name == "<init>" && r.desc == "(Ljava/io/File;Ljava/lang/String;)V":
			if args[1] == nil {
				raiseJava("java/lang/NullPointerException", "File child")
			}
			parent := ""
			if args[0] != nil {
				parent = objectData(args[0]).(*javaFile).path
			}
			child := javaString(args[1])
			joined := child
			if parent != "" {
				joined = strings.TrimRight(parent, "/\\") + "/" + child
			}
			receiver.(*object).data = &javaFile{path: joined}
			return nil, true
		case receiver != nil:
			f, ok := objectData(receiver).(*javaFile)
			if !ok {
				return nil, false
			}
			switch {
			case r.name == "exists" && r.desc == "()Z", r.name == "isFile" && r.desc == "()Z", r.name == "isDirectory" && r.desc == "()Z", r.name == "canRead" && r.desc == "()Z":
				gp := cleanGamePath(f.path)
				_, file := h.readGameFile(gp)
				dir := h.isGameDir(gp)
				hit := r.name == "exists" && (file || dir) || (r.name == "isFile" || r.name == "canRead") && file || r.name == "isDirectory" && dir
				if hit {
					return int32(1), true
				}
				return int32(0), true
			case r.name == "length" && r.desc == "()J":
				data, _ := h.readGameFile(cleanGamePath(f.path))
				return int64(len(data)), true
			case r.name == "mkdir" && r.desc == "()Z" || r.name == "mkdirs" && r.desc == "()Z":
				gp := cleanGamePath(f.path)
				if gp.foreign || h.isGameDir(gp) {
					return int32(0), true
				}
				o := h.overlay()
				if r.name == "mkdir" && !h.isGameDir(javaPath{p: parentPath(gp.p)}) {
					return int32(0), true
				}
				for q := gp.p; q != ""; q = parentPath(q) {
					o.dirs[q] = true
				}
				return int32(1), true
			case r.name == "getPath" && r.desc == "()Ljava/lang/String;" || r.name == "getAbsolutePath" && r.desc == "()Ljava/lang/String;" || r.name == "toString" && r.desc == "()Ljava/lang/String;":
				return f.path, true
			case r.name == "getName" && r.desc == "()Ljava/lang/String;":
				name := f.path
				if i := strings.LastIndexAny(name, "/\\"); i >= 0 {
					name = name[i+1:]
				}
				return name, true
			case r.name == "getParent" && r.desc == "()Ljava/lang/String;":
				if i := strings.LastIndexAny(f.path, "/\\"); i >= 0 {
					return f.path[:i], true
				}
				return nil, true
			case r.name == "getParentFile" && r.desc == "()Ljava/io/File;":
				if i := strings.LastIndexAny(f.path, "/\\"); i >= 0 {
					return v.alloc("java/io/File", &javaFile{path: f.path[:i]}), true
				}
				return nil, true
			case r.name == "toPath" && r.desc == "()Ljava/nio/file/Path;":
				return h.newPath(cleanGamePath(f.path)), true
			case r.name == "listFiles" && (r.desc == "()[Ljava/io/File;" || r.desc == "(Ljava/io/FileFilter;)[Ljava/io/File;"):
				// Same visibility as exists(): the emulated game directory is
				// the overlay plus the readable config roots, so a directory
				// outside them does not exist and listFiles returns null.
				gp := cleanGamePath(f.path)
				if !h.isGameDir(gp) {
					return nil, true
				}
				o := h.overlay()
				prefix := strings.TrimSuffix(gp.p, "/") + "/"
				if gp.p == "" {
					prefix = ""
				}
				seen := map[string]bool{}
				for _, name := range slices.Sorted(maps.Keys(o.files)) {
					if rest, ok := strings.CutPrefix(name, prefix); ok && rest != "" {
						seen[strings.SplitN(rest, "/", 2)[0]] = true
					}
				}
				for _, name := range slices.Sorted(maps.Keys(o.dirs)) {
					if rest, ok := strings.CutPrefix(name, prefix); ok && rest != "" {
						seen[strings.SplitN(rest, "/", 2)[0]] = true
					}
				}
				var values []any
				for _, child := range slices.Sorted(maps.Keys(seen)) {
					file := v.alloc("java/io/File", &javaFile{path: prefix + child})
					if len(args) == 1 && args[0] != nil && integer(v.method(reference{"java/io/FileFilter", "accept", "(Ljava/io/File;)Z"}, args[0], true, []any{file})) != 0 {
						values = append(values, file)
					} else if len(args) == 0 || args[0] == nil {
						values = append(values, file)
					}
				}
				return &array{values: values, descriptor: "[Ljava/io/File;"}, true
			}
		}
	}
	if (r.owner == "java/io/FileInputStream" || r.owner == "java/io/FileReader") && r.name == "<init>" && (r.desc == "(Ljava/io/File;)V" || r.desc == "(Ljava/lang/String;)V") {
		if args[0] == nil {
			raiseJava("java/lang/NullPointerException", "file")
		}
		name := ""
		if f, ok := objectData(args[0]).(*javaFile); ok {
			name = f.path
		} else {
			name = javaString(args[0])
		}
		data, ok := h.readGameFile(cleanGamePath(name))
		if !ok {
			raiseJava("java/io/FileNotFoundException", name)
		}
		receiver.(*object).data = &javaBytes{data: append([]byte(nil), data...)}
		return nil, true
	}
	if r.owner == "java/lang/Class" && r.name == "getResourceAsStream" && r.desc == "(Ljava/lang/String;)Ljava/io/InputStream;" {
		name, ok := receiver.(classLiteral)
		if !ok {
			if o, isObj := receiver.(*object); isObj && o.class == "java/lang/Class" {
				if lit, isLit := o.data.(classLiteral); isLit {
					name, ok = lit, true
				} else if s, isStr := o.data.(string); isStr {
					name, ok = classLiteral(s), true
				}
			}
		}
		if !ok {
			return nil, false
		}
		res := javaString(args[0])
		if res == "" {
			return nil, true
		}
		stream := h.classpathResource(string(name), res)
		return stream, true
	}
	if r.owner == "java/io/ByteArrayInputStream" || r.owner == "java/io/InputStream" || r.owner == "java/io/FileInputStream" {
		if r.name == "<init>" && r.desc == "([B)V" {
			a := args[0].(*array)
			b := make([]byte, len(a.values))
			for i, x := range a.values {
				b[i] = byte(integer(x))
			}
			receiver.(*object).data = &javaBytes{data: b}
			return nil, true
		}
		if receiver != nil {
			if b, ok := objectData(receiver).(*javaBytes); ok {
				return h.byteStreamCall(r, b, args)
			}
		}
	}
	if r.owner == "java/io/StringReader" && r.name == "<init>" && r.desc == "(Ljava/lang/String;)V" {
		if args[0] == nil {
			raiseJava("java/lang/NullPointerException", "string")
		}
		receiver.(*object).data = &javaStringReader{runes: []rune(javaString(args[0]))}
		return nil, true
	}
	if r.owner == "java/io/StringReader" && receiver != nil {
		sr, ok := objectData(receiver).(*javaStringReader)
		if !ok {
			return nil, false
		}
		switch {
		case r.name == "read" && r.desc == "()I":
			if sr.pos >= len(sr.runes) {
				return int32(-1), true
			}
			ch := int32(sr.runes[sr.pos])
			sr.pos++
			return ch, true
		case r.name == "read" && r.desc == "([CII)I":
			buf, _ := args[0].(*array)
			off, n := int(integer(args[1])), int(integer(args[2]))
			if buf == nil {
				raiseJava("java/lang/NullPointerException", "read")
			}
			if off < 0 || n < 0 || off+n > len(buf.values) {
				raiseJava("java/lang/IndexOutOfBoundsException", "read")
			}
			if sr.pos >= len(sr.runes) {
				return int32(-1), true
			}
			got := 0
			for got < n && sr.pos < len(sr.runes) {
				buf.values[off+got] = int32(sr.runes[sr.pos])
				sr.pos++
				got++
			}
			return int32(got), true
		case r.name == "close" && r.desc == "()V":
			return nil, true
		case r.name == "ready" && r.desc == "()Z":
			return int32(1), true
		case r.name == "skip" && r.desc == "(J)J":
			n := args[0].(int64)
			if n < 0 {
				raiseJava("java/lang/IllegalArgumentException", "skip")
			}
			left := int64(len(sr.runes) - sr.pos)
			if n > left {
				n = left
			}
			sr.pos += int(n)
			return n, true
		}
	}
	if r.owner == "java/io/InputStreamReader" && r.name == "<init>" && r.desc == "(Ljava/io/InputStream;Ljava/nio/charset/CharsetDecoder;)V" {
		if args[0] == nil || args[1] == nil {
			raiseJava("java/lang/NullPointerException", "InputStreamReader")
		}
		if objectData(args[1]) != "UTF-8" {
			fail("unbound reader charset %v", objectData(args[1]))
		}
		if _, ok := objectData(args[0]).(*javaBytes); !ok {
			return nil, false
		}
		receiver.(*object).data = objectData(args[0])
		return nil, true
	}
	if r.owner == "java/io/InputStreamReader" && r.name == "<init>" && (r.desc == "(Ljava/io/InputStream;)V" || r.desc == "(Ljava/io/InputStream;Ljava/nio/charset/Charset;)V" || r.desc == "(Ljava/io/InputStream;Ljava/lang/String;)V") {
		if args[0] == nil {
			raiseJava("java/lang/NullPointerException", "InputStreamReader")
		}
		if _, ok := objectData(args[0]).(*javaBytes); !ok {
			return nil, false
		}
		receiver.(*object).data = objectData(args[0])
		return nil, true
	}
	// Character writers over an overlay output share its buffer (UTF-8 only).
	if (r.owner == "java/io/OutputStreamWriter" && (r.desc == "(Ljava/io/OutputStream;)V" || r.desc == "(Ljava/io/OutputStream;Ljava/nio/charset/Charset;)V") || r.owner == "java/io/BufferedWriter" && (r.desc == "(Ljava/io/Writer;)V" || r.desc == "(Ljava/io/Writer;I)V")) && r.name == "<init>" {
		if args[0] == nil {
			raiseJava("java/lang/NullPointerException", "out")
		}
		out, ok := objectData(args[0]).(*javaOutput)
		if !ok {
			return nil, false
		}
		if r.desc == "(Ljava/io/OutputStream;Ljava/nio/charset/Charset;)V" && objectData(args[1]) != "UTF-8" {
			fail("unbound writer charset")
		}
		receiver.(*object).data = out
		return nil, true
	}
	if r.owner == "java/io/BufferedReader" && r.name == "<init>" && (r.desc == "(Ljava/io/Reader;)V" || r.desc == "(Ljava/io/Reader;I)V") {
		if args[0] == nil {
			raiseJava("java/lang/NullPointerException", "BufferedReader")
		}
		if _, ok := objectData(args[0]).(*javaBytes); !ok {
			return nil, false
		}
		receiver.(*object).data = objectData(args[0])
		return nil, true
	}
	if (r.owner == "java/io/Reader" || r.owner == "java/io/InputStreamReader" || r.owner == "java/io/BufferedReader" || r.owner == "java/io/FileReader" || r.owner == "java/io/StringReader") && receiver != nil {
		if b, ok := objectData(receiver).(*javaBytes); ok {
			if r.name == "close" && r.desc == "()V" {
				return nil, true
			}
			if r.name == "read" && r.desc == "([CII)I" {
				return h.readBytesAsChars(b, args), true
			}
			if r.name == "read" && r.desc == "()I" {
				buf := &array{values: []any{int32(0)}, descriptor: "[C"}
				if integer(h.readBytesAsChars(b, []any{buf, int32(0), int32(1)})) <= 0 {
					return int32(-1), true
				}
				return integer(buf.values[0]), true
			}
			if r.name == "read" && r.desc == "([C)I" {
				buf, ok := args[0].(*array)
				if !ok {
					raiseJava("java/lang/NullPointerException", "read")
				}
				return h.readBytesAsChars(b, []any{buf, int32(0), int32(len(buf.values))}), true
			}
		}
	}
	// java.io.Reader as the superclass of interpreted readers: its state is
	// only the lock, and its concrete methods reduce to read([CII)I.
	if r.owner == "java/io/Reader" {
		if o, ok := receiver.(*object); ok && o.class != "java/io/Reader" {
			if _, bound := objectData(o).(*javaBytes); !bound {
				read := func(buf *array, off, n int32) int32 {
					return integer(v.method(reference{o.class, "read", "([CII)I"}, o, true, []any{buf, off, n}))
				}
				switch {
				case r.name == "<init>" && (r.desc == "()V" || r.desc == "(Ljava/lang/Object;)V"):
					if r.desc != "()V" && args[0] == nil {
						raiseJava("java/lang/NullPointerException", "lock")
					}
					return nil, true
				case r.name == "read" && r.desc == "()I":
					buf := &array{values: []any{int32(0)}, descriptor: "[C"}
					if read(buf, 0, 1) == -1 {
						return int32(-1), true
					}
					return integer(buf.values[0]), true
				case r.name == "read" && r.desc == "([C)I":
					buf, ok := args[0].(*array)
					if !ok {
						raiseJava("java/lang/NullPointerException", "read")
					}
					return read(buf, 0, int32(len(buf.values))), true
				case r.name == "ready" && r.desc == "()Z", r.name == "markSupported" && r.desc == "()Z":
					return int32(0), true
				case r.name == "mark" && r.desc == "(I)V":
					raiseJava("java/io/IOException", "mark() not supported")
				case r.name == "reset" && r.desc == "()V":
					raiseJava("java/io/IOException", "reset() not supported")
				case r.name == "skip" && r.desc == "(J)J":
					n := args[0].(int64)
					if n < 0 {
						raiseJava("java/lang/IllegalArgumentException", "skip value is negative")
					}
					var done int64
					buf := &array{values: make([]any, min(n, 8192)), descriptor: "[C"}
					for done < n {
						got := read(buf, 0, int32(min(n-done, int64(len(buf.values)))))
						if got == -1 {
							break
						}
						done += int64(got)
					}
					return done, true
				}
			}
		}
	}
	// java.io.StringWriter is a pure in-memory character buffer; every other
	// Writer method reduces to its write([CII)V through the block below.
	if r.owner == "java/io/StringWriter" && r.name == "<init>" && (r.desc == "()V" || r.desc == "(I)V") {
		if r.desc == "(I)V" && integer(args[0]) < 0 {
			raiseJava("java/lang/IllegalArgumentException", "Negative buffer size")
		}
		receiver.(*object).data = &stringWriter{}
		return nil, true
	}
	if o, isObj := receiver.(*object); isObj && o != nil {
		if w, ok := o.data.(*stringWriter); ok && (r.owner == "java/io/StringWriter" || r.owner == "java/io/Writer" || r.owner == "java/lang/Object" || r.owner == "java/io/Flushable" || r.owner == "java/io/Closeable" || r.owner == "java/lang/AutoCloseable") {
			switch r.name + r.desc {
			case "write([CII)V":
				buf, ok := args[0].(*array)
				if !ok {
					raiseJava("java/lang/NullPointerException", "write")
				}
				off, n := int(integer(args[1])), int(integer(args[2]))
				if off < 0 || n < 0 || off+n > len(buf.values) {
					raiseJava("java/lang/IndexOutOfBoundsException", "write")
				}
				v.reserveCollection(n)
				for _, c := range buf.values[off : off+n] {
					w.units = append(w.units, uint16(integer(c)))
				}
				return nil, true
			case "flush()V", "close()V":
				return nil, true
			case "toString()Ljava/lang/String;":
				return string(utf16.Decode(w.units)), true
			}
		}
	}
	// java.io.Writer as a superclass: concrete methods reduce to the
	// subclass's write([CII)V.
	if r.owner == "java/io/Writer" {
		if o, ok := receiver.(*object); ok && o.class != "java/io/Writer" {
			write := func(runes []rune) {
				buf := &array{values: make([]any, len(runes)), descriptor: "[C"}
				for i, c := range runes {
					buf.values[i] = int32(c)
				}
				v.method(reference{o.class, "write", "([CII)V"}, o, true, []any{buf, int32(0), int32(len(runes))})
			}
			text := func(x any) []rune {
				if x == nil {
					return []rune("null")
				}
				return []rune(javaString(v.method(reference{"java/lang/Object", "toString", "()Ljava/lang/String;"}, x, true, nil)))
			}
			switch {
			case r.name == "<init>" && (r.desc == "()V" || r.desc == "(Ljava/lang/Object;)V"):
				if r.desc != "()V" && args[0] == nil {
					raiseJava("java/lang/NullPointerException", "lock")
				}
				return nil, true
			case r.name == "write" && r.desc == "(I)V":
				write([]rune{rune(uint16(integer(args[0])))})
				return nil, true
			case r.name == "write" && r.desc == "([C)V":
				buf := args[0].(*array)
				v.method(reference{o.class, "write", "([CII)V"}, o, true, []any{buf, int32(0), int32(len(buf.values))})
				return nil, true
			case r.name == "write" && r.desc == "(Ljava/lang/String;)V":
				if args[0] == nil {
					raiseJava("java/lang/NullPointerException", "write")
				}
				write([]rune(javaString(args[0])))
				return nil, true
			case r.name == "write" && r.desc == "(Ljava/lang/String;II)V":
				s := []rune(javaString(args[0]))
				off, n := int(integer(args[1])), int(integer(args[2]))
				if off < 0 || n < 0 || off+n > len(s) {
					raiseJava("java/lang/IndexOutOfBoundsException", "write")
				}
				write(s[off : off+n])
				return nil, true
			case r.name == "append" && r.desc == "(C)Ljava/io/Writer;":
				write([]rune{rune(integer(args[0]))})
				return o, true
			case r.name == "append" && r.desc == "(Ljava/lang/CharSequence;)Ljava/io/Writer;":
				write(text(args[0]))
				return o, true
			case r.name == "append" && r.desc == "(Ljava/lang/CharSequence;II)Ljava/io/Writer;":
				s := text(args[0])
				i, j := int(integer(args[1])), int(integer(args[2]))
				if i < 0 || i > j || j > len(s) {
					raiseJava("java/lang/IndexOutOfBoundsException", "append")
				}
				write(s[i:j])
				return o, true
			}
		}
	}
	if r.owner == "java/util/Properties" {
		if r.name == "<init>" && (r.desc == "()V" || r.desc == "(I)V") {
			receiver.(*object).data = &javaMap{}
			return nil, true
		}
		if m, ok := objectData(receiver).(*javaMap); ok && receiver != nil {
			switch r.name + r.desc {
			case "stringPropertyNames()Ljava/util/Set;":
				// No defaults chain is modelled; only String?String entries count.
				var names []any
				for _, e := range m.entries {
					_, ks := e.left.(string)
					_, vs := e.right.(string)
					if ks && vs {
						names = append(names, e.left)
					}
				}
				return v.alloc("java/util/Set", &javaSet{values: names, readonly: true}), true
			case "getProperty(Ljava/lang/String;)Ljava/lang/String;", "getProperty(Ljava/lang/String;Ljava/lang/String;)Ljava/lang/String;":
				key := javaString(args[0])
				for _, e := range m.entries {
					if s, ok := e.left.(string); ok && s == key {
						if value, ok := e.right.(string); ok {
							return value, true
						}
					}
				}
				if len(args) == 2 {
					return args[1], true
				}
				return nil, true
			case "setProperty(Ljava/lang/String;Ljava/lang/String;)Ljava/lang/Object;":
				rr := r
				rr.owner, rr.name, rr.desc = "java/util/HashMap", "put", "(Ljava/lang/Object;Ljava/lang/Object;)Ljava/lang/Object;"
				return h.collectionCall(rr, receiver, args)
			}
		}
		if r.name == "load" && (r.desc == "(Ljava/io/InputStream;)V" || r.desc == "(Ljava/io/Reader;)V") {
			m := objectData(receiver).(*javaMap)
			src := objectData(args[0])
			b, ok := src.(*javaBytes)
			if !ok {
				fail("properties load source is not a bound stream")
			}
			parsed, err := readJavaProperties(b.data)
			if err != nil {
				raiseJava("java/io/IOException", err.Error())
			}
			for k, val := range parsed {
				m.entries = append(m.entries, javaPair{k, val})
			}
			return nil, true
		}
		if r.name == "forEach" && r.desc == "(Ljava/util/function/BiConsumer;)V" {
			m := objectData(receiver).(*javaMap)
			for _, e := range append([]javaPair(nil), m.entries...) {
				v.method(reference{"java/util/function/BiConsumer", "accept", "(Ljava/lang/Object;Ljava/lang/Object;)V"}, args[0], true, []any{e.left, e.right})
			}
			return nil, true
		}
		// Properties extends Hashtable: reuse map methods via javaMap data.
		if receiver != nil {
			if _, ok := objectData(receiver).(*javaMap); ok {
				rr := r
				rr.owner = "java/util/HashMap"
				if x, ok := h.collectionCall(rr, receiver, args); ok {
					return x, true
				}
			}
		}
	}
	return nil, false
}

func (h *bindings) byteStreamCall(r reference, b *javaBytes, args []any) (any, bool) {
	switch {
	case r.name == "close" && r.desc == "()V":
		return nil, true
	case r.name == "available" && r.desc == "()I":
		n := len(b.data) - b.pos
		if n < 0 {
			n = 0
		}
		return int32(n), true
	case r.name == "read" && r.desc == "()I":
		if b.pos >= len(b.data) {
			return int32(-1), true
		}
		x := b.data[b.pos]
		b.pos++
		return int32(x), true
	case r.name == "read" && r.desc == "([B)I":
		return h.readBytesInto(b, args[0].(*array), 0, len(args[0].(*array).values)), true
	case r.name == "read" && r.desc == "([BII)I":
		return h.readBytesInto(b, args[0].(*array), int(integer(args[1])), int(integer(args[2]))), true
	}
	return nil, false
}

func (h *bindings) readBytesInto(b *javaBytes, a *array, off, length int) int32 {
	if off < 0 || length < 0 || off+length > len(a.values) {
		raiseJava("java/lang/IndexOutOfBoundsException", "InputStream.read")
	}
	if b.pos >= len(b.data) {
		return -1
	}
	n := length
	if n > len(b.data)-b.pos {
		n = len(b.data) - b.pos
	}
	for i := 0; i < n; i++ {
		a.values[off+i] = int32(int8(b.data[b.pos+i]))
	}
	b.pos += n
	return int32(n)
}

// readBytesAsChars decodes UTF-8 into UTF-16 chars as InputStreamReader does.
// Malformed input fails closed instead of choosing a replacement policy.
func (h *bindings) readBytesAsChars(b *javaBytes, args []any) int32 {
	a := args[0].(*array)
	off, length := int(integer(args[1])), int(integer(args[2]))
	if off < 0 || length < 0 || off+length > len(a.values) {
		raiseJava("java/lang/IndexOutOfBoundsException", "Reader.read")
	}
	if length == 0 {
		return 0
	}
	if b.low == 0 && b.pos >= len(b.data) {
		return -1
	}
	n := 0
	for n < length {
		if b.low != 0 {
			a.values[off+n] = b.low
			b.low = 0
			n++
			continue
		}
		if b.pos >= len(b.data) {
			break
		}
		r, size := utf8.DecodeRune(b.data[b.pos:])
		if r == utf8.RuneError && size <= 1 {
			fail("malformed UTF-8 reader input")
		}
		b.pos += size
		if r >= 0x10000 {
			hi, lo := utf16.EncodeRune(r)
			a.values[off+n] = int32(hi)
			b.low = int32(lo)
		} else {
			a.values[off+n] = int32(r)
		}
		n++
	}
	return int32(n)
}

func (h *bindings) classpathResource(className, resource string) any {
	name := resource
	if strings.HasPrefix(name, "/") {
		name = strings.TrimPrefix(name, "/")
	} else {
		pkg := className
		if i := strings.LastIndex(pkg, "/"); i >= 0 {
			pkg = pkg[:i]
		} else {
			pkg = ""
		}
		if pkg != "" {
			name = pkg + "/" + name
		}
	}
	name = path.Clean(name)
	if name == "." || strings.HasPrefix(name, "../") {
		fail("invalid classpath resource %q", resource)
	}
	resources := h.options.Resources
	if resources == nil {
		return nil
	}
	all, err := resources(name)
	if err != nil || len(all) == 0 {
		return nil
	}
	// First match matches ClassLoader.getResource ordering for a single open archive set.
	return h.vm.alloc("java/io/ByteArrayInputStream", &javaBytes{data: append([]byte(nil), all[0]...)})
}

type javaStringReader struct {
	runes []rune
	pos   int
}
