package nativeui

import (
	"context"
	"fmt"
	"math"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/egoist/mygo/ui"
	"mcl/internal/launcher"
)

const (
	jvmPresetG1      = "-XX:+UseG1GC -XX:+ParallelRefProcEnabled -XX:MaxGCPauseMillis=50 -XX:+UnlockExperimentalVMOptions -XX:+DisableExplicitGC -XX:G1NewSizePercent=20 -XX:G1ReservePercent=20 -XX:G1HeapRegionSize=32M"
	jvmPresetZGC     = "-XX:+UseZGC -XX:+UnlockExperimentalVMOptions -XX:+DisableExplicitGC -XX:+AlwaysPreTouch"
	jvmPresetCompact = "-XX:+UseG1GC -XX:MaxGCPauseMillis=35 -XX:+UseStringDeduplication -XX:G1HeapRegionSize=16M"
)

func (a *App) refreshMemory() {
	info, e := launcher.SystemMemory()
	if e != nil {
		a.memoryError = e.Error()
		return
	}
	a.memoryInfo = info
	a.memoryError = ""
	a.memory = math.Max(512, math.Min(float64(info.MaxMB), a.memory))
	a.memoryText = fmt.Sprint(int(a.memory))
}

func (a *App) memoryMaximum() float64 {
	if a.memoryInfo.MaxMB > 0 {
		return float64(a.memoryInfo.MaxMB)
	}
	return max(32768, a.memory)
}

func (a *App) startJavaScan(deep bool) {
	if a.javaScanning {
		if !deep || a.javaDeep {
			return
		}
		if a.javaScanCancel != nil {
			a.javaScanCancel()
		}
	}
	a.javaScanRevision++
	revision := a.javaScanRevision
	var ctx context.Context
	var cancel context.CancelFunc
	if deep {
		ctx, cancel = context.WithCancel(a.ctx)
	} else {
		ctx, cancel = context.WithTimeout(a.ctx, 12*time.Second)
	}
	a.javaScanCancel = cancel
	a.javaScanning = true
	a.javaDeep = deep
	a.javaPhase = "正在检测已安装的 Java"
	a.javaScanned = 0
	a.javaElapsed = 0
	go func() {
		defer cancel()
		_, e := a.service.ScanJava(ctx, deep, func(p launcher.JavaScanProgress) {
			a.post(func() {
				if revision != a.javaScanRevision {
					return
				}
				a.java = p.Found
				a.javaScanning = p.Running
				a.javaScanned = p.Visited
				a.javaElapsed = p.ElapsedMS
				a.javaPhase = p.Phase
				a.javaScannedOnce = true
			})
		})
		a.post(func() {
			if revision != a.javaScanRevision {
				return
			}
			a.javaScanning = false
			if e == context.Canceled {
				a.javaPhase = "扫描已停止"
			} else if e != nil {
				a.javaPhase = "快速扫描已结束，可继续全盘扫描"
			} else {
				a.javaPhase = a.formatJavaScanSummary(len(a.java), a.javaElapsed)
			}
		})
	}()
}

// settingsGroup is a System Settings-style inset grouped list: a caption
// header above a frosted card, rows split by hairlines inset to the text
// column, and an optional footnote below explaining the whole group.
func settingsGroup(c *ui.Context, key, titleText, desc string, rows ...func()) {
	s := uiScale(c)
	ui.Column(c).FillWidth().Gap(space2 * s).Children(func() {
		ui.Text(c, titleText).FontSize(typeCallout * s).FontWeight(650).TextColor(inkSecondary).PaddingX(space4 * s).SingleLine()
		glass(ui.Column(c.Key(key)).Label(key).FillWidth().Clip(), radiusCard*s-4*s, s).Children(func() {
			for i, row := range rows {
				if i > 0 {
					ui.Box(c).Height(1).FillWidth().MarginX(settingPadX * s).Background(line).Shrink(0)
				}
				row()
			}
		})
		_ = desc // group descriptions are omitted to keep the page calm
	})
}

// Row geometry: 18 padding gives titles, dividers and below-row controls one text column.
const (
	settingPadX      float32 = 18
	settingIcon      float32 = 32
	settingGap       float32 = 14
	settingTextInset         = settingPadX + settingIcon + settingGap
)

