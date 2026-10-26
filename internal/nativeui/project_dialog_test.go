package nativeui

import (
	"fmt"
	"math"
	"strings"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"

	"github.com/Viking602/YoloMCL/internal/launcher"
)

func projectDialogFixture(t testing.TB, count int) (*App, *ui.Tester, *int) {
	t.Helper()
	s, err := launcher.New(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	a, err := New(s)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { a.cancel(); a.bgWG.Wait() })
	a.OverrideLanguage(LangZH)
	a.project = &launcher.Project{ID: "atm11", Source: "curseforge", Type: "modpack", Title: "All the Mods 11 - ATM11", Description: "Super early alpha for 26.1.2", Downloads: 1100000, Categories: []string{"adventure", "technology", "large"}}
	a.form.Parent, a.form.Name = t.TempDir(), a.project.Title
	a.packRemote = map[string]*launcher.PackPreview{}
	for i := range count {
		id := fmt.Sprintf("version-%03d", i)
		downloads := int64(230000 / (i + 1))
		a.projectVersions = append(a.projectVersions, launcher.ProjectVersion{
			ID: id, Number: fmt.Sprintf("All the Mods 11-0.1.%03d", count-i), Name: fmt.Sprintf("ATM11 release %d", count-i),
			GameVersions: []string{"26.1.2"}, Loaders: []string{"neoforge"}, Type: "release",
			Downloads: &downloads,
			Published: time.Now().Add(-time.Duration(i+1) * 24 * time.Hour).Format(time.RFC3339),
			Files:     []launcher.DownloadFile{{Size: 116 << 20, Primary: true}},
		})
		a.packRemote[id] = &launcher.PackPreview{}
	}
	frames := new(int)
	tt := ui.NewTester(func(c *ui.Context) {
		*frames++
		c.SetTheme(theme)
		root := ui.Box(c).Fill().Background(bg)
		a.dialogView(c, root)
	}, 1536, 1024)
	return a, tt, frames
}

func BenchmarkProjectDialogAnimation(b *testing.B) {
	for _, count := range []int{39, 400} {
		b.Run(fmt.Sprint(count), func(b *testing.B) {
			a, tt, frames := projectDialogFixture(b, count)
			b.ReportAllocs()
			b.ResetTimer()
			*frames = 0
			for b.Loop() {
				a.dialog, a.dialogOpen = "project", true
				tt.Frame()
				tt.Advance(300 * time.Millisecond)
				a.closeDialog()
				tt.Frame()
				tt.Advance(300 * time.Millisecond)
			}
			b.ReportMetric(float64(b.Elapsed().Nanoseconds())/float64(*frames)/1e6, "ms/frame")
		})
	}
}

func TestProjectDialogPreview(t *testing.T) {
	a, tt, _ := projectDialogFixture(t, 39)
	a.dialog, a.dialogOpen = "project", true
	tt.Frame()
	tt.Advance(300 * time.Millisecond)
	panel, ok := tt.Find("活动弹窗")
	button, buttonOK := tt.Find("按钮文字 install-confirm")
	if !ok || !buttonOK || button.Y+button.H > panel.Y+panel.H {
		t.Fatalf("install action outside dialog: %+v %+v", panel, button)
	}
	snapshotUI(t, tt, "project-dialog")
}

func TestProjectDialogFitsSmallAndHighDPIWindows(t *testing.T) {
	for _, layout := range []struct {
		width, height int
		dpi           float32
	}{{900, 540, 1}, {1536, 1024, 1.5}} {
		for _, lang := range []string{LangZH, LangEN} {
			t.Run(fmt.Sprintf("%dx%d-%g-%s", layout.width, layout.height, layout.dpi, lang), func(t *testing.T) {
				a, tt, _ := projectDialogFixture(t, 39)
				a.OverrideLanguage(lang)
				tt.SetSize(layout.width, layout.height)
				tt.SetScale(layout.dpi)
				a.dialog, a.dialogOpen = "project", true
				tt.Frame()
				tt.Advance(300 * time.Millisecond)
				panel, ok := tt.Find("活动弹窗")
				button, buttonOK := tt.Find("按钮文字 install-confirm")
				if !ok || !buttonOK || panel.Y < 0 || panel.X < 0 || panel.X+panel.W > float32(layout.width)+.5 || panel.Y+panel.H > float32(layout.height)+.5 || button.Y+button.H > panel.Y+panel.H {
					t.Fatalf("dialog or primary action outside viewport: %+v %+v", panel, button)
				}
				snapshotUI(t, tt, fmt.Sprintf("project-dialog-%dx%d-%g-%s", layout.width, layout.height, layout.dpi, lang))
			})
		}
	}
}

