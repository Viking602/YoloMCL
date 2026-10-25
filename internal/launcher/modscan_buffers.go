package launcher

import (
	"archive/zip"
	"context"
	"io"
	"sync"
)

const (
	modReadChunk   = 64 << 10
	smallNestedJar = 256 << 10
	maxNestedJar   = 16 << 20
	keepNestedJar  = 4 << 20
)

// A workspace lives only while scans are using it. Unlike a sync.Pool, it does
// not retain archive contents after the last scan finishes or is cancelled.
// Overlapping scans share both worker slots and the large nested-jar buffer.
var modScanMemory struct {
	sync.Mutex
	users int
	work  *modScanWorkspace
}

type modScanWorkspace struct {
	workers  chan *modScanBuffer
	metadata chan struct{}
	nested   chan []byte
}

type modScanBuffer struct {
	jar    []byte
	nested []byte
}

func beginModScan() *modScanWorkspace {
	modScanMemory.Lock()
	defer modScanMemory.Unlock()
	if modScanMemory.work == nil {
		w := &modScanWorkspace{
			workers:  make(chan *modScanBuffer, modWorkers),
			metadata: make(chan struct{}, 2),
			nested:   make(chan []byte, 1),
		}
		for range modWorkers {
			w.workers <- &modScanBuffer{}
		}
		w.nested <- nil
		modScanMemory.work = w
	}
	modScanMemory.users++
	return modScanMemory.work
}

// Hashing can overlap disk I/O in all six workers, but ZIP directories can
// contain tens of thousands of allocated entries. Keep just two metadata
// parsers active, independently of hashing and without blocking the UI.
func (w *modScanWorkspace) readMetadata(ctx context.Context, ra io.ReaderAt, size int64, scratch *modScanBuffer, m *Mod) {
	if ctx.Err() != nil {
		return
	}
	select {
	case w.metadata <- struct{}{}:
	case <-ctx.Done():
		return
	}
	defer func() { <-w.metadata }()
	reader := modReaderAt{ctx: ctx, r: ra}
	if z, err := zip.NewReader(reader, size); err == nil {
		w.readModMetadataFrom(ctx, z, reader, scratch, m)
	} else if ctx.Err() == nil {
		m.Status = "invalid"
	}
}

func endModScan() {
	modScanMemory.Lock()
	defer modScanMemory.Unlock()
	modScanMemory.users--
	if modScanMemory.users == 0 {
		modScanMemory.work = nil
	}
}

func (w *modScanWorkspace) acquire(ctx context.Context) (*modScanBuffer, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	select {
	case b := <-w.workers:
		if err := ctx.Err(); err != nil {
			w.workers <- b
			return nil, err
		}
		return b, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

// Grow geometrically within the caller's size limit, then reuse for the rest
// of the scan. Small archive reads never queue behind a large nested archive.
func resizeModBuffer(b []byte, size int) []byte {
	if cap(b) < size {
		n := modReadChunk
		for n < size {
			n *= 2
		}
		b = make([]byte, n)
	}
	return b[:size]
}

// ZIP needs ReaderAt for its directory. Check cancellation during that read
// as well as during decompression, including stored nested archives.
type modReaderAt struct {
	ctx context.Context
	r   io.ReaderAt
}

func (r modReaderAt) ReadAt(p []byte, off int64) (int, error) {
	if err := r.ctx.Err(); err != nil {
		return 0, err
	}
	return r.r.ReadAt(p, off)
}

type modReader struct {
	ctx context.Context
	r   io.Reader
}

func (r modReader) Read(p []byte) (int, error) {
	if err := r.ctx.Err(); err != nil {
		return 0, err
	}
	// Limit a single inflate/read call so leaving a view does not finish
	// decompressing a many-megabyte entry before noticing cancellation.
	return r.r.Read(p[:min(len(p), modReadChunk)])
}
