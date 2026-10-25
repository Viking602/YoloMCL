package nativeui

import (
	"fmt"
	"slices"
	"testing"

	"github.com/Viking602/YoloMCL/internal/launcher"
)

func TestAddonPreparationRefreshesInputsAndFilters(t *testing.T) {
	a := &App{addonTab: "mods", mods: []launcher.Mod{
		{File: "one.jar", Name: "One", ModID: "one", SHA1: "hash", Enabled: true, Size: 20, Requires: []string{"library"}},
		{File: "lib.jar", Name: "Library", ModID: "library", Enabled: true, Size: 10, Provides: []string{"alias"}},
	}, addonRemote: map[string]launcher.RemoteInfo{"hash": {Summary: "Remote summary", Cats: []string{"utility"}}}, modIconURLs: map[string]string{}}
	all := a.addonEntries()
	shown := a.filteredAddons(all)
	if len(all[0].it.missing) != 0 || shown[0].mod.File != "lib.jar" {
		t.Fatal("initial dependencies or order are incorrect")
	}
	if next := a.filteredAddons(a.addonEntries()); &next[0] != &shown[0] {
		t.Fatal("unchanged data was prepared again")
	}
	a.mods[1].Enabled = false
	all = a.addonEntries()
	if !slices.Equal(all[0].it.missing, []string{"library"}) {
		t.Fatal("disabling a dependency left its users unchanged")
	}
	a.mods[0].Requires[0] = "alias"
	a.mods[1].Enabled = true
	if all = a.addonEntries(); len(all[0].it.missing) != 0 {
		t.Fatal("changed dependency did not use the provided alias")
	}
	a.mods[1].Provides[0] = "different"
	if all = a.addonEntries(); !slices.Equal(all[0].it.missing, []string{"alias"}) {
		t.Fatal("in-place provided ID change retained old dependencies")
	}
	a.modIconURLs["hash"] = "new-icon"
	a.addonRemote["hash"].Cats[0] = "changed"
	all = a.addonEntries()
	if all[0].it.iconURL != "new-icon" || !slices.Equal(all[0].it.cats, []string{"changed"}) {
		t.Fatal("remote metadata did not refresh")
	}
	a.mods[0].Latest = &launcher.ProjectVersion{ID: "next", Number: "2"}
	a.addonFilterKind = "update"
	shown = a.filteredAddons(a.addonEntries())
	if len(shown) != 1 || shown[0].it.latest != "2" {
		t.Fatal("update filter retained old rows")
	}
	a.mods[0].Latest.Number = "3"
	if shown = a.filteredAddons(a.addonEntries()); shown[0].it.latest != "3" {
		t.Fatal("in-place update metadata remained stale")
	}
	a.mods[0].Latest = nil
	if len(a.filteredAddons(a.addonEntries())) != 0 {
		t.Fatal("removed update remained in results")
	}
	a.addonFilterKind, a.addonSortKey = "", "size"
	shown = a.filteredAddons(a.addonEntries())
	if shown[0].mod.File != "one.jar" {
		t.Fatal("size sort retained the name order")
	}
	a.modFilter = "Library"
	if shown = a.filteredAddons(a.addonEntries()); len(shown) != 1 || shown[0].mod.File != "lib.jar" {
		t.Fatal("search did not refresh results")
	}
	// A refreshed scan can contain identical values in a new backing array.
	// Actions must still address the current mod objects.
	a.mods = slices.Clone(a.mods)
	if shown = a.filteredAddons(a.addonEntries()); shown[0].mod != &a.mods[1] {
		t.Fatal("new scan retained pointers into the previous list")
	}
	a.mods[0].Pending = true
	if !a.addonEntries()[0].it.pending {
		t.Fatal("a pending scan could decode artwork before its content hash was known")
	}
	a.mods[0].Pending, a.mods[0].SHA1 = false, "replacement-hash"
	if it := a.addonEntries()[0].it; it.pending || it.sha1 != "replacement-hash" {
		t.Fatal("completed scan retained the previous artwork identity")
	}
	a.mods = nil
	if len(a.filteredAddons(a.addonEntries())) != 0 {
		t.Fatal("switching away retained old results")
	}
}

func TestAddonPreparationFollowsLanguageAndInstanceLoader(t *testing.T) {
	a := &App{addonTab: "mods", mods: []launcher.Mod{{File: "one.jar", Name: "One", Loader: "vanilla"}},
		snapshot: launcher.Snapshot{ActiveRoot: "root", Instances: []launcher.Instance{{
			Root: launcher.Root{ID: "root", SelectedVersion: "pack"}, Versions: []launcher.VersionSummary{{ID: "pack", Loader: "fabric"}},
		}}}}
	a.settings.Language = LangEN
	if got := a.addonEntries()[0].it.loader; got != "Vanilla" {
		t.Fatalf("initial loader label: %q", got)
	}
	a.settings.Language = LangZH
	if got := a.addonEntries()[0].it.loader; got != "原版" {
		t.Fatalf("language change retained loader label: %q", got)
	}
	a.root().Versions[0].Loader = "vanilla"
	if got := a.addonEntries()[0].it.loader; got != "" {
		t.Fatalf("instance loader change retained a redundant label: %q", got)
	}
	a.addonTab = "shader"
	a.assetPacks = []launcher.AssetPack{{File: "shader.zip", Name: "Shader", Kind: "shader"}}
	if got := a.addonEntries(); len(got) != 1 || got[0].pack == nil || got[0].mod != nil {
		t.Fatal("pack tab retained mod entries")
	}
	a.addonTab = "mods"
	if got := a.addonEntries(); len(got) != 1 || got[0].mod != &a.mods[0] {
		t.Fatal("switching back did not restore mods")
	}
}

func BenchmarkAddonListPreparation(b *testing.B) {
	a := &App{addonTab: "mods", mods: make([]launcher.Mod, 500)}
	for i := range a.mods {
		a.mods[i] = launcher.Mod{
			Name: fmt.Sprintf("Example Mod %03d", i), File: fmt.Sprintf("example-mod-%03d.jar", i),
			ModID: fmt.Sprintf("example_%03d", i), Version: "1.21.1-2.1.0", Loader: "neoforge",
			Enabled: true, Size: 700000, Description: "A mod with dependencies and updates.",
			Requires: []string{"common"}, Provides: []string{"example"},
		}
	}
	b.ReportAllocs()
	for b.Loop() {
		all := a.addonEntries()
		if len(a.filteredAddons(all)) != 500 {
			b.Fatal("lost visible entries")
		}
	}
}
