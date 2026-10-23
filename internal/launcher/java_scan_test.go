package launcher

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"
)

func javaFixture(t *testing.T, home, version string) string {
	t.Helper()
	name := "java"
	if runtime.GOOS == "windows" {
		name += ".exe"
	}
	path := filepath.Join(home, "bin", name)
	writeTestFile(t, path, []byte("metadata-only test fixture; never execute"))
	writeTestFile(t, filepath.Join(home, "release"), []byte("JAVA_VERSION=\""+version+"\"\nIMPLEMENTOR=\"Test Runtime\"\nOS_ARCH=\"x86_64\"\n"))
	return canonicalJava(path)
}

func TestJavaReleaseVersionsWithoutStartingJVM(t *testing.T) {
	for _, item := range []struct {
		version string
		major   int
	}{{"1.8.0_452", 8}, {"17.0.15", 17}, {"21+35", 21}, {"26-ea+15", 26}} {
		t.Run(item.version, func(t *testing.T) {
			path := javaFixture(t, t.TempDir(), item.version)
			j, e := inspectJavaRelease(context.Background(), path)
			if e != nil || j.Major != item.major || j.Version != item.version || j.Vendor != "Test Runtime" || j.Architecture != "x86_64" {
				t.Fatalf("release metadata: %+v, %v", j, e)
			}
		})
	}
}

func TestJavaDiscoveryDedupPortableCacheAndRefresh(t *testing.T) {
	s := testService(t)
	disk := s.dir
	common := filepath.Join(disk, "installed")
	j8 := javaFixture(t, filepath.Join(common, "jdk8"), "1.8.0_452")
	j21 := javaFixture(t, filepath.Join(common, "jdk21"), "21+35")
	portableHome := filepath.Join(disk, "games", "unusual", "nested", "folder", "portable-runtime")
	portable := javaFixture(t, portableHome, "25.0.2")
	explicit := []string{j21, j21, filepath.Join(disk, "missing-java")}
	if runtime.GOOS == "windows" {
		javaw := filepath.Join(filepath.Dir(j21), "javaw.exe")
		writeTestFile(t, javaw, []byte("test fixture"))
		explicit = append(explicit, javaw)
	}
	items, e := s.scanJava(context.Background(), explicit, []string{common, common}, nil, nil)
	if e != nil || len(items) != 2 || items[0].Path != j21 || items[1].Path != j8 {
		t.Fatalf("quick scan/dedup: %+v, %v", items, e)
	}
	var final JavaScanProgress
	items, e = s.scanJava(context.Background(), explicit, []string{common}, []string{disk}, func(p JavaScanProgress) { final = p })
	if e != nil || len(items) != 3 || items[0].Path != portable || final.Running || final.Visited == 0 || final.Phase != "正在扫描整合包与启动器运行时" {
		t.Fatalf("portable scan/progress: %+v / %+v, %v", items, final, e)
	}
	if cached := s.CachedJavaList(); len(cached) != 3 || cached[0].Path != portable {
		t.Fatalf("cache did not preserve portable Java: %+v", cached)
	}
	// The next quick scan must retain a full-scan discovery and read its new
	// release metadata if the runtime was upgraded in place.
	javaFixture(t, portableHome, "26.0.2")
	hints, _ := s.javaHints()
	items, e = s.scanJava(context.Background(), hints, nil, nil, nil)
	if e != nil {
		t.Fatal(e)
	}
	matched := 0
	for _, j := range items {
		if j.Path == portable {
			matched++
			if j.Version != "26.0.2" {
				t.Fatalf("stale cached version: %+v", j)
			}
		}
	}
	if matched != 1 {
		t.Fatalf("cached portable runtime not refreshed exactly once: %d", matched)
	}
	if e = os.Remove(portable); e != nil {
		t.Fatal(e)
	}
	for _, j := range s.CachedJavaList() {
		if j.Path == portable {
			t.Fatal("deleted runtime remained in cache results")
		}
	}
}

func TestJavaScanCancellationPreservesPreviousCache(t *testing.T) {
	s := testService(t)
	path := javaFixture(t, s.runtimeDir(), "21.0.10")
	if _, e := s.scanJava(context.Background(), []string{path}, nil, nil, nil); e != nil {
		t.Fatal(e)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	started := time.Now()
	var final JavaScanProgress
	_, e := s.scanJava(ctx, []string{path}, []string{filepath.Dir(path)}, []string{t.TempDir()}, func(p JavaScanProgress) { final = p })
	if !errors.Is(e, context.Canceled) || time.Since(started) > time.Second || final.Running {
		t.Fatalf("scan did not stop promptly: %+v, %v", final, e)
	}
	if cached := s.CachedJavaList(); len(cached) != 1 || cached[0].Path != path {
		t.Fatalf("cancelled scan replaced the previous cache: %+v", cached)
	}
}
