package nativeui

import (
	"testing"

	"github.com/Viking602/YoloMCL/internal/launcher"
)

func TestPartialJavaRefreshKeepsKnownInstallations(t *testing.T) {
	known := []launcher.JavaInfo{{Path: "java21/bin/java", Version: "21.0.9"}, {Path: "java17/bin/java", Version: "17.0.15"}}
	if got := mergeJavaScanRows(known, nil); len(got) != 2 {
		t.Fatal("partial refresh blanked Java list")
	}
	found := []launcher.JavaInfo{{Path: known[0].Path, Version: "21.0.10"}, {Path: "java25/bin/java", Version: "25.0.1"}}
	got := mergeJavaScanRows(known, found)
	if len(got) != 3 || got[0].Version != "21.0.10" || got[1].Path != known[1].Path || got[2].Path != found[1].Path {
		t.Fatalf("lost/doubled Java during refresh: %+v", got)
	}
	if known[0].Version != "21.0.9" {
		t.Fatal("mutated previous snapshot")
	}
}
