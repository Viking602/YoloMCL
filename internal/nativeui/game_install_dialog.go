package nativeui

import (
	"fmt"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/egoist/mygo/ui"
	"github.com/lobehub/lobe-mygo/pkg/lucide"

	"github.com/Viking602/YoloMCL/internal/launcher"
)

func installTargetSwitch(c *ui.Context, selected *string, options []string) bool {
	if picked, ok := segmented(c, "install-target", options, *selected, 28*uiScale(c)); ok && picked != *selected {
		*selected = picked
		return true
	}
	return false
}

// installRequiredJava returns the expected Java major version for installing or
// launching a Minecraft + loader pair (0 when unknown).
func installRequiredJava(mc, loader string) int {
	if loader == "horizon" {
		return 25
	}
	mc = strings.TrimSpace(mc)
	if mc == "" {
		return 0
	}
	lower := strings.ToLower(mc)
	for _, prefix := range []string{"b1.", "a1.", "c0.", "inf-", "rd-"} {
		if strings.HasPrefix(lower, prefix) {
			return 8
		}
	}
	// Weekly snapshot format: YYwWW[a-z] (e.g. 24w14a, 25w02a, 26w01a).
	if len(lower) >= 5 && lower[2] == 'w' {
		yy, errY := strconv.Atoi(lower[:2])
		ww, errW := strconv.Atoi(lower[3:5])
		if errY == nil && errW == nil {
			switch {
			case yy >= 26:
				return 25
			case yy == 25 || (yy == 24 && ww >= 14):
				return 21
			case yy >= 22 || (yy == 21 && ww >= 37):
				return 17
			case yy == 21 && ww >= 19:
				return 16
			default:
				return 8
			}
		}
	}
	// Release / year-based versioning: X.Y[.Z][-suffix] (e.g. 26.3, 1.21.4, 1.20.5).
	core := lower
	if idx := strings.IndexAny(core, "-_ "); idx > 0 {
		core = core[:idx]
	}
	parts := strings.Split(core, ".")
	if len(parts) >= 2 {
		major, errMaj := strconv.Atoi(parts[0])
		minor, errMin := strconv.Atoi(parts[1])
		patch := 0
		if len(parts) >= 3 {
			patch, _ = strconv.Atoi(parts[2])
		}
		if errMaj == nil && errMin == nil {
			if major >= 26 {
				return 25
			}
			if major == 1 {
				switch {
				case minor >= 21 || (minor == 20 && patch >= 5):
					return 21
				case minor >= 18:
					return 17
				case minor == 17:
					return 16
				default:
					return 8
				}
			}
		}
	}
	return 0
}

func (a *App) installMatchedJava(need int) *launcher.JavaInfo {
	if need <= 0 {
		return nil
	}
	for i := range a.java {
		if a.java[i].Major == need {
			return &a.java[i]
		}
	}
	return nil
}

func installHasSameCore(r *launcher.Instance, mc, curLoader string) bool {
	if r == nil || mc == "" {
		return false
	}
	for _, v := range r.Versions {
		if v.Minecraft == mc && loaderKey(v.Loader) == curLoader {
			return true
		}
	}
	return false
}

func installLoaderDescKey(curLoader string) string {
	switch curLoader {
	case "fabric", "forge", "neoforge", "quilt":
		return "install.loader_desc." + curLoader
	default:
		return ""
	}
}

// gameInstallWorkspace renders the full-bleed split game installer: a virtualized
// Minecraft version browser on the left rail and loader/destination controls on
// the right content panel, matching the Instances and Discover split layout.
func (a *App) gameInstallWorkspace(c *ui.Context) {
	a.normalizeStateValues()
	inDialog, dialogPrimaryKey = true, ""
	defer func() { inDialog, lastDialogPrimary = false, dialogPrimaryKey }()

	s := uiScale(c)
	winW, _ := c.Size()
	railW := min(304*s, max(236*s, winW*.26))

	root := ui.Row(c.Key("task-page-install")).Lang(a.lang()).Label(a.dialogTitle("install")).
		Fill().Gap(0).AlignItems(ui.Stretch).Focusable().AutoFocus().FocusRing(false)
	if root.Shortcut(0, ui.KeyEscape) {
		a.requestCloseDialog()
	}
	if root.Focused() && root.Shortcut(0, ui.KeyEnter) && lastDialogPrimary != "" {
		enterFired = lastDialogPrimary
	}
	root.Children(func() {
		a.installVersionRail(c, railW)
		contentPanel(ui.Column(c.Key("install-workspace-panel")), s).Grow(1).Basis(0).MinWidth(0).Children(func() {
			a.gameInstallHeader(c)
			ui.Box(c).FillWidth().Height(1).Background(line).Shrink(0)
			scrollArea(c, "dialog-content-install").FillWidth().Grow(1).MinHeight(0).
				Padding(18*s, 24*s, 22*s, 24*s).Gap(16 * s).Children(func() {
				a.dialogFeedback(c)
				a.installDialog(c)
			})
			a.gameInstallFooter(c)
		})
	})
}

