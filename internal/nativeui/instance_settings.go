package nativeui

import (
	"fmt"
	"math"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/egoist/mygo/ui"

	"github.com/Viking602/YoloMCL/internal/launcher"
)

// setCard is one titled card of the instance 设置 tab.
func setCard(c *ui.Context, key, icon, title, sub string, basis float32, fn func()) {
	s := uiScale(c)
	e := ui.Column(c.Key(key)).Label(title).Gap(14*s).Padding(18*s, 20*s).MinWidth(0)
	if basis > 0 {
		e = e.Grow(1).Basis(basis * s)
	} else {
		e = e.FillWidth()
	}
	glass(e, radiusCard*s, s).Children(func() {
		ui.Row(c).FillWidth().Gap(10 * s).AlignItems(ui.Center).Children(func() {
			ui.Box(c).Size(28*s, 28*s).Radius(8 * s).Background(accentSoft).Center().Shrink(0).Children(func() {
				ui.Icon(c, icons[icon]).Size(14*s, 14*s).TextColor(accent)
			})
			ui.Column(c).Grow(1).Basis(0).MinWidth(0).Gap(2 * s).Children(func() {
				ui.Text(c, title).FontSize(typeBody * s).FontWeight(650).TextColor(ink).SingleLine()
				if sub != "" {
					label(c, sub).FontSize(typeCaption * s).FillWidth().MinWidth(0)
				}
			})
		})
		fn()
	})
}

func gbText(mb int) string {
	v := float64(mb) / 1024
	if v == math.Trunc(v) {
		return fmt.Sprintf("%.0f GB", v)
	}
	return fmt.Sprintf("%.1f GB", v)
}

// instanceSettingsView lays the per-instance settings out as cards that
// use the width: 名称 / Java and 内存 side by side, JVM below, pack last.
func (a *App) instanceSettingsView(c *ui.Context, r launcher.Instance) {
	s := uiScale(c)
	t := a.tr()
	if a.form.ConfigVersion == "" {
		a.loadVersionConfigForm(r.SelectedVersion)
	}
	var vv *launcher.VersionSummary
	for i := range r.Versions {
		if r.Versions[i].ID == a.form.ConfigVersion {
			vv = &r.Versions[i]
		}
	}
	ui.Column(c).FillWidth().MaxWidth(1180 * s).Gap(16 * s).Children(func() {
		a.instanceScopeNote(c)
		if vv == nil {
			a.packSection(c, r)
			return
		}
		v := *vv
		ui.Row(c).FillWidth().Wrap().Gap(16 * s).AlignItems(ui.Stretch).Children(func() {
			setCard(c, "set-basic", "gamepad", a.L("名称与 Java", "Name and Java"), "", 380, func() {
				ui.Column(c).FillWidth().Gap(6 * s).Children(func() {
					label(c, t.VersionNameLabel).FontSize(typeFootnote * s).FontWeight(600)
					def := v
					def.DisplayName = ""
					textField(c.Key("ver-display-name"), &a.form.VerName).Label(t.VersionNameLabel).Placeholder(a.nameOf(r, def).Title).FillWidth()
					label(c, t.VersionNameHint).FontSize(typeCaption * s).FillWidth().MinWidth(0)
				})
				ui.Column(c).FillWidth().Gap(6 * s).Children(func() {
					ui.Row(c).Gap(8 * s).AlignItems(ui.Center).Children(func() {
						label(c, "Java").FontSize(typeFootnote * s).FontWeight(600)
						javaTag(c, javaFor(r, v))
					})
					ui.Row(c).FillWidth().Gap(8 * s).AlignItems(ui.Center).Children(func() {
						names := []string{t.RootCfgVerJavaFollow}
						paths := map[string]string{t.RootCfgVerJavaFollow: ""}
						sel := t.RootCfgVerJavaFollow
						for _, j := range a.java {
							name := fmt.Sprintf("Java %s %s (%s)", j.Version, javaDisplay(j.Vendor), javaDisplay(filepath.Base(filepath.Dir(filepath.Dir(j.Path)))))
							if need := javaFor(r, v); need > 0 && j.Major == need {
								name = a.L("推荐  ", "Recommended  ") + name
							}
							if _, dup := paths[name]; dup {
								name += "   " + j.Path
							}
							names = append(names, name)
							paths[name] = j.Path
							if j.Path == a.form.VerJava {
								sel = name
							}
						}
						if a.form.VerJava != "" && sel == t.RootCfgVerJavaFollow {
							sel = a.form.VerJava
							names = append(names, sel)
							paths[sel] = a.form.VerJava
						}
						if a.javaSelectBox(c, "ver-java-select", t.RootCfgVerJavaLabel, &sel, names, paths).Grow(1).MinWidth(0).Changed() {
							a.form.VerJava = paths[sel]
						}
						has := false
						for _, j := range a.java {
							has = has || j.Major == javaFor(r, v)
						}
						if need := javaFor(r, v); need > 0 && !has && fixedButton(c, "ver-java-get", fmt.Sprintf(a.L("下载 Java %d", "Download Java %d"), need), "download", 0, btnMd*s, true, false) {
							a.openJavaGet(need, "", false)
						}
						if iconOnlyButton(c, "ver-java-browse", "folder", t.DialogBrowseBtn, btnMd*s, false) {
							a.chooseFiles("java", func(p []string) { a.form.VerJava = p[0] })
						}
					})
					if a.form.VerJava != "" {
						label(c, a.form.VerJava).FontSize(typeCaption * s).Ellipsis("…").SingleLine().FillWidth().MinWidth(0)
					}
				})
			})
			a.memoryCard(c, v)
		})
		setCard(c, "set-jvm", "terminal", a.L("JVM 参数", "JVM arguments"), a.L("附加在启动命令里，留空就用全局设置", "Added to the launch command; leave empty to use the global setting"), 0, func() {
			textField(c.Key("ver-jvm-input"), &a.form.VerJVM).Label(t.RootCfgVerJVMLabel).Placeholder(t.RootCfgVerJVMHint).FillWidth()
			renderJVMPresetBar(c, "ver-jvm-preset", &a.form.VerJVM, t, nil)
		})
		a.packSection(c, r)
		ui.Row(c).FillWidth().Gap(10 * s).AlignItems(ui.Center).Children(func() {
			if button(c, "save-version-settings", t.RootCfgSaveBtn, "check", true, a.instancePending("updateVersion", "setVersionName")) {
				a.saveVersionConfig(r.ID)
			}
		})
	})
}

