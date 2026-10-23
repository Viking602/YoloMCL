package nativeui

import (
	"testing"

	"github.com/Viking602/YoloMCL/internal/launcher"
)

func TestOptimisticToggleFlipsRowInPlace(t *testing.T) {
	a := &App{}
	a.snapshot.Instances = []launcher.Instance{{ID: "r", SelectedVersion: "v"}}
	a.snapshot.ActiveRoot = "r"
	a.mods = []launcher.Mod{{File: "a.jar", Enabled: true}, {File: "b.jar.disabled"}}
	if !a.applyToggle("mods", "a.jar", toggledName("a.jar")) || a.mods[0].File != "a.jar.disabled" || a.mods[0].Enabled {
		t.Fatalf("disable: %+v", a.mods[0])
	}
	if !a.applyToggle("mods", "b.jar.disabled", toggledName("b.jar.disabled")) || a.mods[1].File != "b.jar" || !a.mods[1].Enabled {
		t.Fatalf("enable: %+v", a.mods[1])
	}
	if a.modItem(a.mods[0]).key != "mod-a.jar" {
		t.Fatal("row key must not change with .disabled")
	}
}