func (a *App) gameInstallHeader(c *ui.Context) {
	s := uiScale(c)
	t := a.tr()
	selectedMV := a.selectedMinecraftVersion()

	ui.Row(c.Key("install-hero-header")).FillWidth().Shrink(0).Padding(18*s, 24*s, 16*s, 24*s).
		Gap(14 * s).Background(panel).AlignItems(ui.Center).Children(func() {
		if !loaderLogo(c, a.form.Loader, 42*s) {
			ui.Box(c).Size(42*s, 42*s).Radius(iconRadius(42*s, s)).Shrink(0).Background(raised).Border(1, line).Center().Children(func() {
				ui.Icon(c, icons[loaderIconName(a.form.Loader)]).Size(22*s, 22*s).TextColor(loaderTone(a.form.Loader))
			})
		}
		ui.Column(c).Grow(1).Basis(0).MinWidth(0).Gap(3 * s).Children(func() {
			heroTitle := t.DialogTitleInstall
			if a.form.Minecraft != "" {
				heroTitle = "Minecraft " + a.form.Minecraft
			}
			ui.Row(c).FillWidth().MinWidth(0).Gap(8 * s).AlignItems(ui.Center).Children(func() {
				ui.Text(c, heroTitle).FontSize(typeTitle2 * s).Bold().TextColor(ink).SingleLine().Shrink(1).MinWidth(0)
				if selectedMV != nil {
					tagChip(c, a.versionKindLabel(minecraftKindKey(selectedMV.Type)))
					if len(selectedMV.ReleaseTime) >= 10 {
						ui.Text(c, selectedMV.ReleaseTime[:10]).FontSize(typeFootnote * s).TextColor(muted).NoWrap().Shrink(0)
					}
				}
			})
			ui.Text(c, t.DialogTitleInstall).FontSize(typeFootnote * s).TextColor(muted).SingleLine()
		})
		if button(c, "close-dialog", t.DialogCloseBtn, "cross", false, false) {
			a.requestCloseDialog()
		}
	})
}

func (a *App) installDialog(c *ui.Context) {
	s := uiScale(c)
	t := a.tr()
	curLoader := loaderKey(a.form.Loader)
	winW, _ := c.Size()

	// Section 1: Core / Mod Loader selector grid
	ui.Column(c.Key("install-loader-section")).FillWidth().Gap(10 * s).Children(func() {
		ui.Text(c, t.InstallStep3Title).FontSize(typeCallout * s).FontWeight(650).TextColor(inkSecondary).SingleLine()
		type loaderChoice struct{ name, sub string }
		loaderRows := [][]loaderChoice{
			{
				{t.LoaderVanilla, t.InstallLoaderVanillaSub},
				{t.LoaderFabric, t.InstallLoaderFabricSub},
				{t.LoaderForge, t.InstallLoaderForgeSub},
			},
			{
				{t.LoaderNeoForge, t.InstallLoaderNeoSub},
				{t.LoaderQuilt, t.InstallLoaderQuiltSub},
				{t.LoaderHorizon, t.InstallLoaderHorizonSub},
			},
		}
		ui.Column(c).FillWidth().Gap(8 * s).Children(func() {
			for ri, row := range loaderRows {
				ui.Row(c.Key(fmt.Sprintf("loader-row-%d", ri))).FillWidth().Gap(10 * s).Children(func() {
					for _, item := range row {
						ld := item
						selected := curLoader == loaderKey(ld.name)
						cardBg := panel
						borderCol := line
						borderW := float32(1)
						if selected {
							cardBg = raised
							borderCol = lobePrimary
							borderW = 1.5 * s
						}
						b := ui.ButtonBase(c.Key("loader-card-"+loaderKey(ld.name))).Label(ld.name).Grow(1).Basis(0).MinWidth(0).
							Padding(11*s, 13*s).Radius(radiusCard * s).AlignItems(ui.Center)
						if b.Hovered() && !selected {
							cardBg = raised
							borderCol = lineStrong
						}
						b.Background(b.AnimateColor("bg", cardBg, fastAnim)).Border(borderW, b.AnimateColor("border", borderCol, fastAnim))
						if !selected {
							b.Shadow(0, 1*s, 2*s, 0, ui.RGBA(0, 0, 0, .02))
						}
						if b.Children(func() {
							ui.Row(c).FillWidth().Gap(10 * s).AlignItems(ui.Center).Children(func() {
								if !loaderLogo(c, ld.name, 34*s) {
									ui.Box(c).Size(34*s, 34*s).Radius(iconRadius(34*s, s)).Shrink(0).Background(panel).Border(1, line).Center().Children(func() {
										ui.Icon(c, icons[loaderIconName(ld.name)]).Size(18*s, 18*s).TextColor(loaderTone(ld.name))
									})
								}
								ui.Column(c).Grow(1).MinWidth(0).Gap(2 * s).Children(func() {
									ui.Text(c, ld.name).FontSize(typeCallout * s).FontWeight(650).TextColor(ink).SingleLine()
									ui.Text(c, ld.sub).FontSize(typeCaption * s).TextColor(muted).SingleLine()
								})
								if selected {
									ui.Box(c).Size(18*s, 18*s).Radius(9 * s).Shrink(0).Background(lobePrimary).Center().Children(func() {
										ui.Icon(c, icons["check"]).Size(11*s, 11*s).TextColor(lobeLight.PrimaryForeground)
									})
								}
							})
						}).Clicked() && !selected {
							a.form.Loader = ld.name
							a.loadLoaderVersions()
						}
					}
				})
			}
		})
	})

	// Section 2: Balanced two-column configuration & readiness workspace on wide
	// windows, stacking cleanly into a single column on compact viewports.
	if winW >= 1060 {
		ui.Row(c.Key("install-detail-columns")).FillWidth().Gap(16 * s).AlignItems(ui.Start).Children(func() {
			ui.Column(c.Key("install-col-left")).Grow(13).Basis(0).MinWidth(0).Gap(16 * s).Children(func() {
				a.installLoaderDetailCard(c, curLoader)
				a.installLocationCard(c, curLoader)
			})
			ui.Column(c.Key("install-col-right")).Grow(9).Basis(0).MinWidth(0).Gap(16 * s).Children(func() {
				a.installPreflightCard(c, curLoader)
				a.installStepsCard(c, curLoader)
			})
		})
	} else {
		ui.Column(c.Key("install-detail-stack")).FillWidth().Gap(14 * s).Children(func() {
			a.installLoaderDetailCard(c, curLoader)
			a.installLocationCard(c, curLoader)
			a.installPreflightCard(c, curLoader)
			a.installStepsCard(c, curLoader)
		})
	}
}

