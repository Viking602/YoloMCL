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

func (a *App) versionDialog(c *ui.Context) {
	s := uiScale(c)
	t := a.tr()
	r := a.root()
	if r == nil {
		return
	}

	// Section 1: Per-Version Dedicated Java, Memory & JVM Arguments
	if len(r.Versions) > 0 {
		ui.Column(c).FillWidth().Gap(13 * s).Children(func() {
			if a.form.ConfigVersion == "" {
				a.loadVersionConfigForm(r.SelectedVersion)
			}
			targetVer := a.form.ConfigVersion
			for _, ver := range r.Versions {
				if ver.ID != targetVer {
					continue
				}
				vv := ver
				if !a.embedMods {
					ui.Row(c).Label("版本设置标题").FillWidth().Gap(12 * s).AlignItems(ui.Center).Children(func() {
						if !loaderLogo(c, vv.Loader, 36*s) {
							appleSoftIconBox(c, 36*s, "gamepad", ui.Hex("#eaf3ff"), ui.Hex("#dbeafe"), accent)
						}
						ui.Column(c).Grow(1).Basis(0).MinWidth(0).Gap(2 * s).Children(func() {
							title, sub := a.cardText(*r, vv)
							titleWithTags(c, title, vv.ID, 17*s, 700, a.versionTags(*r, vv, false), 3)
							label(c, joinDistinct(sub, r.Name)).FillWidth().MinWidth(0).FontSize(typeFootnote * s).SingleLine()
						})
					})
				}
				ui.Column(c).FillWidth().Gap(6 * s).Children(func() {
					ui.Row(c).FillWidth().Gap(8 * s).AlignItems(ui.Center).Children(func() {
						label(c, t.VersionNameLabel).FontWeight(600).NoWrap().Shrink(0)
						label(c, t.VersionNameHint).FontSize(typeCaption * s).SingleLine().MinWidth(0)
					})
					def := vv
					def.DisplayName = ""
					textField(c.Key("ver-display-name"), &a.form.VerName).Label(t.VersionNameLabel).Placeholder(a.nameOf(*r, def).Title).FillWidth()
				})
			}

			// Per-Version Java Runtime Selector (with detected Java runtimes + Browse)
			ui.Row(c).Gap(8 * s).AlignItems(ui.Center).Children(func() {
				label(c, t.RootCfgVerJavaLabel).FontWeight(600)
				if r := a.root(); r != nil {
					for _, v := range r.Versions {
						if v.ID == targetVer {
							javaTag(c, javaFor(*r, v))
						}
					}
				}
			})
			ui.Row(c).FillWidth().Gap(10 * s).AlignItems(ui.Center).Children(func() {
				javaNames := []string{t.RootCfgVerJavaFollow}
				javaPaths := map[string]string{t.RootCfgVerJavaFollow: ""}
				selectedJava := t.RootCfgVerJavaFollow
				for _, j := range a.java {
					name := fmt.Sprintf("Java %s %s (%s)", j.Version, javaDisplay(j.Vendor), javaDisplay(filepath.Base(filepath.Dir(filepath.Dir(j.Path)))))
					if _, exists := javaPaths[name]; exists {
						name += "   " + j.Path
					}
					javaNames = append(javaNames, name)
					javaPaths[name] = j.Path
					if j.Path == a.form.VerJava {
						selectedJava = name
					}
				}
				if a.form.VerJava != "" && selectedJava == t.RootCfgVerJavaFollow {
					selectedJava = a.form.VerJava
					javaNames = append(javaNames, selectedJava)
					javaPaths[selectedJava] = a.form.VerJava
				}
				if a.javaSelectBox(c, "ver-java-select", t.RootCfgVerJavaLabel, &selectedJava, javaNames, javaPaths).Grow(1).MinWidth(0).Changed() {
					a.form.VerJava = javaPaths[selectedJava]
				}
				if button(c, "ver-java-browse", t.DialogBrowseBtn, "folder", false, a.dialogPending()) {
					a.chooseFiles("java", func(paths []string) { a.form.VerJava = paths[0] })
				}
			})
			if a.form.VerJava != "" {
				label(c, a.form.VerJava).FontSize(typeFootnote * s).Ellipsis("…")
			}

			// Per-Version Max Memory: 自动 / 手动
			a.memoryModeRow(c, "ver-mem-mode", t.RootCfgVerMemLabel, &a.form.VerMemAuto, a.autoMemoryFor(a.form.ConfigVersion))
			if !a.form.VerMemAuto {
				ui.Row(c).FillWidth().Wrap().Gap(8 * s).AlignItems(ui.Center).Children(func() {
					for _, preset := range []struct {
						label string
						mb    string
					}{
						{"2 GB", "2048"},
						{"4 GB", "4096"},
						{"6 GB", "6144"},
						{"8 GB", "8192"},
						{"12 GB", "12288"},
						{"16 GB", "16384"},
					} {
						p := preset
						active := strings.TrimSpace(a.form.VerMemory) == p.mb
						chip := ui.ButtonBase(c.Key("mem-preset-"+p.mb)).Label(p.label).Height(btnSm*s).PaddingX(11*s).Radius(6*s).Border(1, line).AlignItems(ui.Center)
						if active {
							chip.Background(chip.AnimateColor("bg", accent, motionFast)).TextColor(panel)
						} else {
							chip.Background(chip.AnimateColor("bg", fieldColor, motionFast)).TextColor(inkSecondary)
						}
						if chip.Children(func() {
							ui.Text(c, p.label).FontSize(typeFootnote * s).NoWrap()
						}).Clicked() {
							a.form.VerMemory = p.mb
						}
					}
				})
				textField(c.Key("ver-memory-input"), &a.form.VerMemory).Label(t.RootCfgVerMemLabel + "输入").Placeholder("4096").FillWidth()
			}

			// Per-Version Custom JVM Arguments & Recommended Presets
			label(c, t.RootCfgVerJVMLabel).FontWeight(600)
			textField(c.Key("ver-jvm-input"), &a.form.VerJVM).Label(t.RootCfgVerJVMLabel + "输入").Placeholder(t.RootCfgVerJVMHint).FillWidth()
			renderJVMPresetBar(c, "ver-jvm-preset", &a.form.VerJVM, t, nil)
		})
	}
	a.packSection(c, *r)
}