// settingRow lays out one row: leading icon, title and description on the
// left, the control on the right, and optionally a full-width control below
// aligned with the title.
func settingRow(c *ui.Context, leading func(), titleText, desc string, trailing, below func()) {
	s := uiScale(c)
	ui.Column(c).FillWidth().Padding(space2*s+2*s, settingPadX*s).Gap(space2 * s).Children(func() {
		ui.Row(c).FillWidth().MinHeight(36 * s).Gap(settingGap * s).AlignItems(ui.Center).Children(func() {
			if leading != nil {
				leading()
			}
			ui.Column(c).Grow(1).MinWidth(0).Clip().Gap(3 * s).Children(func() {
				ui.Text(c, titleText).FontSize(typeBody * s).FontWeight(600).TextColor(ink).SingleLine()
				if desc != "" {
					label(c, desc).FillWidth().MinWidth(0).FontSize(12.5 * s) // descriptions wrap, never ellipsize
				}
			})
			if trailing != nil {
				ui.Row(c).Shrink(0).Gap(space2 * s).AlignItems(ui.Center).Children(trailing)
			}
		})
		if below != nil {
			ui.Column(c).FillWidth().Gap(space3 * s).Children(below)
		}
	})
}

// rowIcon used to give each settings row a colored squircle; the decluttered
// layout drops them so titles carry the hierarchy. Kept for call-site clarity.
func rowIcon(c *ui.Context, icon string, tone [2]ui.Color) func() {
	return nil
}

func rowIconBox(c *ui.Context, icon string, tone [2]ui.Color) func() {
	return func() {
		s := uiScale(c)
		ui.Box(c).Size(settingIcon*s, settingIcon*s).Radius(9*s).Shrink(0).LinearGradient(tintGradient(tone[0], tone[1])).InsetShadow(0, 1, 0, 0, ui.RGBA(255, 255, 255, .3)).Center().Children(func() {
			ui.Icon(c, icons[icon]).Size(18*s, 18*s).TextColor(panel)
		})
	}
}

// segmented is a macOS segmented control; it returns the clicked option.
func segmented(c *ui.Context, key string, options []string, selected string, height float32) (string, bool) {
	s := uiScale(c)
	clicked, changed := selected, false
	track := ui.Row(c.Key(key)).Shrink(0).Padding(3 * s).Radius(height/2*.62 + 3*s).Background(fieldColor)
	track.Children(func() {
		var index float32
		for i, option := range options {
			if option == selected {
				index = float32(i)
			}
		}
		slide := track.AnimateTo("thumb", index, motionMove)
		track.Draw(func(p *ui.Painter, r ui.Rect) {
			if len(options) == 0 {
				return
			}
			inner := r.W - 6*s
			cell := inner / float32(len(options))
			thumb := ui.Rect{X: r.X + 3*s + cell*slide, Y: r.Y + 3*s, W: cell, H: r.H - 6*s}
			p.Shadow(thumb, height/2*.62, 0, 1*s, 3*s, 0, ui.RGBA(40, 52, 110, .14))
			p.Fill(thumb, panel, height/2*.62)
		})
		for _, option := range options {
			opt := option
			active := opt == selected
			b := ui.ButtonBase(c.Key(key + "-" + opt)).Label(opt).Height(height).PaddingX(12 * s).Radius(height / 2 * .62).AlignItems(ui.Center).Justify(ui.Center)
			fg := inkSecondary
			if active {
				fg = ink
			} else if b.Hovered() {
				b.Background(ui.RGBA(255, 255, 255, .4))
			}
			if b.Children(func() {
				ui.Text(c, opt).FontSize(min(14*s, height*.5)).FontWeight(600).TextColor(fg).NoWrap()
			}).Clicked() {
				clicked, changed = opt, true
			}
		}
	})
	return clicked, changed
}

func nativeToggle(c *ui.Context, key, label string, on *bool) ui.Element {
	s := uiScale(c)
	toggle := ui.SwitchBase(c.Key(key), on).Label(label).Size(52*s, 31*s).Shrink(0)
	targetFill := ui.RGBA(120, 120, 128, .2)
	targetOffset := float32(2)
	if *on {
		targetFill = success
		targetOffset = 23
	}
	animOffset := toggle.AnimateTo("knob", targetOffset, motionMove)
	toggle.Radius(15.5 * s).Background(toggle.AnimateColor("bg", targetFill, fastAnim)).Draw(func(p *ui.Painter, r ui.Rect) {
		knob := ui.Rect{X: r.X + animOffset*s, Y: r.Y + 2*s, W: 27 * s, H: 27 * s}
		p.Shadow(knob, 13.5*s, 0, 2*s, 5*s, 0, ui.RGBA(0, 0, 0, .18))
		p.Fill(knob, panel, 13.5*s)
	})
	return toggle
}

