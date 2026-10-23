package nativeui

import (
	"bytes"
	"encoding/json"
	"image"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"

	"github.com/Viking602/YoloMCL/internal/launcher"
)

func worldsTester(t *testing.T) (*App, *ui.Tester, string) {
	t.Helper()
	t.Setenv("MCL_DEVELOPMENT", "")
	s, err := launcher.New(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	mc := t.TempDir()
	verDir := filepath.Join(mc, "versions", "1.21.1")
	if err = os.MkdirAll(verDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(filepath.Join(verDir, "1.21.1.json"), []byte(`{"id":"1.21.1","mainClass":"net.minecraft.client.main.Main","type":"release"}`), 0644); err != nil {
		t.Fatal(err)
	}
	r, err := s.AddRoot(mc, "UI regression fixture")
	if err != nil {
		t.Fatal(err)
	}
	if err = s.Select(r.ID, "", "1.21.1"); err != nil {
		t.Fatal(err)
	}
	saves, err := s.InstanceFolder(r.ID, "1.21.1", "saves")
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"world-one", "world-two"} {
		p := filepath.Join(saves, name)
		if err = os.MkdirAll(p, 0755); err != nil {
			t.Fatal(err)
		}
		for _, file := range []string{"level.dat", "blocks.dat"} {
			if err = os.WriteFile(filepath.Join(p, file), []byte("before-backup"), 0644); err != nil {
				t.Fatal(err)
			}
		}
	}
	a, err := New(s)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { a.cancel(); a.bgWG.Wait() })
	a.OverrideLanguage("zh-CN")
	a.instTab = "worlds"
	tt := ui.NewTester(a.View, 1536, 1024)
	waitUI(t, tt, func() bool { return !a.wv.loading && len(a.wv.list) == 2 && len(a.imagePending) == 0 })
	tt.Advance(400 * time.Millisecond)
	return a, tt, saves
}

// Hold requests pending to cover both fast local commands and slow network
// responses. The surrounding controls must paint exactly as they did before.
func TestPendingWorldActionsDoNotFlashChrome(t *testing.T) {
	a, tt, _ := worldsTester(t)
	r := a.root()
	regions := []image.Rectangle{image.Rect(0, 0, 1536, 42), image.Rect(0, 42, 340, 1024), image.Rect(366, 65, 1510, 185)}
	baseline := image.NewRGBA(tt.Image().Bounds())
	copy(baseline.Pix, tt.Image().Pix)
	for _, method := range []string{"backupWorld", "restoreWorld", "deleteWorldBackup", "removeMod", "addLocalMods", "checkPackUpdate"} {
		a.pendingRequests = map[string]pendingRequest{"held": {method: method, fields: map[string]string{"rootId": r.ID, "version": r.SelectedVersion, "name": "world-one"}}}
		a.pending = map[string]int{method: 1}
		tt.Frame()
		tt.Advance(200 * time.Millisecond)
		frame := tt.Image()
		for _, region := range regions {
			for y := region.Min.Y; y < region.Max.Y; y++ {
				start, end := baseline.PixOffset(region.Min.X, y), baseline.PixOffset(region.Max.X, y)
				if !bytes.Equal(baseline.Pix[start:end], frame.Pix[start:end]) {
					t.Fatalf("%s changed unrelated chrome in %v, row %d", method, region, y)
				}
			}
		}
		if a.instancePending("launch") || a.dialogPending() || a.worldBusy("world-two") {
			t.Fatalf("%s blocks an unrelated action", method)
		}
	}
	a.pendingRequests, a.pending = nil, nil
}

func TestCommandsDeduplicateAndKeepNewDialogOpen(t *testing.T) {
	a, tt, _ := worldsTester(t)
	a.openDialog("account")
	called := false
	payload := map[string]string{"name": "FixturePlayer"}
	for i := 0; i < 2; i++ {
		a.execute("offlineLogin", payload, func(json.RawMessage) { called = true; a.closeDialog() })
	}
	if a.pending["offlineLogin"] != 1 || len(a.pendingRequests) != 1 || !a.dialogPending() {
		t.Fatal("duplicate request was accepted or dialog lost local pending state")
	}
	a.closeDialog()
	a.openDialog("account") // even reopening the same dialog is a new context
	if a.dialogPending() {
		t.Fatal("old request dimmed the new dialog")
	}
	waitUI(t, tt, func() bool { return len(a.pendingRequests) == 0 })
	if called || !a.dialogOpen || a.dialog != "account" || len(a.snapshot.Accounts) != 1 {
		t.Fatal("old completion closed a new dialog or failed to apply saved data")
	}
}

