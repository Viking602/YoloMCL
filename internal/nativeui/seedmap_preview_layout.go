package nativeui

import (
	"fmt"
	"math"
	"strings"

	"github.com/egoist/mygo/ui"

	"github.com/Viking602/YoloMCL/internal/structure3d"
)

// Keep controls and explanations outside the image. The content can scroll
// when the map is short; the header and its expand/close actions stay reachable.
func (a *App) seedMapStructurePreviewPanel(c *ui.Context, v *seedMapView, p *seedMapStructurePreview, bounds ui.Rect, s float32, expanded bool) {
	frame := ui.Column(c.Key("seedmap-structure3d")).Label(a.L("seedmap.structure_preview.window")).
		Absolute().Left(bounds.X).Top(bounds.Y).Width(bounds.W).Height(bounds.H).
		Padding(previewPad*s).Gap(space2*s).Radius(radiusChrome*s).Background(panel).Border(1, line).Clip().
		Shadow(0, 8*s, 24*s, -6*s, ui.RGBA(0, 0, 0, .14))
	frame.Role(ui.RoleDialog).Focusable().AutoFocus().FocusRing(false)
	frame.Bind(&v.previewPanelFocus)
	if expanded {
		frame.Modal().Focusable().AutoFocus().FocusRing(false)
		if frame.Shortcut(0, ui.KeyEscape) {
			p.expanded = false
			v.previewExpandFocus.Focus(c)
			c.AnimationFrame()
		}
	}
	frame.Children(func() {
		head := ui.ButtonBase(c.Key("seedmap-structure3d-head")).Label(a.L("seedmap.structure_preview")).
			FillWidth().Shrink(0).MinWidth(0).Gap(8 * s).AlignItems(ui.Center)
		head.Children(func() {
			ui.Row(c).Grow(1).MinWidth(0).Gap(8 * s).GapY(4 * s).Wrap().AlignItems(ui.Center).Children(func() {
				ui.Text(c, a.L("seedmap.structure_preview")).Font(uiSansFamily()).FontSize(typeCallout * s).Bold().TextColor(ink).NoWrap().Shrink(0)
				ui.Box(c.Key("seedmap-structure3d-alpha")).Shrink(0).Padding(4*s, 8*s).Radius(radiusMin * s).Background(accent.Alpha(.12)).Children(func() {
					ui.Text(c, a.L("seedmap.alpha")).FontSize(typeCaption * s).Bold().TextColor(accent).NoWrap()
				})
			})
			icon, action := "expand", "seedmap.structure_preview.expand"
			if expanded {
				icon, action = "restore", "seedmap.structure_preview.restore"
			}
			expand := iconButtonElement(c, "seedmap-structure3d-expand", icon, a.L(action), btnSm*s, false).Bind(&v.previewExpandFocus)
			if expand.Clicked() {
				p.expanded = !expanded
				if p.expanded {
					v.previewPanelFocus.Focus(c)
				} else {
					v.previewExpandFocus.Focus(c)
				}
				p.scroll.st.Y = 0
				c.AnimationFrame()
			}
			if iconOnlyButton(c, "seedmap-structure3d-close", "cross", a.L("seedmap.structure_preview.close"), btnSm*s, false) {
				a.closeStructurePreview(c, v)
				c.AnimationFrame()
			}
		})
		if v.structurePreview != p {
			return
		}
		scroll := overlayScrollbar(ui.Scroll(c.Key("seedmap-structure3d-scroll")).TrackScroll(&p.scroll.st), &p.scroll).
			Label(a.L("seedmap.structure_preview.content")).FillWidth().Grow(1).MinHeight(0)
		availableH := scroll.LayoutBounds().H
		scroll.Draw(func(_ *ui.Painter, r ui.Rect) {
			if math.Abs(float64(r.H-availableH)) > .1 {
				c.AnimationFrame()
			}
		}).Children(func() {
			ui.Column(c).FillWidth().MinWidth(0).MinHeight(availableH).Padding(0, scrollGutter*s, 0, 0).Gap(8 * s).Children(func() {
				if p.title != "" {
					ui.Text(c, p.title).FillWidth().MinWidth(0).Shrink(0).FontSize(typeCaption * s).TextColor(muted)
				}
				ready := p.bitmap != nil && p.mesh != nil && p.err == "" && p.unsupported == ""
				if ready {
					a.seedMapStructurePreviewTools(c, v, p, s)
				}
				msg := func(title, body, detail string) {
					measure := max(1, min(30*typeCaption*s, bounds.W-(2*previewPad+scrollGutter+40)*s-2))
					ui.Column(c.Key("seedmap-structure3d-msg")).FillWidth().Grow(1).Basis(0).MinHeight(200 * s).
						Radius(nestedRadius(radiusChrome, previewPad) * s).Background(ui.Hex("#e8ecef")).Padding(20 * s).Gap(8 * s).
						Font(uiSansFamily()).AlignItems(ui.Center).Justify(ui.Center).Children(func() {
						// Keep short messages centered together on a compact
						// measure, with balanced wrapping in narrow previews.
						ui.Column(c).FillWidth().MinWidth(0).MaxWidth(measure).Gap(space2 * s).AlignItems(ui.Center).Children(func() {
							if title != "" {
								balancedText(c, title, ui.Font{Family: uiSansFamily(), Size: typeCallout * s, Weight: 700}, measure).TextColor(ink).TextAlign(ui.Center)
							}
							if body != "" {
								tooltip(c, balancedText(c, body, ui.Font{Family: uiSansFamily(), Size: typeCaption * s, Weight: 400}, measure).TextColor(muted).TextAlign(ui.Center), detail)
							}
						})
					})
				}
				switch {
				case p.loading && p.bitmap == nil:
					msg("", a.L("seedmap.structure_preview.loading"), "")
				case p.err != "":
					msg(a.L("seedmap.structure_preview.failed"), a.L(structurePreviewReasonKey(p.errKind)), p.err)
				case p.unsupported != "":
					title, body := a.L("seedmap.structure_preview.unsupported"), a.L(structurePreviewReasonKey(p.unsupKind))
					if p.unsupKind == structure3d.KindCodeGenerated {
						title, body = body, a.L("seedmap.structure_preview.reason.code_generated_hint")
					}
					msg(title, body, p.unsupported)
				default:
					// Ignore the bitmap's intrinsic height when allocating spare
					// space, so it cannot push the instructions out of the panel.
					ui.Row(c.Key("seedmap-structure3d-body")).FillWidth().Grow(1).Basis(0).MinHeight(200 * s).Gap(8 * s).AlignItems(ui.Stretch).Children(func() {
						a.seedMapStructureStage(c, v, p, s)
						if p.terrain {
							a.seedMapHeightGauge(c, p, s)
						}
					})
				}
				if ready {
					ui.Row(c.Key("seedmap-structure3d-hints")).Label(a.L("seedmap.structure_preview.controls")).
						FillWidth().Shrink(0).MinWidth(0).Gap(12 * s).GapY(4 * s).Wrap().Children(func() {
						for _, key := range []string{"hint_click", "hint_rotate", "hint_zoom", "hint_pan"} {
							ui.Text(c, a.L("seedmap.structure_preview."+key)).Font(uiSansFamily()).FontSize(typeCaption * s).TextColor(inkSecondary).NoWrap().Shrink(0)
						}
					})
					a.seedMapStructurePreviewInfo(c, p, s)
				}
			})
		})
	})
}