func jvmAdviceText(raw string, t *Copy) string {
	v := strings.TrimSpace(raw)
	switch {
	case strings.Contains(v, "-XX:+UseZGC"):
		return t.JVMAdviceZGC
	case strings.Contains(v, "-XX:+UseStringDeduplication"):
		return t.JVMAdviceCompact
	case strings.Contains(v, "-XX:+UseG1GC"):
		return t.JVMAdviceG1
	default:
		return t.JVMAdviceDefault
	}
}

func renderJVMPresetBar(c *ui.Context, keyPrefix string, target *string, t *Copy, changed func()) {
	s := uiScale(c)
	ui.Row(c).FillWidth().Wrap().Gap(8 * s).AlignItems(ui.Center).Children(func() {
		label(c, t.JVMPresetTitle).FontSize(12.5 * s).FontWeight(600).NoWrap().Shrink(0)
		for _, item := range []struct {
			id, label, args string
		}{
			{"g1", t.JVMPresetG1, jvmPresetG1},
			{"zgc", t.JVMPresetZGC, jvmPresetZGC},
			{"compact", t.JVMPresetCompact, jvmPresetCompact},
		} {
			preset := item
			active := strings.TrimSpace(*target) == preset.args
			bgCol := ui.RGBA(255, 255, 255, .7)
			fgCol := inkSecondary
			borderCol := lineStrong
			if active {
				bgCol = accentSoft
				fgCol = accent
				borderCol = ui.RGBA(0, 122, 255, .35)
			}
			chip := ui.ButtonBase(c.Key(keyPrefix+"-"+preset.id)).Label(preset.label).Height(30*s).PaddingX(12*s).Radius(999).Border(1, borderCol).AlignItems(ui.Center)
			if chip.Hovered() && !active {
				bgCol = panel
			}
			chip.Background(chip.AnimateColor("bg", bgCol, fastAnim))
			if chip.Children(func() {
				ui.Text(c, preset.label).FontSize(12.5 * s).FontWeight(600).TextColor(fgCol).NoWrap()
			}).Clicked() {
				*target = preset.args
				if changed != nil {
					changed()
				}
			}
		}
		if strings.TrimSpace(*target) != "" {
			clearChip := ui.ButtonBase(c.Key(keyPrefix + "-clear")).Label(t.JVMPresetClear).Height(30 * s).PaddingX(12 * s).Radius(999).Background(fieldColor).AlignItems(ui.Center)
			if clearChip.Children(func() {
				ui.Text(c, t.JVMPresetClear).FontSize(12.5 * s).TextColor(muted).NoWrap()
			}).Clicked() {
				*target = ""
				if changed != nil {
					changed()
				}
			}
		}
	})
	ui.Row(c).FillWidth().Clip().Padding(9*s, 12*s).Radius(12 * s).LinearGradient(ui.LinearGradient{From: ui.RGBA(10, 132, 255, .08), To: ui.RGBA(175, 82, 222, .07), Angle: 90, Oklab: true}).Gap(8 * s).AlignItems(ui.Center).Children(func() {
		ui.Icon(c, icons["info"]).Size(14*s, 14*s).TextColor(accent).Shrink(0)
		label(c, jvmAdviceText(*target, t)).Grow(1).MinWidth(0).FontSize(12.5 * s)
	})
}

// preferences is the settings page: a fixed header that keeps the save
// actions in reach, over a scrolling two-column body of grouped lists.
func (a *App) preferences(c *ui.Context) {
	s := uiScale(c)
	ui.Column(c.Key("settings-layout")).Grow(1).MinHeight(0).FillWidth().Children(func() {
		// Fixed header (title + live status + reset) above the scrolling body,
		// sharing the page's content width so both edges line up.
		ui.Box(c).FillWidth().Padding(pageTop*s, pageGutter*s, space4*s).Shrink(0).Children(func() {
			ui.Row(c).FillWidth().MaxWidth(settingsColumnWidth * s).AlignSelf(ui.Center).Children(func() { a.settingsHeader(c) })
		})
		scrollArea(c, "settings-scroll").Grow(1).MinHeight(0).FillWidth().Padding(space2*s, pageGutter*s, a.scrollEndPad(s)+(space8-space6)*s).Children(func() {
			// One calm column of grouped lists, like macOS System Settings:
			// general preferences, launch runtime, then a quiet
			// about row as the page footer.
			ui.Column(c).FillWidth().MaxWidth(settingsColumnWidth * s).AlignSelf(ui.Center).Gap(space5 * s).Children(func() {
				a.versionAndLangSettings(c)
				a.runtimeSettings(c)
				a.aboutSettings(c)
			})
		})
	})
}