func installCard(c *ui.Context, key string, s float32) ui.Element {
	return ui.Column(c.Key(key)).FillWidth().Padding(16*s, 18*s).Gap(12*s).
		Radius(radiusCard*s).Background(panel).Border(1, line).
		Shadow(0, 1*s, 2*s, 0, ui.RGBA(0, 0, 0, .03))
}

func (a *App) installLoaderDetailCard(c *ui.Context, curLoader string) {
	s := uiScale(c)
	t := a.tr()
	verLabel := t.InstallLoaderVerLabel
	if curLoader == "horizon" {
		verLabel = a.L("horizon.build_version_label")
	}

	installCard(c, "install-loader-detail", s).Children(func() {
		switch {
		case curLoader == "vanilla":
			ui.Row(c).FillWidth().Gap(8 * s).AlignItems(ui.Center).Children(func() {
				ui.Text(c, t.LoaderVanilla).FontSize(typeCallout * s).FontWeight(650).TextColor(ink).NoWrap()
				ui.Spacer(c).Grow(1)
				badge(c, a.L("install.preflight.ready"), success)
			})
			lobeAlert(c, "success", a.L("install.vanilla_ready"))
		case a.loaderLoading:
			ui.Text(c, verLabel).FontSize(typeCallout * s).FontWeight(650).TextColor(ink).SingleLine()
			ui.Row(c).FillWidth().Gap(10 * s).AlignItems(ui.Center).Children(func() {
				statusSpinner(c, 16*s)
				ui.Text(c, t.InstallFetchingLoader).FontSize(typeCallout * s).TextColor(muted).Grow(1).MinWidth(0).SingleLine()
			})
		case a.loaderError != "":
			ui.Text(c, verLabel).FontSize(typeCallout * s).FontWeight(650).TextColor(ink).SingleLine()
			ui.Row(c.Key("loader-failed")).FillWidth().Gap(10 * s).AlignItems(ui.Center).Children(func() {
				ui.Icon(c, icons["info"]).Size(16*s, 16*s).TextColor(danger).Shrink(0)
				bodyText(c, a.loaderError).FontSize(typeFootnote * s).TextColor(inkSecondary).Grow(1).MinWidth(0)
				if button(c, "loader-retry", t.NoticeRetry, "refresh", false, a.loaderLoading) {
					a.loadLoaderVersions()
				}
			})
		case len(a.loaders) == 0:
			ui.Text(c, verLabel).FontSize(typeCallout * s).FontWeight(650).TextColor(ink).SingleLine()
			if curLoader == "horizon" {
				targetMC := a.horizonTargetMC
				if targetMC == "" {
					targetMC = "26.3"
				}
				ui.Row(c.Key("horizon-mc-mismatch")).FillWidth().Padding(10*s, 12*s).Gap(12*s).
					Radius(radiusControl*s).Background(warning.Alpha(.09)).Border(1, warning.Alpha(.28)).AlignItems(ui.Center).Children(func() {
					ui.Icon(c, icons["info"]).Size(15*s, 15*s).TextColor(warningInk).Shrink(0)
					ui.Text(c, fmt.Sprintf(a.L("horizon.only_supports_mc"), targetMC)).FontSize(typeCallout * s).FontWeight(500).TextColor(ink).Grow(1).MinWidth(0)
					if a.form.Minecraft != targetMC && button(c, "horizon-switch-mc", fmt.Sprintf(a.L("horizon.switch_mc"), targetMC), "arrow", false, false) {
						kindKey := "all"
						for _, mv := range a.minecraftVersions {
							if mv.ID == targetMC {
								kindKey = minecraftKindKey(mv.Type)
								break
							}
						}
						a.versionKind = a.versionKindLabel(kindKey)
						a.versionQuery = ""
						a.form.Minecraft = targetMC
						a.installVerScrollPending = true
						a.loadLoaderVersions()
					}
				})
				ui.Box(c.Key("horizon-install-note")).FillWidth().Children(func() {
					lobeAlert(c, "info", a.L("horizon.install_note"))
				})
			} else {
				lobeAlert(c, "warning", t.InstallNoLoaderVer)
			}
		default:
			showQuickList := len(a.loaders) > 1
			ui.Row(c).FillWidth().Gap(8 * s).AlignItems(ui.Center).Children(func() {
				ui.Text(c, verLabel).FontSize(typeCallout * s).FontWeight(650).TextColor(ink).NoWrap()
				tagChip(c, fmt.Sprintf(a.L("install.available_builds"), len(a.loaders)))
				if !showQuickList && a.form.LoaderVersion == a.loaders[0] {
					badge(c, a.L("install.latest_badge"), accent)
				}
			})
			selectBox(c, "install-loader-ver", verLabel, &a.form.LoaderVersion, a.loaders).FillWidth()
			if showQuickList {
				ui.Column(c.Key("install-recent-builds")).FillWidth().Gap(6 * s).Children(func() {
					ui.Text(c, a.L("install.recent_builds")).FontSize(typeCaption * s).FontWeight(600).TextColor(muted).SingleLine()
					limit := min(len(a.loaders), 4)
					for i := 0; i < limit; i++ {
						ver := a.loaders[i]
						sel := a.form.LoaderVersion == ver
						rowBg := panel
						rowEdge := line
						if sel {
							rowBg = raised
							rowEdge = lobePrimary
						}
						b := ui.ButtonBase(c.Key(fmt.Sprintf("install-quick-build-%d", i))).Label(ver).FillWidth().
							Padding(8*s, 11*s).Radius(radiusControl * s).AlignItems(ui.Center)
						if b.Hovered() && !sel {
							rowBg = raised
							rowEdge = lineStrong
						}
						b.Background(b.AnimateColor("bg", rowBg, fastAnim)).Border(1, b.AnimateColor("border", rowEdge, fastAnim))
						if b.Children(func() {
							ui.Row(c).FillWidth().Gap(9 * s).AlignItems(ui.Center).Children(func() {
								if sel {
									ui.Box(c).Size(16*s, 16*s).Radius(8 * s).Shrink(0).Background(lobePrimary).Center().Children(func() {
										ui.Icon(c, icons["check"]).Size(10*s, 10*s).TextColor(lobeLight.PrimaryForeground)
									})
								} else {
									ui.Box(c).Size(16*s, 16*s).Radius(8*s).Shrink(0).Background(panel).Border(1.25*s, muted.Alpha(.45))
								}
								weight := 450
								fg := inkSecondary
								if sel {
									weight = 600
									fg = ink
								}
								ui.Text(c, ver).FontSize(typeFootnote * s).FontWeight(weight).TextColor(fg).SingleLine().Grow(1).MinWidth(0)
								if i == 0 {
									badge(c, a.L("install.latest_badge"), accent)
								}
							})
						}).Clicked() && !sel {
							a.form.LoaderVersion = ver
						}
					}
				})
			}
			if curLoader == "horizon" {
				ui.Box(c.Key("horizon-install-note")).FillWidth().Children(func() {
					lobeAlert(c, "info", a.L("horizon.install_note"))
				})
			} else if descKey := installLoaderDescKey(curLoader); descKey != "" {
				ui.Box(c.Key("loader-install-note")).FillWidth().Children(func() {
					lobeAlert(c, "info", a.L(descKey))
				})
			}
		}
	})
}

