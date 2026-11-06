package nativeui

import (
	"fmt"
	"math"
	"strconv"
	"strings"

	"github.com/egoist/mygo/ui"

	"github.com/Viking602/YoloMCL/internal/launcher"
)

func (a *App) seedMapPanel(c *ui.Context) {
	if a.seedMap == nil {
		a.openSeedMap("")
	}
	v := a.seedMap
	if v == nil {
		return
	}
	s := uiScale(c)
	ui.Column(c.Key("seedmap-panel")).Fill().MinHeight(0).Gap(12 * s).Children(func() {
		a.seedMapToolbar(c, v)
		if a.seedMap != v {
			return
		}
		ui.Row(c.Key("seedmap-workspace")).Grow(1).MinHeight(0).FillWidth().Gap(12 * s).AlignItems(ui.Stretch).Children(func() {
			ui.Column(c.Key("seedmap-map-card")).Grow(1).Basis(0).MinWidth(0).MinHeight(0).Background(panel).Radius(radiusCard*s).Border(1, line).Padding(8 * s).Gap(8 * s).Children(func() {
				a.seedMapCanvas(c, v)
			})
			a.seedMapSidebar(c, v)
		})
		if v.err != "" {
			ui.Text(c, v.err).FontSize(typeCaption * s).TextColor(danger).FillWidth()
		}
		if v.sourceError != "" {
			ui.Text(c, v.sourceError).FontSize(typeCaption * s).TextColor(warningInk).FillWidth()
		}
	})
	a.scheduleSeedMap(c, v)
	if a.seedMap == v && v.searchQueued && !v.pending && v.viewport == v.requested {
		a.startSeedMapSearch(v)
	}
}

func (a *App) seedMapToolbar(c *ui.Context, v *seedMapView) {
	s := uiScale(c)
	ui.Column(c.Key("seedmap-toolbar")).FillWidth().Shrink(0).Padding(12*s).Gap(12*s).Radius(radiusCard*s).Background(raised).Border(1, line).Children(func() {
		ui.Row(c).FillWidth().Gap(12 * s).AlignItems(ui.Center).Children(func() {
			ui.Icon(c, icons["cube"]).Size(20*s, 20*s).TextColor(accent)
			ui.Text(c, a.L("seedmap.title")).FontSize(typeHeadline * s).Bold().TextColor(ink).Shrink(0)
			ui.Box(c.Key("seedmap-alpha")).Shrink(0).Padding(4*s, 8*s).Radius(radiusChip*s).Background(accent.Alpha(.12)).Border(1, accent.Alpha(.45)).Children(func() {
				ui.Text(c, a.L("seedmap.alpha")).FontSize(typeCaption * s).Bold().TextColor(accent).NoWrap()
			})
			world := v.world
			options := []string{""}
			names := map[string]string{"": a.L("seedmap.manual")}
			for _, save := range v.saves {
				options = append(options, save.Dir)
				names[save.Dir] = save.Name
			}
			if world != "" && names[world] == "" {
				options = append(options, world)
				names[world] = world
			}
			selectBoxFull(c, "seedmap-source", a.L("seedmap.source"), &world, options, nil, func(id string) string { return names[id] }).Width(180 * s)
			if world != v.world {
				a.openSeedMap(world)
			}
			ui.Spacer(c).Grow(1).MinWidth(0)
			textField(c.Key("seedmap-seed"), &v.seedText).Label(a.L("seedmap.seed")).Placeholder(a.L("seedmap.seed_help")).Width(235 * s).MinWidth(135 * s).Shrink(1)
			if button(c, "seedmap-apply", a.L("seedmap.apply"), "", true, v.loading) {
				a.loadSeedMap(v, false)
			}
		})
		ui.Row(c).FillWidth().Gap(8 * s).AlignItems(ui.Center).Children(func() {
			ui.Text(c, a.L("seedmap.dimension")).FontSize(typeCaption * s).TextColor(muted).Shrink(0)
			dim := v.viewport.Dimension
			options := []string{"minecraft:overworld", "minecraft:the_nether", "minecraft:the_end"}
			if v.sampler != nil {
				options = v.dimensionOptions()
			}
			selectBoxFull(c, "seedmap-dimension", a.L("seedmap.dimension"), &dim, options, nil, func(id string) string { return a.seedMapName("dimension", id, a.isEN()) }).Width(155 * s)
			if v.sampler != nil && dim != v.viewport.Dimension {
				a.seedMapDimension(v, dim)
			}
			ui.Text(c, "Y").FontSize(typeCaption * s).TextColor(muted)
			textField(c.Key("seedmap-y"), &v.yText).Label(a.L("seedmap.y")).Width(56 * s)
			if variantButton(c, "seedmap-y-apply", a.L("seedmap.y_apply"), "", buttonTertiary, v.loading) {
				y, err := strconv.Atoi(v.yText)
				if err != nil || y < -2048 || y > 2047 {
					v.err = a.L("seedmap.invalid_y")
				} else {
					v.viewport.Y = y
					v.viewport.Surface, v.sliceMode = false, true
					v.yText = strconv.Itoa(v.viewport.Y)
					v.err = ""
					v.selected = nil
					v.searched = false
				}
			}
			ui.Spacer(c).Grow(1).MinWidth(0)
			ui.Text(c, "X").FontSize(typeCaption * s).TextColor(muted)
			textField(c.Key("seedmap-x"), &v.xText).Label(a.L("seedmap.position_x")).Width(83 * s)
			ui.Text(c, "Z").FontSize(typeCaption * s).TextColor(muted)
			textField(c.Key("seedmap-z"), &v.zText).Label(a.L("seedmap.position_z")).Width(83 * s)
			if button(c, "seedmap-jump", a.L("seedmap.jump"), "", false, v.loading) {
				x, e1 := strconv.Atoi(v.xText)
				z, e2 := strconv.Atoi(v.zText)
				if e1 != nil || e2 != nil || x < -29900000 || x > 29900000 || z < -29900000 || z > 29900000 {
					v.err = a.L("seedmap.invalid_position")
				} else {
					v.center(x, z)
					v.err = ""
				}
			}
			if iconOnlyButton(c, "seedmap-origin", "refresh", a.L("seedmap.origin"), btnMd*s, v.loading) {
				v.center(0, 0)
			}
			if iconOnlyButton(c, "seedmap-zoom-out", "minus", a.L("seedmap.zoom_out"), btnMd*s, v.loading) {
				v.zoom(-1)
			}
			if iconOnlyButton(c, "seedmap-zoom-in", "plus", a.L("seedmap.zoom_in"), btnMd*s, v.loading) {
				v.zoom(1)
			}
		})
	})
}

