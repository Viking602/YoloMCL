package launcher

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestManagedJavaAndUninstall(t *testing.T) {
	dir := t.TempDir()
	s := &Service{dir: dir, running: map[string]*runningGame{}}
	mine := filepath.Join(dir, "runtime", "java-runtime-delta")
	os.MkdirAll(filepath.Join(mine, "bin"), 0o755)
	atomicJSON(filepath.Join(mine, javaMarker), javaMarkerFile{Source: "mojang", Major: 21})
	other := filepath.Join(t.TempDir(), "jdk-21")
	os.MkdirAll(other, 0o755)
	unmarked := filepath.Join(dir, "runtime", "custom")
	os.MkdirAll(unmarked, 0o755)
	if ok, src := s.isManagedJava(mine); !ok || src != "mojang" {
		t.Fatal("own runtime not managed")
	}
	for _, p := range []string{other, unmarked, filepath.Join(dir, "runtime")} {
		if ok, _ := s.isManagedJava(p); ok {
			t.Fatalf("%s should not be managed", p)
		}
		if s.UninstallJava(p) == nil {
			t.Fatalf("uninstall of %s allowed", p)
		}
	}
	if e := s.UninstallJava(mine); e != nil {
		t.Fatal(e)
	}
	if _, e := os.Stat(mine); !os.IsNotExist(e) {
		t.Fatal("still there")
	}
}

func TestNeedJavaError(t *testing.T) {
	var e error = &NeedJavaError{Major: 21}
	var n *NeedJavaError
	if !errors.As(e, &n) || n.Major != 21 || e.Error() != "需要 Java 21，还没有安装" {
		t.Fatal(e)
	}
}
