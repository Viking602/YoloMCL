package nativeui

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
)

func replacePreferenceInput(t *testing.T, tt *ui.Tester, label, value string) {
	t.Helper()
	clickUI(t, tt, label)
	tt.Key(ui.Cmd, ui.KeyA)
	tt.Type(value)
}

func TestGameSettingsEditVideoAudioAndCustomOptions(t *testing.T) {
	a, tt, saves := worldsTester(t)
	path := filepath.Join(filepath.Dir(saves), "options.txt")
	original := "version:3955\r\nkey_key.forward:key.keyboard.w\r\nrenderDistance:12\r\nfullscreen:false\r\nfov:0.25\r\nsoundCategory_music:0.4\r\ncustomOption:old:value\r\n"
	if err := os.WriteFile(path, []byte(original), 0644); err != nil {
		t.Fatal(err)
	}
	r := a.root()
	a.openKeyBindings(r.ID, r.SelectedVersion)
	waitUI(t, tt, func() bool { return a.keyBindings.view != nil && !a.dialogPending() })
	clickUI(t, tt, a.L("game_options.tab.video"))
	a.keyBindings.query = "renderDistance"
	tt.Frame()
	replacePreferenceInput(t, tt, a.L("game_options.value")+" "+a.L("game_options.option.renderDistance"), "24")
	if a.keyBindings.options["renderDistance"] == "" {
		t.Fatalf("numeric input did not update: inputs=%v invalid=%v tab=%s texts=%v", a.keyBindings.inputs, a.keyBindings.invalid, a.keyBindings.tab, tt.Texts())
	}
	a.keyBindings.query = "fullscreen"
	tt.Frame()
	clickUI(t, tt, a.L("game_options.value")+" "+a.L("game_options.option.fullscreen"))
	clickUI(t, tt, a.L("game_options.tab.audio"))
	replacePreferenceInput(t, tt, a.L("game_options.value")+" "+a.L("game_options.option.soundCategory_music"), "65")
	clickUI(t, tt, a.L("game_options.tab.other"))
	replacePreferenceInput(t, tt, a.L("game_options.value")+" customOption", "new:custom:value")
	clickUI(t, tt, "按钮文字 keybindings-save")
	waitUI(t, tt, func() bool { return !a.dialogPending() && !a.keyBindings.dirty() })
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	expected := strings.NewReplacer("renderDistance:12", "renderDistance:24", "fullscreen:false", "fullscreen:true", "soundCategory_music:0.4", "soundCategory_music:0.65", "customOption:old:value", "customOption:new:custom:value").Replace(original)
	if string(got) != expected {
		t.Fatalf("combined edit changed wrong fields:\n%s", got)
	}
	clickUI(t, tt, a.L("game_options.tab.video"))
	a.keyBindings.query = "renderDistance"
	tt.Frame()
	replacePreferenceInput(t, tt, a.L("game_options.value")+" "+a.L("game_options.option.renderDistance"), "999")
	if !a.keyBindings.invalid["renderDistance"] {
		t.Fatal("out-of-range value accepted")
	}
	clickUI(t, tt, a.L("keybindings.inherit")+" "+a.L("game_options.option.renderDistance"))
	if len(a.keyBindings.invalid) != 0 {
		t.Fatal("reset did not clear invalid draft")
	}
	clickUI(t, tt, "按钮文字 keybindings-save")
	waitUI(t, tt, func() bool { return !a.dialogPending() && !a.keyBindings.dirty() })
	got, _ = os.ReadFile(path)
	if !strings.Contains(string(got), "renderDistance:12\r\n") || !strings.Contains(string(got), "fullscreen:true\r\n") {
		t.Fatal("reset changed unrelated settings", string(got))
	}
}

