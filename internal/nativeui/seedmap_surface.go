package nativeui

import (
	"fmt"
	"math"
	"net/url"
	"strings"

	"github.com/egoist/mygo/ui"

	"github.com/Viking602/YoloMCL/internal/launcher"
)

func seedMapChunkGrid(c *ui.Context, raster launcher.SeedMapViewport, w, h, s, scale float32, point func(int, int) (float32, float32)) {
	ui.Box(c.Key("seedmap-fine-lines")).Absolute().Left(0).Top(0).Size(w, h).Draw(func(p *ui.Painter, r ui.Rect) {
		// A chunk is 16 by 16 blocks. Show its grid only when cells are large
		// enough to read, and keep it anchored at multiples of 16 below zero.
		if 16*scale/float32(raster.BlocksPerPixel) < 20*s {
			return
		}
		startX, startZ := raster.Origin()
		for x := (startX >> 4) << 4; x <= startX+raster.Width*raster.BlocksPerPixel; x += 16 {
			px, _ := point(x, 0)
			p.Fill(ui.Rect{X: r.X + float32(math.Round(float64(px))), Y: r.Y, W: 1, H: h}, ink.Alpha(.28), 0)
		}
		for z := (startZ >> 4) << 4; z <= startZ+raster.Height*raster.BlocksPerPixel; z += 16 {
			_, pz := point(0, z)
			p.Fill(ui.Rect{X: r.X, Y: r.Y + float32(math.Round(float64(pz))), W: w, H: 1}, ink.Alpha(.28), 0)
		}
	})
}

func seedMapCoordinates(c *ui.Context, v launcher.SeedMapViewport, w, h, s, scale float32, point func(int, int) (float32, float32)) {
	// Major grid spacing follows world coordinates so labels remain stable on pan.
	target := float64(110*s/scale) * float64(v.BlocksPerPixel)
	power := math.Pow(10, math.Floor(math.Log10(max(1, target))))
	step := int(power)
	for _, n := range []float64{1, 2, 5, 10} {
		if power*n >= target {
			step = max(1, int(power*n))
			break
		}
	}
	startX, startZ := v.Origin()
	first := func(n int) int { return int(math.Ceil(float64(n)/float64(step))) * step }
	for x := first(startX); x < startX+v.Width*v.BlocksPerPixel; x += step {
		px, _ := point(x, 0)
		ui.Box(c).Absolute().Left(float32(math.Round(float64(px)))).Top(0).Size(1, h).Background(ink.Alpha(.10))
		if px < 110*s || px > w-58*s {
			continue
		}
		ui.Box(c).Absolute().Left(px-27*s).Bottom(0).Width(54*s).Padding(4*s, 0).Radius(radiusChip * s).Background(panel.Alpha(.92)).Center().Children(func() { ui.Text(c, fmt.Sprintf("%d", x)).FontSize(typeCaption * s).TextColor(ink).SingleLine() })
	}
	for z := first(startZ); z < startZ+v.Height*v.BlocksPerPixel; z += step {
		_, pz := point(0, z)
		ui.Box(c).Absolute().Left(0).Top(float32(math.Round(float64(pz)))).Size(w, 1).Background(ink.Alpha(.10))
		if pz < 48*s || pz > h-38*s {
			continue
		}
		ui.Box(c).Absolute().Right(0).Top(pz-9*s).Padding(4*s, 8*s).Radius(radiusChip * s).Background(panel.Alpha(.92)).Children(func() { ui.Text(c, fmt.Sprintf("%d", z)).FontSize(typeCaption * s).TextColor(ink).SingleLine() })
	}
}

func (a *App) seedMapPointCard(c *ui.Context, v *seedMapView) {
	p := v.inspected
	if p == nil {
		return
	}
	s := uiScale(c)
	ui.Row(c).FillWidth().Gap(8 * s).AlignItems(ui.Center).Children(func() {
		ui.Box(c).Size(15*s, 15*s).Radius(radiusMin * s).Background(ui.RGBA(p.Biome.Color.R, p.Biome.Color.G, p.Biome.Color.B, 1)).Shrink(0)
		ui.Text(c, a.seedMapName("biome", p.Biome.ID, a.isEN())).FontSize(typeCallout * s).Bold().TextColor(ink).Grow(1).MinWidth(0).SingleLine()
	})
	ui.Text(c, fmt.Sprintf("X %d  Z %d", p.X, p.Z)).FontSize(typeCaption * s).TextColor(inkSecondary).SingleLine()
	ui.Text(c, fmt.Sprintf(a.L("seedmap.chunk_position"), p.X>>4, p.Z>>4)).FontSize(typeCaption * s).TextColor(muted).SingleLine()
	height := fmt.Sprintf("Y %d", p.Y)
	if p.Surface {
		height = fmt.Sprintf(a.L("seedmap.height_estimate"), p.Height)
	}
	ui.Text(c, height).FontSize(typeCaption * s).TextColor(muted).SingleLine()
	ui.Text(c, p.Biome.ID).FontSize(typeCaption * s).TextColor(muted).SingleLine().FillWidth()
	command, canCopy := a.seedMapTeleportHeight(c, v, p.X, p.Z, !p.Surface && !p.Void, p.Y)
	ui.Row(c).Gap(8 * s).FillWidth().Wrap().Children(func() {
		if button(c, "seedmap-copy-tp", a.L("seedmap.copy_tp"), "", false, !canCopy) {
			c.WriteClipboard(command)
		}
		if strings.HasPrefix(p.Biome.ID, "minecraft:") {
			if iconOnlyButton(c, "seedmap-biome-wiki", "external", a.L("seedmap.biome_wiki"), btnMd*s, false) {
				name := a.seedMapName("biome", p.Biome.ID, true)
				c.OpenURL("https://minecraft.wiki/?search=" + url.QueryEscape(name))
			}
		}
		if variantButton(c, "seedmap-point-close", a.L("seedmap.point_close"), "", buttonGhost, false) {
			v.inspected = nil
		}
	})
}
