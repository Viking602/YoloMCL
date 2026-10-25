package nativeui

import (
	"archive/zip"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"

	"github.com/Viking602/YoloMCL/internal/launcher"
)

func TestPackExportDialogChoicesAndLayout(t *testing.T) {
	if err := loadAssets(); err != nil {
		t.Fatal(err)
	}
	for _, language := range []string{LangZH, LangEN} {
		for _, width := range []int{520, 820} {
			t.Run(fmt.Sprintf("%s-%d", language, width), func(t *testing.T) {
				preview := &launcher.PackExportPreview{Name: "Pack", Minecraft: "1.21.1", Loader: "neoforge", LoaderVersion: "21.1.243", Entries: []launcher.PackExportEntry{
					{Name: "config", Folder: true, Selected: true}, {Name: "mods", Folder: true, Selected: true}, {Name: "saves", Folder: true},
				}}
				a := &App{packExport: &packExportState{preview: preview, entries: slices.Clone(preview.Entries), format: "modrinth", name: "My Pack", packVersion: "1.0.0"}}
				a.OverrideLanguage(language)
				tt := ui.NewTester(func(c *ui.Context) {
					c.SetTheme(theme)
					ui.Column(c).Fill().Padding(24).Gap(16).Background(bg).Children(func() { a.packExportDialog(c); a.packExportFooter(c) })
				}, width, 720)
				clickUI(t, tt, "导出内容 saves")
				clickUI(t, tt, "导出内容 mods")
				if !a.packExport.entries[2].Selected || a.packExport.entries[1].Selected {
					t.Fatal("content choices did not toggle independently")
				}
				clickUI(t, tt, "按钮文字 export-defaults")
				if a.packExport.entries[2].Selected || !a.packExport.entries[1].Selected {
					t.Fatal("reset lost the original defaults")
				}
				for _, format := range []string{"curseforge", "modrinth", "curseforge"} {
					name := "Modrinth"
					if format == "curseforge" {
						name = "CurseForge"
					}
					clickUI(t, tt, name)
					tt.Frame()
					if a.packExport.format != format {
						t.Fatal("format selection jumped back")
					}
				}
				button, ok := tt.Find("按钮文字 export-save")
				if !ok || button.W <= 0 || button.X < 0 || button.X+button.W > float32(width) || button.Y+button.H > 720 {
					t.Fatalf("export action is clipped: %+v", button)
				}
				snapshotUI(t, tt, fmt.Sprintf("pack-export-%s-%d", language, width))
			})
		}
	}
}

func TestNativePackExportFlow(t *testing.T) {
	a, tt, _ := worldsTester(t)
	r := *a.root()
	a.form.VerJVM = "-Dunsaved.instance.setting=true"
	previousForm := a.form
	a.openPackExport(r.ID, r.SelectedVersion)
	if a.form != previousForm {
		t.Fatal("opening export discarded unsaved instance settings")
	}
	waitUI(t, tt, func() bool { return a.packExport != nil && a.packExport.preview != nil && !a.dialogPending() })
	tt.Advance(300 * time.Millisecond)
	clickUI(t, tt, "CurseForge")
	clickUI(t, tt, "导出内容 saves")
	snapshotUI(t, tt, "pack-export-native")
	dest := filepath.Join(t.TempDir(), "share.zip")
	old := savePackDialog
	t.Cleanup(func() { a.bgWG.Wait(); savePackDialog = old })
	var options mygo.SaveDialogOptions
	savePackDialog = func(o mygo.SaveDialogOptions) (string, error) { options = o; return dest, nil }
	clickUI(t, tt, "按钮文字 export-save")
	waitUI(t, tt, func() bool {
		// A small archive can finish before the command's completion is
		// posted back to the UI. Wait for both halves of the operation.
		if a.pendingOp("exportPack") || a.dialogOpen || a.page != "tasks" {
			return false
		}
		a.snapshot = a.service.Snapshot()
		for _, j := range a.snapshot.Jobs {
			if j.TitleKey == "pack.export" {
				return j.State != "running"
			}
		}
		return false
	})
	if !strings.HasSuffix(options.DefaultPath, ".zip") || options.Filters[0].Name != "CurseForge" {
		t.Fatal("save dialog used the wrong format")
	}
	if a.dialogOpen || a.page != "tasks" {
		t.Fatal("export did not enter the task flow")
	}
	z, err := zip.OpenReader(dest)
	if err != nil {
		t.Fatal(err)
	}
	defer z.Close()
	wanted := map[string]bool{"manifest.json": false, "overrides/saves/world-one/level.dat": false, "overrides/saves/world-two/level.dat": false}
	for _, f := range z.File {
		if _, ok := wanted[f.Name]; ok {
			wanted[f.Name] = true
		}
	}
	for name, ok := range wanted {
		if !ok {
			t.Fatalf("native export missing %s", name)
		}
	}
	wantedFile, err := os.Stat(dest)
	if err != nil {
		t.Fatal(err)
	}
	for _, j := range a.snapshot.Jobs {
		if j.TitleKey != "pack.export" {
			continue
		}
		if j.TargetRootID != r.ID || j.TargetVersion != r.SelectedVersion {
			t.Fatalf("wrong export target: %+v", j)
		}
		if resultFile, err := os.Stat(j.ResultRef); err != nil || !os.SameFile(wantedFile, resultFile) {
			t.Fatalf("export result points to a different file: %q (%v)", j.ResultRef, err)
		}
	}
	tt.Advance(300 * time.Millisecond)
	tt.Frame()
	if _, ok := tt.Find("按钮文字 job-export-open-" + a.snapshot.Jobs[0].ID); !ok {
		t.Fatal("completed export lacks a reveal-file action")
	}
}

func TestPackExportIgnoresDismissedSaveDialog(t *testing.T) {
	a, tt, _ := worldsTester(t)
	r := *a.root()
	a.openPackExport(r.ID, r.SelectedVersion)
	waitUI(t, tt, func() bool { return a.packExport != nil && a.packExport.preview != nil && !a.dialogPending() })
	old := savePackDialog
	opened, finish := make(chan struct{}), make(chan struct{})
	savePackDialog = func(mygo.SaveDialogOptions) (string, error) {
		close(opened)
		<-finish
		return filepath.Join(t.TempDir(), "stale.mrpack"), nil
	}
	t.Cleanup(func() { a.bgWG.Wait(); savePackDialog = old })
	a.savePackExport(a.packExport)
	<-opened
	a.openDialog("account")
	close(finish)
	waitUI(t, tt, func() bool { return len(a.events) == 0 && !a.pendingOp("exportPack") })
	a.bgWG.Wait()
	tt.Frame()
	if a.dialog != "account" || len(a.service.Snapshot().Jobs) != 0 {
		t.Fatal("late save completion started an export or closed a newer dialog")
	}
}
