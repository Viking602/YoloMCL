package launcher

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestExplicitJavaTakesPrecedenceOverRecommendedMajor(t *testing.T) {
	java, err := exec.LookPath("java")
	if err != nil {
		t.Skip("Java is unavailable")
	}
	info, err := inspectJava(context.Background(), java)
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name     string
		root     Root
		settings Settings
	}{
		{"version", Root{SelectedVersion: "pack", JavaPath: "unused-directory-java", VersionConfigs: map[string]VersionConfig{"pack": {JavaPath: java}}}, Settings{JavaPath: "unused-global-java", UseRootSettings: true}},
		{"directory", Root{SelectedVersion: "pack", JavaPath: java}, Settings{JavaPath: "unused-global-java", UseRootSettings: true}},
		{"global", Root{SelectedVersion: "pack", JavaPath: "ignored-directory-java"}, Settings{JavaPath: java}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s := testService(t)
			tc.settings.MemoryMB = 512
			if err := s.SaveSettings(tc.settings); err != nil {
				t.Fatal(err)
			}
			fakeHTTP(t, s, nil)
			for _, major := range []int{8, info.Major, info.Major + 1} {
				got, err := s.ensureJava(context.Background(), tc.root, major, nil)
				if err != nil || got != java {
					t.Fatalf("manual Java %d with recommended Java %d: %q, %v", info.Major, major, got, err)
				}
			}
			if _, err := os.Stat(filepath.Join(s.dir, "cache", "java-list.json")); !os.IsNotExist(err) {
				t.Fatal("an explicit Java selection unnecessarily scanned other runtimes")
			}
		})
	}
}

func TestInvalidSelectedJavaDoesNotRequestDownload(t *testing.T) {
	s := testService(t)
	r := Root{SelectedVersion: "pack", VersionConfigs: map[string]VersionConfig{"pack": {JavaPath: filepath.Join(t.TempDir(), "missing-java")}}}
	_, err := s.ensureJava(context.Background(), r, 17, nil)
	var need *NeedJavaError
	if err == nil || errors.As(err, &need) || !strings.Contains(err.Error(), "所选 Java 路径无效") {
		t.Fatalf("invalid manual selection should be reported directly: %v", err)
	}
	if _, err := os.Stat(filepath.Join(s.dir, "cache", "java-list.json")); !os.IsNotExist(err) {
		t.Fatal("invalid manual selection silently fell back to automatic detection")
	}
}

func TestAutomaticJavaStillRequestsMissingMajor(t *testing.T) {
	s := testService(t)
	_, err := s.ensureJava(context.Background(), Root{}, 999, nil)
	var need *NeedJavaError
	if !errors.As(err, &need) || need.Major != 999 {
		t.Fatalf("automatic mode lost its install prompt: %v", err)
	}
}
