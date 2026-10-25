package nativeui

import (
	"strings"

	"github.com/egoist/mygo/ui"
)

var emptyArt = map[string]*ui.SVG{}

// Static, shared artwork: no timers, image downloads or per-frame SVG parsing.
func loadEmptyArt() {
	for name, glyph := range map[string]string{
		"world":    `<path d="m12 2 9 5v10l-9 5-9-5V7Zm-9 5 9 5 9-5M12 12v10M7.5 4.5l9 5"/>`,
		"server":   `<rect x="3" y="3" width="18" height="7" rx="2"/><rect x="3" y="14" width="18" height="7" rx="2"/><path d="M7 6.5h.01M7 17.5h.01M15 6.5h3M15 17.5h3"/>`,
		"image":    `<rect x="2" y="3" width="20" height="18" rx="3"/><circle cx="8" cy="9" r="2"/><path d="m3 18 5-4 4 3 5-6 5 7"/>`,
		"folder":   `<path d="M3 7V5a2 2 0 0 1 2-2h5l3 3h6a2 2 0 0 1 2 2v1M3 7h6l2 2h10v10a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2Z"/>`,
		"mods":     `<rect x="3" y="3" width="7" height="7" rx="2"/><rect x="14" y="3" width="7" height="7" rx="2"/><rect x="3" y="14" width="7" height="7" rx="2"/><path d="M14 17.5h7M17.5 14v7"/>`,
		"layers":   `<path d="m12 3 10 5-10 5L2 8Zm-10 10 10 5 10-5M2 18l10 5 10-5"/>`,
		"sun":      `<circle cx="12" cy="12" r="5"/><path d="M12 1v2M12 21v2M1 12h2M21 12h2M4 4l1.5 1.5M18.5 18.5 20 20M4 20l1.5-1.5M18.5 5.5 20 4"/>`,
		"gamepad":  `<path d="M7 6h10a4 4 0 0 1 4 3l1 8a2 2 0 0 1-3 2l-4-3H9l-4 3a2 2 0 0 1-3-2l1-8a4 4 0 0 1 4-3ZM7 9v5M4.5 11.5h5M16 10h.01M19 13h.01"/>`,
		"search":   `<circle cx="10" cy="10" r="7"/><path d="m15 15 6 6M7 10h6"/>`,
		"tasks":    `<path d="M12 2v13m-5-5 5 5 5-5M3 15v4a2 2 0 0 0 2 2h14a2 2 0 0 0 2-2v-4"/>`,
		"account":  `<circle cx="12" cy="7" r="4"/><path d="M4 21v-2a8 6 0 0 1 16 0v2"/>`,
		"terminal": `<rect x="2" y="3" width="20" height="18" rx="3"/><path d="m6 8 4 4-4 4M13 16h5"/>`,
		"info":     `<circle cx="12" cy="12" r="9"/><path d="M12 11v6M12 7h.01"/>`,
	} {
		source := `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 128 80">
<ellipse cx="64" cy="43" rx="57" ry="30" fill="currentColor" opacity=".045"/>
<rect x="15" y="25" width="34" height="38" rx="9" fill="currentColor" opacity=".06"/>
<rect x="79" y="21" width="34" height="38" rx="9" fill="currentColor" opacity=".06"/>
<path d="M22 36h12m-12 6h8m60-9h13m-13 6h9" fill="none" stroke="currentColor" stroke-opacity=".13" stroke-width="2" stroke-linecap="round"/>
<ellipse cx="64" cy="72" rx="29" ry="3" fill="currentColor" opacity=".045"/>
<rect x="38" y="8" width="52" height="59" rx="13" fill="#fff"/>
<rect x="38" y="8" width="52" height="59" rx="13" fill="none" stroke="currentColor" stroke-opacity=".2"/>
<g transform="translate(52 22)" fill="none" stroke="currentColor" stroke-width="1.6" stroke-linecap="round" stroke-linejoin="round">` + glyph + `</g>
<path d="M57 55h14" stroke="currentColor" stroke-opacity=".2" stroke-width="2" stroke-linecap="round"/>
<path d="M103 9v6m-3-3h6" stroke="currentColor" stroke-opacity=".25" stroke-width="1.5" stroke-linecap="round"/>
</svg>`
		emptyArt[name] = ui.MustParseSVG([]byte(strings.ReplaceAll(source, "currentColor", emptyArtColor(name))))
	}
	emptyArt["cube"], emptyArt["globe"] = emptyArt["world"], emptyArt["world"]
	emptyArt["skin"] = emptyArt["account"]
}