func (a *App) memoryCard(c *ui.Context, v launcher.VersionSummary) {
	s := uiScale(c)
	t := a.tr()
	mi := a.memoryInfo
	sub := ""
	if mi.TotalMB > 0 {
		sub = fmt.Sprintf(a.L("本机内存 %s，当前可用 %s", "This PC has %s, %s free now"), gbText(mi.TotalMB), gbText(mi.AvailableMB/256*256))
	}
	setCard(c, "set-mem", "layers", a.L("内存", "Memory"), sub, 380, func() {
		cur := t.MemModeManual
		if a.form.VerMemAuto {
			cur = t.MemModeAuto
		}
		ui.Row(c).FillWidth().Gap(10 * s).AlignItems(ui.Center).Children(func() {
			val := ""
			if a.form.VerMemAuto {
				if mb := a.autoMemoryFor(a.form.ConfigVersion); mb > 0 {
					val = gbText(mb)
				}
			} else if n, e := strconv.Atoi(strings.TrimSpace(a.form.VerMemory)); e == nil && n > 0 {
				val = gbText(n)
			}
			lbl := "内存值"
			if a.form.VerMemAuto {
				lbl = "自动内存值"
				if val == "" {
					val = t.MemModeAuto
				}
			}
			ui.Text(c, val).Label(lbl).FontSize(typeTitle2 * s).FontWeight(700).TextColor(accent).SingleLine().Grow(1).Basis(0).MinWidth(0)
			if picked, ok := segmented(c, "ver-mem-mode-seg", []string{t.MemModeAuto, t.MemModeManual}, cur, 25*s); ok {
				a.form.VerMemAuto = picked == t.MemModeAuto
				if !a.form.VerMemAuto {
					if n, _ := strconv.Atoi(strings.TrimSpace(a.form.VerMemory)); n < 1024 {
						mb := a.autoMemoryFor(a.form.ConfigVersion)
						if mb < 1024 {
							mb = 4096
						}
						a.form.VerMemory = fmt.Sprint(mb)
					}
				}
			}
		})
		if a.form.VerMemAuto {
			label(c, t.MemAutoExplain).FontSize(typeCaption * s).FillWidth().MinWidth(0)
			return
		}
		hi := 16384.0
		if mi.MaxMB > 0 {
			hi = float64(mi.MaxMB)
		}
		n, _ := strconv.Atoi(strings.TrimSpace(a.form.VerMemory))
		a.verMemSlider = math.Max(1024, math.Min(hi, float64(n)))
		if ui.Slider(c, &a.verMemSlider, 1024, hi).Label(a.L("内存大小", "Memory size")).FillWidth().Changed() {
			a.form.VerMemory = fmt.Sprint(int(math.Round(a.verMemSlider/256)) * 256)
		}
		ui.Row(c).FillWidth().Wrap().Gap(6 * s).AlignItems(ui.Center).Children(func() {
			for _, mb := range []int{2048, 4096, 6144, 8192, 12288, 16384} {
				if float64(mb) > hi {
					continue
				}
				m := mb
				active := strings.TrimSpace(a.form.VerMemory) == fmt.Sprint(m)
				chip := ui.ButtonBase(c.Key(fmt.Sprint("mem-preset-", m))).Label(gbText(m)).Height(26 * s).PaddingX(10 * s).Radius(999).AlignItems(ui.Center)
				bg, fg := ui.RGBA(120, 130, 160, .08), inkSecondary
				if active {
					bg, fg = accentSoft, accent
				}
				chip.Background(chip.AnimateColor("bg", bg, motionFast))
				if chip.Children(func() {
					ui.Text(c, gbText(m)).FontSize(typeFootnote * s).FontWeight(600).TextColor(fg).NoWrap()
				}).Clicked() {
					a.form.VerMemory = fmt.Sprint(m)
				}
			}
			ui.Spacer(c).Grow(1)
			textField(c.Key("ver-memory-input"), &a.form.VerMemory).Label(t.RootCfgVerMemLabel).Placeholder("4096").Width(92 * s).Shrink(0)
			label(c, "MB").FontSize(typeCaption * s).NoWrap()
		})
	})
}