func (a *App) rootDialog(c *ui.Context) {
	s := uiScale(c)
	t := a.tr()
	r := a.root()
	if r == nil {
		return
	}
	// Section 2: Instance Directory & Default Settings
	ui.Column(c).FillWidth().Gap(12*s).Padding(18*s).Radius(12*s).Background(raised).Border(1, line).Children(func() {
		ui.Row(c).FillWidth().AlignItems(ui.Center).Gap(10 * s).Children(func() {
			ui.Icon(c, icons["folder"]).Size(18*s, 18*s).TextColor(accent).Shrink(0)
			pathLabel(c, r.Path, 13*s)
			if button(c, "root-folder", t.HomeOpenFolderBtn, "folder", false, false) {
				a.openFolder(r.ID)
			}
			if button(c, "root-logs", t.HomeLogsBtn, "terminal", false, false) {
				a.openDialog("logs")
			}
		})
		field(c, t.AddRootNameLabel, &a.form.Name, "")
		ui.Column(c).FillWidth().Gap(6 * s).Children(func() {
			label(c, a.L("runtime_dialog.default_directory_java")).FontWeight(600)
			ui.Row(c).FillWidth().Gap(8 * s).AlignItems(ui.Center).Children(func() {
				follow := a.L("runtime_dialog.use_global_java_setting")
				names, paths := []string{follow}, map[string]string{follow: ""}
				selected := follow
				for _, j := range a.java {
					name := fmt.Sprintf("Java %s  %s", j.Version, javaDisplay(j.Vendor))
					if _, exists := paths[name]; exists {
						name += "  " + j.Path
					}
					names, paths[name] = append(names, name), j.Path
					if strings.EqualFold(j.Path, a.form.Java) {
						selected = name
					}
				}
				if a.form.Java != "" && selected == follow {
					selected = a.form.Java
					names, paths[selected] = append(names, selected), selected
				}
				if a.javaSelectBox(c, "root-java-select", t.RootCfgJavaLabel, &selected, names, paths).Grow(1).MinWidth(0).Changed() {
					a.form.Java = paths[selected]
				}
				if iconOnlyButton(c, "root-java-browse", "folder", t.DialogBrowseBtn, btnMd*s, false) {
					a.chooseFiles("java", func(paths []string) { a.form.Java = paths[0] })
				}
			})
		})
		a.memoryModeRow(c, "root-mem-mode", a.L("runtime_dialog.default_directory_memory"), &a.form.MemAuto, a.autoMemoryFor(r.SelectedVersion))
		if !a.form.MemAuto {
			hi := min(65536., max(1024., a.memoryMaximum()))
			mb, _ := strconv.Atoi(strings.TrimSpace(a.form.Memory))
			if mb < 512 {
				mb = max(1024, a.autoMemoryFor(r.SelectedVersion))
			}
			position := math.Max(512, math.Min(hi, float64(mb)))
			ui.Row(c).FillWidth().Gap(16 * s).AlignItems(ui.Center).Children(func() {
				if ui.Slider(c.Key("root-memory-slider"), &position, 512, hi).Label(a.L("runtime_dialog.directory_memory_slider")).Step(256).Grow(1).MinWidth(0).Height(btnMd * s).Changed() {
					position = math.Max(512, math.Min(hi, math.Round(position/256)*256))
				}
				a.form.Memory = strconv.Itoa(int(position))
				numText(c, gbText(int(position)), typeBody*s, accent).Label(a.L("runtime_dialog.directory_memory_value")).FontWeight(700).Width(88 * s).TextAlign(ui.End)
			})
			ui.Row(c).FillWidth().Justify(ui.SpaceBetween).Children(func() {
				label(c, "512 MB").FontSize(typeCaption * s)
				label(c, gbText(int(hi))).FontSize(typeCaption * s)
			})
		}
	})
}

