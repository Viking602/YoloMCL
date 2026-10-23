package launcher

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"testing"
)

func TestCachedSystemJavaSurvivesMinecraftScopeChange(t *testing.T) {
	s := testService(t)
	outside := javaFixture(t, t.TempDir(), "21.0.10")
	missing := filepath.Join(t.TempDir(), "missing", "bin", "java")
	if err := atomicJSON(filepath.Join(s.dir, "cache", "java-list.json"), []JavaInfo{{Path: outside, Major: 21, Version: "21.0.10"}, {Path: outside, Major: 21}, {Path: missing, Major: 17}}); err != nil {
		t.Fatal(err)
	}
	if got := s.CachedJavaList(); len(got) != 1 || got[0].Path != outside {
		t.Fatalf("known system installation discarded: %+v", got)
	}
	explicit, roots := s.javaHints()
	got, err := s.scanJavaHints(context.Background(), false, explicit, roots, nil)
	if err != nil || len(got) != 1 || got[0].Path != outside {
		t.Fatalf("refresh erased known Java: %+v, %v", got, err)
	}
}

func TestSystemJavaDiscoveryFindsEnvironmentWithoutWalkingParents(t *testing.T) {
	s := testService(t)
	home := t.TempDir()
	java := javaFixture(t, filepath.Join(home, "jdk21"), "21.0.10")
	pathJava := javaFixture(t, filepath.Join(home, "jdk17"), "17.0.15")
	javaFixture(t, filepath.Join(home, "unrelated", "nested", "jdk25"), "25.0.1")
	env := map[string]string{"JAVA_HOME": `"` + filepath.Dir(filepath.Dir(java)) + `"`, "JDK_HOME": filepath.Dir(filepath.Dir(java)), "PATH": filepath.Dir(pathJava) + string(os.PathListSeparator) + "." + string(os.PathListSeparator) + "relative-folder"}
	candidates := environmentJavaPaths(func(k string) string { return env[k] }, runtime.GOOS)
	got, err := s.scanJava(context.Background(), candidates, nil, nil, nil)
	if err != nil || len(got) != 2 {
		t.Fatalf("environment Java not detected/deduplicated: %+v, %v", got, err)
	}
	if got[0].Path != java || got[1].Path != pathJava {
		t.Fatalf("wrong executables: %+v", got)
	}
}

func TestSystemJavaLocationsInspectOnlyImmediateInstallations(t *testing.T) {
	s := testService(t)
	base := t.TempDir()
	java := javaFixture(t, filepath.Join(base, "jdk21"), "21.0.10")
	javaFixture(t, filepath.Join(base, "documents", "private", "jdk26"), "26.0.1")
	javaFixture(t, filepath.Join(base, "unrelated-app"), "25.0.2")
	paths := installedJavaPaths(context.Background(), []javaInstallLocation{{base, "jdk"}, {base, "jdk"}}, runtime.GOOS)
	got, err := s.scanJava(context.Background(), paths, nil, nil, nil)
	if err != nil || len(got) != 1 || got[0].Path != java {
		t.Fatalf("install directory bounds: %+v, %v", got, err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if got := installedJavaPaths(ctx, []javaInstallLocation{{base, ""}}, runtime.GOOS); len(got) != 0 {
		t.Fatal("cancelled discovery read directories")
	}
}

func TestSystemJavaPlatformLocations(t *testing.T) {
	home := t.TempDir()
	programFiles := filepath.Join(home, "Program Files")
	getenv := func(k string) string {
		if k == "ProgramFiles" {
			return programFiles
		}
		return ""
	}
	for _, goos := range []string{"windows", "darwin", "linux"} {
		locations := javaInstallLocations(goos, home, getenv)
		if len(locations) == 0 {
			t.Fatalf("no Java locations for %s", goos)
		}
		for _, loc := range locations {
			if loc.path == home || loc.path == programFiles || loc.path == "/" || loc.path == "/opt" || loc.path == "/Applications" {
				t.Fatalf("overbroad discovery root: %s", loc.path)
			}
		}
		if !slices.Contains(locations, javaInstallLocation{filepath.Join(home, ".jdks"), ""}) {
			t.Fatalf("user Java installation directory missing for %s", goos)
		}
	}
	macHome := filepath.Join(home, "temurin-21.jdk")
	if !slices.Contains(javaHomePaths(macHome, "darwin"), filepath.Join(macHome, "Contents", "Home", "bin", "java")) {
		t.Fatal("macOS JDK bundle missed")
	}
	if got := environmentJavaPaths(func(k string) string {
		if k == "PATH" {
			return "/usr/bin"
		}
		return ""
	}, "darwin"); slices.Contains(got, "/usr/bin/java") {
		t.Fatal("macOS installer stub was treated as Java")
	}
}

func TestScanJavaCombinesSystemAndManagedRuntimes(t *testing.T) {
	s := testService(t)
	managed := javaFixture(t, filepath.Join(s.runtimeDir(), "jdk17"), "17.0.15")
	home := t.TempDir()
	system := javaFixture(t, filepath.Join(home, "jdk21"), "21.0.10")
	t.Setenv("JAVA_HOME", filepath.Dir(filepath.Dir(system)))
	got, err := s.ScanJava(context.Background(), false, nil)
	if err != nil {
		t.Fatal(err)
	}
	found := map[string]bool{}
	for _, j := range got {
		found[j.Path] = true
	}
	if !found[managed] || !found[system] {
		t.Fatalf("system/managed discovery not combined: %+v", got)
	}
}
