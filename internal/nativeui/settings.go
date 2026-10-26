package nativeui

import (
	"fmt"
	"math"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"

	"github.com/egoist/mygo/ui"

	"github.com/Viking602/YoloMCL/internal/launcher"
)

func (a *App) refreshMemory() {
	info, e := launcher.SystemMemory()
	if e != nil {
		a.memoryError = a.friendlyErr(e)
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

func (a *App) preferences(c *ui.Context) {
	s := uiScale(c)
	surface := ui.Hex("#f9fafd")
	ui.Column(c.Key("settings-layout")).Grow(1).MinHeight(0).FillWidth().Children(func() {
		header := ui.Box(c.Key("settings-header")).Label("设置固定标题栏").FillWidth().Padding(22*s, pageGutter*s, 20*s).Shrink(0).Background(surface)
		target := float32(0)
		if track := scrollTracks["settings-scroll"]; track != nil && track.st.Y > 1 {
			target = 1
		}
		edge := header.AnimateTo("scroll-edge", target, motionFast)
		if c.Preferences().ReduceMotion {
			edge = header.Jump("scroll-edge", target)
		}
		header.Children(func() {
			ui.Row(c).FillWidth().MaxWidth(settingsColumnWidth * s).AlignSelf(ui.Center).Children(func() { a.settingsHeader(c) })
		})
		ui.Column(c.Key("settings-viewport")).Label("设置滚动区域").Grow(1).MinHeight(0).FillWidth().Clip().Children(func() {
			scrollArea(c, "settings-scroll").Grow(1).MinHeight(0).FillWidth().Padding(space4*s, pageGutter*s, a.scrollEndPad(s)+(space8-space6)*s).Children(func() {
				ui.Column(c).FillWidth().MaxWidth(settingsColumnWidth * s).AlignSelf(ui.Center).Gap(space5 * s).Children(func() {
					a.accountSettings(c)
					a.versionAndLangSettings(c)
					a.runtimeSettings(c)
					a.javaSection(c)
					a.modCacheSettings(c)
					a.aboutSettings(c)
				})
			})
			// A short fade softens clipped rows without blurring the scrolling
			// page. The fixed toolbar never changes height as the user scrolls.
			ui.Box(c.Key("settings-scroll-edge")).Label("设置滚动过渡").Absolute().Left(0).Right(0).Top(0).Height(12 * s).Opacity(edge).
				LinearGradient(ui.LinearGradient{From: surface, To: surface.Alpha(0), Angle: 180})
			ui.Box(c).Absolute().Left(0).Right(0).Top(0).Height(1).Background(line).Opacity(edge)
		})
	})
}

// accountSettings is the global account entry: accounts are shared by every
// instance, so they live here (and in the title-bar menu), never per pack.
func (a *App) accountSettings(c *ui.Context) {
	s := uiScale(c)
	t := a.tr()
	settingsGroup(c, "账号面板", a.L("logpanels.account"), "", func() {
		ac := a.account()
		name, desc := t.SideAccountAdd, a.L("settings.no_account_yet_one_account_is_shared_by")
		if ac != nil {
			name = ac.Name
			desc = a.accountKindTag(*ac)
			if n := len(a.snapshot.Accounts); n > 1 {
				desc += fmt.Sprintf(a.L("settings.accounts"), n)
			}
		}
		lead := func() {
			a.headAvatar(c, ac, settingIcon*s, func() {
				ui.Box(c).Size(settingIcon*s, settingIcon*s).Radius(999).LinearGradient(tintGradient(ui.Hex("#64d2ff"), accentIndigo)).Center().Shrink(0).Children(func() {
					ui.Icon(c, icons["account"]).Size(15*s, 15*s).TextColor(panel)
				})
			})
		}
		settingRow(c, lead, name, desc, func() {
			text, icon := a.L("settings.manage"), "account"
			if ac == nil {
				text, icon = t.SideAccountAdd, "plus"
			}
			if fixedButton(c, "settings-account", text, icon, 0, btnSm*s, ac == nil, false) {
				a.openDialog("account")
			}
		}, nil)
	})
}

// settingsColumnWidth caps line length on very wide windows.
const settingsColumnWidth float32 = 860

func (a *App) settingsHeader(c *ui.Context) {
	s := uiScale(c)
	t := a.tr()
	ui.Row(c).Label("页面标题栏").Font(uiSansFamily()).FillWidth().Gap(14 * s).AlignItems(ui.Center).Children(func() {
		a.settingsBack(c)
		ui.Column(c).Grow(1).Basis(0).MinWidth(0).Gap(5 * s).Children(func() {
			ui.Text(c, t.SettingsTitle).Label("页面标题 " + t.SettingsTitle).FontSize(typeDisplay * s).FontWeight(700).TextColor(ink).SingleLine()
			tooltip(c, ui.Text(c, t.SettingsDesc).Label("设置简要说明").FontSize(typeFootnote*s).TextColor(muted).FillWidth().MinWidth(0).SingleLine(), t.SettingsDesc)
		})
		ui.Row(c.Key("settings-actions")).Label("设置操作栏").Shrink(0).Gap(12 * s).AlignItems(ui.Center).Children(func() {
			tooltip(c, ui.Row(c).Label("设置自动保存提示"), t.SettingsDefaultHint).Padding(5*s, 9*s).Gap(5 * s).Radius(999).Background(successInk.Alpha(.07)).AlignItems(ui.Center).Children(func() {
				ui.Icon(c, icons["check"]).Size(11*s, 11*s).TextColor(successInk)
				ui.Text(c, a.L("settings.auto_save")).FontSize(typeCaption * s).FontWeight(550).TextColor(successInk).NoWrap()
			})
			if fixedButton(c, "reset-settings", t.SettingsResetBtn, "refresh", 0, btnMd*s, false, false) {
				recommended := 4096
				if a.memoryInfo.RecommendedMB > 0 {
					recommended = a.memoryInfo.RecommendedMB
				}
				a.settings = launcher.Settings{MemoryMB: recommended}
				a.languageOverride = ""
				a.normalizeStateValues()
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
		toggle := ui.ButtonBase(c.Key("advanced-toggle")).Label(t.SettingsAdvancedTitle).AlignSelf(ui.Start).Height(btnSm * s).PaddingX(space3 * s).Gap(space2 * s).AlignItems(ui.Center).Radius(radiusControl * s)
		if toggle.Hovered() {
			toggle.Background(ui.RGBA(255, 255, 255, .5))
		}
		if toggle.Children(func() {
			ui.Text(c, t.SettingsAdvancedTitle).FontSize(typeCallout * s).FontWeight(650).TextColor(inkSecondary).NoWrap()
			ic := "chevron"
			if a.advancedOpen || a.settings.JVMArgs != "" {
				ic = "chevron-down"
			}
			ui.Icon(c, icons[ic]).Size(10*s, 10*s).TextColor(muted)
		}).Clicked() {
			// Configured arguments keep the section open so they are never hidden.
			a.advancedOpen = !a.advancedOpen
		}
		if a.advancedOpen || a.settings.JVMArgs != "" {
			glass(ui.Column(c.Key("高级设置面板")).FillWidth().Clip(), radiusCard*s-4*s, s).Children(func() {
				settingRow(c, nil, t.SettingsJVMArgsLabel, "", nil, func() {
					if textField(c.Key("global-jvm-input"), &a.settings.JVMArgs).Label(t.SettingsJVMArgsAria).Placeholder(t.SettingsJVMArgsHint).FillWidth().Height(btnMd * s).Changed() {
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
		hint = a.L("settings.launch_games_with_the_selected_java_runtime")
	} else if a.javaScanning {
		hint = t.SettingsJavaScanningHint
	}
	settingRow(c, rowIcon(c, "cube", [2]ui.Color{ui.Hex("#ff9f0a"), ui.Hex("#ff453a")}), t.SettingsJavaLabel, hint, func() {
		if fixedButton(c, "choose-java", t.SettingsJavaBrowseBtn, "folder", 0, btnSm*s, false, false) {
			a.chooseFiles("java", func(paths []string) {
				a.settings.JavaPath = paths[0]
				a.persistSettings("")
			})
		}
		if fixedButton(c, "detect-java", t.SettingsJavaScanBtn, "search", 0, btnSm*s, false, false) {
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
			name := fmt.Sprintf("Java %s %s", j.Version, javaDisplay(filepath.Base(filepath.Dir(filepath.Dir(j.Path)))))
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
		if a.javaSelectBox(c, "java-selection", t.SettingsJavaAria, &selected, names, paths).FillWidth().Height(btnMd * s).Changed() {
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
		if picked, ok := segmented(c, "global-mem-mode", []string{t.MemModeAuto, t.MemModeManual}, cur, 25*s); ok && picked != cur {
			a.settings.MemoryMode = ""
			if picked == t.MemModeManual {
				a.settings.MemoryMode = "manual"
			}
			a.persistSettings("")
		}
		if auto {
			return
		}
		ui.Text(c, fmt.Sprintf("%.1f GB", a.memory/1024)).FontSize(typeBody * s).Bold().TextColor(accent).NoWrap()
		input := textField(c.Key("memory-input"), &a.memoryText).Label(t.SettingsMemoryInputAria).Width(84*s).Height(btnMd*s).Padding(4*s, 8*s)
		if input.Changed() {
			if value, e := strconv.Atoi(a.memoryText); e == nil && value >= 512 && value <= int(a.memoryMaximum()) {
				a.memory = float64(value)
				a.persistSettings("")
			}
		}
		if !input.Focused() {
			a.memoryText = fmt.Sprint(int(a.memory))
		}
		label(c, "MB").FontSize(typeCallout * s).FontWeight(600).NoWrap()
	}, func() {
		if auto {
			label(c, t.MemAutoExplain).FontSize(typeFootnote * s).FillWidth().MinWidth(0)
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
			// Divide the full logarithmic range so both ends are reachable,
			// including physical memory limits that are not powers of two.
			slider := ui.SliderBase(c.Key("memory-slider"), &position, lo, hi).Label(t.SettingsMemorySliderAria).Grow(1).MinWidth(0).Height(btnMd * s).PaddingX(10 * s).Step((hi - lo) / 128)
			if slider.Changed() {
				value := math.Pow(2, position)
				a.memory = math.Max(minMB, math.Min(maxMB, math.Round(value/128)*128))
				if position >= hi-1e-9 {
					a.memory = maxMB
				}
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
			if picked, ok := segmented(c, "global-mem", presets, current, 24*s); ok {
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
	rows := []func(){
		func() {
			settingRow(c, rowIcon(c, "globe", [2]ui.Color{ui.Hex("#0a84ff"), ui.Hex("#5e5ce6")}), t.SettingsLanguageLabel, "", func() {
				auto := a.L("settings.system")
				zh, en := a.L("language.zh"), a.L("language.en")
				current := auto
				switch launcher.NormalizeLanguage(a.settings.Language) {
				case LangZH:
					current = zh
				case LangEN:
					current = en
				}
				if picked, ok := segmented(c, "lang", []string{auto, zh, en}, current, 25*s); ok {
					switch picked {
					case en:
						a.setLanguage(LangEN)
					case zh:
						a.setLanguage(LangZH)
					default:
						a.setLanguage("")
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
	}
	if runtime.GOOS == "windows" {
		rows = append(rows, func() {
			settingRow(c, nil, a.L("settings.interface_hardware_acceleration"),
				a.L("settings.off_uses_less_memory_and_keeps_animations_enable"), func() {
					if nativeToggle(c, "hardware-acceleration", a.L("settings.enable_interface_hardware_acceleration"), &a.settings.HardwareAcceleration).Changed() {
						a.persistSettings(a.L("settings.rendering_preference_saved_restart_the_launcher_to_apply"))
					}
				}, nil)
		})
	}
	settingsGroup(c, "游戏版本面板", t.SettingsPrefsTitle, "", rows...)
}
