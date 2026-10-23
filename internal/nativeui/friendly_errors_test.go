package nativeui

import (
	"errors"
	"io/fs"
	"os"
	"strings"
	"testing"

	"github.com/Viking602/YoloMCL/internal/launcher"
)

func TestClassifyErrorNeverRaw(t *testing.T) {
	cases := map[string]string{
		`open D:\MC\.minecraft\versions\X\mods: Access is denied.`:                        "denied",
		`open /x/mods: permission denied`:                                                 "denied",
		`The process cannot access the file because it is being used by another process.`: "busy",
		`write C:\a\b.jar: There is not enough space on the disk.`:                        "disk",
		`Get "https://api.modrinth.com/v2": context deadline exceeded`:                    "network",
		`dial tcp: lookup api.curseforge.com: no such host`:                               "network",
		`open C:\nope\x.json: The system cannot find the path specified.`:                 "missing",
		`json: cannot unmarshal string into Go value of type int`:                         "other",
	}
	for raw, kind := range cases {
		for _, en := range []bool{false, true} {
			u := classifyError(en, raw, nil)
			if u.Kind != kind {
				t.Errorf("%q en=%v: kind %s, want %s", raw, en, u.Kind, kind)
			}
			txt := u.Text()
			if strings.Contains(txt, raw) || strings.Contains(strings.ToLower(txt), "access is denied") || strings.Contains(txt, "json:") || strings.Contains(txt, `D:\`) {
				t.Errorf("raw text leaked: %q", txt)
			}
		}
	}
	if u := classifyError(false, `open D:\MC\mods: Access is denied.`, nil); u.Path != `D:\MC\mods` {
		t.Errorf("path = %q", u.Path)
	}
	if u := classifyError(false, "", &fs.PathError{Op: "open", Path: "x", Err: fs.ErrPermission}); u.Kind != "denied" {
		t.Errorf("typed permission error: %s", u.Kind)
	}
	if u := classifyError(false, "版本 1.0 没有可下载文件", errors.New("x")); u.Kind != "plain" {
		t.Errorf("launcher's own Chinese message should pass through: %s", u.Kind)
	}
	_ = os.ErrNotExist
}

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
