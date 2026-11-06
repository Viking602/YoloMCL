package modjar

import (
	"path"
	"strings"
	"unicode/utf8"
)

// javaPath is a game-directory-relative path in the emulated dedicated-server
// filesystem. The root is the server's game directory. Reads see only the
// instance's configuration files exposed through Options.Config (config/...)
// and files the mods themselves wrote earlier in this session. Writes land in
// a per-session in-memory overlay and never reach the disk, matching a first
// server launch where mods generate their own default files.
type javaPath struct {
	p       string
	foreign bool // absolute or outside the game directory: never readable
}

type pathOverlay struct {
	files map[string][]byte
	dirs  map[string]bool
	// deleted hides instance files the session removed; the disk is untouched.
	deleted map[string]bool
}

type javaOutput struct {
	path string
	buf  []byte
	open bool
}

func cleanGamePath(raw string) javaPath {
	s := strings.ReplaceAll(raw, "\\", "/")
	if strings.HasPrefix(s, "/") || len(s) > 1 && s[1] == ':' {
		return javaPath{p: path.Clean(s), foreign: true}
	}
	c := path.Clean(s)
	if c == "." {
		c = ""
	}
	if c == ".." || strings.HasPrefix(c, "../") {
		return javaPath{p: c, foreign: true}
	}
	return javaPath{p: c}
}

func (p javaPath) String() string {
	if p.p == "" {
		return "."
	}
	return p.p
}

func (p javaPath) join(child string) javaPath {
	c := strings.ReplaceAll(child, "\\", "/")
	if strings.HasPrefix(c, "/") || len(c) > 1 && c[1] == ':' {
		return cleanGamePath(c)
	}
	if p.p == "" {
		return cleanGamePath(c)
	}
	j := cleanGamePath(p.p + "/" + c)
	j.foreign = j.foreign || p.foreign
	return j
}

func (h *bindings) overlay() *pathOverlay {
	if h.paths == nil {
		// A started server's game directory always holds config/ and mods/.
		h.paths = &pathOverlay{files: map[string][]byte{}, dirs: map[string]bool{"": true, "config": true, "mods": true}, deleted: map[string]bool{}}
	}
	return h.paths
}

func (h *bindings) newPath(p javaPath) *object { return h.vm.alloc("java/nio/file/Path", p) }

func pathArg(x any) (javaPath, bool) {
	if x == nil {
		raiseJava("java/lang/NullPointerException", "null path")
	}
	p, ok := objectData(x).(javaPath)
	return p, ok
}

// readGameFile resolves a path exactly as a dedicated server on this instance
// would see it: session-written files first, then config/ from the instance.
func (h *bindings) readGameFile(p javaPath) ([]byte, bool) {
	if p.foreign {
		return nil, false
	}
	o := h.overlay()
	if data, ok := o.files[p.p]; ok {
		return data, true
	}
	if o.deleted[p.p] {
		return nil, false
	}
	// Server precedence is applied by the instance reader: config/ is read as
	// is; defaultconfigs/ and serverconfig/ resolve per-world serverconfig
	// first, then defaultconfigs. Nothing outside these roots is readable.
	name := ""
	if rest, ok := strings.CutPrefix(p.p, "config/"); ok {
		name = rest
	} else if rest, ok := strings.CutPrefix(p.p, "defaultconfigs/"); ok {
		name = "serverconfig/" + rest
	} else if rest, ok := strings.CutPrefix(p.p, "serverconfig/"); ok {
		name = "serverconfig/" + rest
	}
	if name == "" || strings.HasSuffix(name, "/") || h.options.Config == nil {
		return nil, false
	}
	data, err := h.options.Config(name)
	if err != nil {
		fail("configuration %s: %v", name, err)
	}
	if data == nil {
		return nil, false
	}
	// A successful instance config read implies the parent directories exist on
	// the readable tree. Mark them in the overlay so later writes (which only
	// check overlay dirs) match Java Files semantics for an already-present file.
	for q := path.Dir(p.p); q != "" && q != "."; q = path.Dir(q) {
		o.dirs[q] = true
	}
	return data, true
}