func (a *App) installLocationCard(c *ui.Context, curLoader string) {
	s := uiScale(c)
	t := a.tr()

	installCard(c, "install-location-section", s).Children(func() {
		ui.Row(c).FillWidth().Gap(10 * s).AlignItems(ui.Center).Children(func() {
			ui.Text(c, t.InstallStep1Title).FontSize(typeCallout * s).FontWeight(650).TextColor(ink).SingleLine()
			ui.Spacer(c).Grow(1)
			if a.root() != nil {
				if installTargetSwitch(c, &a.form.Target, []string{t.InstallTargetCurrent, t.InstallTargetNew}) {
					a.form.Name = ""
				}
			}
		})
		if normalizeInstallTarget(a.form.Target) == "current" && a.root() != nil {
			r := a.root()
			ui.Row(c).FillWidth().Gap(12 * s).AlignItems(ui.Center).Children(func() {
				ui.Box(c).Size(36*s, 36*s).Radius(iconRadius(36*s, s)).Shrink(0).Background(raised).Border(1, line).Center().Children(func() {
					ui.Icon(c, icons["folder"]).Size(16*s, 16*s).TextColor(inkSecondary)
				})
				ui.Column(c).Grow(1).Basis(0).MinWidth(0).Gap(3 * s).Children(func() {
					ui.Text(c, dirLabel(*r)).FontSize(typeCallout * s).FontWeight(600).TextColor(ink).SingleLine()
					ui.Row(c).FillWidth().MinWidth(0).Children(func() {
						pathLabel(c, r.Path, typeFootnote*s)
					})
				})
				if fixedButton(c, "install-open-folder", a.L("install.open_folder"), "external", 0, btnSm*s, false, false) {
					a.openFolder(r.ID)
				}
			})
			ui.Row(c.Key("install-root-meta")).FillWidth().Padding(8*s, 12*s).Gap(8*s).
				Radius(radiusControl*s).Background(raised).Border(1, line).AlignItems(ui.Center).Children(func() {
				ui.Icon(c, icons["layers"]).Size(13*s, 13*s).TextColor(muted).Shrink(0)
				ui.Text(c, fmt.Sprintf(a.L("install.root_versions_count"), len(r.Versions))).FontSize(typeCaption * s).TextColor(inkSecondary).SingleLine().Grow(1).MinWidth(0)
				if installHasSameCore(r, a.form.Minecraft, curLoader) {
					badge(c, a.L("install.same_core_installed"), accent)
				}
			})
		} else {
			nameHint := t.InstallNamePlaceholder
			if a.form.Minecraft != "" {
				nameHint = a.form.Minecraft + "-" + a.form.Loader
			}
			ui.Row(c).FillWidth().Gap(12 * s).Children(func() {
				ui.Column(c).Grow(3).Basis(0).MinWidth(0).Children(func() {
					a.folderField(c, t.InstallParentLabel, &a.form.Parent)
				})
				ui.Column(c).Grow(2).Basis(0).MinWidth(0).Children(func() {
					field(c, t.InstallNameLabel, &a.form.Name, nameHint)
				})
			})
			previewName := strings.TrimSpace(a.form.Name)
			if previewName == "" {
				previewName = nameHint
			}
			previewPath := previewName
			if parent := strings.TrimSpace(a.form.Parent); parent != "" {
				previewPath = filepath.Join(parent, previewName)
			}
			ui.Row(c.Key("install-dest-preview")).FillWidth().Padding(8*s, 12*s).Gap(8*s).
				Radius(radiusControl*s).Background(raised).Border(1, line).AlignItems(ui.Center).Children(func() {
				ui.Icon(c, icons["folder-add"]).Size(14*s, 14*s).TextColor(muted).Shrink(0)
				ui.Text(c, a.L("install.dest_preview")).FontSize(typeCaption * s).FontWeight(550).TextColor(muted).NoWrap().Shrink(0)
				pathLabel(c, previewPath, typeCaption*s)
			})
		}
	})
}

