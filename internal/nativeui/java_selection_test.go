package nativeui

import (
	"fmt"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"

	"github.com/Viking602/YoloMCL/internal/launcher"
)

func TestLaunchAppliesVisibleJavaBeforeStarting(t *testing.T) {
	a, tt, _ := worldsTester(t)
	if _, err := a.service.AddOffline("Java_Tester"); err != nil {
		t.Fatal(err)
	}
	a.snapshot = a.service.Snapshot()
	a.setInstTab("settings")
	r := a.root()
	// A missing executable makes the launch stop at Java validation, before
	// preparing any game files. It must validate this selection, not auto-pick.
	selected := filepath.Join(t.TempDir(), "chosen-java", "bin", "java")
	a.java = []launcher.JavaInfo{{Path: selected, Version: "26.0.2", Major: 26}}
	tt.Frame()
	clickUI(t, tt, a.tr().RootCfgVerJavaLabel)
	tt.Advance(250 * time.Millisecond)
	tt.Key(0, ui.KeyEnd)
	tt.Key(0, ui.KeyEnter)
	if a.form.VerJava != selected || !a.versionDraftDirty() {
		t.Fatal("picker did not create the Java selection draft")
	}
	clickUI(t, tt, a.tr().HomeLaunchBtn)
	waitUI(t, tt, func() bool {
		a.applySnapshot(a.service.Snapshot(), a.selectionEpoch.Load())
		return len(a.snapshot.Jobs) == 1 && a.snapshot.Jobs[0].State == "failed" && len(a.pendingRequests) == 0
	})
	if got := a.root().VersionConfigs[r.SelectedVersion].JavaPath; got != selected {
		t.Fatalf("launch did not save the visible Java first: %q", got)
	}
	if !strings.Contains(a.snapshot.Jobs[0].Message, "所选 Java 路径无效") || a.dialog == "javaGet" {
		t.Fatalf("launch ignored the visible Java selection: %+v", a.snapshot.Jobs[0])
	}
	if a.versionDraftDirty() {
		t.Fatal("launch left its applied settings marked unsaved")
	}
}

func TestLaunchDoesNotStartWhenRuntimeSaveFails(t *testing.T) {
	a, tt, _ := worldsTester(t)
	if _, err := a.service.AddOffline("Java_Tester"); err != nil {
		t.Fatal(err)
	}
	a.snapshot = a.service.Snapshot()
	a.setInstTab("settings")
	a.form.VerJava = filepath.Join(t.TempDir(), "chosen-java")
	a.form.VerJVM = `-Dunfinished="`
	a.launch()
	waitUI(t, tt, func() bool { return len(a.pendingRequests) == 0 && a.errorMessage })
	if len(a.service.Snapshot().Jobs) != 0 || a.dialog == "javaGet" || !a.versionDraftDirty() {
		t.Fatal("failed settings validation launched with stale settings or discarded the draft")
	}
}

func TestAnotherLaunchAppliesJavaBeforeAccountChooser(t *testing.T) {
	a, tt, _ := worldsTester(t)
	a.setInstTab("settings")
	r := *a.root()
	selected := filepath.Join(t.TempDir(), "selected-java")
	a.form.VerJava = selected
	a.openLaunchAgain(r)
	waitUI(t, tt, func() bool { return a.dialog == "launchAgain" && len(a.pendingRequests) == 0 })
	if got := a.root().VersionConfigs[r.SelectedVersion].JavaPath; got != selected {
		t.Fatalf("another launch ignored the Java selection: %q", got)
	}
	a.closeDialog()
	if a.form.VerJava != selected || a.versionDraftDirty() {
		t.Fatal("account chooser lost the applied runtime settings")
	}
}

func TestSelectedJavaSuppressesDownloadSuggestion(t *testing.T) {
	a, tt, _ := worldsTester(t)
	a.setInstTab("settings")
	a.root().Versions[0].JavaVersion = 17
	a.java = []launcher.JavaInfo{{Path: "selected-java", Version: "26.0.2", Major: 26}}
	download := fmt.Sprintf(a.L("instance_settings.download_java"), 17)
	tt.Frame()
	if !tt.HasText(download) {
		t.Fatal("automatic mode should offer the missing Java version")
	}
	for _, source := range []string{"version", "directory", "global"} {
		a.form.VerJava, a.root().JavaPath, a.settings.JavaPath = "", "", ""
		a.settings.UseRootSettings = source == "directory"
		switch source {
		case "version":
			a.form.VerJava = "selected-java"
		case "directory":
			a.root().JavaPath = "selected-java"
		case "global":
			a.settings.JavaPath = "selected-java"
		}
		tt.Frame()
		if tt.HasText(download) {
			t.Fatalf("%s selection still suggests an unnecessary download", source)
		}
	}
}

func TestRuntimeJavaDisplayFollowsLaunchPrecedence(t *testing.T) {
	java17 := launcher.JavaInfo{Path: "runtimes/java17/bin/java", Version: "17.0.15", Major: 17}
	java21 := launcher.JavaInfo{Path: "runtimes/java21/bin/java", Version: "21.0.10", Major: 21}
	java26 := launcher.JavaInfo{Path: "runtimes/java26/bin/java", Version: "26.0.2", Major: 26}
	for _, tc := range []struct {
		name, version, directory, global, want string
		rootDefaults                           bool
	}{
		{"version", java26.Path, java21.Path, java17.Path, java26.Path, true},
		{"directory", "", java21.Path, java26.Path, java21.Path, true},
		{"global", "", java21.Path, java26.Path, java26.Path, false},
		{"directory-follows-global", "", "", java26.Path, java26.Path, true},
		{"automatic", "", "", "", java17.Path, false},
		{"unscanned-selection", "portable/java/bin/java", "", "", "portable/java/bin/java", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			a := &App{settings: launcher.Settings{JavaPath: tc.global, UseRootSettings: tc.rootDefaults}, java: []launcher.JavaInfo{java26, java21, java17}}
			v := launcher.VersionSummary{ID: "pack", JavaVersion: 17, JavaPath: tc.version}
			r := launcher.Instance{Root: launcher.Root{JavaPath: tc.directory}, Versions: []launcher.VersionSummary{v}}
			got, need := a.ovJava(r, v)
			if got.Path != tc.want || need != 17 {
				t.Fatalf("display disagrees with launch: %+v, need %d", got, need)
			}
			a.snapshot.Instances = []launcher.Instance{r}
			use, _ := a.javaUse()
			if use[strings.ToLower(tc.want)] != 1 || len(use) != 1 {
				t.Fatalf("usage list ignored the effective runtime: %v", use)
			}
		})
	}
}