func (h *bindings) isGameDir(p javaPath) bool {
	if p.foreign {
		return false
	}
	o := h.overlay()
	if o.dirs[p.p] {
		return true
	}
	prefix := p.p + "/"
	for name := range o.files {
		if strings.HasPrefix(name, prefix) {
			return true
		}
	}
	return false
}

func (h *bindings) writeGameFile(p javaPath, data []byte) {
	if p.foreign {
		raiseJava("java/nio/file/AccessDeniedException", p.p)
	}
	o := h.overlay()
	parent := path.Dir(p.p)
	if parent == "." {
		parent = ""
	}
	if !h.isGameDir(javaPath{p: parent}) {
		// Materialize missing parents when a readable ancestor already exists
		// (e.g. config/), matching createDirectories + write for first-time paths.
		ancestor := parent
		for ancestor != "" && ancestor != "." && !h.isGameDir(javaPath{p: ancestor}) {
			ancestor = path.Dir(ancestor)
			if ancestor == "." {
				ancestor = ""
			}
		}
		if !h.isGameDir(javaPath{p: ancestor}) {
			raiseJava("java/nio/file/NoSuchFileException", parent)
		}
		for q := parent; q != "" && q != "." && q != ancestor; q = path.Dir(q) {
			o.dirs[q] = true
		}
		if parent != "" && parent != "." {
			o.dirs[parent] = true
		}
	}
	if len(o.files) >= 512 || len(data) > 1<<20 {
		fail("emulated filesystem budget exceeded")
	}
	o.files[p.p] = append([]byte(nil), data...)
	delete(o.deleted, p.p)
}

func javaByteArray(b []byte) *array {
	values := make([]any, len(b))
	for i, x := range b {
		values[i] = int32(int8(x))
	}
	return &array{values: values, descriptor: "[B"}
}

func arrayBytes(x any) []byte {
	a, ok := x.(*array)
	if !ok || a.descriptor != "[B" {
		fail("invalid byte array")
	}
	b := make([]byte, len(a.values))
	for i, v := range a.values {
		b[i] = byte(integer(v))
	}
	return b
}

