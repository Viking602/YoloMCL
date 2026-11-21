package nativeui

import (
	"strings"
	"time"

	"github.com/egoist/mygo/ui"
	"github.com/lobehub/lobe-mygo/pkg/choice"
	"github.com/lobehub/lobe-mygo/pkg/motion"
)

const (
	jvmPresetG1      = "-XX:+UseG1GC -XX:+ParallelRefProcEnabled -XX:MaxGCPauseMillis=50 -XX:+UnlockExperimentalVMOptions -XX:+DisableExplicitGC -XX:G1NewSizePercent=20 -XX:G1ReservePercent=20 -XX:G1HeapRegionSize=32M"
	jvmPresetZGC     = "-XX:+UseZGC -XX:+UnlockExperimentalVMOptions -XX:+DisableExplicitGC -XX:+AlwaysPreTouch"
	jvmPresetCompact = "-XX:+UseG1GC -XX:MaxGCPauseMillis=35 -XX:+UseStringDeduplication -XX:G1HeapRegionSize=16M"
)

// settingsGroup is a System Settings-style inset grouped list: a caption
// header above a frosted card, rows split by hairlines inset to the text
// column, and an optional footnote below explaining the whole group.
func settingsGroup(c *ui.Context, key, titleText, desc string, rows ...func()) {
	s := uiScale(c)
	ui.Column(c).FillWidth().Gap(space2 * s).Children(func() {
		ui.Text(c, titleText).FontSize(typeCallout * s).FontWeight(650).TextColor(inkSecondary).PaddingX(space4 * s).SingleLine()
		glass(ui.Column(c.Key(key)).Label(key).FillWidth().Clip(), radiusCard*s, s).Children(func() {
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

// Row geometry keeps titles, dividers and below-row controls on one text column.
const (
	settingPadX      float32 = 20
	settingIcon      float32 = 30
	settingGap       float32 = space4
	settingTextInset         = settingPadX + settingIcon + settingGap
)

// settingRow lays out one row: leading icon, title and description on the
// left, the control on the right, and optionally a full-width control below
// aligned with the title.
func settingRow(c *ui.Context, leading func(), titleText, desc string, trailing, below func()) {
	var detail func()
	if desc != "" {
		detail = func() { label(c, desc).FillWidth().MinWidth(0).FontSize(typeFootnote * uiScale(c)) }
	}
	settingRowWithDetail(c, leading, titleText, detail, trailing, below)
}

// settingRowWithDetail keeps the shared row geometry while allowing an
// existing badge or other structured metadata below its title.
func settingRowWithDetail(c *ui.Context, leading func(), titleText string, detail, trailing, below func()) {
	s := uiScale(c)
	ui.Column(c).FillWidth().Padding(12*s, settingPadX*s).Gap(8 * s).Children(func() {
		ui.Row(c).FillWidth().MinHeight(30 * s).Gap(settingGap * s).AlignItems(ui.Center).Children(func() {
			if leading != nil {
				leading()
			}
			ui.Column(c).Grow(1).MinWidth(0).Clip().Gap(4 * s).Children(func() {
				ui.Text(c, titleText).FontSize(typeBody * s).FontWeight(600).TextColor(ink).SingleLine()
				if detail != nil {
					detail()
				}
			})
			if trailing != nil {
				ui.Row(c).Shrink(0).Gap(space2 * s).AlignItems(ui.Center).Children(trailing)
			}
		})
		if below != nil {
			ui.Column(c).FillWidth().Gap(space2 * s).Children(below)
		}
	})
}

// rowIcon used to give each settings row a colored squircle; the decluttered
// layout drops them so titles carry the hierarchy. Kept for call-site clarity.
func rowIcon(c *ui.Context, icon string, tone [2]ui.Color) func() {
	return nil
}

type lobeSegMatch struct{ selected *int }

var lobeSegPress = ui.Animation{Duration: 120 * time.Millisecond, Ease: motion.EaseOut}

// segmented renders a scaled Lobe UI segmented control (matching lobe-mygo
// action.Segmented's SegmentedBase + FLIP sliding thumb + press scale, with
// high-contrast neutral track and elevated white active pill that stay crisp
// on pure-white cards across all window scales).
func segmented(c *ui.Context, key string, options []string, selected string, height float32) (string, bool) {
	if len(options) == 0 {
		return selected, false
	}
	s := uiScale(c)
	clicked, changed := selected, false
	host := ui.Box(c.Key(key)).Shrink(0)
	idx := ui.Local(host, "seg-idx", func() int { return -1 })
	cur := -1
	for i, opt := range options {
		if opt == selected {
			cur = i
			break
		}
	}
	*idx = cur
	innerRadius := radiusControl * s
	outerRadius := (radiusControl + 2) * s
	padX := space3 * s
	fontSize := min(typeFootnote*s, height*.52)
	slide := ui.ElementTransition{Duration: 220 * time.Millisecond, Ease: motion.EaseOut, Match: lobeSegMatch{idx}}
	host.Children(func() {
		parts := ui.SegmentedBase(c, idx, len(options))
		track := parts.Track.Padding(3*s).Gap(2*s).Radius(outerRadius).Background(lobeLight.FillSecondary).Border(1, lobeLight.FillQuaternary)
		track.Children(func() {
			for i, option := range options {
				active := i == *idx
				seg := parts.Segment(i).Height(height).PaddingX(padX).Radius(innerRadius).
					FontSize(fontSize).FontWeight(550).Cursor(ui.CursorPointer)
				fg := inkSecondary
				if active {
					fg = ink
					seg.FontWeight(650)
				} else if seg.Hovered() {
					fg = ink
				}
				pose := ui.Pose{}
				if seg.Pressed() {
					pose.Scale = 0.98
				}
				seg.TextColor(fg).AnimatePose(pose, lobeSegPress)
				seg.Children(func() {
					if active {
						ui.Box(c).Absolute().Top(0).Left(0).Right(0).Bottom(0).
							Radius(innerRadius).Background(panel).Border(1, lineStrong).
							Shadow(0, s, 2*s, 0, ui.RGBA(0, 0, 0, .06)).
							ZIndex(-1).IgnoreInput(true).Transition(slide)
					}
					ui.Text(c, option).SingleLine()
				})
			}
		})
		if track.Changed() && *idx >= 0 && *idx < len(options) {
			clicked, changed = options[*idx], true
		}
	})
	return clicked, changed
}

// nativeToggle renders Lobe UI's spring-stretching choice.Switch.
func nativeToggle(c *ui.Context, key, label string, on *bool) ui.Element {
	return choice.Switch(c.Key(key), on, choice.SwitchProps{}).Label(label).Shrink(0)
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
		label(c, t.JVMPresetTitle).FontSize(typeFootnote * s).FontWeight(600).NoWrap().Shrink(0)
		for _, item := range []struct {
			id, label, args string
		}{
			{"g1", t.JVMPresetG1, jvmPresetG1},
			{"zgc", t.JVMPresetZGC, jvmPresetZGC},
			{"compact", t.JVMPresetCompact, jvmPresetCompact},
		} {
			preset := item
			active := strings.TrimSpace(*target) == preset.args
			bgCol := panel
			fgCol := inkSecondary
			borderCol := lineStrong
			if active {
				bgCol = lobePrimary
				fgCol = lobeLight.PrimaryForeground
				borderCol = lobePrimary
			}
			chip := ui.ButtonBase(c.Key(keyPrefix+"-"+preset.id)).Label(preset.label).Height(btnSm*s).PaddingX(12*s).Radius(radiusControl*s).Border(1, borderCol).AlignItems(ui.Center)
			if chip.Hovered() && !active {
				bgCol = raised
				fgCol = ink
			}
			chip.Background(chip.AnimateColor("bg", bgCol, fastAnim))
			if chip.Children(func() {
				ui.Text(c, preset.label).FontSize(typeFootnote * s).FontWeight(550).TextColor(fgCol).NoWrap()
			}).Clicked() {
				*target = preset.args
				if changed != nil {
					changed()
				}
			}
		}
		if strings.TrimSpace(*target) != "" {
			clearChip := ui.ButtonBase(c.Key(keyPrefix + "-clear")).Label(t.JVMPresetClear).Height(btnSm * s).PaddingX(12 * s).Radius(radiusControl * s).Background(fieldColor).AlignItems(ui.Center)
			if clearChip.Children(func() {
				ui.Text(c, t.JVMPresetClear).FontSize(typeFootnote * s).TextColor(inkSecondary).NoWrap()
			}).Clicked() {
				*target = ""
				if changed != nil {
					changed()
				}
			}
		}
	})
	lobeAlert(c, "info", jvmAdviceText(*target, t))
}

// Keep navigation and reset in a fixed toolbar above the grouped settings.