func TestVersionSaveCompletesAfterLeavingDialog(t *testing.T) {
	a, tt, _ := worldsTester(t)
	r := a.root()
	a.openDialog("version")
	a.form.ConfigVersion, a.form.VerName = r.SelectedVersion, "Renamed pack"
	a.form.VerMemAuto, a.form.VerMemory = false, "4096"
	a.saveVersionConfig(r.ID)
	a.closeDialog()
	a.openDialog("account")
	waitUI(t, tt, func() bool { return len(a.pendingRequests) == 0 })
	vc := a.root().VersionConfigs[r.SelectedVersion]
	if vc.DisplayName != "Renamed pack" || vc.MemoryMB != 4096 || a.dialog != "account" || !a.dialogOpen {
		t.Fatalf("save was interrupted or affected the new dialog: %+v, %s", vc, a.dialog)
	}
}

func TestNativeWorldBackupRestoreDeleteFlow(t *testing.T) {
	a, tt, saves := worldsTester(t)
	waitIdle := func() {
		t.Helper()
		waitUI(t, tt, func() bool {
			a.applySnapshot(a.service.Snapshot(), a.selectionEpoch.Load())
			a.watchWorldJobs()
			return len(a.pendingRequests) == 0 && !a.worldBusy("") && !a.wv.loading && !a.wv.backupsLoading
		})
		for _, job := range a.snapshot.Jobs {
			if job.State != "completed" {
				t.Fatalf("world operation failed: %+v", job)
			}
		}
	}
	clickUI(t, tt, "按钮文字 world-backup-world-one")
	waitIdle()
	clickUI(t, tt, "按钮文字 world-restore-world-one")
	waitIdle()
	if len(a.wv.backups) != 1 {
		t.Fatalf("completed backup did not appear: %+v", a.wv)
	}
	file := a.wv.backups[0].File
	p := filepath.Join(saves, "world-one", "blocks.dat")
	if err := os.WriteFile(p, []byte("after-backup"), 0644); err != nil {
		t.Fatal(err)
	}
	clickUI(t, tt, "按钮文字 wb-restore-"+file)
	clickUI(t, tt, "按钮文字 wb-do-restore-"+file)
	waitIdle()
	data, err := os.ReadFile(p)
	if err != nil || string(data) != "before-backup" || len(a.wv.backups) != 2 {
		t.Fatalf("restore or automatic backup failed: %q, %v, %+v", data, err, a.wv.backups)
	}
	snapshotUI(t, tt, "worlds-after-restore")
	clickUI(t, tt, "删除备份")
	if a.wv.confirm == "" {
		t.Fatal("delete must still require confirmation")
	}
	deleteFile := a.wv.confirm[len("delete:"):]
	clickUI(t, tt, "按钮文字 wb-do-delete-"+deleteFile)
	waitIdle()
	if len(a.wv.backups) != 1 {
		t.Fatal("backup deletion did not refresh the local list")
	}
}

func TestWorldJobsAreScopedToRootAndVersion(t *testing.T) {
	a, _, _ := worldsTester(t)
	r := a.root()
	job := launcher.Job{ID: "held", TitleKey: "world.backup", TitleArg: "world-one", State: "running", TargetRootID: r.ID, TargetVersion: r.SelectedVersion}
	for _, scope := range [][2]string{{"another-root", r.SelectedVersion}, {r.ID, "another-version"}, {r.ID, r.SelectedVersion}} {
		job.TargetRootID, job.TargetVersion = scope[0], scope[1]
		a.snapshot.Jobs = []launcher.Job{job}
		want := scope == [2]string{r.ID, r.SelectedVersion}
		if a.worldBusy("world-one") != want || a.worldBusy("world-two") || a.worldBusy("") != want {
			t.Fatalf("job affected wrong world or instance: %+v", job)
		}
	}
	job.TitleKey = "world.backupAll"
	a.snapshot.Jobs = []launcher.Job{job}
	if !a.worldBusy("world-two") {
		t.Fatal("backup-all must protect every world in its own instance")
	}
}
