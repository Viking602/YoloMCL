package nativeui

import (
	"math"
	"strconv"
	"time"

	"github.com/egoist/mygo/ui"

	"github.com/Viking602/YoloMCL/internal/launcher"
)

const seedMapRequestInterval = 80 * time.Millisecond

// The camera keeps sub-block movement separate from integer sampling requests.
// Slow drags must not round every individual pointer delta back to zero.
type seedMapCamera struct {
	x, z, vx, vz, startX, startZ float64
	ready, pressed               bool
	lastMove, coast, requestedAt time.Time
}

func (v *seedMapView) syncCamera() {
	n := &v.camera
	if !n.ready || int(math.Round(n.x)) != v.viewport.X || int(math.Round(n.z)) != v.viewport.Z {
		*n = seedMapCamera{x: float64(v.viewport.X), z: float64(v.viewport.Z), ready: true}
	}
}

func (v *seedMapView) placeCamera(x, z float64) {
	n := &v.camera
	n.x, n.z, n.ready = max(-29900000, min(29900000, x)), max(-29900000, min(29900000, z)), true
	v.viewport.X, v.viewport.Z = int(math.Round(n.x)), int(math.Round(n.z))
	v.xText, v.zText = strconv.Itoa(v.viewport.X), strconv.Itoa(v.viewport.Z)
}

func (n *seedMapCamera) release(now time.Time, lessMotion bool, units float64) {
	if lessMotion || now.Sub(n.lastMove) > 80*time.Millisecond || math.Hypot(n.vx, n.vz)*units < 100 {
		n.coast = time.Time{}
		return
	}
	n.coast, n.startX, n.startZ = now, n.x, n.z
}

func (n *seedMapCamera) advance(now time.Time) (x, z float64, moving bool) {
	if n.coast.IsZero() {
		return n.x, n.z, false
	}
	t := min(.45, max(0, now.Sub(n.coast).Seconds()))
	distance := (1 - math.Exp(-9*t)) / 9
	x, z = n.startX+n.vx*distance, n.startZ+n.vz*distance
	if t >= .45 {
		n.coast = time.Time{}
	}
	return x, z, !n.coast.IsZero()
}

func (v *seedMapView) moveCamera(c *ui.Context, stage ui.Element, units float32) {
	v.syncCamera()
	n := &v.camera
	now, pressed := c.Now(), stage.Pressed()
	if pressed || c.Preferences().ReduceMotion {
		n.coast = time.Time{}
	}
	if dx, dz, ok := stage.Dragged(); ok && (dx != 0 || dz != 0) {
		v.placeCamera(n.x-float64(dx/units), n.z-float64(dz/units))
		vx, vz := stage.DragVelocity()
		limit := max(1, math.Hypot(float64(vx), float64(vz))/2400)
		n.vx, n.vz = -float64(vx)/(float64(units)*limit), -float64(vz)/(float64(units)*limit)
		n.lastMove = now
		v.inspected, v.markerGroup = nil, nil
	}
	if n.pressed && !pressed {
		n.release(now, c.Preferences().ReduceMotion, float64(units))
	}
	n.pressed = pressed
	if !n.coast.IsZero() {
		x, z, moving := n.advance(now)
		v.placeCamera(x, z)
		if moving {
			c.After(time.Second / 60)
		}
	}
}

// During motion, resample only after entering another tile row/column, at most
// once per 80 ms. The release always requests the exact final viewport.
func (a *App) scheduleSeedMap(c *ui.Context, v *seedMapView) {
	if v.tiles == nil || v.viewport == v.requested || v.seedText == "" {
		return
	}
	moving := v.camera.pressed || !v.camera.coast.IsZero() || len(v.keys) > 0
	if moving && sameSeedMapLayer(v.requested, v.viewport) {
		if sameSeedMapTileRange(v.requested, v.viewport) {
			return
		}
		if wait := seedMapRequestInterval - c.Now().Sub(v.camera.requestedAt); wait > 0 {
			c.After(wait)
			return
		}
	}
	v.camera.requestedAt = c.Now()
	a.requestSeedMap(v)
}

