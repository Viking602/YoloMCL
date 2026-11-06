package nativeui

import (
	"cmp"
	"fmt"
	"math"
	"slices"
	"strconv"
	"strings"

	"github.com/egoist/mygo/ui"

	"github.com/Viking602/YoloMCL/internal/launcher"
)

type seedMapMarker struct {
	x, y     float32
	hits     []launcher.FeatureHit
	selected bool
}

type seedMapMarkerLayout struct {
	hitsVersion          uint64
	x, z                 float64
	view                 launcher.SeedMapViewport
	width, height, scale float32
	selection            launcher.FeatureHit
	hasSelection         bool
	markers              []seedMapMarker
}

func (v *seedMapView) mapMarkers(w, h, s float32, point func(int, int) (float32, float32)) []seedMapMarker {
	key := seedMapMarkerLayout{hitsVersion: v.hitsVersion, view: v.viewport, x: v.camera.x, z: v.camera.z, width: w, height: h, scale: s, hasSelection: v.selected != nil}
	if v.selected != nil {
		key.selection = *v.selected
	}
	old := v.markerLayout
	if old.hitsVersion == key.hitsVersion && old.view == key.view && old.x == key.x && old.z == key.z && old.width == w && old.height == h && old.scale == s && old.hasSelection == key.hasSelection && old.selection == key.selection {
		return old.markers
	}
	hits := v.hits
	if v.viewport.Structure == "" {
		// Like web seed maps, common structures fade out as the map zooms out.
		hits = make([]launcher.FeatureHit, 0, len(v.hits))
		for _, hit := range v.hits {
			if seedMapMarkerVisible(hit.ID, v.viewport.BlocksPerPixel) || v.selected != nil && hit.ID == v.selected.ID && hit.X == v.selected.X && hit.Z == v.selected.Z {
				hits = append(hits, hit)
			}
		}
	}
	key.markers = layoutSeedMapMarkers(hits, v.selected, w, h, s, point)
	v.markerLayout = key
	return key.markers
}

// Anchors never move while merging, so later members cannot make two badges
// overlap. World-coordinate ordering keeps grouping independent of tile order.
// The spatial index bounds the work even when a zoomed-out map has many hits.
func layoutSeedMapMarkers(hits []launcher.FeatureHit, selected *launcher.FeatureHit, w, h, s float32, point func(int, int) (float32, float32)) []seedMapMarker {
	ordered := slices.Clone(hits)
	isSelected := func(hit launcher.FeatureHit) bool {
		return selected != nil && hit.ID == selected.ID && hit.X == selected.X && hit.Z == selected.Z
	}
	slices.SortStableFunc(ordered, func(a, b launcher.FeatureHit) int {
		if isSelected(a) != isSelected(b) {
			if isSelected(a) {
				return -1
			}
			return 1
		}
		return cmp.Or(cmp.Compare(seedMapMarkerTier(a.ID), seedMapMarkerTier(b.ID)), cmp.Compare(a.Z, b.Z), cmp.Compare(a.X, b.X), cmp.Compare(a.ID, b.ID))
	})
	spacing, margin := 38*s, 18*s
	// Keep a dense modpack overview readable. Zooming or selecting one
	// structure reduces hit density and reveals the individual icons again.
	if len(hits) > 120 {
		spacing = max(spacing, float32(math.Sqrt(float64(w*h/100))))
	}
	grid := map[[2]int][]int{}
	var markers []seedMapMarker
	for _, hit := range ordered {
		x, y := point(hit.X, hit.Z)
		if x < margin || y < margin || x > w-margin || y > h-margin {
			continue
		}
		cell := [2]int{int(math.Floor(float64(x / spacing))), int(math.Floor(float64(y / spacing)))}
		nearest, distance := -1, float32(math.MaxFloat32)
		for dz := -1; dz <= 1; dz++ {
			for dx := -1; dx <= 1; dx++ {
				for _, i := range grid[[2]int{cell[0] + dx, cell[1] + dz}] {
					mx, my := markers[i].x-x, markers[i].y-y
					if mx > -spacing && mx < spacing && my > -spacing && my < spacing && mx*mx+my*my < distance {
						nearest, distance = i, mx*mx+my*my
					}
				}
			}
		}
		if nearest >= 0 {
			markers[nearest].hits = append(markers[nearest].hits, hit)
			continue
		}
		grid[cell] = append(grid[cell], len(markers))
		markers = append(markers, seedMapMarker{x: x, y: y, hits: []launcher.FeatureHit{hit}, selected: isSelected(hit)})
	}
	return markers
}

// seedMapMarkerTier ranks structures by rarity: 0 big or rare ones stay at
// every zoom, 1 regional ones show from 8 blocks per pixel, 2 common small
// ones show from 2 blocks per pixel.
func seedMapMarkerTier(id string) int {
	name := strings.ToLower(id)
	for _, k := range []string{"stronghold", "mansion", "monument", "fortress", "complex", "bastion", "end_cit", "ancient_cit", "trial_chamber"} {
		if strings.Contains(name, k) {
			return 0
		}
	}
	for _, k := range []string{"ruined_portal", "shipwreck", "mineshaft", "buried_treasure", "ocean_ruin", "trail_ruin", "nether_fossil", "fossil"} {
		if strings.Contains(name, k) {
			return 2
		}
	}
	return 1
}

func seedMapMarkerVisible(id string, blocksPerPixel int) bool {
	switch seedMapMarkerTier(id) {
	case 0:
		return true
	case 1:
		return blocksPerPixel <= 8
	}
	return blocksPerPixel <= 2
}

