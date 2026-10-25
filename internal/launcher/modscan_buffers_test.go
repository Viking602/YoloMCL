package launcher

import (
	"archive/zip"
	"bytes"
	"compress/flate"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"sync"
	"testing"
	"time"
)

func nestedArchive(t testing.TB, id string, size int, method uint16) (*zip.Reader, *bytes.Reader, []byte) {
	t.Helper()
	var inner bytes.Buffer
	z := zip.NewWriter(&inner)
	for _, entry := range []struct{ name, content string }{
		{"fabric.mod.json", fmt.Sprintf(`{"id":%q,"provides":["alias-%s"]}`, id, id)},
		{"META-INF/neoforge.mods.toml", fmt.Sprintf("[[mods]]\nmodId = %q\n", "forge-"+id)},
		{"quilt.mod.json", fmt.Sprintf(`{"quilt_loader":{"id":%q}}`, "quilt-"+id)},
	} {
		f, err := z.Create(entry.name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := io.WriteString(f, entry.content); err != nil {
			t.Fatal(err)
		}
	}
	f, err := z.CreateHeader(&zip.FileHeader{Name: "payload.bin", Method: zip.Store})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.Write(bytes.Repeat([]byte{7}, size)); err != nil {
		t.Fatal(err)
	}
	if err := z.Close(); err != nil {
		t.Fatal(err)
	}
	var outer bytes.Buffer
	z = zip.NewWriter(&outer)
	f, err = z.CreateHeader(&zip.FileHeader{Name: "META-INF/jars/lib.jar", Method: method})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.Write(inner.Bytes()); err != nil {
		t.Fatal(err)
	}
	if err := z.Close(); err != nil {
		t.Fatal(err)
	}
	ra := bytes.NewReader(outer.Bytes())
	r, err := zip.NewReader(ra, ra.Size())
	if err != nil {
		t.Fatal(err)
	}
	return r, ra, inner.Bytes()
}

func TestNestedJarScratchPreservesMetadataAndIsNotResident(t *testing.T) {
	func() {
		w := beginModScan()
		defer endModScan()
		scratch, err := w.acquire(context.Background())
		if err != nil {
			t.Fatal(err)
		}
		defer func() { w.workers <- scratch }()
		var first []string
		var previous *byte
		for _, id := range []string{"first", "other"} {
			z, ra, _ := nestedArchive(t, id, 512<<10, zip.Deflate)
			got := w.readNestedModIDs(context.Background(), z.File[0], ra, scratch)
			want := []string{id, "alias-" + id, "forge-" + id, "quilt-" + id}
			if !slices.Equal(got, want) {
				t.Fatalf("nested metadata: got %v want %v", got, want)
			}
			buffer := <-w.nested
			address := &buffer[0]
			w.nested <- buffer
			if previous != nil && previous != address {
				t.Fatal("equally sized nested jars allocated another large buffer")
			}
			previous = address
			if first == nil {
				first = got
			}
		}
		if !slices.Equal(first, []string{"first", "alias-first", "forge-first", "quilt-first"}) {
			t.Fatal("reusing archive bytes corrupted already decoded metadata")
		}
	}()
	modScanMemory.Lock()
	retained := modScanMemory.work != nil || modScanMemory.users != 0
	modScanMemory.Unlock()
	if retained {
		t.Fatal("completed scan retained its workspace")
	}
	w := beginModScan()
	defer endModScan()
	b := <-w.nested
	w.nested <- b
	if cap(b) != 0 {
		t.Fatal("opening another view retained the old large scratch allocation")
	}
}

type cancellingNestedReader struct {
	io.Reader
	cancel context.CancelFunc
	read   int
	closed bool
}

func (r *cancellingNestedReader) Read(p []byte) (int, error) {
	n, err := r.Reader.Read(p)
	r.read += n
	r.cancel()
	return n, err
}

func (r *cancellingNestedReader) Close() error { r.closed = true; return nil }

func TestNestedJarCancellationStopsInflateAndReleasesBuffer(t *testing.T) {
	temp := t.TempDir()
	for _, name := range []string{"TMP", "TEMP", "TMPDIR"} {
		t.Setenv(name, temp)
	}
	z, ra, data := nestedArchive(t, "cancelled", 4<<20, zip.Deflate)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	r := &cancellingNestedReader{Reader: bytes.NewReader(data), cancel: cancel}
	z.RegisterDecompressor(zip.Deflate, func(io.Reader) io.ReadCloser { return r })
	w := beginModScan()
	defer endModScan()
	if ids := w.readNestedModIDs(ctx, z.File[0], ra, &modScanBuffer{}); len(ids) != 0 {
		t.Fatal("cancelled nested read published metadata")
	}
	if !r.closed || r.read > modReadChunk || r.read == 0 {
		t.Fatalf("cancelled inflate kept reading: read=%d closed=%v", r.read, r.closed)
	}
	if files, err := os.ReadDir(temp); err != nil || len(files) != 0 {
		t.Fatalf("cancelled inflate left a temporary archive: %v, %v", files, err)
	}
	select {
	case buffer := <-w.nested:
		w.nested <- buffer
	default:
		t.Fatal("cancelled inflate kept the shared buffer checked out")
	}
	// A new scan can use the same buffer immediately; a cancelled request
	// must not publish incomplete results or poison the next request.
	z, ra, _ = nestedArchive(t, "ready", 4<<20, zip.Deflate)
	if ids := w.readNestedModIDs(context.Background(), z.File[0], ra, &modScanBuffer{}); len(ids) == 0 || ids[0] != "ready" {
		t.Fatalf("buffer not usable after cancellation: %v", ids)
	}
	if files, err := os.ReadDir(temp); err != nil || len(files) != 0 {
		t.Fatalf("completed inflate left a temporary archive: %v, %v", files, err)
	}
}

func TestOversizedNestedJarDoesNotInflateRetainedScratch(t *testing.T) {
	w := beginModScan()
	defer endModScan()
	// Prime a reusable medium buffer, then read one unusually large library.
	// Subsequent small files must not keep that large allocation reachable.
	for _, size := range []int{512 << 10, keepNestedJar + 1} {
		z, ra, _ := nestedArchive(t, "sized", size, zip.Deflate)
		ids := w.readNestedModIDs(context.Background(), z.File[0], ra, &modScanBuffer{})
		if len(ids) == 0 || ids[0] != "sized" {
			t.Fatalf("large nested metadata lost: %v", ids)
		}
		b := <-w.nested
		capacity := cap(b)
		w.nested <- b
		if capacity > keepNestedJar {
			t.Fatalf("oversized dependency retained %d bytes", capacity)
		}
	}
}

func TestNestedJarTemporaryStorageFallbackAndChecksum(t *testing.T) {
	temp := t.TempDir()
	for _, name := range []string{"TMP", "TEMP", "TMPDIR"} {
		t.Setenv(name, temp)
	}
	w := beginModScan()
	defer endModScan()
	z, ra, _ := nestedArchive(t, "large", keepNestedJar+1, zip.Deflate)
	z.File[0].CRC32++
	if ids := w.readNestedModIDs(context.Background(), z.File[0], ra, &modScanBuffer{}); len(ids) != 0 {
		t.Fatal("file-backed read accepted a corrupt checksum")
	}
	if files, err := os.ReadDir(temp); err != nil || len(files) != 0 {
		t.Fatalf("failed read left a temporary archive: %v, %v", files, err)
	}
	z.File[0].CRC32--
	// A recoverable file I/O error may occur after the temporary file was
	// created. Reopen the archive for the memory fallback instead of caching
	// a successful-looking result with missing dependency IDs.
	attempts := 0
	z.RegisterDecompressor(zip.Deflate, func(r io.Reader) io.ReadCloser {
		attempts++
		if attempts == 1 {
			return io.NopCloser(nestedIOErrorReader{})
		}
		return flate.NewReader(r)
	})
	if ids := w.readNestedModIDs(context.Background(), z.File[0], ra, &modScanBuffer{}); len(ids) == 0 || ids[0] != "large" || attempts != 2 {
		t.Fatalf("file I/O fallback lost metadata: ids=%v attempts=%d", ids, attempts)
	}
	if files, err := os.ReadDir(temp); err != nil || len(files) != 0 {
		t.Fatalf("file I/O fallback left a temporary archive: %v, %v", files, err)
	}
	for _, name := range []string{"TMP", "TEMP", "TMPDIR"} {
		t.Setenv(name, filepath.Join(temp, "unavailable"))
	}
	ids := w.readNestedModIDs(context.Background(), z.File[0], ra, &modScanBuffer{})
	if len(ids) == 0 || ids[0] != "large" {
		t.Fatalf("unavailable temporary storage lost dependency IDs: %v", ids)
	}
	buffer := <-w.nested
	w.nested <- buffer
	if cap(buffer) > keepNestedJar {
		t.Fatal("temporary-storage fallback retained its oversized allocation")
	}
}

type nestedIOErrorReader struct{}

func (nestedIOErrorReader) Read([]byte) (int, error) {
	return 0, &os.PathError{Op: "read", Path: "scratch.jar", Err: os.ErrPermission}
}

func TestNestedJarQueueKeepsSmallAndStoredReadsAvailable(t *testing.T) {
	w := beginModScan()
	defer endModScan()
	large := <-w.nested
	defer func() { w.nested <- large }()
	for _, tc := range []struct {
		name   string
		method uint16
		size   int
	}{{"small", zip.Deflate, 1024}, {"stored", zip.Store, 1 << 20}} {
		z, ra, _ := nestedArchive(t, tc.name, tc.size, tc.method)
		done := make(chan []string, 1)
		ctx, cancel := context.WithCancel(context.Background())
		go func() { done <- w.readNestedModIDs(ctx, z.File[0], ra, &modScanBuffer{}) }()
		select {
		case ids := <-done:
			cancel()
			if len(ids) == 0 || ids[0] != tc.name {
				t.Fatalf("%s metadata: %v", tc.name, ids)
			}
		case <-time.After(time.Second):
			cancel()
			<-done
			t.Fatalf("%s read queued behind the large buffer", tc.name)
		}
	}
	z, ra, _ := nestedArchive(t, "waiting", 512<<10, zip.Deflate)
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan []string, 1)
	go func() { done <- w.readNestedModIDs(ctx, z.File[0], ra, &modScanBuffer{}) }()
	cancel()
	select {
	case ids := <-done:
		if len(ids) != 0 {
			t.Fatal("cancelled queued read published metadata")
		}
	case <-time.After(time.Second):
		t.Fatal("cancelled queued read waited for a large buffer")
	}
}

