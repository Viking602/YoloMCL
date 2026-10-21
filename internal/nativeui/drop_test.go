package nativeui

import (
	"archive/zip"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func writeZip(t *testing.T, path string, files map[string]string) {
	t.Helper()
	f, e := os.Create(path)
	if e != nil {
		t.Fatal(e)
	}
	z := zip.NewWriter(f)
	for n, b := range files {
		w, _ := z.Create(n)
		w.Write([]byte(b))
	}
	z.Close()
	f.Close()
}

func TestDroppedZipsAreClassified(t *testing.T) {
	a, tt := homeTester(t, "zh-CN")
	dir := t.TempDir()
	random := filepath.Join(dir, "photos.zip")
	writeZip(t, random, map[string]string{"readme.txt": "hi"})
	a.handleDroppedFiles([]string{random})
	tt.Frame()
	if a.dialog != "" || a.page != "home" || !a.errorMessage || !strings.Contains(a.message, "无法识别") {
		t.Errorf("unknown zip must show an error, not route anywhere: page=%s dialog=%q msg=%q", a.page, a.dialog, a.message)
	}
	cf, _ := json.Marshal(map[string]any{"manifestType": "minecraftModpack", "manifestVersion": 1, "name": "CF", "minecraft": map[string]any{"version": "1.21.1", "modLoaders": []map[string]any{{"id": "fabric-0.16.14", "primary": true}}}, "files": []map[string]any{{"projectID": 1, "fileID": 2}}})
	cfZip := filepath.Join(dir, "cf.zip")
	writeZip(t, cfZip, map[string]string{"manifest.json": string(cf)})
	a.handleDroppedFiles([]string{cfZip})
	a.form.Parent, a.form.Name = dir, "cf"
	tt.Advance(400 * time.Millisecond)
	if _, ok := tt.Find("整合包不可用"); !ok {
		t.Error("CurseForge reason missing in the dialog")
	}
	before := len(a.service.Snapshot().Jobs)
	if b, ok := tt.Find("按钮文字 import-pack-now"); ok {
		tt.ClickAt(b.X+b.W/2, b.Y+b.H/2)
		tt.Frame()
	}
	if len(a.service.Snapshot().Jobs) != before {
		t.Error("import button must be disabled for an unavailable pack")
	}
	// confirming a task closes the dialog without a lingering exit
	a.taskAdded(nil)
	tt.Frame()
	if _, ok := tt.Find("活动弹窗"); ok || a.page != "tasks" {
		t.Error("dialog should vanish at once when switching to tasks")
	}
}
