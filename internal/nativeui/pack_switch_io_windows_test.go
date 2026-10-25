package nativeui

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"unsafe"

	"github.com/egoist/mygo/ui"

	"github.com/Viking602/YoloMCL/internal/launcher"
)

func processReadBytes(t *testing.T) uint64 {
	t.Helper()
	var counters [6]uint64
	proc := syscall.NewLazyDLL("kernel32.dll").NewProc("GetProcessIoCounters")
	if ok, _, err := proc.Call(^uintptr(0), uintptr(unsafe.Pointer(&counters))); ok == 0 {
		t.Fatal(err)
	}
	return counters[3]
}

// Exercise real navigation and filesystem reads, not just cache flags. The
// large stored entries make an accidental whole-JAR scan clearly measurable.
func TestPackOverviewNavigationDoesNotReadJarContents(t *testing.T) {
	game := t.TempDir()
	chunk := bytes.Repeat([]byte{7, 9, 32, 255}, 16<<10)
	for _, version := range []string{"pack-a", "pack-b"} {
		dir := filepath.Join(game, "versions", version)
		for _, sub := range []string{"mods", ".mcl"} {
			if err := os.MkdirAll(filepath.Join(dir, sub), 0755); err != nil {
				t.Fatal(err)
			}
		}
		for name, data := range map[string]string{version + ".json": `{"id":"` + version + `","inheritsFrom":"1.21.1"}`, ".mcl/mod-checks.json": "{}"} {
			if err := os.WriteFile(filepath.Join(dir, name), []byte(data), 0600); err != nil {
				t.Fatal(err)
			}
		}
		for i := range 8 {
			f, err := os.Create(filepath.Join(dir, "mods", fmt.Sprintf("mod-%d.jar", i)))
			if err != nil {
				t.Fatal(err)
			}
			z := zip.NewWriter(f)
			w, err := z.Create("fabric.mod.json")
			if err == nil {
				_, err = fmt.Fprintf(w, `{"id":"mod%d","name":"%s %d","version":"1"}`, i, version, i)
			}
			if err == nil {
				w, err = z.CreateHeader(&zip.FileHeader{Name: "assets/data.bin", Method: zip.Store})
			}
			for n := 0; n < 64 && err == nil; n++ {
				_, err = w.Write(chunk)
			}
			if e := z.Close(); err == nil {
				err = e
			}
			if e := f.Close(); err == nil {
				err = e
			}
			if err != nil {
				t.Fatal(err)
			}
		}
	}
	s, err := launcher.New(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	r, err := s.AddRoot(game, "Disk IO fixture")
	if err != nil {
		t.Fatal(err)
	}
	a, err := New(s)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { a.cancel(); a.bgWG.Wait() }()
	settle := func() {
		for {
			a.bgWG.Wait()
			if len(a.events) == 0 {
				return
			}
			for len(a.events) > 0 {
				(<-a.events)()
			}
		}
	}
	before := processReadBytes(t)
	a.ShowPage("home")
	tt := ui.NewTester(a.View, 800, 600)
	settle()
	a.selectVersion(r.ID, "pack-b")
	tt.Frame()
	settle()
	a.selectVersion(r.ID, "pack-a")
	tt.Frame()
	settle()
	a.goPage("tasks")
	a.goPage("home")
	tt.Frame()
	settle()
	read := processReadBytes(t) - before
	t.Logf("Opening, switching and returning to two 32 MiB packs read %.3f MiB", float64(read)/(1<<20))
	if read > 1<<20 {
		t.Fatalf("overview navigation read JAR contents: %d bytes", read)
	}
	if a.instStatus.Mods != 8 || a.instStatus.EnabledMods != 8 || a.instStatus.Sizes["mods"] < 32<<20 {
		t.Fatalf("skipping JAR contents lost overview counts or storage: %+v", a.instStatus)
	}
	for _, version := range []string{"pack-a", "pack-b"} {
		if _, err := os.Stat(filepath.Join(game, "versions", version, ".mcl/modcache.json")); !os.IsNotExist(err) {
			t.Fatal("overview unexpectedly populated a full mod scan cache")
		}
	}
}

// Opt-in read-only QA for installed packs. Only the isolated launcher's
// temporary configuration changes; the user's instance selection is untouched.
func TestLocalPackOverviewDiskIOQA(t *testing.T) {
	game := os.Getenv("MCL_LOCAL_INSTANCE_ROOT")
	versions := strings.Split(os.Getenv("MCL_LOCAL_INSTANCE_VERSIONS"), "|")
	if game == "" || len(versions) < 2 {
		t.Skip("local pack disk IO QA not requested")
	}
	data := t.TempDir()
	config := map[string]any{"schema": 1, "activeRoot": "qa", "roots": []launcher.Root{{ID: "qa", Name: "Disk IO QA", Path: game, SelectedVersion: versions[0]}}}
	b, err := json.Marshal(config)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(data, "config.json"), b, 0600); err != nil {
		t.Fatal(err)
	}
	s, err := launcher.New(data)
	if err != nil {
		t.Fatal(err)
	}
	a, err := New(s)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { a.cancel(); a.bgWG.Wait() }()
	before := processReadBytes(t)
	a.ShowPage("home")
	tt := ui.NewTester(a.View, 1536, 1024)
	for _, version := range append(versions, versions[0]) {
		a.selectVersion("qa", version)
		tt.Frame()
		for {
			a.bgWG.Wait()
			if len(a.events) == 0 {
				break
			}
			for len(a.events) > 0 {
				(<-a.events)()
			}
		}
		if a.modsLoadKey != "" || a.instStatus.StoragePending || a.instStatus.Dir == "" || a.instStatusErr != "" {
			t.Fatalf("unnecessary mod scan or incomplete overview: scan=%q status=%+v error=%s", a.modsLoadKey, a.instStatus, a.instStatusErr)
		}
		after := processReadBytes(t)
		t.Logf("%s: %d mods, %.1f MiB of JARs; overview read %.3f MiB", version, a.instStatus.Mods, float64(a.instStatus.Sizes["mods"])/(1<<20), float64(after-before)/(1<<20))
		before = after
	}
}
