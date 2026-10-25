package launcher

import (
	"context"
	"encoding/json"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

type JavaScanProgress struct {
	Found     []JavaInfo
	Visited   int
	Phase     string
	Running   bool
	ElapsedMS int64
}

var javaReleaseVersion = regexp.MustCompile(`^[0-9][0-9._+a-zA-Z-]*$`)

func javaMajor(version string) int {
	version = strings.TrimPrefix(version, "1.")
	end := 0
	for end < len(version) && version[end] >= '0' && version[end] <= '9' {
		end++
	}
	major, _ := strconv.Atoi(version[:end])
	return major
}

func readJavaMetadata(path string) ([]byte, error) {
	f, e := os.Open(path)
	if e != nil {
		return nil, e
	}
	defer f.Close()
	return io.ReadAll(io.LimitReader(f, 64<<10))
}

func canonicalJava(path string) string {
	path = strings.Trim(strings.TrimSpace(path), `"`)
	if strings.HasSuffix(strings.ToLower(path), "javaw.exe") {
		path = filepath.Join(filepath.Dir(path), "java.exe")
	}
	if info, e := os.Stat(path); e != nil || info.IsDir() {
		return ""
	}
	abs, e := filepath.Abs(path)
	if e != nil {
		return ""
	}
	if real, e := filepath.EvalSymlinks(abs); e == nil {
		abs = real
	}
	return filepath.Clean(abs)
}
func javaPathKey(path string) string {
	if runtime.GOOS == "windows" {
		return strings.ToLower(path)
	}
	return path
}

// Read release metadata before launching a JVM. All modern JDK/JRE distributions
// ship this file, making discovery independent of JVM cold-start time.
func inspectJavaRelease(ctx context.Context, path string) (JavaInfo, error) {
	home := filepath.Dir(filepath.Dir(path))
	b, e := readJavaMetadata(filepath.Join(home, "release"))
	if e != nil && strings.EqualFold(filepath.Base(home), "jre") {
		b, e = readJavaMetadata(filepath.Join(filepath.Dir(home), "release"))
	}
	if e == nil && len(b) < 64<<10 {
		fields := map[string]string{}
		for _, line := range strings.Split(string(b), "\n") {
			k, v, ok := strings.Cut(line, "=")
			if ok {
				fields[strings.TrimSpace(k)] = strings.Trim(strings.TrimSpace(v), `"`)
			}
		}
		version := fields["JAVA_VERSION"]
		if javaReleaseVersion.MatchString(version) {
			major := javaMajor(version)
			if major > 0 {
				return JavaInfo{Path: path, Version: version, Major: major, Vendor: fields["IMPLEMENTOR"], Architecture: fields["OS_ARCH"]}, nil
			}
		}
	}
	return inspectJava(ctx, path)
}

func (s *Service) CachedJavaList() []JavaInfo {
	b, e := os.ReadFile(filepath.Join(s.dir, "cache", "java-list.json"))
	if e != nil || len(b) > 1<<20 {
		return nil
	}
	var items []JavaInfo
	if json.Unmarshal(b, &items) != nil {
		return nil
	}
	out := []JavaInfo{}
	seen := map[string]bool{}
	for _, j := range items {
		// These are exact executables already recorded by this launcher, not
		// directories to traverse. Java installations are independent of the
		// Minecraft content scan boundary.
		if !filepath.IsAbs(j.Path) {
			continue
		}
		if path := canonicalJava(j.Path); path != "" {
			key := javaPathKey(path)
			if seen[key] {
				continue
			}
			seen[key] = true
			j.Path = path
			out = append(out, j)
		}
	}
	return out
}

func (s *Service) javaConfigPaths() (explicit, allowed []string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	explicit = append(explicit, s.cfg.Settings.JavaPath)
	allowed = append(allowed, s.dir)
	for _, r := range s.cfg.Roots {
		allowed = append(allowed, r.Path)
		explicit = append(explicit, r.JavaPath)
		for _, v := range r.VersionConfigs {
			explicit = append(explicit, v.JavaPath)
		}
	}
	return
}

// javaScope describes the recursively searched Minecraft/launcher runtime
// directories. System installations are discovered separately as exact Java
// candidates; they never become recursive directory scan roots.
func (s *Service) javaScope() (explicit, roots, allowed []string) {
	explicit, allowed = s.javaConfigPaths()
	roots = append(roots, s.runtimeDir())
	for _, path := range allowed[1:] {
		bases := []string{path}
		if dir, err := safeJoin(path, "versions"); err == nil {
			entries, _ := os.ReadDir(dir)
			for _, en := range entries {
				if en.IsDir() && en.Type()&os.ModeSymlink == 0 {
					if base, err := safeJoin(path, "versions/"+en.Name()); err == nil {
						bases = append(bases, base)
					}
				}
			}
		}
		for _, base := range bases {
			for _, folder := range []string{"runtime", "runtimes", "java", "jre", ".jdks"} {
				// Most instances have none of these folders. Avoid validating
				// every ancestor five times just to discard a missing path in
				// the walker. Existing candidates still pass safeJoin's checks.
				info, err := os.Lstat(filepath.Join(base, folder))
				if err != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
					continue
				}
				if path, err := safeJoin(base, folder); err == nil {
					roots = append(roots, path)
				}
			}
		}
	}
	return
}

