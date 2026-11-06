package modjar

import (
	"context"
	"testing"
)

func newServerFSFixture(t *testing.T, files map[string]string) (*machine, *bindings, *[]string) {
	t.Helper()
	v := newMachine(context.Background(), map[string][]byte{})
	var asked []string
	h := &bindings{vm: v, options: Options{Config: func(name string) ([]byte, error) {
		asked = append(asked, name)
		if s, ok := files[name]; ok {
			return []byte(s), nil
		}
		return nil, nil
	}}}
	v.host = h
	return v, h, &asked
}

func serverPath(t *testing.T, h *bindings, s string) *object {
	t.Helper()
	x, ok := h.pathCall(reference{"java/nio/file/Paths", "get", "(Ljava/lang/String;[Ljava/lang/String;)Ljava/nio/file/Path;"}, nil, []any{s, &array{descriptor: "[Ljava/lang/String;"}})
	if !ok {
		t.Fatal("Paths.get unbound")
	}
	return x.(*object)
}

func filesCall(t *testing.T, h *bindings, name, desc string, args ...any) any {
	t.Helper()
	x, ok := h.pathCall(reference{"java/nio/file/Files", name, desc}, nil, args)
	if !ok {
		t.Fatalf("Files.%s%s unbound", name, desc)
	}
	return x
}

func TestServerFSReadsExistingTunedConfig(t *testing.T) {
	v, h, _ := newServerFSFixture(t, map[string]string{"biomesoplenty/generation.toml": "region_weight = 7\n"})
	p := serverPath(t, h, "config/biomesoplenty/generation.toml")
	if filesCall(t, h, "exists", "(Ljava/nio/file/Path;[Ljava/nio/file/LinkOption;)Z", p, &array{}) != int32(1) {
		t.Fatal("existing instance config reported missing")
	}
	if got := filesCall(t, h, "readString", "(Ljava/nio/file/Path;)Ljava/lang/String;", p); got != "region_weight = 7\n" {
		t.Fatal(got)
	}
	f := v.alloc("java/io/File", nil)
	h.ioCall(reference{"java/io/File", "<init>", "(Ljava/lang/String;)V"}, f, []any{"config/biomesoplenty/generation.toml"})
	if x, _ := h.ioCall(reference{"java/io/File", "exists", "()Z"}, f, nil); x != int32(1) {
		t.Fatal("File.exists does not reflect the instance")
	}
	if x, _ := h.ioCall(reference{"java/io/File", "length", "()J"}, f, nil); x != int64(18) {
		t.Fatal(x)
	}
}

func TestServerFSServerConfigPrecedence(t *testing.T) {
	// The instance reader resolves serverconfig/ as world serverconfig >
	// defaultconfigs; defaultconfigs/ paths must go through that same lookup.
	_, h, asked := newServerFSFixture(t, map[string]string{"serverconfig/mod-server.toml": "x = 1\n"})
	p := serverPath(t, h, "defaultconfigs/mod-server.toml")
	if got := filesCall(t, h, "readString", "(Ljava/nio/file/Path;)Ljava/lang/String;", p); got != "x = 1\n" {
		t.Fatal(got)
	}
	if (*asked)[len(*asked)-1] != "serverconfig/mod-server.toml" {
		t.Fatal(*asked)
	}
}

func TestServerFSMissingConfigFallsBackAndWritesStayInOverlay(t *testing.T) {
	_, h, asked := newServerFSFixture(t, nil)
	p := serverPath(t, h, "config/newmod/defaults.toml")
	if filesCall(t, h, "exists", "(Ljava/nio/file/Path;[Ljava/nio/file/LinkOption;)Z", p, &array{}) != int32(0) {
		t.Fatal("missing config reported present")
	}
	func() {
		defer func() {
			e, ok := recover().(javaException)
			if !ok || e.class != "java/nio/file/NoSuchFileException" {
				t.Fatal("missing config read did not raise NoSuchFileException", e)
			}
		}()
		filesCall(t, h, "readAllBytes", "(Ljava/nio/file/Path;)[B", p)
	}()
	dir := serverPath(t, h, "config/newmod")
	filesCall(t, h, "createDirectories", "(Ljava/nio/file/Path;[Ljava/nio/file/attribute/FileAttribute;)Ljava/nio/file/Path;", dir, &array{})
	filesCall(t, h, "writeString", "(Ljava/nio/file/Path;Ljava/lang/CharSequence;[Ljava/nio/file/OpenOption;)Ljava/nio/file/Path;", p, "default = true\n", &array{})
	if got := filesCall(t, h, "readString", "(Ljava/nio/file/Path;)Ljava/lang/String;", p); got != "default = true\n" {
		t.Fatal(got)
	}
	if string(h.paths.files["config/newmod/defaults.toml"]) != "default = true\n" {
		t.Fatal("write not kept in the overlay")
	}
	for _, name := range *asked {
		if name != "newmod/defaults.toml" && name != "newmod" {
			t.Fatal("unexpected instance read", name)
		}
	}
	outside := serverPath(t, h, "../escape.toml")
	if filesCall(t, h, "exists", "(Ljava/nio/file/Path;[Ljava/nio/file/LinkOption;)Z", outside, &array{}) != int32(0) {
		t.Fatal("path outside the instance is visible")
	}
	func() {
		defer func() {
			if e, ok := recover().(javaException); !ok || e.class != "java/nio/file/AccessDeniedException" {
				t.Fatal("write outside the instance accepted", e)
			}
		}()
		filesCall(t, h, "write", "(Ljava/nio/file/Path;[B[Ljava/nio/file/OpenOption;)Ljava/nio/file/Path;", outside, &array{descriptor: "[B"}, &array{})
	}()
}

func TestServerFSDeleteHidesInstanceFileOnlyInSession(t *testing.T) {
	_, h, _ := newServerFSFixture(t, map[string]string{"mod/trades.json": "{}"})
	p := serverPath(t, h, "config/mod/trades.json")
	filesCall(t, h, "delete", "(Ljava/nio/file/Path;)V", p)
	if filesCall(t, h, "exists", "(Ljava/nio/file/Path;[Ljava/nio/file/LinkOption;)Z", p, &array{}) != int32(0) {
		t.Fatal("deleted file still visible")
	}
	if filesCall(t, h, "deleteIfExists", "(Ljava/nio/file/Path;)Z", p) != int32(0) {
		t.Fatal("second delete reported a file")
	}
	h.overlay().dirs["config/mod"] = true
	filesCall(t, h, "writeString", "(Ljava/nio/file/Path;Ljava/lang/CharSequence;[Ljava/nio/file/OpenOption;)Ljava/nio/file/Path;", p, "[]", &array{})
	if got := filesCall(t, h, "readString", "(Ljava/nio/file/Path;)Ljava/lang/String;", p); got != "[]" {
		t.Fatal(got)
	}
}
