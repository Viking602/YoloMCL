package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestArchitectureRejectsBackendUIAndPluginApplicationDependencies(t *testing.T) {
	t.Chdir(t.TempDir())
	for _, tc := range []struct {
		path, imported string
		allowed        bool
	}{
		{"internal/launcher/check.go", "github.com/egoist/mygo/ui", false},
		{"internal/launcher/check.go", modulePath + "/internal/nativeui", false},
		{"plugins/startup/check.go", modulePath + "/internal/launcher", false},
		{"internal/nativeui/check.go", modulePath + "/internal/launcher", true},
		{"internal/launcher/check.go", modulePath + "/internal/distribution", true},
	} {
		if err := os.MkdirAll(filepath.Dir(tc.path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(tc.path, []byte("package fixture\nimport \""+tc.imported+"\"\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		err := checkArchitecture([]string{tc.path})
		if (err == nil) != tc.allowed {
			t.Fatalf("dependency %s -> %s: %v", tc.path, tc.imported, err)
		}
	}
}