func (a *App) installPreflightCard(c *ui.Context, curLoader string) {
	s := uiScale(c)
	t := a.tr()
	needJava := installRequiredJava(a.form.Minecraft, curLoader)
	matchedJava := a.installMatchedJava(needJava)
	javaReady := needJava == 0 || matchedJava != nil
	ac := a.account()
	accountReady := ac != nil && (curLoader != "horizon" || ac.Kind == "microsoft")
	allReady := javaReady && accountReady
	isCurrent := normalizeInstallTarget(a.form.Target) == "current" && a.root() != nil

	installCard(c, "install-preflight-card", s).Children(func() {
		ui.Row(c).FillWidth().Gap(8 * s).AlignItems(ui.Center).Children(func() {
			ui.Text(c, a.L("install.preflight.title")).FontSize(typeCallout * s).FontWeight(650).TextColor(ink).SingleLine().Grow(1).MinWidth(0)
			switch {
			case a.javaScanning && !javaReady:
				statusSpinner(c, 14*s)
			case allReady:
				badge(c, a.L("install.preflight.ready"), success)
			default:
				badge(c, a.L("install.preflight.attention"), warning)
			}
		})

		// Java runtime readiness check
		ui.Column(c.Key("install-preflight-java")).FillWidth().Gap(8 * s).Children(func() {
			ui.Row(c).FillWidth().Gap(10 * s).AlignItems(ui.Start).Children(func() {
				switch {
				case matchedJava != nil || needJava == 0:
					ui.Box(c).Size(28*s, 28*s).Radius(iconRadius(28*s, s)).Shrink(0).Background(successSoft).Center().Children(func() {
						ui.Icon(c, lucide.CircleCheck()).Size(15*s, 15*s).TextColor(successInk)
					})
				case a.javaScanning:
					ui.Box(c).Size(28*s, 28*s).Radius(iconRadius(28*s, s)).Shrink(0).Background(raised).Border(1, line).Center().Children(func() {
						statusSpinner(c, 14*s)
					})
				default:
					ui.Box(c).Size(28*s, 28*s).Radius(iconRadius(28*s, s)).Shrink(0).Background(lobeLight.WarningBg).Border(1, lobeLight.WarningBorder).Center().Children(func() {
						ui.Icon(c, lucide.TriangleAlert()).Size(14*s, 14*s).TextColor(warningInk)
					})
				}
				ui.Column(c).Grow(1).Basis(0).MinWidth(0).Gap(3 * s).Children(func() {
					ui.Row(c).FillWidth().Gap(6 * s).AlignItems(ui.Center).Children(func() {
						ui.Text(c, a.L("install.preflight.java_title")).FontSize(typeFootnote * s).FontWeight(600).TextColor(ink).NoWrap().Shrink(0)
						if needJava > 0 {
							tagChip(c, fmt.Sprintf(a.L("install.preflight.java_required"), needJava))
						}
						ui.Spacer(c).Grow(1)
						if matchedJava != nil {
							ui.Text(c, a.L("install.preflight.java_ready")).FontSize(typeCaption * s).FontWeight(600).TextColor(successInk).NoWrap().Shrink(0)
						} else if !a.javaScanning && needJava > 0 {
							ui.Text(c, a.L("install.preflight.java_missing")).FontSize(typeCaption * s).FontWeight(600).TextColor(warningInk).NoWrap().Shrink(0)
						}
					})
					switch {
					case matchedJava != nil:
						detail := "Java " + matchedJava.Version
						if matchedJava.Vendor != "" {
							detail += " " + matchedJava.Vendor
						}
						ui.Text(c, detail).FontSize(typeCaption * s).TextColor(muted).SingleLine()
					case a.javaScanning:
						ui.Text(c, a.L("install.preflight.java_scanning")).FontSize(typeCaption * s).TextColor(muted).SingleLine()
					case needJava > 0:
						bodyText(c, fmt.Sprintf(a.L("install.preflight.java_missing_hint"), needJava)).FontSize(typeCaption * s).TextColor(muted)
					}
				})
			})
			if !a.javaScanning && matchedJava == nil && needJava > 0 {
				ui.Row(c).FillWidth().Padding(0, 0, 0, 38*s).Children(func() {
					if fixedButton(c, "install-get-java", fmt.Sprintf(a.L("install.preflight.java_download"), needJava), "download", 0, btnSm*s, false, false) {
						a.openJavaGet(needJava, "", false)
					}
				})
			}
		})

		ui.Box(c).FillWidth().Height(1).Background(line)

		// Account readiness check
		ui.Column(c.Key("install-preflight-account")).FillWidth().Gap(8 * s).Children(func() {
			ui.Row(c).FillWidth().Gap(10 * s).AlignItems(ui.Start).Children(func() {
				a.headAvatar(c, ac, 28*s, func() {
					ui.Box(c).Size(28*s, 28*s).Radius(iconRadius(28*s, s)).Shrink(0).Background(raised).Border(1, line).Center().Children(func() {
						ui.Icon(c, icons["account"]).Size(14*s, 14*s).TextColor(muted)
					})
				})
				ui.Column(c).Grow(1).Basis(0).MinWidth(0).Gap(3 * s).Children(func() {
					ui.Row(c).FillWidth().Gap(6 * s).AlignItems(ui.Center).Children(func() {
						ui.Text(c, a.L("install.preflight.account_title")).FontSize(typeFootnote * s).FontWeight(600).TextColor(ink).NoWrap().Shrink(0)
						ui.Spacer(c).Grow(1)
						if ac != nil {
							a.accountTypeTag(c, *ac)
						} else {
							ui.Text(c, a.L("install.preflight.account_none")).FontSize(typeCaption * s).FontWeight(600).TextColor(warningInk).NoWrap().Shrink(0)
						}
					})
					if ac != nil {
						ui.Text(c, ac.Name).FontSize(typeCaption * s).FontWeight(550).TextColor(inkSecondary).SingleLine()
					}
					if curLoader == "horizon" {
						if ac != nil && ac.Kind == "microsoft" {
							ui.Row(c).FillWidth().Gap(5 * s).AlignItems(ui.Center).Children(func() {
								ui.Icon(c, lucide.CircleCheck()).Size(12*s, 12*s).TextColor(successInk).Shrink(0)
								ui.Text(c, a.L("install.preflight.horizon_ms_ok")).FontSize(typeCaption * s).TextColor(successInk).SingleLine().Grow(1).MinWidth(0)
							})
						} else {
							ui.Row(c).FillWidth().Gap(5 * s).AlignItems(ui.Start).Children(func() {
								ui.Box(c).Height(16 * s).AlignItems(ui.Center).Shrink(0).Children(func() {
									ui.Icon(c, lucide.TriangleAlert()).Size(12*s, 12*s).TextColor(warningInk)
								})
								bodyText(c, a.L("install.preflight.horizon_ms_warn")).FontSize(typeCaption * s).TextColor(warningInk).Grow(1).MinWidth(0)
							})
						}
					} else if ac != nil {
						ui.Text(c, a.L("install.preflight.account_ready")).FontSize(typeCaption * s).TextColor(successInk).SingleLine()
					}
				})
			})
			if !accountReady {
				btnKey, btnText, targetDialog := "install-add-account", a.L("install.preflight.account_add"), "accountAdd"
				if ac != nil {
					btnKey, btnText, targetDialog = "install-switch-account", a.L("install.preflight.account_switch"), "account"
				}
				ui.Row(c).FillWidth().Padding(0, 0, 0, 38*s).Children(func() {
					if fixedButton(c, btnKey, btnText, "account", 0, btnSm*s, false, false) {
						a.openDialog(targetDialog)
					}
				})
			}
		})

		ui.Box(c).FillWidth().Height(1).Background(line)

		// Compact target configuration spec sheet
		specVersion := "-"
		if a.form.Minecraft != "" {
			specVersion = "Minecraft " + a.form.Minecraft
		}
		specLoader := a.form.Loader
		if curLoader != "vanilla" && a.form.LoaderVersion != "" {
			specLoader = a.form.Loader + " " + a.form.LoaderVersion
		}
		specMode := t.InstallTargetNew
		if isCurrent {
			specMode = t.InstallTargetCurrent
		}
		ui.Column(c.Key("install-preflight-specs")).FillWidth().Padding(10*s, 12*s).Gap(6*s).
			Radius(radiusControl*s).Background(raised).Border(1, line).Children(func() {
			specRow := func(k, v string) {
				ui.Row(c).FillWidth().Gap(10 * s).AlignItems(ui.Center).Children(func() {
					ui.Text(c, k).FontSize(typeCaption * s).TextColor(muted).NoWrap().Shrink(0)
					ui.Spacer(c).Grow(1)
					ui.Text(c, v).FontSize(typeCaption * s).FontWeight(550).TextColor(ink).SingleLine().Shrink(1).MinWidth(0)
				})
			}
			specRow(a.L("install.preflight.spec_version"), specVersion)
			specRow(a.L("install.preflight.spec_loader"), specLoader)
			specRow(a.L("install.preflight.spec_mode"), specMode)
		})
	})
}

