package modjar

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
	"sync"
	"testing"
)

func nestedArchiveBytes(t testing.TB, payload []byte, large bool) []byte {
	t.Helper()
	var b bytes.Buffer
	z := zip.NewWriter(&b)
	if large {
		f, err := z.CreateHeader(&zip.FileHeader{Name: "unrelated/native.bin", Method: zip.Store})
		if err != nil {
			t.Fatal(err)
		}
		if _, err = f.Write(make([]byte, (8<<20)+123)); err != nil {
			t.Fatal(err)
		}
	}
	f, err := z.Create("example/Dependency.class")
	if err != nil {
		t.Fatal(err)
	}
	if _, err = f.Write(payload); err != nil {
		t.Fatal(err)
	}
	if err = z.Close(); err != nil {
		t.Fatal(err)
	}
	return b.Bytes()
}

func TestClasspathFollowsOnlyActiveDeclaredLibraries(t *testing.T) {
	for _, loader := range []string{"forge", "neoforge", "fabric", "quilt"} {
		t.Run(loader, func(t *testing.T) {
			files := map[string][]byte{"libs/active.jar": nestedArchiveBytes(t, []byte("active"), false), "libs/inactive.jar": nestedArchiveBytes(t, []byte("wrong"), false)}
			switch loader {
			case "forge", "neoforge":
				files["META-INF/jarjar/metadata.json"] = []byte(`{"jars":[{"path":"libs/active.jar"}]}`)
			case "fabric":
				files["fabric.mod.json"] = []byte(`{"jars":[{"file":"libs/active.jar"}]}`)
			case "quilt":
				files["quilt.mod.json"] = []byte(`{"quilt_loader":{"jars":["libs/active.jar"]}}`)
				files["fabric.mod.json"] = []byte("invalid inactive template")
			}
			cp, err := NewClasspath(context.Background(), []Archive{{"root", fixtureArchive(t, files)}}, loader)
			if err != nil {
				t.Fatal(err)
			}
			got, err := cp.Class("example/Dependency")
			if err != nil || string(got) != "active" || len(cp.Archives()) != 2 {
				t.Fatalf("%q %v %v", got, err, cp.Archives())
			}
		})
	}
}

func TestLargeNestedClasspathHasBoundedRetainedData(t *testing.T) {
	payload := []byte("only this class is read")
	z := fixtureArchive(t, map[string][]byte{"fabric.mod.json": []byte(`{"jars":[{"file":"large.jar"}]}`), "large.jar": nestedArchiveBytes(t, payload, true)})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	cp, err := NewClasspath(ctx, []Archive{{"root", z}}, "fabric")
	if err != nil {
		t.Fatal(err)
	}
	defer cp.Close()
	if cp.retained != 0 {
		t.Fatalf("retained complete large library: %d", cp.retained)
	}
	var wg sync.WaitGroup
	for range 8 {
		wg.Go(func() {
			got, err := cp.Class("example/Dependency")
			if err != nil || !bytes.Equal(got, payload) {
				t.Errorf("concurrent class read %q %v", got, err)
			}
		})
	}
	wg.Wait()
	cancel()
	if _, err = cp.Class("example/Dependency"); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
}

func TestClasspathRejectsConflictsMissingLibrariesAndBadPaths(t *testing.T) {
	root := func(name, value string) Archive {
		return Archive{name, fixtureArchive(t, map[string][]byte{"example/Dependency.class": []byte(value)})}
	}
	cp, err := NewClasspath(context.Background(), []Archive{root("a", "one"), root("b", "two")}, "neoforge")
	if err != nil {
		t.Fatal(err)
	}
	if _, err = cp.Class("example/Dependency"); err == nil || !strings.Contains(err.Error(), "conflicting class") {
		t.Fatal(err)
	}
	for _, path := range []string{"missing.jar", "../escape.jar", "/absolute.jar", "a\\b.jar"} {
		meta := []byte(fmt.Sprintf(`{"quilt_loader":{"jars":[%q]}}`, path))
		_, err := NewClasspath(context.Background(), []Archive{{"root", fixtureArchive(t, map[string][]byte{"quilt.mod.json": meta})}}, "quilt")
		if err == nil {
			t.Fatalf("accepted %s", path)
		}
	}
}

func TestNestedReaderSupportsRandomWindowsAndEOF(t *testing.T) {
	payload := make([]byte, 3*(64<<10)+17)
	for i := range payload {
		payload[i] = byte(i * 31)
	}
	z := fixtureArchive(t, map[string][]byte{"nested": payload})
	r := &nestedReaderAt{ctx: context.Background(), entry: z.File[0]}
	defer r.Close()
	for _, offset := range []int{65530, 0, 130001, 31, len(payload) - 4} {
		buf := make([]byte, 8192)
		n, err := r.ReadAt(buf, int64(offset))
		want := min(len(buf), len(payload)-offset)
		if n != want || !bytes.Equal(buf[:n], payload[offset:offset+want]) || want < len(buf) && err != io.EOF || want == len(buf) && err != nil {
			t.Fatalf("offset %d n=%d err=%v", offset, n, err)
		}
	}
}