// settingsColumnWidth caps line length on very wide windows.
const settingsColumnWidth float32 = 860

func (a *App) settingsHeader(c *ui.Context) {
	s := uiScale(c)
	t := a.tr()
	// The apply hint is secondary text under the title; results of changes
	// go to the shared toast rather than replacing it.
	pageHeader(c, func() { a.backCrumb(c) }, t.SettingsTitle, t.SettingsDesc+"  ·  "+t.SettingsDefaultHint, func() {
		ui.Row(c.Key("settings-actions")).Label("设置操作栏").Height(44 * s).Shrink(1).MinWidth(0).Gap(space3 * s).AlignItems(ui.Center).Children(func() {
			if fixedButton(c, "reset-settings", t.SettingsResetBtn, "refresh", 116*s, 40*s, false, a.busy) {
				recommended := 4096
				if a.memoryInfo.RecommendedMB > 0 {
					recommended = a.memoryInfo.RecommendedMB
				}
				a.settings = launcher.Settings{Language: a.lang(), MemoryMB: recommended}
				a.memory = float64(recommended)
				a.memoryText = fmt.Sprint(recommended)
				a.persistSettings(t.SettingsResetNotice)
			}
		})
	})
}

func (a *App) runtimeSettings(c *ui.Context) {
	s := uiScale(c)
	t := a.tr()
	settingsGroup(c, "启动环境面板", t.SettingsRuntimeTitle, "",
		func() { a.javaRow(c) },
		func() { a.memoryRow(c) },
		func() {
			settingRow(c, nil, t.SettingsRootOverride, t.SettingsRootOverrideDesc, func() {
				if nativeToggle(c, "root-settings-toggle", t.SettingsRootOverrideAria, &a.settings.UseRootSettings).Changed() {
					a.persistSettings("")
				}
			}, nil)
		},
		func() {
			autoOn := !a.settings.PackAutoCheckOff
			settingRow(c, nil, t.SettingsPackAutoCheck, t.SettingsPackAutoCheckDesc, func() {
				if nativeToggle(c, "pack-autocheck-toggle", t.SettingsPackAutoCheckAria, &autoOn).Changed() {
					a.settings.PackAutoCheckOff = !autoOn
					a.persistSettings("")
				}
			}, nil)
		},
		func() {
			settingRow(c, nil, t.SettingsPackBeta, t.SettingsPackBetaDesc, func() {
				if nativeToggle(c, "pack-beta-toggle", t.SettingsPackBetaAria, &a.settings.PackIncludeBeta).Changed() {
					a.persistSettings("")
				}
			}, nil)
		},
	)
	// Advanced: JVM arguments stay collapsed until asked for.
	ui.Column(c).FillWidth().Gap(space2 * s).Children(func() {
		toggle := ui.ButtonBase(c.Key("advanced-toggle")).Label(t.SettingsAdvancedTitle).AlignSelf(ui.Start).Height(32 * s).PaddingX(space4 * s).Gap(space2 * s).AlignItems(ui.Center).Radius(radiusControl * s)
		if toggle.Hovered() {
			toggle.Background(ui.RGBA(255, 255, 255, .5))
		}
		if toggle.Children(func() {
			ui.Text(c, t.SettingsAdvancedTitle).FontSize(typeCallout * s).FontWeight(650).TextColor(inkSecondary).NoWrap()
			ic := "chevron"
			if a.advancedOpen || a.settings.JVMArgs != "" {
				ic = "chevron-down"
			}
			ui.Icon(c, icons[ic]).Size(11*s, 11*s).TextColor(muted)
		}).Clicked() {
			// Configured arguments keep the section open so they are never hidden.
			a.advancedOpen = !a.advancedOpen
		}
		if a.advancedOpen || a.settings.JVMArgs != "" {
			glass(ui.Column(c.Key("高级设置面板")).FillWidth().Clip(), radiusCard*s-4*s, s).Children(func() {
				settingRow(c, nil, t.SettingsJVMArgsLabel, "", nil, func() {
					if ui.TextInput(c.Key("global-jvm-input"), &a.settings.JVMArgs).Label(t.SettingsJVMArgsAria).Placeholder(t.SettingsJVMArgsHint).FillWidth().Height(40 * s).Changed() {
						a.persistSettings("")
					}
					renderJVMPresetBar(c, "global-jvm-preset", &a.settings.JVMArgs, t, func() { a.persistSettings("") })
				})
			})
		}
	})
}