func (a *App) seedMapHitLabel(hit launcher.FeatureHit) string {
	return fmt.Sprintf("%s / X %d / Z %d", a.seedMapName("structure", hit.ID, a.isEN()), hit.X, hit.Z)
}

func (a *App) seedMapCanvas(c *ui.Context, v *seedMapView) {
	s := uiScale(c)
	var coverage launcher.SeedMapCoverage
	if v.sampler != nil && !v.viewport.Surface {
		coverage = v.sampler.Coverage(v.viewport.Dimension)
	}
	ui.Row(c).FillWidth().Shrink(0).Gap(12 * s).AlignItems(ui.Center).Children(func() {
		ui.Text(c, a.seedMapName("dimension", v.viewport.Dimension, a.isEN())).FontSize(typeCallout * s).Bold().TextColor(ink)
		ui.Spacer(c).Grow(1).MinWidth(0)
		if v.loading {
			ui.Text(c, a.L("seedmap.loading")).FontSize(typeCaption * s).TextColor(muted)
		} else if v.sampler != nil && !v.viewport.Surface {
			badge := "seedmap.coverage." + coverage.Terrain
			if coverage.Opaque {
				badge = "seedmap.coverage.opaque"
			}
			ui.Text(c, a.L(badge)).FontSize(typeCaption * s).TextColor(muted).SingleLine().Shrink(1).MinWidth(0)
		}
		if v.sampler != nil && v.sampler.SupportsSurface(v.viewport.Dimension) {
			label := a.L("seedmap.surface_show")
			if v.viewport.Surface {
				label = a.L("seedmap.surface_active")
			}
			variant := buttonTertiary
			if v.viewport.Surface {
				variant = buttonSecondary
			}
			if variantButton(c, "seedmap-surface", label, "", variant, v.loading) {
				v.viewport.Surface = !v.viewport.Surface
				v.sliceMode = !v.viewport.Surface
			}
		}
		if v.viewport.Surface && v.sampler != nil && v.sampler.SupportsSurface(v.viewport.Dimension) {
			variant := buttonTertiary
			if v.viewport.Terrain {
				variant = buttonSecondary
			}
			if variantButton(c, "seedmap-terrain-toggle", a.L("seedmap.terrain.toggle"), "", variant, v.loading) {
				v.viewport.Terrain = !v.viewport.Terrain
			}
		}
		markersLabel := a.L("seedmap.hide_markers")
		if v.hideMarkers {
			markersLabel = a.L("seedmap.show_markers")
		}
		if v.seedMapCavesSupported() {
			variant := buttonTertiary
			if v.showCaves && !v.viewport.Surface {
				variant = buttonSecondary
			}
			if variantButton(c, "seedmap-caves-toggle", a.L("seedmap.caves.toggle"), "", variant, v.loading) {
				v.showCaves = !v.showCaves || v.viewport.Surface
				v.showOres = false
				if v.showCaves && v.viewport.Surface {
					// Carvers are drawn on the height slice.
					v.viewport.Surface, v.sliceMode = false, true
				}
			}
		}
		if v.seedMapCavesSupported() {
			variant := buttonTertiary
			if v.showOres && !v.viewport.Surface {
				variant = buttonSecondary
			}
			if variantButton(c, "seedmap-ores-toggle", a.L("seedmap.ores.toggle"), "", variant, v.loading) {
				v.showOres = !v.showOres || v.viewport.Surface
				v.showCaves = false
				if v.showOres && v.viewport.Surface {
					v.viewport.Surface, v.sliceMode = false, true
				}
			}
		}
		if variantButton(c, "seedmap-markers-toggle", markersLabel, "", buttonTertiary, v.loading) {
			v.hideMarkers = !v.hideMarkers
			v.markerGroup, v.selected = nil, nil
		}
	})
	if !v.loading && !v.completing && coverage.Terrain == "candidates" {
		ui.Row(c.Key("seedmap-unavailable")).FillWidth().Shrink(0).Gap(8 * s).AlignItems(ui.Center).Children(func() {
			ui.Text(c, a.L("seedmap.unavailable")).FontSize(typeCaption * s).TextColor(warningInk).Grow(1).MinWidth(0)
			if variantButton(c, "seedmap-unavailable-details", a.L("seedmap.unavailable_details"), "", buttonGhost, false) {
				v.details, v.legend = true, false
			}
		})
	}
	stage := ui.ButtonBase(c.Key("seedmap-canvas")).Label(a.L("seedmap.canvas")).FillWidth().Grow(1).MinHeight(0).Clip().Radius(nestedRadius(radiusCard, 8) * s).Background(ui.Hex("#e1e7e8"))
	stage.Bind(&v.canvasFocus)
	bounds := stage.LayoutBounds()
	w, h := bounds.W, bounds.H
	if w <= 0 || h <= 0 {
		width, height := c.Size()
		w = max(200*s, width-(instListWidth+2*pageGutter+320)*s)
		h = max(180*s, height-350*s)
	}
	// Reduce both axes together so the bounded raster keeps the canvas aspect.
	// Scaling each axis to its cap independently leaves a letterbox on wide views.
	pixelScale := s * float32(max(1, v.pixelsPerBlock))
	reduction := max(float32(1), w/(launcher.SeedMapMaxWidth*pixelScale), h/(launcher.SeedMapMaxHeight*pixelScale))
	v.viewport.Width = max(1, min(launcher.SeedMapMaxWidth, int(math.Round(float64(w/(pixelScale*reduction))))))
	v.viewport.Height = max(1, min(launcher.SeedMapMaxHeight, int(math.Round(float64(h/(pixelScale*reduction))))))
	v.viewport.ContourOverlay = v.viewport.Surface && v.viewport.BlocksPerPixel == 1 && v.pixelsPerBlock > 1
	raster := v.viewport
	scale := max(w/float32(raster.Width), h/float32(raster.Height))
	units := scale / float32(raster.BlocksPerPixel)
	v.moveCamera(c, stage, units)
	raster = v.viewport
	barPixels := 64
	for barPixels > 1 && float32(barPixels)*scale > 90*s {
		barPixels /= 2
	}
	barWidth := float32(barPixels) * scale
	point := func(x, z int) (float32, float32) {
		return w/2 + float32(float64(x)-v.camera.x)*units, h/2 + float32(float64(z)-v.camera.z)*units
	}
	v.keyPan(c, stage.Focused(), units)
	if stage.Pressed() && !stage.Focused() {
		stage.Focus()
	}
	stage.HandleInput(func(ev ui.InputEvent) bool {
		switch ev.Kind {
		case ui.InputKeyDown, ui.InputKeyUp:
			return v.seedMapKey(ev, c.Now())
		case ui.InputZoom:
			if ev.Scale <= 0 {
				return false
			}
			if v.pinch <= 0 {
				v.pinch = 1
			}
			// Each zoom step doubles; pinches step once past ~1.4x either way.
			if v.pinch *= ev.Scale; v.pinch > 1.4 {
				v.pinch = 1
				v.zoomAt(1, ev.X, ev.Y, w, h, units)
			} else if v.pinch < 1/1.4 {
				v.pinch = 1
				v.zoomAt(-1, ev.X, ev.Y, w, h, units)
			}
			return true
		case ui.InputScroll:
			if ev.DY == 0 {
				return false
			}
			// Mouse wheels step once per notch; touchpads accumulate pixels.
			step := ev.DY
			if ev.Precise {
				v.wheel += ev.DY
				if v.wheel > -40*s && v.wheel < 40*s {
					return true
				}
				step, v.wheel = v.wheel, 0
			}
			if step < 0 {
				v.zoomAt(1, ev.X, ev.Y, w, h, units)
			} else {
				v.zoomAt(-1, ev.X, ev.Y, w, h, units)
			}
			return true
		}
		return false
	})
	v.hover = nil
	if px, pz, over := stage.PointerPosition(); over && v.mapReady() && !v.camera.pressed && v.camera.coast.IsZero() {
		x := v.camera.x + float64((px-w/2)/units)
		z := v.camera.z + float64((pz-h/2)/units)
		ox, oz := v.frame.Viewport.Origin()
		step := float64(v.frame.Viewport.BlocksPerPixel)
		if p, ok := v.frame.PointAt(int(math.Floor((x-float64(ox))/step)), int(math.Floor((z-float64(oz))/step))); ok {
			v.hover = &p
		}
	}
	if stage.Clicked() && v.hover != nil {
		p := *v.hover
		v.inspected = &p
		v.pointOpen = true
		v.selected, v.selectedSpawn, v.markerGroup = nil, nil, nil
	}
	v.pointPopup = w >= 260*s && h >= 240*s
	target := ""
	if v.selected != nil {
		target = fmt.Sprintf("%s:%d:%d", v.selected.ID, v.selected.X, v.selected.Z)
	} else if v.inspected != nil {
		target = fmt.Sprintf("point:%d:%d", v.inspected.X, v.inspected.Z)
	}
	if target != v.pointTarget {
		v.pointTarget, v.pointOpen = target, target != ""
	}
	tiles := v.drawTiles(c.Now(), w, h, units)
	lessMotion := c.Preferences().ReduceMotion
	stage.Children(func() {
		ui.Box(c.Key("seedmap-raster")).Absolute().Left(0).Top(0).Size(w, h).Draw(func(p *ui.Painter, r ui.Rect) { seedMapPaintTiles(p, r, tiles, lessMotion) })
		caves := v.showCaves && !v.viewport.Surface && v.seedMapCavesSupported()
		cavesDrawn := caves && a.seedMapCaveOverlay(c, v, w, h, units)
		ores := v.showOres && !v.viewport.Surface && v.seedMapCavesSupported()
		oresDrawn := ores && a.seedMapOreOverlay(c, v, w, h, units)
		seedMapChunkGrid(c, raster, w, h, s, scale, point)
		seedMapCoordinates(c, raster, w, h, s, scale, point)
		a.seedMapMarkers(c, v, w, h, s, barWidth, point)
		seedMapStructureFootprint(c, v, w, h, s, point)
		for i, spawn := range v.spawns() {
			if spawn.Dimension != v.viewport.Dimension {
				continue
			}
			px, pz := point(spawn.X, spawn.Z)
			if px < 0 || pz < 0 || px > w || pz > h {
				continue
			}
			name := a.L("seedmap.spawn." + spawn.Kind)
			tip := fmt.Sprintf("%s / X %d Z %d", name, spawn.X, spawn.Z)
			if spawn.Kind != "estimate" {
				tip = fmt.Sprintf("%s / X %d Y %d Z %d", name, spawn.X, spawn.Y, spawn.Z)
			}
			b := tooltip(c, ui.ButtonBase(c.Key("seedmap-spawn-"+strconv.Itoa(i))).Label(name), tip).Absolute().Left(px-14*s).Top(pz-14*s).Size(28*s, 28*s).Radius(radiusPill).Background(successInk).Border(2*s, panel).Center()
			b.Children(func() { ui.Icon(c, icons["seedmap-spawn"]).Size(17*s, 17*s).TextColor(panel) })
			if b.Clicked() {
				v.inspected = nil
				v.selected = nil
				selected := spawn
				v.selectedSpawn = &selected
			}
			if v.selectedSpawn != nil && *v.selectedSpawn == spawn {
				ui.Box(c.Key("seedmap-spawn-label-"+strconv.Itoa(i))).Absolute().Left(px+18*s).Top(pz-12*s).Padding(4*s, 8*s).Radius(radiusChip * s).Background(panel.Alpha(.94)).Children(func() { ui.Text(c, name).FontSize(typeCaption * s).TextColor(successInk).Bold().NoWrap() })
			}
		}
		if caves {
			a.seedMapCaveBadge(c, v, s, cavesDrawn)
		}
		if ores {
			a.seedMapOreBadge(c, v, s, oresDrawn)
		}
		if v.viewport.Surface && v.viewport.Terrain {
			ui.Box(c.Key("seedmap-terrain-badge")).Absolute().Top(10*s).Left(10*s).Padding(8*s, 8*s).Radius(radiusChip*s).Background(panel.Alpha(.94)).Border(1, accent.Alpha(.3)).Children(func() {
				ui.Column(c).Gap(4 * s).Children(func() {
					ui.Text(c, a.L("seedmap.terrain.title")).FontSize(typeCaption * s).Bold().TextColor(ink).NoWrap()
					ui.Text(c, a.L("seedmap.terrain.note")).FontSize(typeCaption * s).TextColor(muted).NoWrap()
				})
			})
		}
		ui.Box(c.Key("seedmap-north")).Absolute().Top(10*s).Right(10*s).Padding(8*s, 8*s).Radius(radiusChip * s).Background(panel.Alpha(.9)).Children(func() { ui.Text(c, a.L("seedmap.north")).FontSize(typeCaption * s).Bold().TextColor(ink) })
		ui.Box(c.Key("seedmap-scale")).Absolute().Left(10*s).Bottom(10*s).Padding(4*s, 8*s).Radius(radiusChip * s).Background(panel.Alpha(.9)).Children(func() {
			ui.Column(c).Gap(4 * s).Children(func() {
				ui.Text(c, fmt.Sprintf(a.L("seedmap.scale"), barPixels*raster.BlocksPerPixel)).FontSize(typeCaption * s).TextColor(ink).SingleLine()
				ui.Box(c).Width(barWidth).Height(2 * s).Background(ink)
			})
		})
		if v.loading || v.completing {
			label := a.L("seedmap.loading")
			if v.progress != "" {
				label = v.progress
			}
			seedMapLoading(c, s, label, func() {
				if variantButton(c, "seedmap-cancel", a.L("seedmap.cancel"), "", buttonTertiary, false) {
					a.cancelSeedMapLoad(v)
				}
			})
		}
		if v.inspected != nil && v.pointPopup && v.pointOpen {
			px, pz := point(v.inspected.X, v.inspected.Z)
			cardWidth := buttonWidth(c, a.L("seedmap.copy_tp"), "", buttonSizeFor(btnSm), s) + buttonWidth(c, a.L("seedmap.point_close"), "", buttonSizeFor(btnSm), s) + 32*s
			if strings.HasPrefix(v.inspected.Biome.ID, "minecraft:") {
				cardWidth += (btnMd + 6) * s
			}
			cardWidth = min(w-16*s, max(232*s, cardWidth))
			a.seedMapPopover(c, v, "seedmap-point-popup", a.L("floating.map_point_popup"), px, pz, cardWidth, s, w, h, func() { a.seedMapPointCard(c, v) })
		}
		if v.selected != nil && v.pointPopup && v.pointOpen {
			px, pz := point(v.selected.X, v.selected.Z)
			bs := buttonSizeFor(btnMd)
			cardWidth := buttonWidth(c, a.L("seedmap.copy_tp"), "", bs, s) + buttonWidth(c, a.L("seedmap.structure_preview"), "", bs, s) + buttonWidth(c, a.L("seedmap.point_close"), "", bs, s) + 36*s
			cardWidth = min(w-16*s, max(232*s, cardWidth))
			a.seedMapPopover(c, v, "seedmap-structure-popup", a.L("floating.map_structure_popup"), px, pz, cardWidth, s, w, h, func() { a.seedMapStructureCard(c, v) })
		}
		if v.structurePreview != nil {
			a.seedMapStructurePreviewFloat(c, v, w, h)
		}
	})
	ui.Row(c).FillWidth().Shrink(0).Gap(8 * s).AlignItems(ui.Center).Children(func() {
		position := fmt.Sprintf("X %d  Y %d  Z %d", v.viewport.X, v.viewport.Y, v.viewport.Z)
		if v.viewport.Surface {
			position = fmt.Sprintf("X %d  Z %d", v.viewport.X, v.viewport.Z)
		}
		if p := v.hover; p != nil {
			position = fmt.Sprintf("%s / X %d  Z %d", a.seedMapName("biome", p.Biome.ID, a.isEN()), p.X, p.Z)
			if p.Void {
				position += " / " + a.L("seedmap.hover_void")
			} else if p.Surface {
				position += fmt.Sprintf(" / Y ≈ %.0f", p.Height)
			} else if v.sampler != nil {
				// Height slice: where the slice sits against the ground here.
				if h, ok := v.sampler.SurfaceHeightAt(v.viewport.Dimension, p.X, p.Z); ok {
					key := "seedmap.hover_below"
					d := h - v.viewport.Y
					if d < 0 {
						key, d = "seedmap.hover_above", -d
					}
					position += " / " + fmt.Sprintf(a.L(key), v.viewport.Y, d)
				}
			}
		}
		positionLabel := ui.Text(c, position).FontFeatures("tnum").FontSize(typeCaption * s).TextColor(inkSecondary).SingleLine().Shrink(1).MinWidth(0)
		textTooltip(c, positionLabel, position, "", ui.Font{Family: c.Theme().Font, Size: typeCaption * s, Features: "tnum"}, 1)
		ui.Spacer(c).Grow(1)
		resolution := fmt.Sprintf(a.L("seedmap.zoom"), v.viewport.BlocksPerPixel)
		if v.viewport.BlocksPerPixel == 1 {
			resolution = a.L("seedmap.block_detail")
			if v.pixelsPerBlock > 1 {
				resolution += " / " + fmt.Sprintf(a.L("seedmap.block_zoom"), v.pixelsPerBlock)
			}
		}
		ui.Text(c, resolution).FontFeatures("tnum").FontSize(typeCaption * s).TextColor(muted).NoWrap().Shrink(0)
	})
	ui.Row(c).FillWidth().Shrink(0).Gap(8 * s).AlignItems(ui.Center).Children(func() {
		spawns := v.spawns()
		if len(spawns) == 0 {
			ui.Text(c, a.L("seedmap.spawn.none")).FontSize(typeCaption * s).TextColor(muted)
		}
		for i, spawn := range spawns {
			if spawn.Dimension != "minecraft:overworld" && spawn.Dimension != "minecraft:the_nether" && spawn.Dimension != "minecraft:the_end" {
				continue
			}
			label := a.L("seedmap.spawn." + spawn.Kind)
			if button(c, "seedmap-spawn-go-"+strconv.Itoa(i), label, "seedmap-spawn", false, v.loading) {
				a.seedMapDimension(v, spawn.Dimension)
				v.center(spawn.X, spawn.Z)
				if spawn.Kind != "estimate" {
					v.viewport.Y = max(-2048, min(2047, spawn.Y))
					v.yText = strconv.Itoa(v.viewport.Y)
				}
				selected := spawn
				v.selectedSpawn = &selected
				v.selected = nil
			}
		}
		ui.Spacer(c).Grow(1).MinWidth(0)
		ui.Row(c).Shrink(1).MinWidth(0).Gap(12 * s).AlignItems(ui.Center).Children(func() {
			for _, part := range strings.Split(a.L("seedmap.navigation"), "\u00b7") {
				ui.Text(c, strings.TrimSpace(part)).FontSize(typeCaption * s).TextColor(muted).SingleLine().NoWrap()
			}
		})
	})
}