func sameSeedMapLayer(a, b launcher.SeedMapViewport) bool {
	return a.Y == b.Y && a.Dimension == b.Dimension && a.Structure == b.Structure && a.Surface == b.Surface && a.ContourOverlay == b.ContourOverlay && a.Terrain == b.Terrain && a.BlocksPerPixel == b.BlocksPerPixel
}

func sameSeedMapTileRange(a, b launcher.SeedMapViewport) bool {
	rangeOf := func(v launcher.SeedMapViewport) [4]int {
		x, z := v.Origin()
		span := float64(launcher.SeedMapTilePixels * v.BlocksPerPixel)
		return [4]int{int(math.Floor(float64(x) / span)), int(math.Floor(float64(z) / span)), int(math.Floor(float64(x+v.Width*v.BlocksPerPixel-1) / span)), int(math.Floor(float64(z+v.Height*v.BlocksPerPixel-1) / span))}
	}
	return rangeOf(a) == rangeOf(b)
}

// zoomAt steps the zoom while keeping the world point under (px, pz) fixed.
// w, h is the canvas size and units the DIPs per block before the step.
func (v *seedMapView) zoomAt(delta int, px, pz, w, h, units float32) {
	v.syncCamera()
	ox, oz := float64(px-w/2), float64(pz-h/2)
	wx, wz := v.camera.x+ox/float64(units), v.camera.z+oz/float64(units)
	before := float64(max(1, v.pixelsPerBlock)) / float64(max(1, v.viewport.BlocksPerPixel))
	v.zoom(delta)
	after := float64(max(1, v.pixelsPerBlock)) / float64(max(1, v.viewport.BlocksPerPixel))
	next := float64(units) * after / before
	v.placeCamera(wx-ox/next, wz-oz/next)
}

// seedMapPanKeys maps held keys to a pan direction (arrows and WASD).
var seedMapPanKeys = map[ui.Key][2]float64{
	ui.KeyLeft: {-1, 0}, ui.KeyA: {-1, 0},
	ui.KeyRight: {1, 0}, ui.KeyD: {1, 0},
	ui.KeyUp: {0, -1}, ui.KeyW: {0, -1},
	ui.KeyDown: {0, 1}, ui.KeyS: {0, 1},
}

// seedMapKey handles keyboard input on the focused canvas: held pan keys
// move continuously; + and - zoom around the centre.
func (v *seedMapView) seedMapKey(ev ui.InputEvent, now time.Time) bool {
	switch ev.Kind {
	case ui.InputKeyDown:
		if _, ok := seedMapPanKeys[ev.Key]; ok && ev.Mods&(ui.Super|ui.Ctrl|ui.Alt) == 0 {
			if len(v.keys) == 0 {
				v.keyTick = now
			}
			if v.keys == nil {
				v.keys = map[ui.Key]bool{}
			}
			v.keys[ev.Key] = true
			v.keyFast = ev.Mods&ui.Shift != 0
			v.camera.coast = time.Time{}
			return true
		}
		switch ev.Key {
		case ui.KeyEqual, ui.KeyPageUp:
			v.zoom(1)
			return true
		case ui.KeyMinus, ui.KeyPageDown:
			v.zoom(-1)
			return true
		}
	case ui.InputKeyUp:
		if v.keys[ev.Key] {
			delete(v.keys, ev.Key)
			return true
		}
	}
	return false
}

// keyPan advances held-key panning at 700 DIPs per second (x3 with Shift).
func (v *seedMapView) keyPan(c *ui.Context, focused bool, units float32) {
	if !focused {
		v.keys = nil
	}
	if len(v.keys) == 0 {
		return
	}
	now := c.Now()
	dt := min(.05, max(0, now.Sub(v.keyTick).Seconds()))
	v.keyTick = now
	var dx, dz float64
	for key := range v.keys {
		d := seedMapPanKeys[key]
		dx, dz = dx+d[0], dz+d[1]
	}
	if n := math.Hypot(dx, dz); n > 0 {
		speed := 700 * dt / float64(units) / n
		if v.keyFast {
			speed *= 3
		}
		v.syncCamera()
		v.placeCamera(v.camera.x+dx*speed, v.camera.z+dz*speed)
		v.inspected, v.markerGroup = nil, nil
	}
	c.After(time.Second / 60)
}
