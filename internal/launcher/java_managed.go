package launcher

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

var dirSizes sync.Map // home -> int64

func dirSize(ctx context.Context, p string) int64 {
	if v, ok := dirSizes.Load(p); ok {
		return v.(int64)
	}
	var n int64
	err := filepath.WalkDir(p, func(_ string, d fs.DirEntry, e error) error {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if e == nil && !d.IsDir() {
			if i, e := d.Info(); e == nil {
				n += i.Size()
			}
		}
		return nil
	})
	if err == nil && ctx.Err() == nil {
		dirSizes.Store(p, n)
	}
	return n
}

func javaHome(exe string) string { return filepath.Dir(filepath.Dir(exe)) }

// isManagedJava: inside MCL's runtime folders and carrying the marker
// (older auto-installs in runtimes/ count too).
func (s *Service) isManagedJava(home string) (bool, string) {
	for _, base := range []string{s.runtimeDir()} {
		rel, e := filepath.Rel(base, home)
		if e != nil || rel == "." || strings.HasPrefix(rel, "..") || strings.ContainsAny(rel, `/\`) {
			continue
		}
		var m javaMarkerFile
		if b, e := os.ReadFile(filepath.Join(home, javaMarker)); e == nil && json.Unmarshal(b, &m) == nil {
			return true, m.Source
		}
	}
	return false, ""
}

// InstalledJavas is the Java list with sizes and which ones MCL installed.
func (s *Service) InstalledJavas(ctx context.Context) []InstalledJava {
	return s.InstalledJavaDetails(ctx, s.JavaList(ctx))
}

// InstalledJavaDetails adds display metadata to an existing discovery result.
// It does not inspect environment/registry paths or restart Java discovery.
func (s *Service) InstalledJavaDetails(ctx context.Context, known []JavaInfo) []InstalledJava {
	out := make([]InstalledJava, 0, len(known))
	for _, j := range known {
		if ctx.Err() != nil {
			break
		}
		home := javaHome(j.Path)
		m, src := s.isManagedJava(home)
		out = append(out, InstalledJava{JavaInfo: j, Home: home, Size: dirSize(ctx, home), Managed: m, Source: src})
	}
	return out
}

// UninstallJava removes a runtime MCL installed; any other is refused.
func (s *Service) UninstallJava(home string) error {
	home = filepath.Clean(home)
	if ok, _ := s.isManagedJava(home); !ok {
		return errors.New("只能卸载 YoloMCL 安装的 Java")
	}
	s.mu.Lock()
	for _, g := range s.running {
		if g != nil && g.cmd != nil && g.cmd.Path != "" && strings.HasPrefix(strings.ToLower(filepath.Clean(g.cmd.Path)), strings.ToLower(home)) {
			s.mu.Unlock()
			return errors.New("这个 Java 正在运行游戏，请先关闭游戏")
		}
	}
	s.mu.Unlock()
	tmp := home + ".del-" + newID()[:6]
	if e := os.Rename(home, tmp); e != nil {
		return fmt.Errorf("卸载失败，Java 可能正在使用: %w", e)
	}
	dirSizes.Delete(home)
	go os.RemoveAll(tmp)
	_, _ = s.ScanJava(context.Background(), false, nil)
	return nil
}

func javaFileName(p string) string {
	if strings.HasPrefix(filepath.Base(p), ".download") {
		return "Java 压缩包"
	}
	return filepath.Base(p)
}

// InstalledJava is a found runtime, with what MCL knows about it.
type InstalledJava struct {
	JavaInfo
	Home    string `json:"home"`
	Size    int64  `json:"size"`
	Managed bool   `json:"managed"`
	Source  string `json:"source,omitempty"`
}