func (a *App) installStepsCard(c *ui.Context, curLoader string) {
	s := uiScale(c)
	stepKind := "mod"
	if curLoader == "horizon" {
		stepKind = "horizon"
	} else if curLoader == "vanilla" {
		stepKind = "vanilla"
	}

	installCard(c, "install-steps-card", s).Children(func() {
		ui.Row(c).FillWidth().Gap(8 * s).AlignItems(ui.Center).Children(func() {
			ui.Text(c, a.L("install.steps.title")).FontSize(typeCallout * s).FontWeight(650).TextColor(ink).SingleLine().Grow(1).MinWidth(0)
			tagChip(c, a.L("install.steps.background"))
		})
		ui.Column(c).FillWidth().Gap(10 * s).Children(func() {
			for i := 1; i <= 3; i++ {
				stepNum := strconv.Itoa(i)
				title := a.L(fmt.Sprintf("install.steps.%s.%d.title", stepKind, i))
				desc := a.L(fmt.Sprintf("install.steps.%s.%d.desc", stepKind, i))
				ui.Row(c.Key("install-step-" + stepNum)).FillWidth().Gap(10 * s).AlignItems(ui.Start).Children(func() {
					ui.Box(c).Size(22*s, 22*s).Radius(11*s).Shrink(0).Background(raised).Border(1, lineStrong).Center().Children(func() {
						ui.Text(c, stepNum).FontSize(typeCaption2 * s).FontWeight(650).TextColor(ink).NoWrap()
					})
					ui.Column(c).Grow(1).Basis(0).MinWidth(0).Gap(2 * s).Children(func() {
						ui.Text(c, title).FontSize(typeFootnote * s).FontWeight(600).TextColor(ink).SingleLine()
						bodyText(c, desc).FontSize(typeCaption * s).TextColor(muted)
					})
				})
			}
		})
	})
}

