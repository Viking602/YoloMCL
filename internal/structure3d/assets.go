package structure3d

import (
	"archive/zip"
	"bytes"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"strings"
	"sync"
)

// Store is a layered read of Minecraft assets and data. Later Open calls win.
type Store struct {
	mu           sync.Mutex
	layers       []layer
	poolEdits    []PoolEdit
	lithoShuffle bool
	// monumentYOffset shifts the vanilla ocean monument building (Tectonic
	// oceans.monument_offset, for its deeper oceans).
	monumentYOffset int
}

// SetMonumentYOffset sets the vertical shift of vanilla ocean monuments.
func (s *Store) SetMonumentYOffset(dy int) { s.monumentYOffset = dy }

// MonumentYOffset is the vertical shift of vanilla ocean monuments.
func (s *Store) MonumentYOffset() int { return s.monumentYOffset }

type layer struct {
	name  string
	read  func(path string) ([]byte, error)
	list  func(prefix string) []string
	close func() error
}

// OpenJar adds a zip (game jar, mod jar, or resource pack zip). Later jars overlay earlier ones.
func (s *Store) OpenJar(name, file string) error {
	z, err := zip.OpenReader(file)
	if err != nil {
		return err
	}
	index := make(map[string]*zip.File, len(z.File))
	for _, f := range z.File {
		index[f.Name] = f
	}
	// Nested jars (Forge/NeoForge JarJar, Fabric jar-in-jar) load below their
	// parent, the way the mod loader puts them on the classpath.
	s.openNested(name, z.File, 0)
	s.mu.Lock()
	s.layers = append(s.layers, layer{
		name: name,
		read: func(p string) ([]byte, error) {
			f := index[p]
			if f == nil {
				return nil, fs.ErrNotExist
			}
			r, err := f.Open()
			if err != nil {
				return nil, err
			}
			defer r.Close()
			return io.ReadAll(io.LimitReader(r, 16<<20))
		},
		list:  zipLister(z.File),
		close: z.Close,
	})
	s.mu.Unlock()
	return nil
}

// OpenJarSubroot opens the entries under root inside a jar as if they were a
// standalone pack (Forge/NeoForge/Fabric resourcepacks/<id>/ embedded packs).
// root must be a clean relative prefix ending with "/". Later layers still overlay.
func (s *Store) OpenJarSubroot(name, file, root string) error {
	if root == "" || root == "/" || strings.Contains(root, "..") || !strings.HasSuffix(root, "/") {
		return fmt.Errorf("invalid jar subroot %q", root)
	}
	z, err := zip.OpenReader(file)
	if err != nil {
		return err
	}
	var files []*zip.File
	index := map[string]*zip.File{}
	for _, f := range z.File {
		rest, ok := strings.CutPrefix(f.Name, root)
		if !ok || rest == "" {
			continue
		}
		c := *f
		c.Name = rest
		files = append(files, &c)
		index[rest] = &c
	}
	if len(index) == 0 {
		z.Close()
		return fs.ErrNotExist
	}
	s.mu.Lock()
	s.layers = append(s.layers, layer{
		name: name,
		read: func(p string) ([]byte, error) {
			f := index[p]
			if f == nil {
				return nil, fs.ErrNotExist
			}
			r, err := f.Open()
			if err != nil {
				return nil, err
			}
			defer r.Close()
			return io.ReadAll(io.LimitReader(r, 16<<20))
		},
		list:  zipLister(files),
		close: z.Close,
	})
	s.mu.Unlock()
	return nil
}

// OpenDir adds a filesystem directory (unzipped pack or testdata).
func (s *Store) OpenDir(name, dir string) error {
	root := filepath.Clean(dir)
	s.mu.Lock()
	s.layers = append(s.layers, layer{
		name: name,
		read: func(p string) ([]byte, error) {
			full := filepath.Join(root, filepath.FromSlash(p))
			if !strings.HasPrefix(full, root+string(os.PathSeparator)) && full != root {
				return nil, fmt.Errorf("path escapes store root")
			}
			return os.ReadFile(full)
		},
		list: func(prefix string) []string {
			var out []string
			base := filepath.Join(root, filepath.FromSlash(prefix))
			_ = filepath.WalkDir(base, func(p string, d fs.DirEntry, err error) error {
				if err == nil && !d.IsDir() {
					if rel, e := filepath.Rel(root, p); e == nil {
						out = append(out, filepath.ToSlash(rel))
					}
				}
				return nil
			})
			return out
		},
	})
	s.mu.Unlock()
	return nil
}