type countedArchive struct {
	*bytes.Reader
	read int
}

func (r *countedArchive) ReadAt(p []byte, off int64) (int, error) {
	n, err := r.Reader.ReadAt(p, off)
	r.read += n
	return n, err
}

func TestAdjacentArchiveWindowsDoNotReplayDecompression(t *testing.T) {
	payload := make([]byte, 8*(64<<10))
	var b bytes.Buffer
	z := zip.NewWriter(&b)
	f, err := z.CreateHeader(&zip.FileHeader{Name: "library", Method: zip.Store})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = f.Write(payload); err != nil {
		t.Fatal(err)
	}
	if err = z.Close(); err != nil {
		t.Fatal(err)
	}
	source := &countedArchive{Reader: bytes.NewReader(b.Bytes())}
	zr, err := zip.NewReader(source, int64(b.Len()))
	if err != nil {
		t.Fatal(err)
	}
	source.read = 0
	r := &nestedReaderAt{ctx: context.Background(), entry: zr.File[0]}
	defer r.Close()
	for offset := 0; offset < len(payload); offset += 64 << 10 {
		var value [1]byte
		if _, err = r.ReadAt(value[:], int64(offset)); err != nil {
			t.Fatal(err)
		}
	}
	if source.read > 2*len(payload) {
		t.Fatalf("sequential windows replayed prior archive data: read=%d size=%d", source.read, len(payload))
	}
	if err = r.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err = r.ReadAt(make([]byte, 1), 0); err == nil {
		t.Fatal("closed archive remained readable")
	}
}

func TestDuplicateUnusedClassDoesNotPoisonClasspath(t *testing.T) {
	var b bytes.Buffer
	z := zip.NewWriter(&b)
	for _, entry := range []struct{ name, data string }{{"example/Unused.class", "first"}, {"example/Unused.class", "second"}, {"example/Active.class", "active"}, {"example/Active.class", "active"}} {
		f, err := z.Create(entry.name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err = f.Write([]byte(entry.data)); err != nil {
			t.Fatal(err)
		}
	}
	if err := z.Close(); err != nil {
		t.Fatal(err)
	}
	r, err := zip.NewReader(bytes.NewReader(b.Bytes()), int64(b.Len()))
	if err != nil {
		t.Fatal(err)
	}
	cp, err := NewClasspath(context.Background(), []Archive{{"duplicate", r}}, "neoforge")
	if err != nil {
		t.Fatal(err)
	}
	got, err := cp.Class("example/Active")
	if err != nil || string(got) != "active" {
		t.Fatalf("%q %v", got, err)
	}
	if _, err = cp.Class("example/Unused"); err == nil {
		t.Fatal("different executed definitions must not be selected by ZIP order")
	}
}

func TestDuplicateServiceResourcesAreNotMergedWithinOneArchive(t *testing.T) {
	for _, different := range []bool{false, true} {
		var b bytes.Buffer
		z := zip.NewWriter(&b)
		for i := 0; i < 2; i++ {
			f, err := z.Create("META-INF/services/sample.Service")
			if err != nil {
				t.Fatal(err)
			}
			data := "sample.First\n"
			if different && i == 1 {
				data = "sample.Second\n"
			}
			if _, err = f.Write([]byte(data)); err != nil {
				t.Fatal(err)
			}
		}
		if err := z.Close(); err != nil {
			t.Fatal(err)
		}
		r, err := zip.NewReader(bytes.NewReader(b.Bytes()), int64(b.Len()))
		if err != nil {
			t.Fatal(err)
		}
		cp, err := NewClasspath(context.Background(), []Archive{{"resource", r}}, "forge")
		if err != nil {
			t.Fatal(err)
		}
		got, err := cp.Resources("META-INF/services/sample.Service")
		cp.Close()
		if different {
			if err == nil {
				t.Fatal("ambiguous duplicate resources accepted")
			}
		} else if err != nil || len(got) != 1 || string(got[0]) != "sample.First\n" {
			t.Fatalf("%q %v", got, err)
		}
	}
}

func TestLoaderJSONPreservesGsonQuotedValues(t *testing.T) {
	raw := "\ufeff{\"description\":\"line1\nline2\\\nline3\\'\t\u0001\",\"quoted\":\"\\\"\\\\\"}"
	var got map[string]string
	if err := json.Unmarshal(loaderJSON([]byte(raw)), &got); err != nil {
		t.Fatal(err)
	}
	if got["description"] != "line1\nline2\nline3'\t\u0001" || got["quoted"] != "\"\\" {
		t.Fatal(got)
	}
	for _, invalid := range []string{`{unquoted:1}`, `{"bad":"\q"}`, `{"x":1,}`, "{\"x\":1}\x01"} {
		if json.Valid(loaderJSON([]byte(invalid))) {
			t.Fatalf("accepted invalid metadata: %q", invalid)
		}
	}
}