func emptyArtColor(icon string) string {
	switch icon {
	case "world", "folder", "cube", "globe":
		return "#45816a"
	case "image", "layers", "account", "skin":
		return "#7867b0"
	case "sun":
		return "#a77d42"
	}
	return "#3777c5"
}

func emptyArtwork(c *ui.Context, key, icon string, size float32) {
	art := emptyArt[icon]
	if art == nil {
		art = emptyArt["folder"]
	}
	ui.Image(c.Key(key+"-art"), art).Label("空状态插画 "+key).Size(128*size, 80*size).Shrink(0).Fit(ui.Contain)
}

// emptyState is the full-list variant; actions wrap instead of overflowing a
// narrow pane. The compact dashboard variant shares the same artwork and type.
func emptyState(c *ui.Context, key, icon, title, description string, actions func()) {
	s := uiScale(c)
	ui.Column(c.Key(key)).Font(uiSansFamily()).FillWidth().Padding(28*s, 24*s).Gap(12*s).Radius(radiusCard*s).Background(ui.RGBA(255, 255, 255, .76)).Border(1, glassEdge).AlignItems(ui.Center).Children(func() {
		emptyArtwork(c, key, icon, 1.15*s)
		ui.Column(c).FillWidth().MaxWidth(520 * s).Gap(7 * s).AlignItems(ui.Center).Children(func() {
			ui.Text(c, title).FillWidth().FontSize(typeTitle * s).FontWeight(600).TextColor(inkSecondary).TextAlign(ui.Center)
			if description != "" {
				ui.Text(c, description).FillWidth().FontSize(typeFootnote * s).TextColor(muted).TextAlign(ui.Center)
			}
		})
		if actions != nil {
			ui.Row(c).FillWidth().Wrap().Gap(8 * s).PaddingY(4 * s).AlignItems(ui.Center).Justify(ui.Center).Children(actions)
		}
	})
}

func emptyPanel(c *ui.Context, key, icon, title, description string) {
	s := uiScale(c)
	ui.Column(c.Key(key)).Font(uiSansFamily()).FillWidth().Grow(1).Basis(0).MinHeight(0).Gap(9 * s).AlignItems(ui.Center).Justify(ui.Center).Children(func() {
		emptyArtwork(c, key, icon, s)
		tooltip(c, ui.Text(c, title).FillWidth().FontSize(typeHeadline*s).FontWeight(600).TextColor(inkSecondary).TextAlign(ui.Center).SingleLine(), title)
		tooltip(c, ui.Text(c, description).FillWidth().MaxWidth(290*s).Height(38*s).Shrink(0).FontSize(typeFootnote*s).TextColor(muted).TextAlign(ui.Center).MaxLines(2), description)
	})
}

// Small lists (recent logs and backups) use the same typography in a quiet row.
func emptyNote(c *ui.Context, key, icon, title, description string) {
	s := uiScale(c)
	ui.Row(c.Key(key)).Font(uiSansFamily()).FillWidth().Padding(14*s, 16*s).Gap(12 * s).Radius(radiusControl * s).Background(raised).AlignItems(ui.Center).Children(func() {
		ui.Image(c, emptyArt[icon]).Size(56*s, 35*s).Shrink(0).Fit(ui.Contain)
		ui.Column(c).Grow(1).Basis(0).MinWidth(0).Gap(4 * s).Children(func() {
			ui.Text(c, title).FillWidth().FontSize(typeFootnote * s).FontWeight(550).TextColor(inkSecondary)
			if description != "" {
				ui.Text(c, description).FillWidth().FontSize(typeCaption * s).TextColor(muted)
			}
		})
	})
}