func (a *App) seedMapStructurePreviewTools(c *ui.Context, v *seedMapView, p *seedMapStructurePreview, s float32) {
	ui.Row(c.Key("seedmap-structure3d-tools")).FillWidth().Shrink(0).MinWidth(0).Gap(8 * s).Wrap().AlignItems(ui.Center).Children(func() {
		if button(c, "seedmap-structure3d-real", a.L("seedmap.structure_preview.real"), "", !p.boxes, false) && p.boxes {
			p.boxes = false
			a.rerenderStructurePreview(v)
		}
		if button(c, "seedmap-structure3d-boxes", a.L("seedmap.structure_preview.boxes"), "", p.boxes, false) && !p.boxes {
			p.boxes = true
			a.rerenderStructurePreview(v)
		}
		if button(c, "seedmap-structure3d-fit", a.L("seedmap.structure_preview.fit"), "", false, false) {
			a.fitStructurePreview(c, v, p)
		}
		if p.terrain && !p.xray {
			if button(c, "seedmap-structure3d-ghost", a.L("seedmap.structure_preview.ghost"), "", p.ghost, false) {
				p.ghost = !p.ghost
				a.rerenderStructurePreview(v)
			}
		}
		if p.terrain && button(c, "seedmap-structure3d-xray", a.L("seedmap.structure_preview.xray"), "", p.xray, false) {
			p.xray = !p.xray
			a.rerenderStructurePreview(v)
		}
	})
}