func (a *App) seedMapSidebar(c *ui.Context, v *seedMapView) {
	s := uiScale(c)
	ui.Column(c.Key("seedmap-sidebar")).Width(268*s).MinHeight(0).FillHeight().Padding(12*s).Gap(12*s).Background(raised).Radius(radiusCard*s).Border(1, line).Children(func() {
		ui.Row(c).FillWidth().AlignItems(ui.Center).Children(func() {
			title := a.L("seedmap.structures")
			if v.legend {
				title = a.L("seedmap.legend")
			}
			if v.details {
				title = a.L("seedmap.details_toggle")
			}
			ui.Text(c, title).FontSize(typeCallout * s).Bold().TextColor(ink)
			ui.Spacer(c).Grow(1)
			if iconOnlyButton(c, "seedmap-refresh", "refresh", a.L("seedmap.refresh"), btnSm*s, v.loading) {
				v.snapshot = nil
				a.loadSeedMap(v, false)
			}
			label := a.L("seedmap.show_legend")
			if v.legend {
				label = a.L("seedmap.hide_legend")
			}
			if iconOnlyButton(c, "seedmap-legend", "layers", label, btnSm*s, false) {
				v.legend = !v.legend
				v.details = false
			}
		})
		if v.details {
			a.seedMapCoverage(c, v)
		} else if v.legend {
			a.seedMapLegend(c, v)
		} else if len(v.markerGroup) > 0 {
			a.seedMapMarkerGroup(c, v)
		} else {
			a.seedMapStructureList(c, v)
		}
		if v.selected != nil && !v.pointPopup && !v.legend && !v.details {
			a.seedMapSelection(c, v)
		}
		if v.selectedSpawn != nil && !v.legend && !v.details {
			a.seedMapSpawnSelection(c, v)
		}
		if v.inspected != nil && !v.pointPopup {
			a.seedMapPointCard(c, v)
		}
		if variantButton(c, "seedmap-details", a.L("seedmap.details_toggle"), "", buttonGhost, v.loading) {
			v.details = !v.details
			v.legend = false
		}
	})
}

