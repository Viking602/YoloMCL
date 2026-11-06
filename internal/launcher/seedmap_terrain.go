package launcher

import (
	"image/color"
	"math"
	"strings"

	"github.com/Viking602/YoloMCL/cubiomes"
)

// seedMapTerrainBase picks an approximate top-surface block colour for a
// biome in the style of an in-game world map (Xaero's World Map style).
// Surface blocks are not generated here, so the colour is approximate.
func seedMapTerrainBase(id string, h float64, sea int) color.RGBA {
	name := strings.TrimPrefix(id, "minecraft:")
	water := h < float64(sea) || strings.Contains(name, "ocean") || strings.Contains(name, "river")
	if water {
		if strings.HasPrefix(name, "frozen") || strings.HasPrefix(name, "deep_frozen") {
			return color.RGBA{160, 190, 238, 255}
		}
		depth := float64(sea) - h
		f := max(.42, min(1, 1-depth/48))
		base := [3]float64{62, 100, 214}
		if strings.Contains(name, "warm") || strings.Contains(name, "lukewarm") {
			base = [3]float64{54, 130, 210}
		} else if name == "swamp" || name == "mangrove_swamp" {
			base = [3]float64{70, 100, 120}
		}
		return color.RGBA{byte(base[0] * f), byte(base[1] * f), byte(base[2] * f), 255}
	}
	switch {
	case strings.HasPrefix(name, "snowy") || strings.Contains(name, "frozen") || name == "ice_spikes" || name == "grove" || name == "jagged_peaks":
		return color.RGBA{240, 245, 250, 255}
	case name == "stony_peaks" || name == "stony_shore" || name == "windswept_gravelly_hills" || name == "dripstone_caves":
		return color.RGBA{128, 128, 130, 255}
	case strings.Contains(name, "badlands"):
		return color.RGBA{186, 98, 54, 255}
	case name == "desert" || name == "beach":
		return color.RGBA{219, 208, 160, 255}
	case name == "mushroom_fields":
		return color.RGBA{118, 100, 120, 255}
	case name == "swamp" || name == "mangrove_swamp":
		return color.RGBA{74, 104, 56, 255}
	case strings.Contains(name, "jungle"):
		return color.RGBA{58, 140, 38, 255}
	case name == "dark_forest":
		return color.RGBA{48, 88, 34, 255}
	case name == "pale_garden":
		return color.RGBA{168, 176, 160, 255}
	case name == "cherry_grove":
		return color.RGBA{226, 170, 190, 255}
	case strings.Contains(name, "savanna"):
		return color.RGBA{150, 156, 78, 255}
	case strings.Contains(name, "taiga"):
		return color.RGBA{58, 98, 56, 255}
	case strings.Contains(name, "forest") || name == "windswept_hills":
		return color.RGBA{72, 120, 44, 255}
	}
	return color.RGBA{112, 164, 70, 255}
}

// seedMapTerrainShade layers world-map shading on a colour: brighter when a
// cell is higher than its north neighbour, darker when lower, plus a gentle
// tone by height above sea level. Water keeps only its depth darkening.
func seedMapTerrainShade(c color.RGBA, cells []cubiomes.SurfaceSample, x, z, step, sea int) color.RGBA {
	i := z*seedMapSurfaceWidth + x
	h := float64(cells[i].Height)
	if h < float64(sea) {
		return c
	}
	n := max(float64(sea), float64(cells[i-seedMapSurfaceWidth].Height))
	slope := (h - n) / float64(max(1, step))
	shade := 1 + max(-.28, min(.22, slope*.12))
	shade *= max(.9, min(1.12, 1+(h-float64(sea))/600))
	ch := func(v byte) byte { return byte(max(0, min(255, math.Round(float64(v)*shade)))) }
	return color.RGBA{ch(c.R), ch(c.G), ch(c.B), 255}
}