// saveRootConfig saves only the directory's name and default Java/memory.
func (a *App) saveRootConfig(rootID string) {
	t := a.tr()
	memStr := strings.TrimSpace(a.form.Memory)
	if memStr == "" {
		memStr = "0"
	}
	n, e := strconv.Atoi(memStr)
	if a.form.MemAuto {
		n, e = -1, nil // explicit directory auto
	}
	if e != nil || (n > 0 && (n < 512 || n > 65536)) || n == 0 {
		a.message = t.RootCfgMemErr
		a.errorMessage = true
		return
	}
	a.execute("updateRoot", map[string]any{"rootId": rootID, "name": a.form.Name, "javaPath": a.form.Java, "memoryMB": n, "jvmArgs": a.form.JVM}, a.finished(t.RootCfgSaved))
}

// Save the overrides and name in one background request, so leaving the view
// cannot interrupt a second write previously chained through a UI callback.
func (a *App) saveVersionConfig(rootID string) {
	t := a.tr()
	verMemStr := strings.TrimSpace(a.form.VerMemory)
	if verMemStr == "" {
		verMemStr = "0"
	}
	vn, ve := strconv.Atoi(verMemStr)
	if a.form.VerMemAuto {
		vn, ve = 0, nil
		if r := a.root(); r != nil {
			probe := *r
			probe.VersionConfigs = nil
			if inherited, _ := probe.MemoryPlan(a.form.ConfigVersion, a.settings); !inherited {
				vn = -1 // directory/global is manual: pin auto on this version
			}
		}
	}
	if ve != nil || (vn > 0 && (vn < 512 || vn > 65536)) || (!a.form.VerMemAuto && vn == 0) {
		a.message = t.RootCfgMemErr
		a.errorMessage = true
		return
	}
	ver, name := strings.TrimSpace(a.form.ConfigVersion), strings.TrimSpace(a.form.VerName)
	a.execute("updateVersion", map[string]any{"rootId": rootID, "version": ver, "javaPath": strings.TrimSpace(a.form.VerJava), "memoryMB": vn, "jvmArgs": strings.TrimSpace(a.form.VerJVM), "displayName": name}, a.finished(t.RootCfgSaved))
}

// autoMemoryFor is the computed automatic heap (MiB) of a version of the
// current directory, 0 when unknown.
func (a *App) autoMemoryFor(versionID string) int {
	if r := a.root(); r != nil {
		for _, v := range r.Versions {
			if v.ID == versionID {
				st := a.statusCache[r.ID+"|"+versionID]
				if a.instStatusKey == r.ID+"|"+versionID {
					st = a.instStatus
				}
				return a.versionAutoMemory(*r, v, st)
			}
		}
	}
	return 0
}

func (a *App) versionAutoMemory(r launcher.Instance, v launcher.VersionSummary, st launcher.InstanceStatus) int {
	if st.Dir == "" && v.AutoMemoryMB > 0 {
		return v.AutoMemoryMB
	}
	mods := st.EnabledMods
	if st.Dir == "" {
		mods = r.ModCount
	}
	// Free memory is deliberately not an input: it would make this number
	// differ from Snapshot's and flicker while idle.
	return launcher.AutoMemoryMB(launcher.AutoMemoryInput{TotalMB: a.memoryInfo.TotalMB, Minecraft: v.Minecraft, Loader: v.Loader, Mods: mods})
}

// memoryModeRow is the 自动 / 手动 switch with the computed value and a
// one-line explanation when automatic.
func (a *App) memoryModeRow(c *ui.Context, key, title string, auto *bool, autoMB int) {
	s := uiScale(c)
	t := a.tr()
	ui.Column(c.Key(key)).FillWidth().Gap(6 * s).Children(func() {
		ui.Row(c).FillWidth().Gap(10 * s).AlignItems(ui.Center).Children(func() {
			label(c, title).FontWeight(600).Grow(1).MinWidth(0).SingleLine()
			cur := t.MemModeManual
			if *auto {
				cur = t.MemModeAuto
			}
			if picked, ok := segmented(c, key+"-seg", []string{t.MemModeAuto, t.MemModeManual}, cur, 25*s); ok {
				*auto = picked == t.MemModeAuto
			}
		})
		if *auto {
			val := t.MemModeAuto
			if autoMB > 0 {
				val = fmt.Sprintf(t.MemAutoValue, float64(autoMB)/1024)
			}
			ui.Text(c, val).Label("自动内存值").FontSize(typeCallout * s).FontWeight(700).TextColor(accent).SingleLine()
			label(c, t.MemAutoExplain).FontSize(typeCaption * s).FillWidth().MinWidth(0)
		}
	})
}