func (a *App) seedMapStructureList(c *ui.Context, v *seedMapView) {
	s := uiScale(c)
	previous := v.query
	in := textField(c.Key("seedmap-search"), &v.query).Label(a.L("seedmap.search")).Placeholder(a.L("seedmap.search_hint")).FillWidth()
	if previous != v.query {
		v.searched = false
	}
	// One box: coordinates jump, biomes locate the nearest cell, structures
	// filter the list and locate the nearest start. Enter takes the first.
	gx, gz, isCoords := parseSeedMapCoords(v.query)
	var biomes []launcher.SeedMapBiome
	var structures []string
	if v.sampler != nil {
		for _, b := range v.sampler.Biomes(v.viewport.Dimension) {
			if a.seedMapBiomeMatches(b.ID, v.query) {
				biomes = append(biomes, b)
			}
		}
		for _, structure := range v.sampler.Structures(v.viewport.Dimension) {
			if a.seedMapStructureMatches(structure, v.query) {
				structures = append(structures, structure.ID)
			}
		}
	}
	pickStructure := func(id string) {
		v.viewport.Structure = id
		v.selected = nil
		v.results = nil
		v.searched = false
		v.searchError = ""
		v.jumpNearest = true
		a.searchSeedMap(v)
	}
	busy := v.loading || v.searching || v.sampler == nil
	if (in.Submitted() || in.Shortcut(0, ui.KeyEnter)) && !busy && strings.TrimSpace(v.query) != "" {
		switch {
		case isCoords:
			v.center(gx, gz)
			v.err = ""
		case len(structures) > 0:
			pickStructure(structures[0])
		case len(biomes) > 0:
			a.findSeedMapBiome(v, biomes[0].ID)
		}
	}
	if v.biomeStatus != "" {
		tone := muted
		if v.biomeStatusWarn {
			tone = warningInk
		}
		ui.Text(c, v.biomeStatus).FontSize(typeCaption * s).TextColor(tone).FillWidth()
	}
	ui.Row(c).FillWidth().Gap(8 * s).AlignItems(ui.Center).Children(func() {
		if selectBoxFull(c, "seedmap-radius", a.L("seedmap.radius"), &v.radius, []string{"2048", "4096", "8192", "16384", "32768"}, nil, nil).Height(btnSm*s).Grow(1).Basis(0).MinWidth(0).Changed() && !v.loading {
			a.searchSeedMap(v)
		}
		if fixedButton(c, "seedmap-find", a.L("seedmap.nearby"), "search", 0, btnSm*s, true, v.loading || v.pending || v.viewport.Structure == "" || v.searching) {
			a.searchSeedMap(v)
		}
	})
	if v.viewport.Structure != "" {
		ui.Row(c).FillWidth().Gap(8 * s).AlignItems(ui.Center).Children(func() {
			ui.Text(c, a.seedMapName("structure", v.viewport.Structure, a.isEN())).FontSize(typeCaption * s).TextColor(accent).Grow(1).MinWidth(0).SingleLine()
			if fixedButton(c, "seedmap-all", a.L("seedmap.all_structures"), "", 0, btnSm*s, false, v.loading) {
				v.viewport.Structure = ""
				v.results = nil
				v.selected = nil
				v.searched = false
				v.searchGeneration++
				v.searching, v.searchQueued, v.jumpNearest = false, false, false
				v.searchError = ""
			}
		})
	}
	scrollArea(c, "seedmap-structures-scroll").Grow(1).MinHeight(0).FillWidth().Children(func() {
		ui.Column(c).FillWidth().Gap(4 * s).Children(func() {
			if v.sampler == nil {
				return
			}
			if v.searching {
				ui.Text(c, a.L("seedmap.searching")).FontSize(typeBody * s).TextColor(muted)
				return
			}
			if v.searchError != "" {
				ui.Text(c, v.searchError).FontSize(typeCaption * s).TextColor(danger)
			}
			if v.searched {
				ui.Text(c, fmt.Sprintf(a.L("seedmap.results"), len(v.results))).FontSize(typeCaption * s).TextColor(muted)
				if v.searchLimited {
					ui.Text(c, a.L("seedmap.search_limited")).FontSize(typeCaption * s).TextColor(warningInk).FillWidth()
				}
				if len(v.results) == 0 {
					ui.Text(c, fmt.Sprintf(a.L("seedmap.none_in_radius"), v.radius)).FontSize(typeCaption * s).TextColor(warningInk).FillWidth()
				}
				for i, hit := range v.results {
					a.seedMapResult(c, v, i, hit)
				}
				return
			}
			if isCoords {
				label := fmt.Sprintf(a.L("seedmap.goto_coords"), gx, gz)
				b := ui.ButtonBase(c.Key("seedmap-goto")).Label(label).FillWidth().Padding(8 * s).Gap(8 * s).Radius(radiusControl * s).AlignItems(ui.Center)
				if b.Hovered() {
					b.Background(fieldHover)
				}
				b.Children(func() {
					ui.Icon(c, icons["search"]).Size(18*s, 18*s).TextColor(accent).Shrink(0)
					ui.Text(c, label).FontSize(typeCallout * s).TextColor(ink).SingleLine()
				})
				if b.Clicked() {
					v.center(gx, gz)
					v.err = ""
				}
			}
			if len(biomes) > 0 {
				ui.Text(c, a.L("seedmap.biomes_section")).FontSize(typeCaption * s).Bold().TextColor(muted)
				for i, biome := range biomes {
					id := biome.ID
					if i == 8 {
						ui.Text(c, fmt.Sprintf(a.L("seedmap.more_matches"), len(biomes)-8)).FontSize(typeCaption * s).TextColor(muted)
						break
					}
					primary, secondary := a.seedMapNames("biome", id)
					b := ui.ButtonBase(c.Key("seedmap-biome-" + id)).Label(a.L("seedmap.biome_nearest") + " " + primary).FillWidth().Padding(8 * s).Gap(8 * s).Radius(radiusControl * s).AlignItems(ui.Center)
					if v.biomeSearch == id {
						b.Background(accentSoft)
					} else if b.Hovered() {
						b.Background(fieldHover)
					}
					b.Children(func() {
						ui.Box(c).Size(14*s, 14*s).Radius(radiusMin * s).Background(ui.RGBA(biome.Color.R, biome.Color.G, biome.Color.B, 1)).Shrink(0)
						ui.Column(c).Grow(1).Basis(0).MinWidth(0).Gap(4 * s).Children(func() {
							ui.Text(c, primary).FontSize(typeCallout * s).TextColor(ink).SingleLine()
							if secondary != "" {
								ui.Text(c, secondary).FontSize(typeCaption * s).TextColor(muted).SingleLine()
							}
						})
						ui.Text(c, a.L("seedmap.biome_nearest")).FontSize(typeCaption * s).TextColor(accent).NoWrap().Shrink(0)
					})
					if b.Clicked() && !busy {
						a.findSeedMapBiome(v, id)
					}
				}
				if len(structures) > 0 {
					ui.Text(c, a.L("seedmap.structures")).FontSize(typeCaption * s).Bold().TextColor(muted)
				}
			}
			count := 0
			for _, structure := range v.sampler.Structures(v.viewport.Dimension) {
				if !a.seedMapStructureMatches(structure, v.query) {
					continue
				}
				count++
				id := structure.ID
				primary, secondary := a.seedMapNames("structure", id)
				b := ui.ButtonBase(c.Key("seedmap-kind-" + id)).Label(a.L("seedmap.structures") + " " + primary).FillWidth().Padding(8 * s).Gap(8 * s).Radius(radiusControl * s).AlignItems(ui.Center)
				if v.viewport.Structure == id {
					b.Background(accentSoft)
				} else if b.Hovered() {
					b.Background(fieldHover)
				}
				b.Children(func() {
					ui.Icon(c, icons[seedMapStructureIcon(id)]).Size(20*s, 20*s).TextColor(seedMapMarkerColor(id)).Shrink(0)
					ui.Column(c).Grow(1).Basis(0).MinWidth(0).Gap(4 * s).Children(func() {
						ui.Text(c, primary).FontSize(typeCallout * s).TextColor(ink).SingleLine()
						if secondary != "" {
							ui.Text(c, secondary).FontSize(typeCaption * s).TextColor(muted).SingleLine()
						}
					})
				})
				if b.Clicked() && !v.loading && !v.searching {
					pickStructure(id)
				}
			}
			if count == 0 && !isCoords && len(biomes) == 0 {
				ui.Text(c, a.L("seedmap.no_structures")).FontSize(typeCaption * s).TextColor(muted).FillWidth()
			}
		})
	})
	if v.viewport.Structure == "" {
		ui.Text(c, a.L("seedmap.choose_structure")).FontSize(typeCaption * s).TextColor(muted).FillWidth()
	}
}

