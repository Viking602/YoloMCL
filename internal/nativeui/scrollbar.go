package nativeui

import (
	"time"

	"github.com/egoist/mygo/ui"
)

// One scroll bar for the whole app, macOS overlay style: a thin rounded
// thumb with no track, hidden while idle. It fades in while the content
// scrolls or the pointer comes near the edge, fades out about a second
// after, and widens and darkens under the pointer or while dragged.
//
// The toolkit still does the hit testing (wheel, thumb drag, click to
// page) on its own invisible bar; this paints the visible thumb over the
// same strip, inside the scroll area's right inset, so it never covers
// content as long as the area keeps scrollGutter free on the right.
const (
	scrollThumbIdle  float32 = 5  // DIPs at 1×
	scrollThumbHot   float32 = 8  // under the pointer / dragging
	scrollHitWidth   float32 = 10 // toolkit bar width = hit strip
	scrollThumbMin   float32 = 32 // minimum thumb length
	scrollEdgeMargin float32 = 3  // thumb ↔ container edge
	scrollGutter     float32 = 14 // right padding a scroll area keeps free
)

var (
	scrollIdleAlpha float32 = .26 // neutral ink, dark-on-light
	scrollHotAlpha  float32 = .46
	scrollFadeOut           = ui.Animation{Duration: 260 * time.Millisecond, Ease: easeInCubic, Delay: 900 * time.Millisecond}
)

type scrollTrack struct {
	st    ui.ScrollState
	lastY float32
}

var scrollTracks = map[string]*scrollTrack{}

// scrollArea is ui.Scroll with the shared overlay scroll bar.
func scrollArea(c *ui.Context, key string) ui.Element {
	tr := scrollTracks[key]
	if tr == nil {
		tr = &scrollTrack{}
		scrollTracks[key] = tr
	}
	e := ui.Scroll(c.Key(key)).TrackScroll(&tr.st)
	return overlayScrollbar(e, tr)
}

func overlayScrollbar(e ui.Element, tr *scrollTrack) ui.Element {
	box := e.LayoutBounds()
	px, _, over := e.PointerPosition()
	near := over && box.W > 0 && px >= box.W-(scrollHitWidth+scrollEdgeMargin+6)
	moved := tr.st.Y != tr.lastY
	tr.lastY = tr.st.Y
	dragging := near && e.Pressed()
	active := moved || near
	target, anim := float32(0), scrollFadeOut
	if active {
		target, anim = 1, motionFast
	}
	alpha := e.AnimateTo("sb-alpha", target, anim)
	hotT := float32(0)
	if near || dragging {
		hotT = 1
	}
	hot := e.AnimateTo("sb-hot", hotT, motionFast)
	maxY := tr.st.MaxY
	if alpha <= .01 || maxY <= .5 {
		return e
	}
	y := tr.st.Y
	return e.DrawOver(func(p *ui.Painter, r ui.Rect) {
		g := thumbRect(r, y, maxY, hot)
		p.Fill(g, ui.RGBA(20, 24, 40, (scrollIdleAlpha+(scrollHotAlpha-scrollIdleAlpha)*hot)*alpha), g.W/2)
	})
}

// thumbRect places the visible thumb: right-aligned in the toolkit's hit
// strip, proportional length with a minimum, as far down as the content
// is scrolled.
func thumbRect(r ui.Rect, y, maxY, hot float32) ui.Rect {
	w := scrollThumbIdle + (scrollThumbHot-scrollThumbIdle)*hot
	inner := r.H - 4
	content := r.H + maxY
	length := min(max(inner*r.H/content, scrollThumbMin), inner)
	travel := inner - length
	at := float32(0)
	if maxY > 0 {
		at = max(0, min(travel*y/maxY, travel))
	}
	return ui.Rect{X: r.X + r.W - scrollEdgeMargin - w, Y: r.Y + 2 + at, W: w, H: length}
}