func (h *bindings) pathCall(r reference, receiver any, args []any) (any, bool) {
	v := h.vm
	if r.owner == "java/nio/file/Paths" && r.name == "get" && r.desc == "(Ljava/lang/String;[Ljava/lang/String;)Ljava/nio/file/Path;" ||
		r.owner == "java/nio/file/Path" && r.name == "of" && r.desc == "(Ljava/lang/String;[Ljava/lang/String;)Ljava/nio/file/Path;" {
		if args[0] == nil {
			raiseJava("java/lang/NullPointerException", "path")
		}
		p := cleanGamePath(javaString(args[0]))
		if more, ok := args[1].(*array); ok {
			for _, x := range more.values {
				if x == nil {
					raiseJava("java/lang/NullPointerException", "path element")
				}
				p = p.join(javaString(x))
			}
		}
		return h.newPath(p), true
	}
	if r.owner == "java/nio/file/Path" && receiver != nil {
		p, ok := objectData(receiver).(javaPath)
		if !ok {
			return nil, false
		}
		switch {
		case r.name == "resolve" && r.desc == "(Ljava/lang/String;)Ljava/nio/file/Path;":
			if args[0] == nil {
				raiseJava("java/lang/NullPointerException", "resolve")
			}
			return h.newPath(p.join(javaString(args[0]))), true
		case r.name == "resolve" && r.desc == "(Ljava/nio/file/Path;)Ljava/nio/file/Path;":
			q, ok := pathArg(args[0])
			if !ok {
				return nil, false
			}
			if q.foreign {
				return h.newPath(q), true
			}
			return h.newPath(p.join(q.p)), true
		case r.name == "resolveSibling" && r.desc == "(Ljava/lang/String;)Ljava/nio/file/Path;":
			parent := path.Dir(p.p)
			if parent == "." {
				parent = ""
			}
			return h.newPath(javaPath{p: parent, foreign: p.foreign}.join(javaString(args[0]))), true
		case r.name == "getParent" && r.desc == "()Ljava/nio/file/Path;":
			if p.p == "" {
				return nil, true
			}
			parent := path.Dir(p.p)
			if parent == "." {
				parent = ""
			}
			return h.newPath(javaPath{p: parent, foreign: p.foreign}), true
		case r.name == "getFileName" && r.desc == "()Ljava/nio/file/Path;":
			return h.newPath(javaPath{p: path.Base(p.String())}), true
		case (r.name == "normalize" || r.name == "toAbsolutePath") && r.desc == "()Ljava/nio/file/Path;":
			// The server's game directory is the working directory.
			return receiver, true
		case r.name == "toString" && r.desc == "()Ljava/lang/String;":
			return p.String(), true
		case r.name == "toFile" && r.desc == "()Ljava/io/File;":
			return v.alloc("java/io/File", &javaFile{path: p.String()}), true
		case r.name == "startsWith" && r.desc == "(Ljava/lang/String;)Z", r.name == "endsWith" && r.desc == "(Ljava/lang/String;)Z":
			q := cleanGamePath(javaString(args[0])).p
			hit := p.p == q || r.name == "startsWith" && (q == "" || strings.HasPrefix(p.p, q+"/")) || r.name == "endsWith" && strings.HasSuffix(p.p, "/"+q)
			if hit {
				return int32(1), true
			}
			return int32(0), true
		case r.name == "equals" && r.desc == "(Ljava/lang/Object;)Z":
			q, ok := objectData(args[0]).(javaPath)
			if ok && q == p {
				return int32(1), true
			}
			return int32(0), true
		case r.name == "hashCode" && r.desc == "()I":
			hash := int32(0)
			for _, ch := range p.p {
				hash = 31*hash + int32(ch)
			}
			return hash, true
		}
		return nil, false
	}
	if r.owner == "java/nio/file/Files" {
		if len(args) == 0 {
			return nil, false
		}
		p, ok := pathArg(args[0])
		if !ok {
			return nil, false
		}
		boolean := func(b bool) any {
			if b {
				return int32(1)
			}
			return int32(0)
		}
		_, isFile := h.readGameFile(p)
		switch r.name + r.desc {
		case "exists(Ljava/nio/file/Path;[Ljava/nio/file/LinkOption;)Z":
			return boolean(isFile || h.isGameDir(p)), true
		case "notExists(Ljava/nio/file/Path;[Ljava/nio/file/LinkOption;)Z":
			return boolean(!isFile && !h.isGameDir(p)), true
		case "isRegularFile(Ljava/nio/file/Path;[Ljava/nio/file/LinkOption;)Z", "isReadable(Ljava/nio/file/Path;)Z":
			return boolean(isFile), true
		case "isDirectory(Ljava/nio/file/Path;[Ljava/nio/file/LinkOption;)Z":
			return boolean(h.isGameDir(p)), true
		case "delete(Ljava/nio/file/Path;)V", "deleteIfExists(Ljava/nio/file/Path;)Z":
			if p.foreign {
				raiseJava("java/nio/file/AccessDeniedException", p.p)
			}
			if !isFile {
				if h.isGameDir(p) {
					raiseJava("java/nio/file/DirectoryNotEmptyException", p.p)
				}
				if r.name == "delete" {
					raiseJava("java/nio/file/NoSuchFileException", p.p)
				}
				return int32(0), true
			}
			o := h.overlay()
			delete(o.files, p.p)
			o.deleted[p.p] = true
			if r.name == "delete" {
				return nil, true
			}
			return int32(1), true
		case "isWritable(Ljava/nio/file/Path;)Z":
			return boolean(!p.foreign && (isFile || h.isGameDir(p))), true
		case "createDirectories(Ljava/nio/file/Path;[Ljava/nio/file/attribute/FileAttribute;)Ljava/nio/file/Path;",
			"createDirectory(Ljava/nio/file/Path;[Ljava/nio/file/attribute/FileAttribute;)Ljava/nio/file/Path;":
			if p.foreign {
				raiseJava("java/nio/file/AccessDeniedException", p.p)
			}
			if isFile {
				raiseJava("java/nio/file/FileAlreadyExistsException", p.p)
			}
			o := h.overlay()
			if r.name == "createDirectory" {
				parent := path.Dir(p.p)
				if parent == "." {
					parent = ""
				}
				if !h.isGameDir(javaPath{p: parent}) {
					raiseJava("java/nio/file/NoSuchFileException", parent)
				}
				if h.isGameDir(p) {
					raiseJava("java/nio/file/FileAlreadyExistsException", p.p)
				}
			}
			for q := p.p; q != "" && q != "."; q = path.Dir(q) {
				o.dirs[q] = true
			}
			return args[0], true
		case "size(Ljava/nio/file/Path;)J":
			return int64(len(h.mustRead(p))), true
		case "readAllBytes(Ljava/nio/file/Path;)[B":
			data := h.mustRead(p)
			return javaByteArray(data), true
		case "readString(Ljava/nio/file/Path;)Ljava/lang/String;", "readString(Ljava/nio/file/Path;Ljava/nio/charset/Charset;)Ljava/lang/String;":
			return h.utf8Text(p, args), true
		case "readAllLines(Ljava/nio/file/Path;)Ljava/util/List;", "readAllLines(Ljava/nio/file/Path;Ljava/nio/charset/Charset;)Ljava/util/List;":
			text := h.utf8Text(p, args)
			var values []any
			for _, line := range strings.SplitAfter(text, "\n") {
				if line == "" {
					continue
				}
				values = append(values, strings.TrimSuffix(strings.TrimSuffix(line, "\n"), "\r"))
			}
			return v.alloc("java/util/ArrayList", &javaList{values: values}), true
		case "newInputStream(Ljava/nio/file/Path;[Ljava/nio/file/OpenOption;)Ljava/io/InputStream;":
			return v.alloc("java/io/ByteArrayInputStream", &javaBytes{data: h.mustRead(p)}), true
		case "newBufferedReader(Ljava/nio/file/Path;)Ljava/io/BufferedReader;", "newBufferedReader(Ljava/nio/file/Path;Ljava/nio/charset/Charset;)Ljava/io/BufferedReader;":
			text := h.utf8Text(p, args)
			return v.alloc("java/io/BufferedReader", &javaBytes{data: []byte(text)}), true
		case "write(Ljava/nio/file/Path;[B[Ljava/nio/file/OpenOption;)Ljava/nio/file/Path;":
			h.writeGameFile(p, arrayBytes(args[1]))
			return args[0], true
		case "writeString(Ljava/nio/file/Path;Ljava/lang/CharSequence;[Ljava/nio/file/OpenOption;)Ljava/nio/file/Path;",
			"writeString(Ljava/nio/file/Path;Ljava/lang/CharSequence;Ljava/nio/charset/Charset;[Ljava/nio/file/OpenOption;)Ljava/nio/file/Path;":
			h.writeGameFile(p, []byte(javaString(args[1])))
			return args[0], true
		case "move(Ljava/nio/file/Path;Ljava/nio/file/Path;[Ljava/nio/file/CopyOption;)Ljava/nio/file/Path;":
			target, ok := pathArg(args[1])
			if !ok {
				return nil, false
			}
			if p.foreign || target.foreign {
				raiseJava("java/nio/file/AccessDeniedException", target.p)
			}
			if !isFile {
				raiseJava("java/nio/file/NoSuchFileException", p.p)
			}
			replace := false
			if opts, ok := args[2].(*array); ok {
				for _, o := range opts.values {
					if name := objectData(o); name == "REPLACE_EXISTING" || name == "ATOMIC_MOVE" {
						replace = true
					}
				}
			}
			if h.isGameDir(target) {
				raiseJava("java/nio/file/FileAlreadyExistsException", target.p)
			}
			if _, exists := h.readGameFile(target); exists && !replace {
				raiseJava("java/nio/file/FileAlreadyExistsException", target.p)
			}
			data, _ := h.readGameFile(p)
			h.writeGameFile(target, append([]byte(nil), data...))
			o := h.overlay()
			delete(o.files, p.p)
			o.deleted[p.p] = true
			return args[1], true
		case "newOutputStream(Ljava/nio/file/Path;[Ljava/nio/file/OpenOption;)Ljava/io/OutputStream;",
			"newBufferedWriter(Ljava/nio/file/Path;[Ljava/nio/file/OpenOption;)Ljava/io/BufferedWriter;",
			"newBufferedWriter(Ljava/nio/file/Path;Ljava/nio/charset/Charset;[Ljava/nio/file/OpenOption;)Ljava/io/BufferedWriter;":
			h.writeGameFile(p, nil)
			class := "java/io/OutputStream"
			if r.name == "newBufferedWriter" {
				class = "java/io/BufferedWriter"
			}
			return v.alloc(class, &javaOutput{path: p.p, open: true}), true
		}
		return nil, false
	}
	if o, ok := receiver.(*object); ok && o != nil {
		if w, ok := o.data.(*javaOutput); ok && (strings.HasPrefix(r.owner, "java/io/")) {
			switch r.name + r.desc {
			case "write([B)V":
				w.buf = append(w.buf, arrayBytes(args[0])...)
			case "write([BII)V":
				b := arrayBytes(args[0])
				off, n := int(integer(args[1])), int(integer(args[2]))
				if off < 0 || n < 0 || off+n > len(b) {
					raiseJava("java/lang/IndexOutOfBoundsException", "write")
				}
				w.buf = append(w.buf, b[off:off+n]...)
			case "write(I)V":
				if r.owner == "java/io/OutputStream" || strings.HasSuffix(r.owner, "Stream") {
					w.buf = append(w.buf, byte(integer(args[0])))
				} else {
					w.buf = append(w.buf, string(rune(uint16(integer(args[0]))))...)
				}
			case "write([CII)V":
				a, _ := args[0].(*array)
				off, n := int(integer(args[1])), int(integer(args[2]))
				if a == nil || off < 0 || n < 0 || off+n > len(a.values) {
					raiseJava("java/lang/IndexOutOfBoundsException", "write")
				}
				for _, c := range a.values[off : off+n] {
					w.buf = append(w.buf, string(rune(integer(c)))...)
				}
			case "write(Ljava/lang/String;II)V":
				s := []rune(javaString(args[0]))
				off, n := int(integer(args[1])), int(integer(args[2]))
				if off < 0 || n < 0 || off+n > len(s) {
					raiseJava("java/lang/IndexOutOfBoundsException", "write")
				}
				w.buf = append(w.buf, string(s[off:off+n])...)
			case "append(C)Ljava/io/Writer;":
				w.buf = append(w.buf, string(rune(integer(args[0])))...)
			case "write(Ljava/lang/String;)V", "append(Ljava/lang/CharSequence;)Ljava/io/Writer;":
				w.buf = append(w.buf, javaString(args[0])...)
				if r.name == "append" {
					return receiver, true
				}
			case "newLine()V":
				w.buf = append(w.buf, '\n')
			case "flush()V":
			case "close()V":
				if w.open {
					h.writeGameFile(javaPath{p: w.path}, w.buf)
					w.open = false
				}
			default:
				return nil, false
			}
			if w.open && r.name != "close" {
				h.overlay().files[w.path] = append([]byte(nil), w.buf...)
				delete(h.overlay().deleted, w.path)
			}
			return nil, true
		}
	}
	return nil, false
}

func (h *bindings) mustRead(p javaPath) []byte {
	data, ok := h.readGameFile(p)
	if !ok {
		raiseJava("java/nio/file/NoSuchFileException", p.String())
	}
	return data
}

func (h *bindings) utf8Text(p javaPath, args []any) string {
	if len(args) > 1 && objectData(args[1]) != "UTF-8" {
		fail("unbound file charset")
	}
	data := h.mustRead(p)
	if !utf8.Valid(data) {
		raiseJava("java/nio/charset/MalformedInputException", p.String())
	}
	return string(data)
}