func (a *App) seedMapResult(c *ui.Context, v *seedMapView, index int, hit launcher.FeatureHit) {
	s := uiScale(c)
	name := fmt.Sprintf("X %d  Z %d", hit.X, hit.Z)
	dx, dz := float64(hit.X-v.viewport.X), float64(hit.Z-v.viewport.Z)
	b := ui.ButtonBase(c.Key("seedmap-result-" + strconv.Itoa(index))).Label(a.L("seedmap.jump") + " " + name).FillWidth().Padding(8 * s).Radius(radiusControl * s).Gap(8 * s).AlignItems(ui.Center).Background(panel)
	b.Children(func() {
		ui.Icon(c, icons[seedMapStructureIcon(hit.ID)]).Size(19*s, 19*s).TextColor(seedMapMarkerColor(hit.ID)).Shrink(0)
		ui.Column(c).Grow(1).MinWidth(0).Gap(4 * s).Children(func() {
			ui.Text(c, name).FontSize(typeCallout * s).TextColor(ink).SingleLine()
			ui.Text(c, fmt.Sprintf(a.L("seedmap.distance"), int(math.Hypot(dx, dz)))).FontSize(typeCaption * s).TextColor(muted)
		})
	})
	if b.Clicked() {
		v.center(hit.X, hit.Z)
		v.pointOpen = true
		selected := hit
		v.selected = &selected
		v.inspected = nil
		v.selectedSpawn = nil
	}
}