func TestGameSettingsCaptureCancelEnterMouseAndManual(t *testing.T) {
	a, tt, saves := worldsTester(t)
	if err := os.WriteFile(filepath.Join(filepath.Dir(saves), "options.txt"), []byte("key_quest.open:key.keyboard.o:CONTROL\n"), 0644); err != nil {
		t.Fatal(err)
	}
	r := a.root()
	a.openKeyBindings(r.ID, r.SelectedVersion)
	waitUI(t, tt, func() bool { return a.keyBindings.view != nil && !a.dialogPending() })
	record := a.L("game_options.record") + " quest.open"
	clickUI(t, tt, record)
	tt.Key(ui.Ctrl, ui.KeyG)
	if a.keyBindings.bindings["key_quest.open"] != "key.keyboard.g:CONTROL" {
		t.Fatal("modifier recording failed", a.keyBindings.bindings)
	}
	clickUI(t, tt, record)
	tt.Key(0, ui.KeyEscape)
	if !a.dialogOpen || a.keyBindings.capture != "" || a.keyBindings.bindings["key_quest.open"] != "key.keyboard.g:CONTROL" {
		t.Fatal("Escape did not cancel only the recorder")
	}
	clickUI(t, tt, record)
	tt.Key(0, ui.KeyEnter)
	if a.keyBindings.bindings["key_quest.open"] != "key.keyboard.enter" || a.dialogPending() {
		t.Fatal("Enter submitted the dialog instead of being recorded")
	}
	clickUI(t, tt, record)
	clickUI(t, tt, record)
	if a.keyBindings.bindings["key_quest.open"] != "key.mouse.left" {
		t.Fatal("mouse binding was not captured")
	}
	clickUI(t, tt, "按钮文字 binding-custom-key_quest.open")
	replacePreferenceInput(t, tt, a.L("game_options.custom_value")+" quest.open", "Shift+H")
	if a.keyBindings.bindings["key_quest.open"] != "key.keyboard.h:SHIFT" {
		t.Fatal("manual custom binding failed")
	}
	replacePreferenceInput(t, tt, a.L("game_options.custom_value")+" quest.open", "Ctrl+Alt+G")
	if !a.keyBindings.invalid["key_quest.open"] {
		t.Fatal("unsupported modifier combination accepted")
	}
}

func TestGameSettingsModGroupsAndVideoLayout(t *testing.T) {
	a, tt, saves := worldsTester(t)
	dir := filepath.Dir(saves)
	writeBindingUIJar(t, dir, "map", "World Map", map[string]string{"key.map.open": "打开地图"}, nil)
	writeBindingUIJar(t, dir, "quest", "Quests", map[string]string{"key.quest.open": "任务手册"}, nil)
	options := "key_key.forward:key.keyboard.w\nkey_key.map.open:key.keyboard.m\nkey_key.quest.open:key.keyboard.q\nrenderDistance:12\nsimulationDistance:8\ngraphicsMode:1\ngamma:0.5\nfov:0.25\nmaxFps:120\nfullscreen:false\nenableVsync:true\nsoundCategory_music:0.8\n"
	if err := os.WriteFile(filepath.Join(dir, "options.txt"), []byte(options), 0644); err != nil {
		t.Fatal(err)
	}
	r := a.root()
	a.openKeyBindings(r.ID, r.SelectedVersion)
	waitUI(t, tt, func() bool { return a.keyBindings.view != nil && !a.keyBindings.labelsLoading && !a.dialogPending() })
	clickUI(t, tt, a.L("game_options.mod"))
	clickUI(t, tt, a.L("game_options.mod")+" World Map (1)")
	if !tt.HasText("打开地图") || tt.HasText("任务手册") || a.keyBindings.mod != "mod:map" {
		t.Fatal("mod filter mixed unrelated groups")
	}
	a.keyBindings.mod = "all"
	tt.Frame()
	tt.Advance(300 * time.Millisecond)
	snapshotUI(t, tt, "game-settings-mod-groups")
	for _, lang := range []string{LangZH, LangEN} {
		a.OverrideLanguage(lang)
		tt.Frame()
		clickUI(t, tt, a.L("game_options.tab.video"))
		for _, size := range [][2]int{{900, 540}, {1536, 1024}} {
			tt.SetSize(size[0], size[1])
			tt.Advance(350 * time.Millisecond)
			for _, label := range []string{"活动弹窗", a.L("game_options.tab.video"), a.L("keybindings.search"), "按钮文字 keybindings-save"} {
				r, ok := tt.Find(label)
				if !ok || r.X < 0 || r.Y < 42 || r.X+r.W > float32(size[0]) || r.Y+r.H > float32(size[1]) {
					t.Errorf("%s clipped at %v: %+v", label, size, r)
				}
			}
			snapshotUI(t, tt, fmt.Sprintf("game-settings-video-%s-%d", lang, size[0]))
		}
	}
}
