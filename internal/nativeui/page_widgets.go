package nativeui

import (
	"time"

	"github.com/egoist/mygo/ui"
	"github.com/lobehub/lobe-mygo/pkg/feedback"
	"github.com/lobehub/lobe-mygo/pkg/lucide"
	lobetheme "github.com/lobehub/lobe-mygo/pkg/theme"
)

func heading(c *ui.Context, _, text, description string, actions func()) {
	pageHeader(c, nil, text, description, actions)
}

// pageHeader is the one page header: optional breadcrumb, large title,
// muted subtitle, actions on the right — aligned to the content grid.
func pageHeader(c *ui.Context, crumb func(), text, description string, actions func()) {
	s := uiScale(c)
	ui.Row(c).Label("页面标题栏").FillWidth().Gap(space5 * s).AlignItems(ui.End).Children(func() {
		ui.Column(c).Grow(1).MinWidth(0).Shrink(1).Clip().Gap(space1 * s).Children(func() {
			if crumb != nil {
				crumb()
			}
			titleLabel := ui.Text(c, text).Label("页面标题 " + text).FontSize(typeDisplay * s).FontWeight(750).TextColor(ink).MinWidth(0).SingleLine()
			textTooltip(c, titleLabel, text, "", ui.Font{Family: c.Theme().Font, Size: typeDisplay * s, Weight: 750}, 1)
			if description != "" {
				bodyText(c, description).FontSize(typeCallout * s).TextColor(muted) // wraps: descriptive text never ellipsizes
			}
		})
		if actions != nil {
			ui.Row(c).Shrink(0).Gap(space2 * s).AlignItems(ui.Center).Children(actions)
		}
	})
}

func appleSoftIconBox(c *ui.Context, size float32, iconName string, from, to, fg ui.Color) {
	ui.Box(c).Size(size, size).Radius(iconRadius(size, uiScale(c))).Shrink(0).LinearGradient(tintGradient(from, to)).InsetShadow(0, 1, 0, 0, ui.RGBA(255, 255, 255, .35)).Shadow(0, size*.08, size*.25, -size*.06, from.Alpha(.35)).Center().Children(func() {
		if icons[iconName] != nil {
			ui.Icon(c, icons[iconName]).Size(size*.52, size*.52).TextColor(fg)
		}
	})
}

// Lobe UI soft-tinted status pill badge.
func badge(c *ui.Context, s string, color ui.Color) {
	scale := uiScale(c)
	fill, edge, fg := raised, line, inkSecondary
	switch color {
	case accent, blue:
		fill, edge, fg = lobeLight.InfoBg, lobeLight.InfoBorder, lobeLight.InfoText
	case success, successInk:
		fill, edge, fg = lobeLight.SuccessBg, lobeLight.SuccessBorder, lobeLight.SuccessText
	case danger:
		fill, edge, fg = lobeLight.ErrorBg, lobeLight.ErrorBorder, lobeLight.ErrorText
	case warning, warningInk:
		fill, edge, fg = lobeLight.WarningBg, lobeLight.WarningBorder, lobeLight.WarningText
	}
	ui.Row(c).Shrink(0).Padding(3*scale, space2*scale).Radius(radiusChip*scale).Background(fill).Border(1, edge).Children(func() {
		ui.Text(c, s).FontSize(typeCaption * scale).FontWeight(550).TextColor(fg).NoWrap().Shrink(0)
	})
}

func tagChip(c *ui.Context, s string) {
	scale := uiScale(c)
	ui.Row(c).Shrink(0).Padding(3*scale, space2*scale).Radius(radiusChip*scale).Background(raised).Border(1, line).Children(func() {
		ui.Text(c, s).FontSize(typeCaption2 * scale).FontWeight(500).TextColor(inkSecondary).NoWrap().Shrink(0)
	})
}

func card(c *ui.Context, key string, fn func()) {
	s := uiScale(c)
	glass(ui.Column(c.Key(key)).FillWidth().Clip().Gap(16*s).Padding(20*s, 24*s), radiusCard*s, s).Children(fn)
}

// textField is the one single-line input styled to match Lobe UI InputOutlined:
// pure container surface, hairline border that deepens on hover and sharpens to
// primary on focus with a quiet focus halo.
func textField(c *ui.Context, value *string) ui.Element {
	s := uiScale(c)
	e := ui.TextInput(c, value).Height(btnMd*s).Padding(0, 12*s).FontSize(typeCallout * s).Radius(radiusControl * s).Background(panel)
	edge := lineStrong
	if e.Focused() {
		edge = lobePrimary
		e.Shadow(0, 0, 0, 2*s, lobeLight.PrimaryBg)
	} else if e.Hovered() {
		edge = ui.Hex("#cfcfcf")
	}
	e.Border(s, e.AnimateColor("input-edge", edge, fastAnim))
	if inDialog && e.Shortcut(0, ui.KeyEnter) && lastDialogPrimary != "" {
		enterFired = lastDialogPrimary
	}
	return e
}

func empty(c *ui.Context, titleText, description string) {
	emptyState(c, "empty", "folder", titleText, description, nil)
}

// loadingState is the shared in-progress placeholder: a Lobe card whose
// icon and message are centered on both axes, so loading text never hugs a
// corner of its container.
func loadingState(c *ui.Context, key, text string) {
	s := uiScale(c)
	glass(ui.Column(c.Key(key)).Label(text).FillWidth().MinHeight(132*s).Padding(space6*s), radiusCard*s, s).AlignItems(ui.Center).Justify(ui.Center).Gap(space3 * s).Children(func() {
		statusSpinner(c, 22*s)
		ui.Text(c, text).FontSize(typeCallout * s).FontWeight(500).TextColor(muted).TextAlign(ui.Center).SingleLine()
	})
}

// statusLine centers a short status message inside a form or dialog body.
func statusLine(c *ui.Context, text string, color ui.Color, busy bool) {
	s := uiScale(c)
	ui.Row(c).FillWidth().MinHeight(44*s).Padding(space2*s, space4*s).Radius(radiusControl*s).Background(raised).Border(1, line).AlignItems(ui.Center).Justify(ui.Center).Gap(space2 * s).Children(func() {
		if busy {
			statusSpinner(c, 16*s)
		}
		tooltip(c, ui.Text(c, text).FontSize(typeCallout*s).FontWeight(500).TextColor(color).TextAlign(ui.Center).MinWidth(0).Shrink(1).SingleLine(), text)
	})
}

// statusSpinner renders Lobe UI's Lucide loader ring rotated on a quiet timer
// so idle frame-pending checks for unrelated controls stay deterministic.
func statusSpinner(c *ui.Context, size float32) {
	angle := float32(0)
	if !c.Preferences().ReduceMotion {
		angle = float32(c.Now().UnixMilli()%900) / 900 * 360
		c.After(33 * time.Millisecond)
	}
	ui.Icon(c, lucide.LoaderCircle()).Size(size, size).TextColor(inkSecondary).Shrink(0).Rotate(angle)
}

// progressBar renders Lobe UI's feedback.Progress line bar.
func progressBar(c *ui.Context, frac float64, status feedback.ProgressStatus) {
	if c.Preferences().ReduceMotion && status == feedback.ProgressActive {
		status = feedback.ProgressNormal
	}
	pct := float32(min(max(frac, 0), 1) * 100)
	feedback.Progress(c, pct, feedback.ProgressProps{
		Status: status,
		Size:   lobetheme.Middle,
	})
}