func (a *App) javaRow(c *ui.Context) {
	s := uiScale(c)
	t := a.tr()
	hint := t.SettingsJavaAutoHint
	if a.settings.JavaPath != "" {
		hint = a.settings.JavaPath
	} else if a.javaScanning {
		hint = t.SettingsJavaScanningHint
	}
	settingRow(c, rowIcon(c, "cube", [2]ui.Color{ui.Hex("#ff9f0a"), ui.Hex("#ff453a")}), t.SettingsJavaLabel, hint, func() {
		if fixedButton(c, "choose-java", t.SettingsJavaBrowseBtn, "folder", 92*s, 34*s, false, false) {
			a.chooseFiles("java", func(paths []string) {
				a.settings.JavaPath = paths[0]
				a.persistSettings("")
			})
		}
		if fixedButton(c, "detect-java", t.SettingsJavaScanBtn, "search", 92*s, 34*s, false, false) {
			a.dialog = "java"
			a.dialogOpen = true
			if !a.javaScanning {
				a.startJavaScan(false)
			}
		}
	}, func() {
		names := []string{t.SettingsJavaAuto}
		paths := map[string]string{t.SettingsJavaAuto: ""}
		selected := t.SettingsJavaAuto
		for _, j := range a.java {
			name := fmt.Sprintf("Java %s · %s", j.Version, javaDisplay(filepath.Base(filepath.Dir(filepath.Dir(j.Path)))))
			if _, ok := paths[name]; ok {
				name += " / " + j.Path
			}
			names = append(names, name)
			paths[name] = j.Path
			if j.Path == a.settings.JavaPath {
				selected = name
			}
		}
		if a.settings.JavaPath != "" && selected == t.SettingsJavaAuto {
			selected = t.SettingsJavaCustom
			names = append(names, selected)
			paths[selected] = a.settings.JavaPath
		}
		if selectBox(c, "java-selection", t.SettingsJavaAria, &selected, names).FillWidth().Height(40 * s).Changed() {
			a.settings.JavaPath = paths[selected]
			a.persistSettings("")
		}
	})
}