func TestProjectInstallTargetUsesAvailableWidth(t *testing.T) {
	for _, kind := range []string{"mod", "resourcepack", "shader"} {
		for _, layout := range []struct {
			width, height int
			dpi           float32
		}{{900, 540, 1}, {1536, 1024, 1}, {1536, 1024, 1.5}} {
			for _, lang := range []string{LangZH, LangEN} {
				key := fmt.Sprintf("%s-%dx%d-%g-%s", kind, layout.width, layout.height, layout.dpi, lang)
				t.Run(key, func(t *testing.T) {
					a, _, _ := projectDialogFixture(t, 8)
					a.OverrideLanguage(lang)
					a.project.Type = kind
					a.snapshot.ActiveRoot = "target"
					a.snapshot.Instances = []launcher.Instance{{
						Root:     launcher.Root{ID: "target", Name: "Games", SelectedVersion: "pack"},
						Versions: []launcher.VersionSummary{{ID: "pack", Minecraft: "26.1.2", Loader: "neoforge"}},
					}}
					a.imageRetry["version:target:pack"] = time.Now().Add(time.Hour)
					a.form.VersionID = a.projectVersions[0].ID
					a.planKey = a.currentPlanKey()
					a.plan = []launcher.PlanItem{{Key: "target", Role: "target", Title: "Example mod", Status: "install"}}
					a.dialog, a.dialogOpen = "project", true
					name := "IATM - All the Mods 11"
					var fullWidth, scale float32
					tt := ui.NewTester(func(c *ui.Context) {
						c.SetTheme(theme)
						scale = uiScale(c)
						fullWidth, _ = c.MeasureText(0, ui.Span{Text: name, Font: uiSansFamily(), Size: typeFootnote * scale, Weight: 600})
						a.gameVersion().DisplayName = name
						a.dialogView(c, ui.Box(c).Fill().Background(bg))
					}, layout.width, layout.height)
					tt.SetScale(layout.dpi)
					tt.Advance(300 * time.Millisecond)
					for _, state := range []string{"", "running", "failed", "completed"} {
						a.planJob, a.planFinished = "job", true
						a.snapshot.Jobs = nil
						if state != "" {
							a.snapshot.Jobs = []launcher.Job{{ID: "job", State: state, Message: "Downloading " + strings.Repeat("example-dependency-", 16) + ".jar", Done: 25, Total: 100}}
						}
						tt.Frame()
						text, ok := tt.Find(name)
						chip, chipOK := tt.Find("安装目标")
						panel, panelOK := tt.Find("活动弹窗")
						if !ok || !chipOK || !panelOK || text.W < fullWidth-.5 || text.X+text.W > chip.X+chip.W || chip.Y+chip.H > panel.Y+panel.H || panel.Y+panel.H > float32(layout.height) {
							t.Fatalf("target clipped in %q state despite available space: text=%+v, chip=%+v, panel=%+v, full width=%.2f", state, text, chip, panel, fullWidth)
						}
						if state == "running" {
							snapshotUI(t, tt, "install-target-running-"+key)
						}
					}
					a.planJob, a.snapshot.Jobs = "", nil
					tt.Frame()
					snapshotUI(t, tt, "install-target-"+key)

					// Only a genuinely oversized name should give way to the actions.
					name = strings.Repeat("我的冒险整合包 - All the Mods 11 ", 4)
					tt.Frame()
					text, ok := tt.Find(name)
					chip, chipOK := tt.Find("安装目标")
					cancel, cancelOK := tt.Find("按钮文字 install-cancel")
					confirm, confirmOK := tt.Find("按钮文字 install-confirm")
					panel, panelOK := tt.Find("活动弹窗")
					if !ok || !chipOK || !cancelOK || !confirmOK || !panelOK || text.W < 80*scale || text.W >= fullWidth || text.X+text.W > chip.X+chip.W || chip.X+chip.W >= cancel.X || confirm.X+confirm.W > panel.X+panel.W {
						t.Fatalf("long target displaced footer actions: text=%+v cancel=%+v confirm=%+v panel=%+v", text, cancel, confirm, panel)
					}
					tt.Move(text.X+text.W/2, text.Y+text.H/2)
					tt.Advance(time.Second)
					shown := 0
					for _, s := range tt.Texts() {
						if s == name {
							shown++
						}
					}
					if shown < 2 {
						t.Fatal("hovering a truncated target did not reveal its full name")
					}
					snapshotUI(t, tt, "install-target-long-"+key)
				})
			}
		}
	}
}

