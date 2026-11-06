package nativeui

import (
	"context"
	"math"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
)

func readPreferenceInput(t *testing.T, tt *ui.Tester, label string) string {
	t.Helper()
	clickUI(t, tt, label)
	tt.Key(ui.Cmd, ui.KeyA)
	tt.Key(ui.Cmd, ui.KeyC)
	return tt.Clipboard()
}

func TestGameSettingsFPSUnlimitedAndVisibleSaveFeedback(t *testing.T) {
	a, tt, _ := worldsTester(t)
	a.openKeyBindings("", "")
	waitUI(t, tt, func() bool { return a.keyBindings.view != nil && !a.dialogPending() })
	clickUI(t, tt, a.L("game_options.tab.video"))
	a.keyBindings.query = "maxFps"
	tt.Frame()
	name := a.L("game_options.option.maxFps") + " " + a.L("game_options.slider")
	r, ok := tt.Find(name)
	if !ok {
		t.Fatal("missing FPS slider")
	}
	tt.ClickAt(r.X+r.W-1, r.Y+r.H/2)
	tt.Key(0, ui.KeyEnd)
	if n, _ := strconv.ParseFloat(a.keyBindings.options["maxFps"], 64); n != 260 {
		t.Fatal("slider endpoint incorrect", a.keyBindings.options)
	}
	clickUI(t, tt, "按钮文字 keybindings-save")
	waitUI(t, tt, func() bool { return !a.dialogPending() && a.keyBindings.saved })
	if !tt.HasText(a.L("keybindings.saved")) || readPreferenceInput(t, tt, a.L("game_options.value")+" "+a.L("game_options.option.maxFps")) != a.L("game_options.unlimited") {
		t.Fatal("save feedback or unlimited label missing", tt.Texts())
	}
	v, err := a.service.ReadKeyBindingValues(context.Background(), "", "")
	if err != nil {
		t.Fatal(err)
	}
	for _, row := range v.Options {
		if row.ID == "maxFps" && row.Override != "260" {
			t.Fatal("unlimited FPS lost on disk", row)
		}
	}
	tt.Advance(400 * time.Millisecond)
	snapshotUI(t, tt, "settings-fps-saved-zh-CN")
	a.OverrideLanguage(LangEN)
	tt.Frame()
	if readPreferenceInput(t, tt, a.L("game_options.value")+" "+a.L("game_options.option.maxFps")) != "Unlimited" {
		t.Fatal("unlimited label not localized", tt.Texts())
	}
	snapshotUI(t, tt, "settings-fps-saved-en-US")
	clickUI(t, tt, a.L("game_options.option.maxFps")+" "+a.L("game_options.slider"))
	tt.Key(0, ui.KeyEnd)
	tt.Key(0, ui.KeyLeft)
	if n, _ := strconv.ParseFloat(a.keyBindings.options["maxFps"], 64); n != 259 {
		t.Fatal("could not return from unlimited", a.keyBindings.options)
	}
}

func TestGameSettingsMouseSensitivityUsesIntegerPercent(t *testing.T) {
	a, tt, saves := worldsTester(t)
	path := filepath.Join(filepath.Dir(saves), "options.txt")
	if err := os.WriteFile(path, []byte("mouseSensitivity:0.204225\n"), 0644); err != nil {
		t.Fatal(err)
	}
	r := a.root()
	a.openKeyBindings(r.ID, r.SelectedVersion)
	waitUI(t, tt, func() bool { return a.keyBindings.view != nil && !a.dialogPending() })
	clickUI(t, tt, a.L("game_options.tab.controls"))
	if readPreferenceInput(t, tt, a.L("game_options.value")+" "+a.L("game_options.option.mouseSensitivity")) != "41" || len(a.keyBindings.options) != 0 {
		t.Fatal("inherited sensitivity should display an integer without changing the file", tt.Texts())
	}
	name := a.L("game_options.option.mouseSensitivity")
	box, ok := tt.Find(name + " " + a.L("game_options.slider"))
	if !ok {
		t.Fatal("slider missing")
	}
	tt.ClickAt(box.X+box.W*.317, box.Y+box.H/2)
	n, err := strconv.ParseFloat(a.keyBindings.options["mouseSensitivity"], 64)
	if err != nil || math.Abs(n*200-math.Round(n*200)) > 1e-8 {
		t.Fatal("mouse drag did not snap to integer percent", n, err)
	}
	tt.Key(0, ui.KeyRight)
	next, _ := strconv.ParseFloat(a.keyBindings.options["mouseSensitivity"], 64)
	if math.Abs(next-n-.005) > 1e-8 {
		t.Fatal("arrow step should be 1%", n, next)
	}
	control := a.L("game_options.value") + " " + name
	replacePreferenceInput(t, tt, control, "40.8")
	if !a.keyBindings.invalid["mouseSensitivity"] {
		t.Fatal("fractional percentage accepted")
	}
	replacePreferenceInput(t, tt, control, "41")
	clickUI(t, tt, "按钮文字 keybindings-save")
	waitUI(t, tt, func() bool { return !a.dialogPending() && a.keyBindings.saved })
	b, err := os.ReadFile(path)
	if err != nil || string(b) != "mouseSensitivity:0.205\n" {
		t.Fatal("wrong game sensitivity value", string(b), err)
	}
	tt.Advance(300 * time.Millisecond)
	snapshotUI(t, tt, "settings-sensitivity-integer")
}