func seedMapMarkerColor(id string) ui.Color {
	switch seedMapStructureIcon(id) {
	case "home":
		return ui.Hex("#9b591b")
	case "seedmap-temple":
		return ui.Hex("#087d79")
	case "seedmap-ship":
		return ui.Hex("#2567ae")
	case "seedmap-city":
		return ui.Hex("#794bb0")
	case "seedmap-fortress":
		return ui.Hex("#855349")
	default:
		return ui.Hex("#4c6077")
	}
}

func (a *App) seedMapMarkers(c *ui.Context, v *seedMapView, w, h, s, scaleBarWidth float32, point func(int, int) (float32, float32)) {
	if v.hideMarkers {
		return
	}
	for i, marker := range v.mapMarkers(w, h, s, point) {
		hit := marker.hits[0]
		// Smaller badges at overview zooms keep the terrain readable.
		small := float32(1)
		if v.viewport.BlocksPerPixel >= 16 {
			small = .8
		}
		size, tint := 22*s*small, seedMapMarkerColor(hit.ID)
		label := a.seedMapHitLabel(hit)
		group := len(marker.hits) > 1
		if group {
			// The rarest member leads the badge; the count says how many more.
			size = 26 * s * small
			label = fmt.Sprintf(a.L("seedmap.marker_group"), len(marker.hits), hit.X, hit.Z)
		} else if marker.selected {
			size = 28 * s
		}
		// Keep badges clear of the north indicator and scale. Panning exposes
		// these positions; their hits remain available to the structure search.
		margin := size/2 + 3*s
		if marker.x+margin > w-60*s && marker.y-margin < 44*s ||
			marker.x-margin < scaleBarWidth+26*s && marker.y+margin > h-46*s {
			continue
		}
		background, foreground := panel.Alpha(.97), tint
		if marker.selected {
			background, foreground = accent, panel
		}
		b := tooltip(c, ui.ButtonBase(c.Key("seedmap-hit-"+strconv.Itoa(i))).Label(label), label).
			Absolute().Left(marker.x-size/2).Top(marker.y-size/2).Size(size, size).
			Radius(radiusPill).Background(background).Border(1.5*s, tint).Center()
		b.Children(func() {
			if group {
				ui.Icon(c, icons[seedMapStructureIcon(hit.ID)]).Size(14*s*small, 14*s*small).TextColor(foreground)
				ui.Box(c).Absolute().Right(-7*s).Top(-5*s).MinWidth(17*s).Height(15*s).Padding(0, 4*s).Radius(radiusChip * s).Background(tint).Center().Children(func() {
					ui.Text(c, strconv.Itoa(len(marker.hits))).FontSize(9 * s).Bold().TextColor(panel)
				})
			} else {
				ui.Icon(c, icons[seedMapStructureIcon(hit.ID)]).Size(14*s*small, 14*s*small).TextColor(foreground)
			}
		})
		if b.Clicked() {
			v.pointOpen = true
			v.selectedSpawn, v.selected = nil, nil
			v.inspected = nil
			v.details, v.legend = false, false
			if group {
				v.markerGroup = slices.Clone(marker.hits)
			} else {
				v.markerGroup = nil
				v.selected = &hit
			}
		}
	}
}

func (a *App) seedMapMarkerGroup(c *ui.Context, v *seedMapView) {
	s := uiScale(c)
	ui.Text(c, fmt.Sprintf(a.L("seedmap.marker_count"), len(v.markerGroup))).FontSize(typeCallout * s).Bold().TextColor(ink)
	ui.Row(c).FillWidth().Gap(8 * s).Children(func() {
		if button(c, "seedmap-group-back", a.L("seedmap.all_structures"), "chevron-left", false, false) {
			v.markerGroup, v.selected = nil, nil
		}
		if button(c, "seedmap-group-zoom", a.L("seedmap.zoom_in"), "plus", false, v.viewport.BlocksPerPixel <= 1 || len(v.markerGroup) == 0) {
			x, z := v.markerGroup[0].X, v.markerGroup[0].Z
			v.center(x, z)
			v.zoom(1)
		}
	})
	scrollArea(c, "seedmap-group-scroll").Grow(1).MinHeight(0).FillWidth().Children(func() {
		ui.Column(c).FillWidth().Gap(4 * s).Children(func() {
			for i, hit := range v.markerGroup {
				b := ui.ButtonBase(c.Key("seedmap-group-item-" + strconv.Itoa(i))).Label(a.seedMapHitLabel(hit)).FillWidth().Padding(8 * s).Radius(radiusControl * s).Gap(8 * s).AlignItems(ui.Center).Background(panel)
				b.Children(func() {
					ui.Icon(c, icons[seedMapStructureIcon(hit.ID)]).Size(18*s, 18*s).TextColor(seedMapMarkerColor(hit.ID)).Shrink(0)
					ui.Column(c).Grow(1).MinWidth(0).Gap(4 * s).Children(func() {
						ui.Text(c, a.seedMapName("structure", hit.ID, a.isEN())).FontSize(typeCallout * s).TextColor(ink).SingleLine()
						ui.Text(c, fmt.Sprintf("X %d  Z %d", hit.X, hit.Z)).FontSize(typeCaption * s).TextColor(muted)
					})
				})
				if b.Clicked() {
					selected := hit
					v.selected, v.selectedSpawn = &selected, nil
				}
			}
		})
	})
}