func TestProjectVersionsWindowScrollAndFilter(t *testing.T) {
	a, tt, _ := projectDialogFixture(t, 400)
	a.dialog, a.dialogOpen = "project", true
	tt.Frame()
	tt.Advance(300 * time.Millisecond)
	checkWindow := func() {
		t.Helper()
		built := 0
		for _, v := range a.projectVersions {
			if _, ok := tt.Find(v.Number); ok {
				built++
			}
		}
		if built == 0 || built > verRows+3 {
			t.Fatalf("built %d version rows for a five-row viewport", built)
		}
	}
	checkWindow()
	list, _ := tt.Find("整合包版本列表")
	tt.Scroll(list.X+list.W/2, list.Y+list.H/2, 0, 50000)
	tt.Advance(time.Second)
	checkWindow()
	last := a.projectVersions[len(a.projectVersions)-1]
	clickUI(t, tt, last.Number)
	if a.form.VersionID != last.ID {
		t.Fatal("last version is not selectable after scrolling")
	}
	a.projectVersionFilter = "no-such-version"
	tt.Frame()
	if a.form.VersionID != "" || !tt.HasText("没有符合筛选的版本") {
		t.Fatal("empty filter kept a hidden installation selection")
	}
	a.projectVersionFilter = a.projectVersions[1].Number
	tt.Frame()
	if a.versionList.track.st.Y != 0 || a.form.VersionID != a.projectVersions[1].ID {
		t.Fatal("filter did not reset the scroll and select its result")
	}
	a.projectVersionFilter = ""
	tt.Frame()
	checkWindow()
}

func TestProjectDialogMotionAndDismissal(t *testing.T) {
	for _, dismiss := range []string{"escape", "backdrop", "cancel"} {
		t.Run(dismiss, func(t *testing.T) {
			a, tt, _ := projectDialogFixture(t, 39)
			a.dialog, a.dialogOpen = "project", true
			tt.Frame()
			tt.Advance(60 * time.Millisecond)
			mid, _ := tt.Find("活动弹窗")
			tt.Advance(250 * time.Millisecond)
			end, _ := tt.Find("活动弹窗")
			if math.Abs(float64(mid.W-end.W)) > .01 || mid.Y <= end.Y {
				t.Fatalf("dialog should slide without scaling: %+v %+v", mid, end)
			}
			switch dismiss {
			case "escape":
				tt.Key(0, ui.KeyEscape)
			case "backdrop":
				tt.ClickAt(24, 24)
			case "cancel":
				clickUI(t, tt, a.L("install_dialog.cancel"))
			}
			if a.dialogOpen {
				t.Fatal("dismissal left dialog open")
			}
			tt.Advance(40 * time.Millisecond)
			if _, ok := tt.Find("活动弹窗"); !ok {
				t.Fatal("dialog disappeared before the exit animation finished")
			}
			// Reopen while leaving: the same panel continues from its current position.
			a.dialog, a.dialogOpen = "project", true
			tt.Frame()
			tt.Advance(250 * time.Millisecond)
			if _, ok := tt.Find("活动弹窗"); !ok {
				t.Fatal("interrupted exit lost the reopened dialog")
			}
			tt.SetPreferences(ui.Preferences{ReduceMotion: true})
			a.closeDialog()
			tt.Frame()
			if _, ok := tt.Find("活动弹窗"); ok {
				t.Fatal("reduced motion retained an exit animation")
			}
		})
	}
}