func (a *App) memoryRow(c *ui.Context) {
	s := uiScale(c)
	t := a.tr()
	memoryHint := a.memoryError
	if a.memoryInfo.TotalMB > 0 {
		memoryHint = a.formatSystemMemory(a.memoryInfo.TotalMB, a.memoryInfo.AvailableMB)
	}
	auto := a.settings.MemoryMode != "manual"
	settingRow(c, rowIcon(c, "layers", [2]ui.Color{ui.Hex("#34c759"), ui.Hex("#30b0c7")}), t.SettingsMemoryLabel, memoryHint, func() {
		cur := t.MemModeManual
		if auto {
			cur = t.MemModeAuto
		}
		if picked, ok := segmented(c, "global-mem-mode", []string{t.MemModeAuto, t.MemModeManual}, cur, 30*s); ok && picked != cur {
			a.settings.MemoryMode = ""
			if picked == t.MemModeManual {
				a.settings.MemoryMode = "manual"
			}
			a.persistSettings("")
		}
		if auto {
			return
		}
		ui.Text(c, fmt.Sprintf("%.1f GB", a.memory/1024)).FontSize(15 * s).Bold().TextColor(accent).NoWrap()
		input := ui.TextInput(c.Key("memory-input"), &a.memoryText).Label(t.SettingsMemoryInputAria).Width(84*s).Height(34*s).Padding(5*s, 10*s)
		if input.Changed() {
			if value, e := strconv.Atoi(a.memoryText); e == nil && value >= 512 && value <= int(a.memoryMaximum()) {
				a.memory = float64(value)
				a.persistSettings("")
			}
		}
		if !input.Focused() {
			a.memoryText = fmt.Sprint(int(a.memory))
		}
		label(c, "MB").FontSize(13.5 * s).FontWeight(600).NoWrap()
	}, func() {
		if auto {
			label(c, t.MemAutoExplain).FontSize(12.5 * s).FillWidth().MinWidth(0)
			return
		}
		ui.Row(c).FillWidth().Gap(14 * s).AlignItems(ui.Center).Children(func() {
			minMB, maxMB := 1024., a.memoryMaximum()
			if maxMB < 1024 {
				minMB = 512
			}
			lo, hi := math.Log2(minMB), math.Log2(maxMB)
			if hi <= lo {
				hi = lo + 1
			}
			position := math.Max(lo, math.Min(hi, math.Log2(max(a.memory, 512))))
			slider := ui.SliderBase(c.Key("memory-slider"), &position, lo, hi).Label(t.SettingsMemorySliderAria).Grow(1).MinWidth(0).Height(28 * s).PaddingX(10 * s).Step(.025)
			if slider.Changed() {
				value := math.Pow(2, position)
				a.memory = math.Max(minMB, math.Min(maxMB, math.Round(value/128)*128))
				a.memoryText = fmt.Sprint(int(a.memory))
				a.persistSettings("")
			}
			slider.Draw(func(p *ui.Painter, r ui.Rect) {
				track := ui.Rect{X: r.X + 10*s, Y: r.Y + r.H/2 - 3*s, W: r.W - 20*s, H: 6 * s}
				fraction := float32((position - lo) / (hi - lo))
				p.Fill(track, ui.RGBA(120, 120, 128, .18), 3*s)
				p.FillGradient(ui.Rect{X: track.X, Y: track.Y, W: max(track.W*fraction, 6*s), H: track.H}, appleAccentGradient(false), 3*s)
				knob := ui.Rect{X: track.X + track.W*fraction - 11*s, Y: r.Y + r.H/2 - 11*s, W: 22 * s, H: 22 * s}
				p.Shadow(knob, 11*s, 0, 2*s, 6*s, 0, ui.RGBA(0, 0, 0, .2))
				p.Fill(knob, panel, 11*s)
			})
			var presets []string
			for _, mb := range []int{2048, 4096, 8192, 12288} {
				if float64(mb) <= a.memoryMaximum() {
					presets = append(presets, fmt.Sprintf("%dG", mb/1024))
				}
			}
			current := fmt.Sprintf("%dG", int(a.memory)/1024)
			if int(a.memory)%1024 != 0 {
				current = ""
			}
			if picked, ok := segmented(c, "global-mem", presets, current, 26*s); ok {
				gb, _ := strconv.Atoi(strings.TrimSuffix(picked, "G"))
				a.memory = float64(gb * 1024)
				a.memoryText = strconv.Itoa(gb * 1024)
				a.persistSettings("")
			}
		})
	})
}

func (a *App) versionAndLangSettings(c *ui.Context) {
	s := uiScale(c)
	t := a.tr()
	settingsGroup(c, "游戏版本面板", t.SettingsPrefsTitle, "",
		func() {
			settingRow(c, rowIcon(c, "globe", [2]ui.Color{ui.Hex("#0a84ff"), ui.Hex("#5e5ce6")}), t.SettingsLanguageLabel, "", func() {
				current := "简体中文"
				if a.isEN() {
					current = "English"
				}
				if picked, ok := segmented(c, "lang", []string{"简体中文", "English"}, current, 30*s); ok && picked != current {
					if picked == "English" {
						a.setLanguage(LangEN)
					} else {
						a.setLanguage(LangZH)
					}
				}
			}, nil)
		},
		func() {
			settingRow(c, rowIcon(c, "gamepad", [2]ui.Color{ui.Hex("#ff375f"), ui.Hex("#ff9f0a")}), t.SettingsSnapshotsLabel, "", func() {
				if nativeToggle(c, "snapshot-toggle", t.SettingsSnapshotsAria, &a.settings.ShowSnapshots).Changed() {
					a.persistSettings("")
				}
			}, nil)
		},
	)
}