func (s *Service) javaHints() ([]string, []string) {
	explicit, roots, _ := s.javaScope()
	for _, j := range s.CachedJavaList() {
		explicit = append(explicit, j.Path)
	}
	return explicit, roots
}

// ScanJava detects registered system Java installations and inspects the
// known runtime folders. Deeper mode adds nesting only inside Minecraft and
// launcher runtime folders; neither mode enumerates local disks.
func (s *Service) ScanJava(ctx context.Context, deep bool, notify func(JavaScanProgress)) ([]JavaInfo, error) {
	explicit, roots := s.javaHints()
	explicit = append(explicit, systemJavaPaths(ctx)...)
	return s.scanJavaHints(ctx, deep, explicit, roots, notify)
}

func (s *Service) scanJavaHints(ctx context.Context, deep bool, explicit, roots []string, notify func(JavaScanProgress)) ([]JavaInfo, error) {
	if deep {
		return s.scanJava(ctx, explicit, nil, roots, notify)
	}
	return s.scanJava(ctx, explicit, roots, nil, notify)
}

func (s *Service) scanJava(ctx context.Context, explicit, roots, deepRoots []string, notify func(JavaScanProgress)) ([]JavaInfo, error) {
	started := time.Now()
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	paths := make(chan string, 128)
	found := make(chan JavaInfo, 32)
	var visited atomic.Int64
	var phase atomic.Value
	phase.Store("正在检测已安装的 Java")
	seen := map[string]bool{}
	var seenMu sync.Mutex
	queue := func(path string) {
		path = canonicalJava(path)
		if path == "" {
			return
		}
		key := javaPathKey(path)
		seenMu.Lock()
		if seen[key] {
			seenMu.Unlock()
			return
		}
		seen[key] = true
		seenMu.Unlock()
		select {
		case paths <- path:
		case <-ctx.Done():
		}
	}
	var workers sync.WaitGroup
	for i := 0; i < 8; i++ {
		workers.Go(func() {
			for path := range paths {
				if ctx.Err() != nil {
					return
				}
				v, e := inspectJavaRelease(ctx, path)
				if e == nil {
					select {
					case found <- v:
					case <-ctx.Done():
						return
					}
				}
			}
		})
	}
	go func() {
		defer close(paths)
		for _, p := range explicit {
			queue(p)
		}
		walk := func(base string, maxDepth int) {
			base = filepath.Clean(base)
			if info, e := os.Lstat(base); e != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
				return
			}
			_ = filepath.WalkDir(base, func(path string, d fs.DirEntry, e error) error {
				if ctx.Err() != nil {
					return ctx.Err()
				}
				if e != nil {
					return nil
				}
				if d.IsDir() {
					visited.Add(1)
					if d.Type()&os.ModeSymlink != 0 {
						return filepath.SkipDir
					}
					if path != base {
						switch strings.ToLower(d.Name()) {
						case "$recycle.bin", "system volume information", "windows", ".git", "node_modules", "gpucache", "code cache":
							return filepath.SkipDir
						}
						if maxDepth >= 0 {
							rel, _ := filepath.Rel(base, path)
							if strings.Count(rel, string(filepath.Separator)) >= maxDepth {
								return filepath.SkipDir
							}
						}
					}
					return nil
				}
				if strings.EqualFold(d.Name(), "java.exe") || strings.EqualFold(d.Name(), "javaw.exe") || (runtime.GOOS != "windows" && d.Name() == "java") {
					queue(path)
				}
				return nil
			})
		}
		walkRoots := func(items []string, depth int) {
			var group sync.WaitGroup
			slots := make(chan struct{}, 6)
			rootSeen := map[string]bool{}
			for _, base := range items {
				if !filepath.IsAbs(base) {
					continue
				}
				key := javaPathKey(filepath.Clean(base))
				if rootSeen[key] {
					continue
				}
				rootSeen[key] = true
				select {
				case slots <- struct{}{}:
				case <-ctx.Done():
					group.Wait()
					return
				}
				group.Go(func() { defer func() { <-slots }(); walk(base, depth) })
			}
			group.Wait()
		}
		walkRoots(roots, 6)
		if len(deepRoots) > 0 && ctx.Err() == nil {
			phase.Store("正在扫描整合包与启动器运行时")
			walkRoots(deepRoots, 10)
		}
	}()
	go func() { workers.Wait(); close(found) }()
	out := []JavaInfo{}
	ticker := time.NewTicker(160 * time.Millisecond)
	defer ticker.Stop()
	send := func(running bool) {
		sort.Slice(out, func(i, j int) bool {
			if out[i].Major != out[j].Major {
				return out[i].Major > out[j].Major
			}
			if out[i].Version != out[j].Version {
				return out[i].Version > out[j].Version
			}
			return out[i].Path < out[j].Path
		})
		if notify != nil {
			notify(JavaScanProgress{Found: append([]JavaInfo(nil), out...), Visited: int(visited.Load()), Phase: phase.Load().(string), Running: running, ElapsedMS: time.Since(started).Milliseconds()})
		}
	}
	for {
		select {
		case v, ok := <-found:
			if !ok {
				send(false)
				if ctx.Err() == nil {
					_ = atomicJSON(filepath.Join(s.dir, "cache", "java-list.json"), out)
				}
				return out, ctx.Err()
			}
			out = append(out, v)
		case <-ticker.C:
			send(true)
		}
	}
}
