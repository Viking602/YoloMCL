package launcher

import (
	"os"
	"path/filepath"
	"testing"
)

func TestMigrateDataRoot(t *testing.T) {
	base := t.TempDir()
	old, nw := filepath.Join(base, "MCL"), filepath.Join(base, "YoloMCL")
	for _, f := range []string{"config.json", "cache/a.bin", "logs/x.log", "runtime/zulu-21/bin/java.exe", "runtimes/java-17-x/release"} {
		p := filepath.Join(old, f)
		_ = os.MkdirAll(filepath.Dir(p), 0o755)
		_ = os.WriteFile(p, []byte(f), 0o644)
	}
	if e := MigrateDataRoot(old, nw); e != nil {
		t.Fatal(e)
	}
	for _, f := range []string{"config.json", "cache/a.bin", "logs/x.log"} {
		if b, e := os.ReadFile(filepath.Join(nw, f)); e != nil || string(b) != f {
			t.Fatalf("%s not migrated: %v", f, e)
		}
	}
	for _, f := range []string{"runtime", "runtimes"} {
		if _, e := os.Stat(filepath.Join(nw, f)); e == nil {
			t.Fatalf("%s should not be migrated", f)
		}
	}
	if _, e := os.Stat(old); !os.IsNotExist(e) {
		t.Fatal("old root should be removed")
	}
	// second run is a no-op and never touches an existing new root
	_ = os.WriteFile(filepath.Join(nw, "config.json"), []byte("new"), 0o644)
	_ = os.MkdirAll(old, 0o755)
	_ = os.WriteFile(filepath.Join(old, "config.json"), []byte("old"), 0o644)
	_ = MigrateDataRoot(old, nw)
	if b, _ := os.ReadFile(filepath.Join(nw, "config.json")); string(b) != "new" {
		t.Fatal("existing root overwritten")
	}
}
