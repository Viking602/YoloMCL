package nativeui

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
)

func writeBindingUIJar(t *testing.T, dir, id, name string, chinese, english map[string]string) {
	t.Helper()
	var data bytes.Buffer
	w := zip.NewWriter(&data)
	for path, value := range map[string]any{
		"fabric.mod.json":                   map[string]string{"id": id, "name": name},
		"assets/" + id + "/lang/zh_cn.json": chinese,
		"assets/" + id + "/lang/en_us.json": english,
	} {
		b, err := json.Marshal(value)
		if err != nil {
			t.Fatal(err)
		}
		f, err := w.Create(path)
		if err != nil {
			t.Fatal(err)
		}
		if _, err = f.Write(b); err != nil {
			t.Fatal(err)
		}
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(dir, "mods"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "mods", id+".jar"), data.Bytes(), 0644); err != nil {
		t.Fatal(err)
	}
}

func TestKeyBindingsTranslatedModSearchFilterAndEdit(t *testing.T) {
	a, tt, saves := worldsTester(t)
	dir, r := filepath.Dir(saves), a.root()
	writeBindingUIJar(t, dir, "map", "World Map", map[string]string{"key.map.open": "打开地图"}, map[string]string{"key.map.open": "Open Map"})
	writeBindingUIJar(t, dir, "waypoint", "Waypoints", map[string]string{"waypoint.open": "打开地图"}, map[string]string{"waypoint.open": "Open Map", "waypoint.extra": "Only English"})
	options := "key_key.forward:key.keyboard.w\nkey_key.map.open:key.keyboard.m\nkey_waypoint.open:key.keyboard.n\nkey_waypoint.extra:key.keyboard.g\nkey_raw.action:key.keyboard.b\nfov:0.25\n"
	path := filepath.Join(dir, "options.txt")
	if err := os.WriteFile(path, []byte(options), 0644); err != nil {
		t.Fatal(err)
	}
	a.openKeyBindings(r.ID, r.SelectedVersion)
	waitUI(t, tt, func() bool { return a.keyBindings.view != nil && !a.keyBindings.labelsLoading && !a.dialogPending() })
	typeUI(t, tt, a.L("keybindings.search"), "打开地图")
	if !tt.HasText("key_key.map.open") || !tt.HasText("key_waypoint.open") || tt.HasText("key_raw.action") {
		t.Fatal("Chinese search did not distinguish matching identifiers", tt.Texts())
	}
	control := "打开地图 (key_key.map.open)"
	clickUI(t, tt, "按钮文字 binding-custom-key_key.map.open")
	clickUI(t, tt, a.L("keybindings.value")+" "+control)
	tt.Key(0, ui.KeyHome)
	tt.Advance(300 * time.Millisecond)
	clickUI(t, tt, a.L("keybindings.value")+" "+control+" "+a.L("keybindings.key.unknown"))
	if len(a.keyBindings.bindings) != 1 || a.keyBindings.bindings["key_key.map.open"] != "key.keyboard.unknown" {
		t.Fatal("same-name control changed wrong binding", a.keyBindings.bindings)
	}
	clickUI(t, tt, "按钮文字 keybindings-save")
	waitUI(t, tt, func() bool { return !a.dialogPending() && !a.keyBindings.dirty() })
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(b) != strings.Replace(options, "key_key.map.open:key.keyboard.m", "key_key.map.open:key.keyboard.unknown", 1) {
		t.Fatal("translated edit lost original IDs or changed other settings", string(b))
	}
	for _, search := range []struct{ query, found, absent string }{
		{"Open Map", "key_key.map.open", "key_raw.action"},
		{"Waypoints", "key_waypoint.open", "key_key.map.open"},
		{"KEY_KEY.MAP.OPEN", "key_key.map.open", "key_waypoint.open"},
	} {
		clickUI(t, tt, a.L("keybindings.search"))
		tt.Key(ui.Cmd, ui.KeyA)
		tt.Type(search.query)
		if !tt.HasText(search.found) || tt.HasText(search.absent) {
			t.Fatal("search failed", search, tt.Texts())
		}
	}
	a.keyBindings.query = ""
	tt.Frame()
	clickUI(t, tt, a.L("keybindings.filter"))
	clickUI(t, tt, a.L("keybindings.filter")+" "+a.L("keybindings.filter.untranslated"))
	if a.keyBindings.scope != "untranslated" || !tt.HasText("Only English") || !tt.HasText("raw.action") || tt.HasText("打开地图") {
		t.Fatal("missing-Chinese filter failed", tt.Texts())
	}
	a.OverrideLanguage(LangEN)
	a.keyBindings.scope, a.keyBindings.query = "mods", "地图"
	tt.Frame()
	if !tt.HasText("Open Map") || tt.HasText("打开地图") {
		t.Fatal("language switch lost translated search or English display", tt.Texts())
	}
}

