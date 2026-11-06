package nativeui

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"

	"github.com/Viking602/YoloMCL/internal/launcher"
)

func TestKeyBindingsEditorScopesSaveApplyAndInherit(t *testing.T) {
	a, tt, saves := worldsTester(t)
	r := a.root()
	path := filepath.Join(filepath.Dir(saves), "options.txt")
	original := "key_key.forward:key.keyboard.w\r\nkey_quest.open:key.keyboard.o:CONTROL\r\nfov:0.25\r\n"
	if err := os.WriteFile(path, []byte(original), 0644); err != nil {
		t.Fatal(err)
	}
	a.openKeyBindings("", "")
	waitUI(t, tt, func() bool { return a.keyBindings.view != nil && !a.dialogPending() })
	for _, row := range a.keyBindings.view.Rows {
		if !row.Vanilla || row.ID == "key_quest.open" {
			t.Fatal("mod action leaked into global controls")
		}
	}
	a.keyBindings.query = a.L("keybindings.action.forward")
	tt.Frame()
	if _, ok := tt.Find(a.L("keybindings.override") + " " + a.L("keybindings.action.forward")); ok {
		t.Fatal("editor still requires enabling overrides")
	}
	clickUI(t, tt, a.L("game_options.record")+" "+a.L("keybindings.action.forward"))
	tt.Key(0, ui.KeyI)
	if a.keyBindings.bindings["key_key.forward"] != "key.keyboard.i" {
		t.Fatal("direct recording did not configure the key")
	}
	clickUI(t, tt, "按钮文字 keybindings-save")
	waitUI(t, tt, func() bool { return !a.dialogPending() && !a.keyBindings.dirty() })
	a.openKeyBindings(r.ID, r.SelectedVersion)
	waitUI(t, tt, func() bool { return a.keyBindings.view != nil && !a.dialogPending() })
	if len(a.keyBindings.view.Rows) != 2 {
		t.Fatal("pack editor did not discover all key actions")
	}
	if !tt.HasText(fmt.Sprintf(a.L("keybindings.source_value"), a.L("keybindings.global"), "I")) {
		t.Fatal("missing global inheritance display")
	}
	clickUI(t, tt, "按钮文字 binding-custom-key_quest.open")
	clickUI(t, tt, a.L("keybindings.value")+" quest.open")
	tt.Key(0, ui.KeyHome)
	tt.Advance(300 * time.Millisecond)
	clickUI(t, tt, a.L("keybindings.value")+" quest.open "+a.L("keybindings.key.unknown"))
	if got := a.keyBindings.bindings["key_quest.open"]; got != "key.keyboard.unknown" {
		t.Fatalf("dropdown selection did not update the binding: %s", got)
	}
	clickUI(t, tt, "按钮文字 keybindings-save")
	waitUI(t, tt, func() bool { return !a.dialogPending() && !a.keyBindings.dirty() })
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "key_key.forward:key.keyboard.i\r\n") || !strings.Contains(string(data), "key_quest.open:key.keyboard.unknown\r\n") || !strings.Contains(string(data), "fov:0.25\r\n") {
		t.Fatalf("incorrect applied options: %q", data)
	}
	clickUI(t, tt, a.L("keybindings.inherit")+" quest.open")
	clickUI(t, tt, "按钮文字 keybindings-save")
	waitUI(t, tt, func() bool { return !a.dialogPending() && !a.keyBindings.dirty() })
	data, _ = os.ReadFile(path)
	if !strings.Contains(string(data), "key_quest.open:key.keyboard.o:CONTROL\r\n") {
		t.Fatal("inheritance did not restore the mod's original shortcut")
	}
	tt.Advance(400 * time.Millisecond)
	snapshotUI(t, tt, "keybindings-pack")
}

func TestKeyBindingsErrorsFollowUILanguage(t *testing.T) {
	a, _, _ := worldsTester(t)
	view, err := a.service.ReadKeyBindings(context.Background(), "", "")
	if err != nil {
		t.Fatal(err)
	}
	_, err = a.service.SaveKeyBindings(context.Background(), launcher.KeyBindingsRequest{Revision: view.Revision, Bindings: map[string]string{"key_mod.action": "key.keyboard.a"}})
	if err == nil {
		t.Fatal("mod action accepted globally")
	}
	a.OverrideLanguage("en-US")
	if got := a.friendlyErr(err); !strings.Contains(got, "vanilla controls") {
		t.Fatalf("untranslated binding error: %q", got)
	}
}

func TestKeyBindingsGlobalEditorCompactWindow(t *testing.T) {
	a, tt := settingsTester(t)
	t.Cleanup(func() { a.cancel(); a.bgWG.Wait() })
	a.openKeyBindings("", "")
	waitUI(t, tt, func() bool { return a.keyBindings.view != nil && !a.dialogPending() })
	for _, lang := range []string{LangZH, LangEN} {
		a.OverrideLanguage(lang)
		for _, size := range [][2]int{{900, 540}, {1536, 1024}} {
			tt.SetSize(size[0], size[1])
			tt.Advance(400 * time.Millisecond)
			for _, name := range []string{"活动弹窗", "按钮文字 keybindings-save", "按钮文字 keybindings-reload"} {
				r, ok := tt.Find(name)
				if !ok || r.W <= 0 || r.H <= 0 || r.X < 0 || r.Y < 42 || r.X+r.W > float32(size[0]) || r.Y+r.H > float32(size[1]) {
					t.Errorf("%s outside %v: %+v", name, size, r)
				}
			}
			snapshotUI(t, tt, fmt.Sprintf("keybindings-global-%s-%d", lang, size[0]))
		}
	}
}