func TestGameSettingsBackupsCreateConfirmRestoreAndMissingFile(t *testing.T) {
	for _, global := range []bool{false, true} {
		t.Run(strconv.FormatBool(global), func(t *testing.T) {
			a, tt, saves := worldsTester(t)
			path := filepath.Join(filepath.Dir(saves), "options.txt")
			original := "\ufeffmaxFps:120\r\nmouseSensitivity:0.204225\r\ncustomOption:a:b\r\n"
			if err := os.WriteFile(path, []byte(original), 0644); err != nil {
				t.Fatal(err)
			}
			r := a.root()
			root, version := r.ID, r.SelectedVersion
			if global {
				root, version = "", ""
			}
			a.openKeyBindings(root, version)
			waitUI(t, tt, func() bool { return a.keyBindings.view != nil && !a.dialogPending() })
			clickUI(t, tt, a.L("game_options.tab.backups"))
			waitUI(t, tt, func() bool { return a.keyBindings.backupsLoaded && !a.dialogPending() })
			clickUI(t, tt, "按钮文字 settings-backup-create")
			waitUI(t, tt, func() bool { return len(a.keyBindings.backups) == 1 && !a.dialogPending() })
			id := a.keyBindings.backups[0].ID
			clickUI(t, tt, a.L("game_options.tab.video"))
			a.keyBindings.query = "maxFps"
			tt.Frame()
			replacePreferenceInput(t, tt, a.L("game_options.value")+" "+a.L("game_options.option.maxFps"), "60")
			clickUI(t, tt, "按钮文字 keybindings-save")
			waitUI(t, tt, func() bool { return !a.dialogPending() && a.keyBindings.saved })
			clickUI(t, tt, a.L("game_options.tab.backups"))
			waitUI(t, tt, func() bool { return a.keyBindings.backupsLoaded && !a.dialogPending() })
			a.keyBindings.mod, a.keyBindings.scope = "mod:removed", "mods"
			clickUI(t, tt, "按钮文字 settings-backup-restore-"+id)
			if a.confirmation != nil {
				t.Fatal("saved settings have an automatic recovery snapshot")
			}
			tt.Advance(300 * time.Millisecond)
			snapshotUI(t, tt, "settings-backup-confirm-"+strconv.FormatBool(global))
			waitUI(t, tt, func() bool {
				return !a.dialogPending() && a.keyBindings.backupMessage == "game_options.backup.restored"
			})
			if a.keyBindings.dirty() || a.keyBindings.options["maxFps"] != "" {
				t.Fatal("restore retained discarded overrides")
			}
			if a.keyBindings.mod != "all" || a.keyBindings.scope != "all" {
				t.Fatal("restore retained a filter from the replaced settings")
			}
			if !tt.HasText(a.L("game_options.backup.restored")) {
				t.Fatal("restore result hidden")
			}
			if !global {
				b, err := os.ReadFile(path)
				if err != nil || string(b) != original {
					t.Fatal("restore changed file formatting", string(b), err)
				}
				if err = os.Remove(path); err != nil {
					t.Fatal(err)
				}
				a.closeDialog()
				a.openKeyBindings(root, version)
				waitUI(t, tt, func() bool { return a.keyBindings.view != nil && !a.dialogPending() })
				clickUI(t, tt, a.L("game_options.tab.backups"))
				waitUI(t, tt, func() bool { return a.keyBindings.backupsLoaded && !a.dialogPending() })
				clickUI(t, tt, "按钮文字 settings-backup-restore-"+id)
				waitUI(t, tt, func() bool { return !a.dialogPending() && a.keyBindings.view.HasOptions })
				b, err = os.ReadFile(path)
				if err != nil || !strings.Contains(string(b), "maxFps:120") {
					t.Fatal("missing file was not restored", err)
				}
			}
			a.OverrideLanguage(LangEN)
			tt.SetSize(900, 540)
			tt.Advance(300 * time.Millisecond)
			snapshotUI(t, tt, "settings-backups-en-US-"+strconv.FormatBool(global))
		})
	}
}