func (a *App) seedMapStructurePreviewInfo(c *ui.Context, p *seedMapStructurePreview, s float32) {
	key, detailKey, color := "seedmap.structure_preview.fidelity", "seedmap.structure_preview.fidelity_detail", warningInk
	if p.seeded {
		key, detailKey, color = "seedmap.structure_preview.seeded", "seedmap.structure_preview.seeded_detail", inkSecondary
	} else if p.layoutSeeded {
		key, detailKey = "seedmap.structure_preview.layout_seeded", p.layoutDetailKey
	}
	ui.Column(c.Key("seedmap-structure3d-info")).Label(a.L("seedmap.structure_preview.information")).
		FillWidth().MinWidth(0).Shrink(0).Padding(space2 * s).Gap(space2 * s).Radius(nestedRadius(radiusChrome, previewPad) * s).Background(raised).Children(func() {
		if p.loading {
			ui.Text(c, a.L("seedmap.structure_preview.terrain_loading")).FillWidth().MinWidth(0).Font(uiSansFamily()).FontSize(typeCaption * s).TextColor(accent)
		}
		ui.Row(c).FillWidth().MinWidth(0).Gap(16 * s).GapY(4 * s).Wrap().Children(func() {
			tooltip(c, ui.Text(c, a.structureYText(p)).Font(uiSansFamily()).FontFeatures("tnum").FontSize(typeCaption*s).TextColor(inkSecondary).NoWrap().Shrink(0), a.L("seedmap.gauge.structure_detail"))
			if p.blocks > 0 {
				ui.Text(c, fmt.Sprintf(a.L("seedmap.structure_preview.blocks"), p.blocks)).Font(uiSansFamily()).FontFeatures("tnum").FontSize(typeCaption * s).TextColor(inkSecondary).NoWrap().Shrink(0)
			}
		})
		if p.sel.Index != 0 {
			bodyText(c, fmt.Sprintf(a.L("seedmap.structure_preview.selected"), p.sel.Name, p.yApprox(), p.sel.Y)).
				FillWidth().Font(uiSansFamily()).FontFeatures("tnum").FontSize(typeCaption * s).TextColor(accent)
		}
		// The complete summary wraps; opening it reveals the longer provenance
		// explanation inline, including on devices without hover.
		if ui.ButtonBase(c.Key("seedmap-structure3d-details")).Label(a.L(key)).FillWidth().MinWidth(0).Gap(8 * s).AlignItems(ui.Start).Children(func() {
			ui.Text(c, a.L(key)).Grow(1).MinWidth(0).Font(uiSansFamily()).FontSize(typeCaption * s).TextColor(color)
			angle := float32(0)
			if p.detailsOpen {
				angle = 180
			}
			ui.Icon(c, icons["chevron-down"]).Size(12*s, 12*s).Margin(3*s, 0, 0, 0).Shrink(0).TextColor(muted).Rotate(angle)
		}).Clicked() {
			p.detailsOpen = !p.detailsOpen
			c.AnimationFrame()
		}
		if p.terrain && (p.dim == "minecraft:the_nether" || p.dim == "minecraft:the_end") {
			key := "seedmap.terrain.approx_nether"
			if p.dim == "minecraft:the_end" {
				key = "seedmap.terrain.approx_end"
			}
			if p.noiseTerrain {
				key = strings.Replace(key, "approx_", "noise_", 1)
			}
			bodyText(c, "≈ "+a.L(key)).FillWidth().Font(uiSansFamily()).FontSize(typeCaption * s).TextColor(warningInk)
		}
		if p.detailsOpen {
			bodyText(c, a.L(detailKey)).FillWidth().Font(uiSansFamily()).FontSize(typeCaption * s).TextColor(muted)
		}
	})
}