func TestKeyBindingsBackgroundLabelsPreserveDrafts(t *testing.T) {
	a, tt, saves := worldsTester(t)
	dir, r := filepath.Dir(saves), a.root()
	writeBindingUIJar(t, dir, "map", "World Map", map[string]string{"key.map.open": "打开地图"}, nil)
	if err := os.WriteFile(filepath.Join(dir, "options.txt"), []byte("key_key.map.open:key.keyboard.m\nmaxFps:120\n"), 0644); err != nil {
		t.Fatal(err)
	}
	a.openKeyBindings(r.ID, r.SelectedVersion)
	waitUI(t, tt, func() bool { return a.keyBindings.view != nil && !a.keyBindings.labelsLoading && !a.dialogPending() })
	g := a.keyBindings
	writeBindingUIJar(t, dir, "map", "World Map", map[string]string{"key.map.open": "打开世界地图"}, nil)
	a.cancelKeyBindingLabels()
	a.loadKeyBindingLabels(g)
	// Apply edits before the UI drains the worker's result, even if IO has
	// already completed. The translated response contains the older values.
	g.bindings["key_key.map.open"] = "key.keyboard.g"
	g.options["maxFps"], g.inputs["maxFps"] = "60", "60"
	waitUI(t, tt, func() bool { return !g.labelsLoading })
	if !tt.HasText("打开世界地图") || g.bindings["key_key.map.open"] != "key.keyboard.g" || g.options["maxFps"] != "60" || g.inputs["maxFps"] != "60" || !g.dirty() {
		t.Fatal("background names reset unsaved settings")
	}
	clickUI(t, tt, "按钮文字 keybindings-save")
	waitUI(t, tt, func() bool { return !a.dialogPending() && g.saved })
	if !tt.HasText("打开世界地图") {
		t.Fatal("save dropped resolved names")
	}
	// Simulate the initial untranslated grouping while a recorder has focus.
	// Translation moves this control to a mod card with a different parent.
	for i := range g.view.Rows {
		g.view.Rows[i].Names, g.view.Rows[i].Namespace, g.view.Rows[i].ModName = nil, "", ""
	}
	tt.Frame()
	clickUI(t, tt, a.L("game_options.record")+" key.map.open")
	a.loadKeyBindingLabels(g)
	waitUI(t, tt, func() bool { return !g.labelsLoading })
	if g.pendingLabels == nil || g.capture != "key_key.map.open" {
		t.Fatal("translation regrouped an active recorder")
	}
	tt.Key(0, ui.KeyZ)
	waitUI(t, tt, func() bool { return g.pendingLabels == nil })
	if g.bindings["key_key.map.open"] != "key.keyboard.z" || !tt.HasText("打开世界地图") {
		t.Fatal("translation update interrupted recording")
	}
	a.loadKeyBindingLabels(g)
	a.closeDialog()
	if g.labelsLoading || g.labelsCancel != nil {
		t.Fatal("closing did not cancel name loading")
	}
}

func TestKeyBindingsManyTranslatedControlsCompactWindow(t *testing.T) {
	a, tt, saves := worldsTester(t)
	dir, r := filepath.Dir(saves), a.root()
	var options strings.Builder
	zh, en := map[string]string{}, map[string]string{}
	for i := range 180 {
		key := fmt.Sprintf("key.demo.action%03d", i)
		fmt.Fprintf(&options, "key_%s:key.keyboard.k\n", key)
		zh[key], en[key] = fmt.Sprintf("打开模组菜单 %03d", i), fmt.Sprintf("Open mod menu %03d", i)
	}
	writeBindingUIJar(t, dir, "demo", "Mod Controls", zh, en)
	if err := os.WriteFile(filepath.Join(dir, "options.txt"), []byte(options.String()), 0644); err != nil {
		t.Fatal(err)
	}
	a.openKeyBindings(r.ID, r.SelectedVersion)
	waitUI(t, tt, func() bool { return a.keyBindings.view != nil && !a.keyBindings.labelsLoading && !a.dialogPending() })
	if len(a.keyBindings.view.Rows) != 180 {
		t.Fatal("many controls were truncated")
	}
	for _, lang := range []string{LangZH, LangEN} {
		a.OverrideLanguage(lang)
		for _, size := range [][2]int{{900, 540}, {1536, 1024}} {
			tt.SetSize(size[0], size[1])
			tt.Advance(400 * time.Millisecond)
			for _, label := range []string{"活动弹窗", a.L("keybindings.search"), a.L("keybindings.filter"), a.L("game_options.mod"), "按钮文字 keybindings-save", "按钮文字 keybindings-reload"} {
				box, ok := tt.Find(label)
				if !ok || box.W <= 0 || box.H <= 0 || box.X < 0 || box.Y < 42 || box.X+box.W > float32(size[0]) || box.Y+box.H > float32(size[1]) {
					t.Errorf("%s outside %v: %+v", label, size, box)
				}
			}
			snapshotUI(t, tt, fmt.Sprintf("keybindings-mods-%s-%d", lang, size[0]))
		}
	}
	a.keyBindings.query = "179"
	tt.Frame()
	if !tt.HasText("key_key.demo.action179") || tt.HasText("key_key.demo.action000") {
		t.Fatal("last action was not searchable")
	}
}
