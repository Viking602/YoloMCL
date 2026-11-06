package launcher

import (
	"archive/zip"
	"context"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/Viking602/YoloMCL/cubiomes/modjar"
)

// zipPrefetch is a side-effect-free, parallel pre-read of one archive source.
// The ordered consumer (readSource) still performs every check, budget and
// accept in source order; prefetched bytes only replace identical reads of
// the same entries, so the snapshot (and its hash) is unchanged.
type zipPrefetch struct {
	f    *os.File
	info os.FileInfo
	z    *zip.Reader
	data map[*zip.File][]byte
}

func (p *zipPrefetch) close() {
	if p != nil && p.f != nil {
		p.f.Close()
		p.f = nil
	}
}

const maxWorldgenPrefetch = 512 << 20

func worldgenPrefetchCandidate(name string) bool {
	if name == "pack.mcmeta" || name == "version.json" {
		return true
	}
	if _, _, ok := worldgenResource(name); ok {
		return true
	}
	// Overlay directories ("<overlay>/data/..."); misses fall back to lazy reads.
	if i := strings.IndexByte(name, '/'); i > 0 {
		if _, _, ok := worldgenResource(name[i+1:]); ok {
			return true
		}
	}
	return false
}

func prefetchWorldgenZip(ctx context.Context, source WorldgenSource, budget *atomic.Int64) *zipPrefetch {
	info, err := os.Lstat(source.Path)
	if err != nil || info.Mode()&os.ModeSymlink != 0 || info.IsDir() || !info.Mode().IsRegular() {
		return nil
	}
	f, err := os.Open(source.Path)
	if err != nil {
		return nil
	}
	if worldgenZipEnvelope(f, info.Size()) != nil {
		f.Close()
		return nil
	}
	z, err := zip.NewReader(f, info.Size())
	if err != nil || len(z.File) > maxWorldgenArchiveEntries {
		f.Close()
		return nil
	}
	p := &zipPrefetch{f: f, info: info, z: z, data: map[*zip.File][]byte{}}
	for _, entry := range z.File {
		if ctx.Err() != nil {
			break
		}
		if entry.UncompressedSize64 > maxWorldgenJSON || entry.Mode()&os.ModeSymlink != 0 || !worldgenPrefetchCandidate(entry.Name) {
			continue
		}
		if budget.Add(int64(entry.UncompressedSize64)) > maxWorldgenPrefetch {
			continue
		}
		if data, err := readZipBytesContext(ctx, entry, maxWorldgenJSON); err == nil {
			p.data[entry] = data
		}
	}
	return p
}

// worldgenPrefetcher pre-reads sources with NumCPU workers in source order.
type worldgenPrefetcher struct {
	results []chan *zipPrefetch
	// discovered holds speculative TerraBlender discovery per archive path,
	// computed while the archive is open for prefetching (pure function).
	discovered sync.Map
	taken      []bool
	wg         sync.WaitGroup
}

func startWorldgenPrefetch(ctx context.Context, sources []WorldgenSource) *worldgenPrefetcher {
	p := &worldgenPrefetcher{results: make([]chan *zipPrefetch, len(sources)), taken: make([]bool, len(sources))}
	for i := range p.results {
		p.results[i] = make(chan *zipPrefetch, 1)
	}
	speculate := false
	for _, s := range sources {
		if strings.Contains(strings.ToLower(filepath.Base(s.Path)), "terrablender") {
			speculate = true
		}
	}
	var next atomic.Int64
	var budget atomic.Int64
	workers := min(max(4, runtime.NumCPU()), len(sources))
	for w := 0; w < workers; w++ {
		p.wg.Add(1)
		go func() {
			defer p.wg.Done()
			for {
				i := int(next.Add(1) - 1)
				if i >= len(sources) {
					return
				}
				if ctx.Err() != nil {
					p.results[i] <- nil
					continue
				}
				pf := prefetchWorldgenZip(ctx, sources[i], &budget)
				if pf != nil && speculate && strings.HasPrefix(sources[i].Name, "mod:") && ctx.Err() == nil {
					d, err := discoverTerraBlenderZip(ctx, jarFactKey(sources[i].Path), pf.z)
					if ctx.Err() == nil {
						p.discovered.Store(sources[i].Path, discoveryResult{d, err})
					}
				}
				p.results[i] <- pf
			}
		}()
	}
	return p
}

type discoveryResult struct {
	d   modjar.Discovery
	err error
}

// discovery returns a speculative discovery result for path, if computed.
func (p *worldgenPrefetcher) discovery(path string) (discoveryResult, bool) {
	if p == nil {
		return discoveryResult{}, false
	}
	v, ok := p.discovered.Load(path)
	if !ok {
		return discoveryResult{}, false
	}
	return v.(discoveryResult), true
}

// take returns source i's prefetch (nil when the source must be read lazily).
func (p *worldgenPrefetcher) take(i int) *zipPrefetch {
	if p == nil || i >= len(p.results) {
		return nil
	}
	p.taken[i] = true
	return <-p.results[i]
}

// finish closes every prefetched archive not yet consumed.
func (p *worldgenPrefetcher) finish() {
	if p == nil {
		return
	}
	p.wg.Wait()
	for i, ch := range p.results {
		if !p.taken[i] {
			(<-ch).close()
		}
	}
}
