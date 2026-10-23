package nativeui

import (
	"strings"

	"github.com/egoist/mygo/ui"

	"github.com/Viking602/YoloMCL/internal/launcher"
)

func loaderBuild(v launcher.VersionSummary) string {
	if loaderKey(v.Loader) == "vanilla" {
		return ""
	}
	if v.LoaderVersion != "" {
		return v.LoaderVersion
	}
	return loaderVersionOf(v)
}

// Show the loader's artwork and build below the instance name. The full loader
// name stays on hover without moving the neighbouring history badge.
func (a *App) heroLoaderBadge(c *ui.Context, v launcher.VersionSummary) {
	s := uiScale(c)
	name, version := a.loaderDisplay(v.Loader), loaderBuild(v)
	if loaderKey(v.Loader) == "vanilla" {
		name = a.tr().LoaderVanilla
	}
	tip := strings.TrimSpace(name + " " + version)
	motion := ui.ElementTransition{}
	if !c.Preferences().ReduceMotion {
		motion = ui.ElementTransition{Animation: &motionFast, Enter: &ui.Motion{Opacity: .3, Y: 3 * s}}
	}
	ui.Row(c.Key("hero-loader-badge")).Label(a.L("实例加载器", "Instance loader")).Tooltip(tip).
		Height(24*s).MinWidth(28*s).Shrink(0).Padding(0, 7*s, 0, 4*s).Gap(5*s).Radius(8*s).
		Background(ui.RGBA(255, 255, 255, .82)).Border(1, ui.RGBA(60, 72, 110, .08)).
		Font(uiSansFamily()).AlignItems(ui.Center).Children(func() {
		ui.Box(c.Key("hero-loader-content-"+loaderKey(v.Loader))).Size(18*s, 18*s).Shrink(0).Center().Transition(motion).Children(func() {
			if !loaderLogo(c, v.Loader, 18*s) {
				ui.Icon(c, icons[loaderIconName(v.Loader)]).Size(16*s, 16*s).TextColor(inkSecondary).Shrink(0)
			}
		})
		if version != "" {
			ui.Text(c.Key("hero-loader-build-"+version), version).Label("加载器版本 " + version).FontSize(typeCaption * s).FontWeight(500).TextColor(inkSecondary).NoWrap().Shrink(0).Transition(motion)
		}
	})
}

type heroHistoryBadge struct {
	label, value, tip, icon string
}

func (a *App) heroHistory(r launcher.Instance) heroHistoryBadge {
	record := heroHistoryBadge{value: a.L("读取中…", "Loading…"), tip: a.L("读取游玩记录…", "Loading play history…"), icon: "clock"}
	if verRunning(&r) {
		record.value, record.tip, record.icon = a.L("正在游玩", "Playing now"), a.L("游戏正在运行", "The game is running"), "play"
	} else if a.instStatusKey == r.ID+"|"+r.SelectedVersion && a.instStatus.Dir != "" {
		st := a.instStatus
		record.tip = a.playSummary(r, st)
		when := st.Play.LastStart
		if st.Play.Launches == 0 {
			when = st.LastLog
		}
		if when.IsZero() {
			record.value = a.L("暂无游玩记录", "Not played yet")
		} else {
			record.label = a.L("上次游玩", "Played")
			record.value = when.Local().Format(a.L("1月2日", "Jan 2"))
			record.tip = when.Local().Format("2006-01-02 15:04") + "\n" + record.tip
		}
	} else if prior := a.contentSwaps["hero-history"]; prior != nil && a.instStatusErr == "" {
		// Hold the previous text during a cold read instead of flashing a loader.
		record = prior.value.(heroHistoryBadge)
	} else if a.instStatusErr != "" {
		record.value, record.tip, record.icon = a.L("记录不可用", "History unavailable"), a.instStatusErr, "info"
	}
	return record
}

// A quiet, fixed-width history readout beside the launch controls. Wide
// windows give the date its own line; compact windows keep it in the metadata
// row. Only the contents transition, and changing date digits flip in place.
func (a *App) heroPlayRecord(c *ui.Context, r launcher.Instance, stacked bool) {
	s := uiScale(c)
	record := a.heroHistory(r)
	signature := record.label + "|" + record.icon
	if record.label == "" {
		signature += "|" + record.value
	}
	shown := swappedContent(a, c, "hero-history", signature, record)
	fg, iconColor := inkSecondary, muted
	if shown.icon == "play" {
		fg, iconColor = successInk, successInk
	}
	badge := ui.Box(c.Key("hero-history-slot")).Label(a.L("游玩记录", "Play history")).
		Width(132 * s).Height(24 * s).Shrink(0).Clip().Justify(ui.Center).
		Font(uiSansFamily()).Tooltip(shown.tip)
	if stacked {
		badge.Width(112*s).Height(44*s).Padding(0, 0, 0, 16*s).Draw(func(p *ui.Painter, b ui.Rect) {
			p.Fill(ui.Rect{X: b.X, Y: b.Y + 5*s, W: s, H: b.H - 10*s}, lineStrong, 0)
		})
	}
	badge.Children(func() {
		a.contentBody(c, "hero-history").Children(func() {
			content := ui.Row(c).Label(strings.TrimSpace(shown.label + " " + shown.value)).FillWidth().MinWidth(0).Gap(7 * s).AlignItems(ui.Center).Justify(ui.End)
			if stacked {
				content.Column().Gap(3 * s).AlignItems(ui.Start).Justify(ui.Center)
			}
			content.Children(func() {
				if stacked || shown.label != "" {
					ui.Row(c).Gap(5 * s).AlignItems(ui.Center).Shrink(0).Children(func() {
						if stacked {
							ui.Icon(c, icons[shown.icon]).Size(11*s, 11*s).TextColor(iconColor).Shrink(0)
						}
						caption := shown.label
						if caption == "" {
							caption = a.L("游玩记录", "Play history")
						}
						ui.Text(c, caption).FontSize(typeCaption2 * s).TextColor(muted).NoWrap()
					})
				}
				if shown.label == "" {
					ui.Text(c, shown.value).FontSize(typeCaption * s).FontWeight(500).TextColor(fg).SingleLine().MinWidth(0).Shrink(1)
				} else {
					size := typeFootnote
					if stacked {
						size = typeBody
					}
					a.flipNumber(c, "hero-history-date", shown.value, size*s, 600, fg).Shrink(1)
				}
			})
		})
	})
}
