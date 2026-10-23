package nativeui

import (
	"fmt"

	"github.com/egoist/mygo/ui"

	"github.com/Viking602/YoloMCL/internal/launcher"
)

// instIcon draws a version's own icon (modpack icon on disk) or its loader
// logo, on a white tile.
func (a *App) instIcon(c *ui.Context, r launcher.Instance, v *launcher.VersionSummary, size float32) {
	ui.Box(c).Size(size, size).Radius(size * .24).Background(panel).Center().Shrink(0).Clip().Children(func() {
		if v != nil {
			if img := a.versionIcon(r.ID, v.ID); img != nil {
				ui.Image(c, img).Size(size, size).Fit(ui.Contain)
				return
			}
			if loaderLogo(c, v.Loader, size*.70) {
				return
			}
		}
		name := "gamepad"
		if v != nil {
			name = loaderIconName(v.Loader)
		}
		ui.Icon(c, icons[name]).Size(size*.48, size*.48).TextColor(accent)
	})
}

// heroChip is one hero meta chip; every chip shares height, radius, border
// and type. Strong chips use the accent tint.
func heroChip(c *ui.Context, icon, text string, strong bool) ui.Element {
	s := uiScale(c)
	bg, fg, edge := ui.RGBA(255, 255, 255, .72), inkSecondary, ui.RGBA(60, 72, 110, .10)
	if strong {
		bg, fg, edge = accent.Alpha(.10), accent, accent.Alpha(.18)
	}
	// Optional metadata gives way to the complete loader build and history.
	return ui.Row(c).Shrink(1).MinWidth(0).Height(24*s).PaddingX(9*s).Gap(5*s).Radius(7*s).Background(bg).Border(1, edge).AlignItems(ui.Center).Tooltip(text).Children(func() {
		if icon != "" && icons[icon] != nil {
			ui.Icon(c, icons[icon]).Size(12*s, 12*s).TextColor(fg).Shrink(0)
		}
		ui.Text(c, text).FontSize(typeFootnote * s).FontWeight(600).TextColor(fg).SingleLine().Shrink(1).MinWidth(0)
	})
}

func (a *App) instanceHero(c *ui.Context, r launcher.Instance) {
	s := uiScale(c)
	w, _ := c.Size()
	historyBesideTitle := w >= 1200
	t := a.tr()
	v := a.gameVersion()
	ui.Box(c).FillWidth().Padding(20*s, 24*s, 0, 24*s).Shrink(0).Children(func() {
		ui.Row(c.Key("inst-hero")).Label("主版本卡").FillWidth().Padding(22*s, 24*s).Gap(20*s).Radius(radiusCard*s).AlignItems(ui.Center).Clip().
			LinearGradient(ui.LinearGradient{Angle: 120, Stops: []ui.GradientStop{{Color: ui.Hex("#e8f0ff"), At: 0}, {Color: ui.Hex("#f1ecfd"), At: .6}, {Color: ui.Hex("#eaf6fb"), At: 1}}}).
			Border(1, ui.RGBA(120, 130, 170, .14)).Children(func() {
			ui.Box(c.Key("hero-icon-slot")).Label("版本图标").Size(68*s, 68*s).Shrink(0).Shadow(0, 4*s, 12*s, 0, ui.RGBA(40, 52, 110, .10)).Radius(16 * s).Background(panel).Children(func() {
				ui.Box(c.Key("hero-icon-" + a.versionSwapKey())).Fill().Transition(heroSwap(c)).Children(func() {
					a.instIcon(c, r, v, 68*s)
				})
			})
			ui.Column(c.Key("hero-text-slot")).Label("版本标题").Height(68 * s).Grow(1).Basis(0).MinWidth(0).Gap(8 * s).Justify(ui.Center).Children(func() {
				ui.Box(c.Key("hero-name-slot")).FillWidth().Height(36 * s).Shrink(0).MinWidth(0).Justify(ui.Center).Children(func() {
					if v == nil {
						ui.Text(c, t.HomeHeroNoVersion).FillWidth().MinWidth(0).FontSize(typeHero * s).Bold().TextColor(ink).SingleLine()
						return
					}
					title, _ := a.cardText(r, *v)
					tags := a.versionTags(r, *v, true)
					ui.Box(c.Key("hero-name-" + title + fmt.Sprint(tags))).FillWidth().MinWidth(0).Clip().Transition(heroSwap(c)).Children(func() {
						titleWithTags(c, title, v.ID, typeHero*s, 750, tags, 4)
					})
				})
				ui.Row(c.Key("hero-meta")).Label("实例信息").FillWidth().Height(24 * s).MinWidth(0).Gap(12 * s).AlignItems(ui.Center).Children(func() {
					ui.Row(c.Key("hero-loader-slot")).Grow(1).Basis(0).MinWidth(0).Gap(6 * s).AlignItems(ui.Center).Clip().Children(func() {
						if v == nil {
							label(c, dirLabel(r)).FontSize(typeCallout * s).SingleLine()
							return
						}
						a.heroLoaderBadge(c, *v)
						if isPackVersion(r, *v) {
							switch r.Pack.Source {
							case "curseforge":
								heroChip(c, "", "CurseForge", false)
							case "modrinth":
								heroChip(c, "", "Modrinth", false)
							}
						}
						if isPackVersion(r, *v) {
							a.heroPackStatus(c, r)
						}
					})
					if !historyBesideTitle {
						a.heroPlayRecord(c, r, false)
					}
				})
			})
			if historyBesideTitle {
				a.heroPlayRecord(c, r, true)
			}
			if iconOnlyButton(c, "hero-folder", "folder", a.L("打开文件夹", "Open folder"), btnLg*s, false) {
				a.openSub("")
			}
			if verRunning(&r) && fixedButton(c, "dock-stop", t.HomeStopBtn, "cross", 0, btnLg*s, false, a.instancePending("stop")) {
				a.execute("stop", map[string]string{"rootId": r.ID}, nil)
			}
			launchText, icon := t.HomeLaunchBtn, "play"
			if verRunning(&r) {
				launchText, icon = a.L("再开一个", "Run another"), "plus"
			}
			why := whyOf(a.instancePending("launch", "launchAgain"), t.WhyBusy, v == nil, t.WhyNoVersion)
			pendingWhy = why
			if fixedButton(c, "launch", launchText, icon, 136*s, btnLg*s, !verRunning(&r), why != "") {
				if verRunning(&r) {
					a.openLaunchAgain(r)
				} else {
					a.launch()
				}
			}
		})
	})
}