func (a *App) seedMapSelection(c *ui.Context, v *seedMapView) {
	s := uiScale(c)
	ui.Column(c.Key("seedmap-selected")).FillWidth().Padding(8 * s).Radius(radiusPanel * s).Background(panel).Gap(8 * s).Shrink(0).Children(func() { a.seedMapStructureCard(c, v) })
}

// seedMapStructureCard is the selected structure's detail, laid out like
// seedMapPointCard so a marker click reads the same as a map click.
func (a *App) seedMapStructureCard(c *ui.Context, v *seedMapView) {
	hit := v.selected
	if hit == nil {
		return
	}
	s := uiScale(c)
	primary, secondary := a.seedMapNames("structure", hit.ID)
	ui.Row(c).FillWidth().Gap(8 * s).AlignItems(ui.Center).Children(func() {
		ui.Icon(c, icons[seedMapStructureIcon(hit.ID)]).Size(15*s, 15*s).TextColor(seedMapMarkerColor(hit.ID)).Shrink(0)
		ui.Text(c, primary).FontSize(typeCallout * s).Bold().TextColor(ink).Grow(1).MinWidth(0).SingleLine()
	})
	if secondary != "" {
		ui.Text(c, secondary).FontSize(typeCaption * s).TextColor(muted).SingleLine()
	}
	// Hit.Y is the biome-filter slice, not a generated structure height.
	ui.Text(c, fmt.Sprintf("X %d  Z %d", hit.X, hit.Z)).FontSize(typeCaption * s).TextColor(inkSecondary).SingleLine()
	ui.Text(c, fmt.Sprintf(a.L("seedmap.chunk_position"), hit.X>>4, hit.Z>>4)).FontSize(typeCaption * s).TextColor(muted).SingleLine()
	tooltip(c, ui.Text(c, a.L("seedmap.confidence_short."+hit.Confidence)).FontSize(typeCaption*s).TextColor(muted).SingleLine().FillWidth(), a.L("seedmap.confidence."+hit.Confidence))
	if hit.Biome != "" {
		ui.Text(c, a.seedMapName("biome", hit.Biome, a.isEN())).FontSize(typeCaption * s).TextColor(muted).SingleLine()
	}
	tooltip(c, ui.Text(c, hit.ID).FontSize(typeCaption*s).TextColor(muted).SingleLine().FillWidth(), hit.Source)
	command, canCopy := a.seedMapTeleportHeight(c, v, hit.X, hit.Z, false, 0)
	ui.Row(c.Key("seedmap-selected-actions")).Gap(8 * s).FillWidth().Wrap().Children(func() {
		if button(c, "seedmap-copy-structure", a.L("seedmap.copy_tp"), "", false, !canCopy) {
			c.WriteClipboard(command)
		}
		preview, clicked := fixedButtonElement(c, "seedmap-structure-preview", a.L("seedmap.structure_preview"), "", 0, btnMd*s, buttonPrimary, false)
		preview.Bind(&v.previewFocus)
		if clicked {
			a.openStructurePreview(v)
			v.previewPanelFocus.Focus(c)
		}
		if variantButton(c, "seedmap-structure-close", a.L("seedmap.point_close"), "", buttonGhost, false) {
			v.selected = nil
		}
	})
}