// OpenBytes adds an in-memory zip for tests.
func (s *Store) OpenBytes(name string, data []byte) error {
	z, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return err
	}
	index := make(map[string]*zip.File, len(z.File))
	for _, f := range z.File {
		index[f.Name] = f
	}
	s.mu.Lock()
	s.layers = append(s.layers, layer{
		name: name,
		read: func(p string) ([]byte, error) {
			f := index[p]
			if f == nil {
				return nil, fs.ErrNotExist
			}
			r, err := f.Open()
			if err != nil {
				return nil, err
			}
			defer r.Close()
			return io.ReadAll(io.LimitReader(r, 16<<20))
		},
		list: zipLister(z.File),
	})
	s.mu.Unlock()
	return nil
}

// Close releases zip handles.
func (s *Store) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	var first error
	for i := len(s.layers) - 1; i >= 0; i-- {
		if s.layers[i].close != nil {
			if err := s.layers[i].close(); err != nil && first == nil {
				first = err
			}
		}
	}
	s.layers = nil
	return first
}

// Read returns the highest-priority bytes for a slash-separated jar path.
func (s *Store) Read(p string) ([]byte, error) {
	p = path.Clean("/" + strings.ReplaceAll(p, "\\", "/"))[1:]
	s.mu.Lock()
	layers := append([]layer(nil), s.layers...)
	s.mu.Unlock()
	last := fs.ErrNotExist
	for i := len(layers) - 1; i >= 0; i-- {
		b, err := layers[i].read(p)
		if err == nil {
			return b, nil
		}
		last = err
	}
	return nil, last
}

// Namespaced turns "minecraft:oak_planks" into ("minecraft","oak_planks").
func Namespaced(id string) (ns, path string) {
	ns, path, ok := strings.Cut(id, ":")
	if !ok {
		return "minecraft", id
	}
	return ns, path
}

// DataPath is data/<ns>/<kind>/<path>.ext under a jar.
func DataPath(ns, kind, name, ext string) string {
	return path.Join("data", ns, kind, name+ext)
}

// AssetPath is assets/<ns>/<kind>/<path>.
func AssetPath(ns, kind, name string) string {
	return path.Join("assets", ns, kind, name)
}

func isNestedJar(p string) bool {
	low := strings.ToLower(p)
	return strings.HasSuffix(low, ".jar") && (strings.HasPrefix(low, "meta-inf/jarjar/") || strings.HasPrefix(low, "meta-inf/jars/"))
}

func (s *Store) openNested(parent string, files []*zip.File, depth int) {
	if depth > 2 {
		return
	}
	for _, f := range files {
		if !isNestedJar(f.Name) || f.UncompressedSize64 > 64<<20 {
			continue
		}
		r, err := f.Open()
		if err != nil {
			continue
		}
		data, err := io.ReadAll(r)
		r.Close()
		if err != nil {
			continue
		}
		z, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
		if err != nil {
			continue
		}
		hasData := false
		for _, e := range z.File {
			// assets/ too: a jar-in-jar library can ship the block models
			// and textures its parent mod's blocks use.
			if strings.HasPrefix(e.Name, "data/") || strings.HasPrefix(e.Name, "assets/") || isNestedJar(e.Name) {
				hasData = true
				break
			}
		}
		if hasData {
			s.openNested(parent+"!"+f.Name, z.File, depth+1)
			_ = s.OpenBytes(parent+"!"+f.Name, data)
		}
	}
}

// Layers lists layer names, lowest priority first (diagnostics and tests).
func (s *Store) Layers() []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]string, len(s.layers))
	for i, l := range s.layers {
		out[i] = l.name
	}
	return out
}

func zipLister(files []*zip.File) func(string) []string {
	return func(prefix string) []string {
		var out []string
		for _, f := range files {
			if strings.HasPrefix(f.Name, prefix) && !strings.HasSuffix(f.Name, "/") {
				out = append(out, f.Name)
			}
		}
		return out
	}
}

// List returns every path under prefix across all layers, deduplicated,
// with the layer that wins each path (the highest one holding it).
func (s *Store) List(prefix string) map[string]string {
	s.mu.Lock()
	layers := append([]layer(nil), s.layers...)
	s.mu.Unlock()
	out := map[string]string{}
	for _, l := range layers {
		if l.list == nil {
			continue
		}
		for _, p := range l.list(prefix) {
			out[p] = l.name
		}
	}
	return out
}
