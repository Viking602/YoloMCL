package launcher

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDefaultGameDirPerOS(t *testing.T) {
	home := t.TempDir()
	app := filepath.Join(home, "AppData")
	for goos, p := range map[string]string{
		"windows": filepath.Join(app, ".minecraft"),
		"darwin":  filepath.Join(home, "Library", "Application Support", "minecraft"),
		"linux":   filepath.Join(home, ".minecraft"),
	} {
		if got := defaultGameDir(goos, app, home); got != "" {
			t.Errorf("%s: missing dir reported %q", goos, got)
		}
		os.MkdirAll(p, 0755)
		if got := defaultGameDir(goos, app, home); got != p {
			t.Errorf("%s: %q want %q", goos, got, p)
		}
	}
}

func TestVersionDisplayNameRoundTrip(t *testing.T) {
	f := newUpdFixture(t)
	ver := f.root.Versions[0].ID
	if e := f.s.SetVersionDisplayName(f.root.ID, ver, "  周末联机  "); e != nil {
		t.Fatal(e)
	}
	got := func() string {
		for _, r := range f.s.Snapshot().Instances {
			for _, v := range r.Versions {
				if r.ID == f.root.ID && v.ID == ver {
					return v.DisplayName
				}
			}
		}
		return "?"
	}
	if g := got(); g != "周末联机" {
		t.Fatalf("display name %q", g)
	}
	f.s.SetVersionDisplayName(f.root.ID, ver, "")
	if g := got(); g != "" {
		t.Fatalf("cleared name %q", g)
	}
}