// aboutSettings is a quiet footer row: brand, version and the data folder.
func (a *App) aboutSettings(c *ui.Context) {
	s := uiScale(c)
	t := a.tr()
	glass(ui.Row(c.Key("关于 MCL 面板")).Label("关于 MCL 面板").FillWidth().Padding(space3*s, settingPadX*s).Gap(space3*s), radiusCard*s-4*s, s).AlignItems(ui.Center).Children(func() {
		brandMark(c, 30*s)
		ui.Text(c, "MCL").Font("MCL Pixel").FontSize(20 * s).Height(24 * s).TextColor(ink).NoWrap().Shrink(0)
		label(c, "v0.2.0").FontSize(typeCaption * s).NoWrap().Shrink(0)
		label(c, t.SettingsAboutDesc).FontSize(typeCaption * s).Grow(1).MinWidth(0)
		if fixedButton(c, "open-data", t.SettingsOpenDataBtn, "folder", 0, 34*s, false, false) {
			a.executeDialog(func() error { return openPath(a.snapshot.DataDir) })
		}
	})
}

// persistSettings writes the current draft immediately. A blank notice keeps
// the header hint; a validation failure shows the error and saves nothing.
func (a *App) persistSettings(notice string) {
	_ = a.tr()
	value, e := strconv.Atoi(strings.TrimSpace(a.memoryText))
	if e != nil || value < 512 || value > int(a.memoryMaximum()) {
		a.message = a.formatMemoryRangeErr(int(a.memoryMaximum()))
		a.errorMessage = true
		return
	}
	a.memory = float64(value)
	a.settings.MemoryMB = value
	a.settings.Language = a.lang()
	draft := a.settings
	go func() {
		e := a.service.SaveSettings(draft)
		snap := a.service.Snapshot()
		a.post(func() {
			a.snapshot = snap
			if e != nil {
				a.message = e.Error()
				a.errorMessage = true
				return
			}
			a.settings = snap.Settings
			a.notice(notice)
		})
	}()
}

func (a *App) javaDialog(c *ui.Context) {
	s := uiScale(c)
	t := a.tr()
	ui.Row(c).Gap(10 * s).Children(func() {
		if button(c, "java-quick", t.JavaQuickScanBtn, "search", !a.javaDeep, a.javaScanning && !a.javaDeep) {
			a.startJavaScan(false)
		}
		if button(c, "java-deep", t.JavaDeepScanBtn, "folder", a.javaDeep, a.javaScanning && a.javaDeep) {
			a.startJavaScan(true)
		}
		if a.javaScanning && button(c, "java-stop", t.JavaStopScanBtn, "cross", false, false) && a.javaScanCancel != nil {
			a.javaScanCancel()
		}
	})
	status := a.translatePhase(a.javaPhase)
	if a.javaScanning {
		status = a.formatJavaScanProgress(a.javaPhase, a.javaScanned, len(a.java))
	}
	statusLine(c, status, muted, a.javaScanning)
	label(c, t.JavaScanHint).FontSize(13 * s)
	ui.TextInput(c, &a.javaFilter).Label(t.JavaFilterAria).Placeholder(t.JavaFilterPlaceholder).FillWidth()
	if button(c, "java-auto", t.JavaUseAutoBtn, "check", a.settings.JavaPath == "", false) {
		a.settings.JavaPath = ""
		a.persistSettings("")
		a.closeDialog()
	}
	for _, item := range a.java {
		j := item
		if a.javaFilter != "" && !strings.Contains(strings.ToLower(j.Path+" "+j.Version+" "+j.Vendor), strings.ToLower(a.javaFilter)) {
			continue
		}
		ui.Row(c.Key(j.Path)).FillWidth().Gap(14*s).Padding(14*s).Radius(16*s).Background(raised).Border(1, glassEdge).AlignItems(ui.Center).Children(func() {
			ui.Column(c).Grow(1).MinWidth(0).Clip().Gap(5 * s).Children(func() {
				ui.Text(c, "Java "+j.Version).Bold().TextColor(ink)
				meta := javaDisplay(j.Vendor)
				if j.Architecture != "" {
					meta += " / " + j.Architecture
				}
				if meta != "" {
					label(c, meta)
				}
				label(c, j.Path).FillWidth().FontSize(13 * s).SingleLine().Tooltip(j.Path)
			})
			if button(c, "pick-"+j.Path, t.JavaUseBtn, "check", a.settings.JavaPath == j.Path, false) {
				a.settings.JavaPath = j.Path
				a.persistSettings("")
				a.closeDialog()
				if a.javaDeep && a.javaScanCancel != nil {
					a.javaScanCancel()
				}
			}
		})
	}
	if len(a.java) == 0 && !a.javaScanning {
		statusLine(c, t.JavaEmptyHint, muted, false)
	}
}