func (a *App) gameInstallWhy(isCurrent, isVanilla bool) string {
	t := a.tr()
	if a.dialogPending() {
		return t.WhyBusy
	}
	if a.form.Minecraft == "" {
		return a.L("install.why_no_mc")
	}
	if !isVanilla {
		if a.loaderLoading {
			return t.InstallFetchingLoader
		}
		if a.loaderError != "" {
			return a.L("install.why_loader_error")
		}
		if a.form.LoaderVersion == "" {
			return a.L("install.why_no_loader")
		}
	}
	if !isCurrent && (strings.TrimSpace(a.form.Parent) == "" || strings.TrimSpace(a.form.Name) == "") {
		return a.L("install.why_no_dest")
	}
	return ""
}

func (a *App) gameInstallFooter(c *ui.Context) {
	s := uiScale(c)
	t := a.tr()
	isCurrent := normalizeInstallTarget(a.form.Target) == "current" && a.root() != nil
	curLoader := loaderKey(a.form.Loader)
	isVanilla := curLoader == "vanilla"
	why := a.gameInstallWhy(isCurrent, isVanilla)

	summaryTitle := a.L("install.summary_empty")
	summaryLoader := ""
	if a.form.Minecraft != "" {
		summaryTitle = "Minecraft " + a.form.Minecraft
		if isVanilla {
			summaryLoader = t.LoaderVanilla
		} else if a.form.LoaderVersion != "" {
			summaryLoader = a.form.Loader + " " + a.form.LoaderVersion
		} else {
			summaryLoader = a.form.Loader
		}
	}
	summaryPath := a.L("install.summary_new_dir")
	if isCurrent {
		summaryPath = a.root().Path
	} else if p, n := strings.TrimSpace(a.form.Parent), strings.TrimSpace(a.form.Name); p != "" && n != "" {
		summaryPath = filepath.Join(p, n)
	} else if n != "" {
		summaryPath = n
	}

	ui.Box(c).FillWidth().Height(1).Background(line).Shrink(0)
	ui.Row(c).FillWidth().Shrink(0).Padding(14*s, 24*s).Gap(16 * s).Background(panel).AlignItems(ui.Center).Children(func() {
		ui.Row(c).Grow(1).MinWidth(0).Gap(10 * s).AlignItems(ui.Center).Children(func() {
			if !loaderLogo(c, a.form.Loader, 32*s) {
				ui.Box(c).Size(32*s, 32*s).Radius(iconRadius(32*s, s)).Shrink(0).Background(raised).Border(1, line).Center().Children(func() {
					ui.Icon(c, icons[loaderIconName(a.form.Loader)]).Size(16*s, 16*s).TextColor(loaderTone(a.form.Loader))
				})
			}
			ui.Column(c).Grow(1).MinWidth(0).Gap(2 * s).Children(func() {
				ui.Row(c).FillWidth().MinWidth(0).Gap(8 * s).AlignItems(ui.Center).Children(func() {
					ui.Text(c, summaryTitle).FontSize(typeCallout * s).FontWeight(650).TextColor(ink).NoWrap().Shrink(0)
					if summaryLoader != "" {
						tagChip(c, summaryLoader)
					}
				})
				ui.Text(c, summaryPath).FontSize(typeCaption * s).TextColor(muted).SingleLine()
			})
		})
		ui.Row(c).Shrink(0).Gap(10 * s).AlignItems(ui.Center).Children(func() {
			if why != "" {
				whyHint(c, why)
			}
			if buttonWhy(c, "install-game-now", t.InstallConfirmBtn, "download", true, why, false) {
				root := ""
				if isCurrent {
					root = a.root().ID
				}
				a.execute("installGame", map[string]string{"rootId": root, "parent": a.form.Parent, "name": a.form.Name, "minecraft": a.form.Minecraft, "loader": curLoader, "loaderVersion": a.form.LoaderVersion}, a.taskAdded)
			}
		})
	})
}
