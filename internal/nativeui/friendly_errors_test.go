package nativeui

import (
	"strings"
	"testing"

	"github.com/Viking602/YoloMCL/internal/launcher"
)

func TestDiscoverCardSingleInstalledIndicator(t *testing.T) {
	a, tt := p2Fixture(t, 1280, 800, 1)
	r := a.root()
	if r == nil {
		t.Skip("fixture has no root")
	}
	ps := []launcher.Project{{ID: "426558", Source: "curseforge", Type: "mod", Title: "Alex's Mobs", Author: "sbom_xela", Categories: []string{"Armor, Tools, and Weapons", "Mobs"}, Description: "85+ New mobs"}}
	a.mods = []launcher.Mod{{File: "alexsmobs.jar", Source: "curseforge", ProjectID: "426558", Enabled: true}}
	a.addonCache = nil
	a.page = "discover"
	a.searchResult = launcher.SearchResult{Projects: ps, Total: 1, Limit: 18}
	settle(tt)
	n := 0
	for _, s := range tt.Texts() {
		if s == "已安装" {
			n++
		}
		if strings.Contains(s, "Armor, Tools") {
			t.Errorf("untranslated category chip shown: %q", s)
		}
	}
	if n != 1 {
		t.Fatalf("want exactly one 已安装 indicator, got %d", n)
	}
	if !tt.HasText("装备、工具与武器") {
		t.Errorf("CF category not translated")
	}
}
