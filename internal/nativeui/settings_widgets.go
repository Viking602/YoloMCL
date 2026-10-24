package nativeui

import (
	"strings"

	"github.com/egoist/mygo/ui"
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
	settingPadX      float32 = 20
	settingIcon      float32 = 30
	settingGap       float32 = 14
	settingTextInset         = settingPadX + settingIcon + settingGap
)

// settingRow lays out one row: leading icon, title and description on the
// left, the control on the right, and optionally a full-width control below
// aligned with the title.
func settingRow(c *ui.Context, leading func(), titleText, desc string, trailing, below func()) {
	s := uiScale(c)
	ui.Column(c).FillWidth().Padding(11*s, settingPadX*s).Gap(8 * s).Children(func() {
		ui.Row(c).FillWidth().MinHeight(30 * s).Gap(settingGap * s).AlignItems(ui.Center).Children(func() {
			if leading != nil {
				leading()
			}
			ui.Column(c).Grow(1).MinWidth(0).Clip().Gap(3 * s).Children(func() {
				ui.Text(c, titleText).FontSize(typeBody * s).FontWeight(600).TextColor(ink).SingleLine()
				if desc != "" {
					label(c, desc).FillWidth().MinWidth(0).FontSize(typeFootnote * s) // descriptions wrap, never ellipsize
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
			b := ui.ButtonBase(c.Key(key + "-" + opt)).Label(opt).Height(height).PaddingX(10 * s).Radius(height / 2 * .62).AlignItems(ui.Center).Justify(ui.Center)
			fg := inkSecondary
			if active {
				fg = ink
			} else if b.Hovered() {
				b.Background(ui.RGBA(255, 255, 255, .4))
			}
			if b.Children(func() {
				ui.Text(c, opt).FontSize(min(typeFootnote*s, height*.48)).FontWeight(600).TextColor(fg).NoWrap()
			}).Clicked() {
				clicked, changed = opt, true
			}
		}
	})
	return clicked, changed
}

func nativeToggle(c *ui.Context, key, label string, on *bool) ui.Element {
	s := uiScale(c)
	toggle := ui.SwitchBase(c.Key(key), on).Label(label).Size(42*s, 24*s).Shrink(0)
	targetFill := ui.RGBA(120, 120, 128, .2)
	targetOffset := float32(2)
	if *on {
		targetFill = success
		targetOffset = 20
	}
	animOffset := toggle.AnimateTo("knob", targetOffset, motionMove)
	toggle.Radius(12 * s).Background(toggle.AnimateColor("bg", targetFill, fastAnim)).Draw(func(p *ui.Painter, r ui.Rect) {
		knob := ui.Rect{X: r.X + animOffset*s, Y: r.Y + 2*s, W: 20 * s, H: 20 * s}
		p.Shadow(knob, 10*s, 0, 1.5*s, 4*s, 0, ui.RGBA(0, 0, 0, .18))
		p.Fill(knob, panel, 10*s)
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
			bgCol := ui.RGBA(255, 255, 255, .7)
			fgCol := inkSecondary
			borderCol := lineStrong
			if active {
				bgCol = accentSoft
				fgCol = accent
				borderCol = ui.RGBA(0, 122, 255, .35)
			}
			chip := ui.ButtonBase(c.Key(keyPrefix+"-"+preset.id)).Label(preset.label).Height(btnSm*s).PaddingX(12*s).Radius(999).Border(1, borderCol).AlignItems(ui.Center)
			if chip.Hovered() && !active {
				bgCol = panel
			}
			chip.Background(chip.AnimateColor("bg", bgCol, fastAnim))
			if chip.Children(func() {
				ui.Text(c, preset.label).FontSize(typeFootnote * s).FontWeight(600).TextColor(fgCol).NoWrap()
			}).Clicked() {
				*target = preset.args
				if changed != nil {
					changed()
				}
			}
		}
		if strings.TrimSpace(*target) != "" {
			clearChip := ui.ButtonBase(c.Key(keyPrefix + "-clear")).Label(t.JVMPresetClear).Height(btnSm * s).PaddingX(12 * s).Radius(999).Background(fieldColor).AlignItems(ui.Center)
			if clearChip.Children(func() {
				ui.Text(c, t.JVMPresetClear).FontSize(typeFootnote * s).TextColor(muted).NoWrap()
			}).Clicked() {
				*target = ""
				if changed != nil {
					changed()
				}
			}
		}
	})
	ui.Row(c).FillWidth().Clip().Padding(9*s, 12*s).Radius(12 * s).LinearGradient(ui.LinearGradient{From: ui.RGBA(10, 132, 255, .08), To: ui.RGBA(175, 82, 222, .07), Angle: 90}).Gap(8 * s).AlignItems(ui.Center).Children(func() {
		ui.Icon(c, icons["info"]).Size(14*s, 14*s).TextColor(accent).Shrink(0)
		label(c, jvmAdviceText(*target, t)).Grow(1).MinWidth(0).FontSize(typeFootnote * s)
	})
}

// Keep navigation and reset in a fixed toolbar above the grouped settings.