func (a *App) seedMapSpawnSelection(c *ui.Context, v *seedMapView) {
	s := uiScale(c)
	spawn := v.selectedSpawn
	ui.Column(c.Key("seedmap-spawn-selection")).FillWidth().Shrink(0).Padding(8 * s).Gap(4 * s).Radius(radiusPanel * s).Background(panel).Children(func() {
		ui.Text(c, a.L("seedmap.spawn."+spawn.Kind)).FontSize(typeCallout * s).Bold().TextColor(successInk)
		position := fmt.Sprintf("X %d  Y %d  Z %d", spawn.X, spawn.Y, spawn.Z)
		key := "seedmap.spawn.saved_hint"
		if spawn.Kind == "estimate" {
			position = fmt.Sprintf("X %d  Z %d", spawn.X, spawn.Z)
			key = "seedmap.spawn.estimate_hint"
		}
		ui.Text(c, position).FontSize(typeCaption * s).TextColor(ink)
		ui.Text(c, a.L(key)).FontSize(typeCaption * s).TextColor(muted).FillWidth()
		if button(c, "seedmap-copy-spawn", a.L("seedmap.copy_position"), "", false, false) {
			value := fmt.Sprintf("%d %d %d", spawn.X, spawn.Y, spawn.Z)
			if spawn.Kind == "estimate" {
				value = fmt.Sprintf("%d %d", spawn.X, spawn.Z)
			}
			c.WriteClipboard(value)
		}
	})
}