func TestOverlappingModScansShareSlotsAndCancelWaiters(t *testing.T) {
	w := beginModScan()
	defer endModScan()
	other := beginModScan()
	defer endModScan()
	if w != other {
		t.Fatal("overlapping scans each allocated their own workspace")
	}
	for range modWorkers {
		b, err := w.acquire(context.Background())
		if err != nil {
			t.Fatal(err)
		}
		defer func() { w.workers <- b }()
	}
	ctx, cancel := context.WithCancel(context.Background())
	var wg sync.WaitGroup
	for range modWorkers {
		wg.Go(func() {
			_, _, _, err := other.readJar(ctx, "must-not-be-opened.jar", 123, &Mod{})
			if !errors.Is(err, context.Canceled) {
				t.Errorf("cancelled waiter touched the next file: %v", err)
			}
		})
	}
	cancel()
	done := make(chan struct{})
	go func() { wg.Wait(); close(done) }()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("obsolete scans held their waiters after cancellation")
	}
}

func TestMetadataQueueCancelsWithoutStartingZIPReads(t *testing.T) {
	w := beginModScan()
	defer endModScan()
	for range cap(w.metadata) {
		w.metadata <- struct{}{}
		defer func() { <-w.metadata }()
	}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	var m Mod
	go func() {
		// nil is intentional: a cancelled waiter must never touch the file.
		w.readMetadata(ctx, nil, 100, &modScanBuffer{}, &m)
		close(done)
	}()
	cancel()
	select {
	case <-done:
		if m.Status == "invalid" {
			t.Fatal("cancellation incorrectly marked the archive invalid")
		}
	case <-time.After(time.Second):
		t.Fatal("leaving the page kept a queued metadata parser")
	}
}