func (a *App) seedMapLegend(c *ui.Context, v *seedMapView) {
	s := uiScale(c)
	scrollArea(c, "seedmap-legend-scroll").Grow(1).MinHeight(0).FillWidth().Children(func() {
		ui.Column(c).FillWidth().Gap(12 * s).Children(func() {
			for _, biome := range v.frame.Biomes {
				primary, secondary := a.seedMapNames("biome", biome.ID)
				ui.Row(c).FillWidth().Gap(8 * s).AlignItems(ui.Center).Children(func() {
					ui.Box(c).Size(14*s, 14*s).Radius(radiusMin * s).Background(ui.RGBA(biome.Color.R, biome.Color.G, biome.Color.B, 1)).Shrink(0)
					ui.Column(c).Grow(1).MinWidth(0).Gap(4 * s).Children(func() {
						ui.Text(c, primary).FontSize(typeCallout * s).TextColor(ink).SingleLine()
						if secondary != "" {
							ui.Text(c, secondary).FontSize(typeCaption * s).TextColor(muted).SingleLine()
						}
					})
				})
			}
		})
	})
}

func (a *App) seedMapCoverage(c *ui.Context, v *seedMapView) {
	s := uiScale(c)
	scrollArea(c, "seedmap-coverage-scroll").Grow(1).MinHeight(0).FillWidth().Children(func() {
		ui.Column(c).FillWidth().Gap(8 * s).Children(func() {
			if v.sampler == nil {
				return
			}
			coverage := v.sampler.Coverage(v.viewport.Dimension)
			ui.Text(c, fmt.Sprintf(a.L("seedmap.details"), coverage.JSONEntries, coverage.Placements)).FontSize(typeCaption * s).TextColor(ink).FillWidth()
			seen := map[string]bool{}
			for _, issue := range coverage.Issues {
				if seen[issue.Code] {
					continue
				}
				seen[issue.Code] = true
				text := a.L("seedmap.issue." + issue.Code)
				if issue.Source != "" {
					text += "\n" + issue.Source
				}
				ui.Text(c, text).FontSize(typeCaption * s).TextColor(muted).FillWidth()
			}
			for _, spawn := range v.spawns() {
				key := "seedmap.spawn.saved_hint"
				if spawn.Kind == "estimate" {
					key = "seedmap.spawn.estimate_hint"
				}
				ui.Text(c, a.L(key)).FontSize(typeCaption * s).TextColor(muted).FillWidth()
			}
		})
	})
}
