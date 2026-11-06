package nativeui

import (
	"context"
	"errors"
	"log"
	"math"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/egoist/mygo/ui"

	"github.com/Viking602/YoloMCL/internal/launcher"
	"github.com/Viking602/YoloMCL/internal/structure3d"
)

type seedMapStructurePreview struct {
	loading              bool
	err, unsupported     string // raw detail: logs and tooltip only
	errKind, unsupKind   string
	seeded, layoutSeeded bool
	title                string
	layoutDetailKey      string
	blocks               int
	gen                  uint64
	dim                  string
	cancel               context.CancelFunc

	// Camera. Every change re-rasterizes the baked mesh off the UI thread.
	yaw, pitch, zoom float64
	panX, panY       float64 // screen pixels at full resolution
	panning          bool
	panLX, panLY     float32
	dragging         bool      // left-drag orbit in progress: low-res frames
	refineAt         time.Time // wheel burst: render sharp again after the pause
	wheelPending     bool      // timestamp the event in the next fresh UI frame
	interactiveScale float64

	mesh      *structure3d.Mesh
	frames    [2]structure3d.Frame // double buffer: one shown, one drawn
	back      int
	bitmap    *ui.Bitmap
	rendering bool
	dirty     bool
	frameMs   float64

	// Ground and heights. The slab and surface heights come from the map's
	// surface sampler (approximate); the structure's Y range from its blocks.
	terrain                bool
	noiseTerrain           bool // ground from the dimension's final_density (noise stage)
	surfaceY               int  // ≈ surface at the structure, world Y
	terrainTop             int
	structMinY, structMaxY int
	xray                   bool
	ghost                  bool // faint see-through terrain once the cut has passed

	// Selection and fly-to. A click on a block selects it and glides the
	// orbit centre (aim) to it; boxes swaps blocks for piece boxes.
	boxes     bool
	sel       structure3d.Picked // Index 0: nothing selected
	aimed     bool
	aim       [3]float64
	fly       *seedMapFly
	dragMoved bool // the current press orbited; its release is not a click

	// Dive: the map view tilts into the slab and the cut plane sinks to the
	// structure. diveT runs 0..1; diveOut plays it backwards and closes.
	diving   bool
	diveOut  bool
	diveFrom time.Time
	diveT    float64
	drawnT   float64 // diveT of the last requested frame

	// Floating panel geometry inside the map stage, in pixels; 0 = default.
	x, y, w, h  float32
	expanded    bool
	detailsOpen bool
	scroll      scrollTrack
}

const (
	structurePreviewFullW, structurePreviewFullH = 720, 540
)

func (v *seedMapView) discardStructurePreview() {
	if p := v.structurePreview; p != nil && p.cancel != nil {
		p.cancel()
	}
	v.structurePreview = nil
}

func (a *App) openStructurePreview(v *seedMapView) {
	if v == nil || v.selected == nil || a.service == nil {
		return
	}
	hit := *v.selected
	p := &seedMapStructurePreview{loading: true, ghost: true, yaw: 0.6, pitch: 0.55, zoom: structurePreviewZoom(hit.ID), layoutDetailKey: layoutSeededDetailKey(hit.ID)}
	if old := v.structurePreview; old != nil {
		if old.cancel != nil {
			old.cancel()
		}
		p.x, p.y, p.w, p.h = old.x, old.y, old.w, old.h // keep where the user put it
		p.expanded = old.expanded
	}
	p.gen = atomic.AddUint64(&v.request, 1)
	v.structurePreview = p
	v.pointOpen = false
	a.loadStructurePreview(v, hit, p.gen)
}

// rerenderStructurePreview asks for a frame of the current camera. At most
// one frame renders at a time; requests made meanwhile collapse into one
// follow-up frame with the newest camera.
func (a *App) rerenderStructurePreview(v *seedMapView) {
	if v == nil || v.structurePreview == nil {
		return
	}
	p := v.structurePreview
	if p.mesh == nil {
		return
	}
	if p.rendering {
		p.dirty = true
		return
	}
	p.rendering, p.dirty = true, false
	mesh, gen := p.mesh, p.gen
	idx := p.back
	frame := &p.frames[idx]
	w, h, k := structurePreviewFullW, structurePreviewFullH, 1.0
	interactive := p.dragging || p.panning || p.fly != nil || p.wheelPending || !p.refineAt.IsZero()
	if interactive {
		k = p.interactiveScale
		if k == 0 {
			k = .5
		}
		w, h = int(float64(w)*k), int(float64(h)*k)
	} else if p.diving {
		w, h, k = w*2/3, h*2/3, 2.0/3
	}
	cam := p.camera()
	cam.Zoom, cam.PanX, cam.PanY = cam.Zoom*k, cam.PanX*k, cam.PanY*k
	go func() {
		start := time.Now()
		img := mesh.Render(frame, w, h, cam)
		ms := float64(time.Since(start).Microseconds()) / 1000
		a.post(func() {
			if a.seedMap != v || v.structurePreview != p || p.gen != gen {
				return
			}
			p.rendering = false
			p.frameMs = ms
			if interactive {
				// Hysteresis keeps resolution from alternating every frame.
				if ms > 18 {
					p.interactiveScale = 1.0 / 3
				} else if ms < 8 {
					p.interactiveScale = .5
				}
			}
			if p.bitmap == nil {
				p.bitmap = ui.NewBitmap(img).Pixelated()
			} else {
				p.bitmap.Update(img)
			}
			p.back = 1 - idx
			if p.dirty {
				a.rerenderStructurePreview(v)
			}
		})
	}()
}

func (a *App) loadStructurePreview(v *seedMapView, hit launcher.FeatureHit, gen uint64) {
	ctx, cancel := context.WithCancel(a.ctx)
	v.structurePreview.cancel = cancel
	rootID, versionID, world, seed := v.rootID, v.versionID, v.world, v.seed
	dim, sampler := v.viewport.Dimension, v.sampler
	caves, version := v.seedMapCavesSupported(), ""
	hits := append([]launcher.FeatureHit(nil), v.hits...)
	if v.snapshot != nil {
		version = v.snapshot.Minecraft
	}
	a.bgWG.Add(1)
	go func() {
		defer a.bgWG.Done()
		defer cancel()
		b := buildStructurePreviewProgressive(ctx, a.service, rootID, versionID, world, seed, dim, sampler, caves, version, hits, hit, func(b structurePreviewBuild) {
			a.post(func() { a.applyStructurePreview(v, gen, dim, b, false) })
		})
		if ctx.Err() != nil {
			return
		}
		a.post(func() { a.applyStructurePreview(v, gen, dim, b, true) })
	}()
}

// The first result is usable immediately. The completed slab replaces a
// separate mesh without resetting the user's camera or selected block.
func (a *App) applyStructurePreview(v *seedMapView, gen uint64, dim string, b structurePreviewBuild, complete bool) {
	res, err, surfY, noiseOK, eraApprox := b.res, b.err, b.surfY, b.noiseOK, b.eraApprox
	if a.seedMap != v || v.structurePreview == nil || v.structurePreview.gen != gen {
		return
	}
	p := v.structurePreview
	p.loading = !complete
	if err != nil {
		log.Printf("structure preview: %v", err)
		p.err, p.errKind = err.Error(), structurePreviewErrKind(err)
		return
	}
	if res == nil {
		p.err, p.errKind = "empty preview", structure3d.KindEmpty
		return
	}
	if res.Unsupported != "" {
		log.Printf("structure preview %s unsupported: %s", res.Title, res.Unsupported)
	}
	if eraApprox {
		res.Seeded, res.LayoutSeeded = false, false
	}
	p.title, p.blocks, p.unsupported, p.unsupKind, p.seeded, p.layoutSeeded = res.Title, res.BlockCount, res.Unsupported, res.UnsupportedKind, res.Seeded, res.LayoutSeeded
	p.err, p.errKind = "", ""
	if res.Image != nil && res.Unsupported == "" {
		first := p.mesh == nil
		p.mesh = res.Mesh
		if first {
			p.bitmap = ui.NewBitmap(res.Image).Pixelated()
		}
		m := res.Mesh
		p.structMinY, p.structMaxY = m.MinY, m.MaxY
		p.surfaceY, p.dim, p.noiseTerrain = surfY, dim, noiseOK
		// surface structures open on solid ground; underground ones on
		// the ghost cutaway. 地形虚影 toggles either way.
		if first {
			p.ghost = !p.onSurface()
		}
		if m.HasTerrain || !complete {
			p.terrain, p.terrainTop = true, m.TerrainTop
			if !complete {
				p.terrainTop = max(surfY, m.MaxY)
				if dim == "minecraft:the_nether" {
					p.terrainTop = netherTop
				}
			}
			if first {
				p.zoom = math.Min(p.zoom, slabFitZoom(previewSlabSize(m)))
				p.startDive(false)
			}
		}
		a.rerenderStructurePreview(a.seedMap)
	}
}

// seedMapStructurePreviewFloat draws the preview as a panel floating over
// the map stage (stageW × stageH). The header drags it; the corner grip
// resizes it. Expanded mode fills the window; Esc restores the floating panel
// before closing it on the next press.
func (a *App) seedMapStructurePreviewFloat(c *ui.Context, v *seedMapView, stageW, stageH float32) {
	p := v.structurePreview
	if p == nil {
		return
	}
	s := uiScale(c)
	if p.wheelPending {
		p.wheelPending = false
		p.refineAt = c.Now().Add(150 * time.Millisecond)
	}
	if !p.refineAt.IsZero() {
		if remaining := p.refineAt.Sub(c.Now()); remaining > 0 {
			c.After(remaining)
		} else {
			p.refineAt = time.Time{}
			a.rerenderStructurePreview(v)
		}
	}
	if a.dialog == "" && c.Shortcut(0, ui.KeyEscape) {
		if p.expanded {
			p.expanded = false
			v.previewExpandFocus.Focus(c)
			c.AnimationFrame()
		} else if !a.closeStructurePreview(c, v) {
			return
		}
	}
	if p.diving {
		a.stepDive(c, v, p)
		if v.structurePreview == nil {
			return
		}
	}
	if p.fly != nil {
		a.stepFly(c, v, p)
	}
	margin := 12 * s
	minW, minH := 340*s, 320*s
	if p.w == 0 {
		p.w = min(stageW-2*margin, 640*s)
		p.h = stageH - 2*margin
		p.x = stageW - p.w - margin
		p.y = margin
	}
	p.w = max(min(p.w, stageW-2*margin), min(minW, stageW-2*margin))
	p.h = max(min(p.h, stageH-2*margin), min(minH, stageH-2*margin))
	p.x = max(margin, min(p.x, stageW-p.w-margin))
	p.y = max(margin, min(p.y, stageH-p.h-margin))
	// The secondary viewer docks to the map edge and keeps the map in view.
	p.x, p.y = stageW-p.w-margin, margin
	expanded := p.expanded
	if !expanded {
		a.seedMapStructurePreviewPanel(c, v, p, ui.Rect{X: p.x, Y: p.y, W: p.w, H: p.h}, s, false)
		grip := ui.ButtonBase(c.Key("seedmap-structure3d-resize")).Label(a.L("seedmap.structure_preview.resize")).Absolute().Left(p.x).Top(p.y+p.h-16*s).Size(16*s, 16*s).Cursor(ui.CursorResizeNWSE)
		if dx, dy, ok := grip.Dragged(); ok && (dx != 0 || dy != 0) {
			p.w = max(minW, p.w-dx)
			p.h = max(minH, p.h+dy)
		}
	}
	ui.Overlay(c, func() {
		if !expanded {
			return
		}
		w, h := c.Size()
		ui.Box(c.Key("seedmap-structure3d-backdrop")).Absolute().Left(0).Top(42).Width(w).Height(h - 42).Background(bg.Alpha(.94))
		a.seedMapStructurePreviewPanel(c, v, p, ui.Rect{X: margin, Y: 42 + margin, W: w - 2*margin, H: h - 42 - 2*margin}, s, true)
	})
}

func (a *App) seedMapStructureStage(c *ui.Context, v *seedMapView, p *seedMapStructurePreview, s float32) {
	stage := ui.ButtonBase(c.Key("seedmap-structure3d-stage")).Label(a.L("seedmap.structure_preview.model")).Grow(1).MinWidth(0).MinHeight(0).Clip().Radius(nestedRadius(radiusChrome, previewPad) * s).Background(ui.Hex("#e8ecef"))
	b := stage.LayoutBounds()
	stage.Children(func() {
		if p.bitmap != nil {
			ui.Image(c, p.bitmap).Fill().Fit(ui.Contain)
		}
		if p.bitmap != nil && p.mesh != nil {
			a.seedMapStructureSelLabel(c, p, b, s)
			seedMapAxisGizmo(c, p, s)
		}
	})
	// Screen pixels of the stage map to frame pixels by the Contain fit.
	perPx := 1.0
	if b.W > 0 && b.H > 0 {
		perPx = float64(max(structurePreviewFullW/b.W, structurePreviewFullH/b.H))
	}
	if p.diving && stage.Clicked() {
		p.diveT, p.diving = 1, false // a click skips the dive
		if p.diveOut {
			v.structurePreview = nil
			return
		}
		a.rerenderStructurePreview(v)
	} else if stage.Clicked() && !p.dragMoved && p.mesh != nil {
		if px, py, _ := stage.PointerPosition(); b.W > 0 && b.H > 0 {
			a.pickStructureBlock(c, v, p, b, px, py)
		}
	}
	if dx, dy, ok := stage.Dragged(); ok && !p.panning {
		if dx != 0 || dy != 0 {
			p.dragging, p.dragMoved = true, true
			p.fly = nil
			p.yaw += float64(dx) * 0.01
			p.pitch = orbitPitch(p.pitch, dy)
			a.rerenderStructurePreview(v)
		}
	} else if p.dragging {
		p.dragging = false
		a.rerenderStructurePreview(v) // refine to full resolution
	}
	stage.HandleInput(func(ev ui.InputEvent) bool {
		switch ev.Kind {
		case ui.InputPointerDown:
			if ev.Button == 0 {
				p.dragMoved = false
			}
			if ev.Button == 1 {
				p.panning, p.panLX, p.panLY = true, ev.X, ev.Y
				return true
			}
		case ui.InputPointerMove:
			if p.panning && ev.Button == 1 {
				p.fly = nil
				p.panX += float64(ev.X-p.panLX) * perPx
				p.panY += float64(ev.Y-p.panLY) * perPx
				p.panLX, p.panLY = ev.X, ev.Y
				a.rerenderStructurePreview(v)
				return true
			}
		case ui.InputPointerUp:
			if p.panning && ev.Button == 1 {
				p.panning = false
				a.rerenderStructurePreview(v)
				return true
			}
		case ui.InputScroll:
			if ev.DY == 0 {
				return false
			}
			if ev.DY < 0 {
				p.zoom *= 1.1
			} else {
				p.zoom /= 1.1
			}
			p.zoom = math.Max(3, math.Min(64, p.zoom))
			p.fly = nil
			// Input handlers can retain the preceding frame's Context clock.
			// Start the idle deadline when the next view reads a fresh clock.
			p.wheelPending = true
			a.rerenderStructurePreview(v)
			return true
		}
		return false
	})
}
func structurePreviewErrKind(err error) string {
	var pe *structure3d.PreviewError
	if errors.As(err, &pe) {
		return pe.Kind
	}
	return ""
}

// structurePreviewReasonKey maps a structure3d kind to a localized sentence;
// raw Go errors never reach the panel.
func structurePreviewReasonKey(kind string) string {
	switch kind {
	case structure3d.KindMissingStructure, structure3d.KindMissingTemplate, structure3d.KindBadTemplate,
		structure3d.KindBadStructure, structure3d.KindCodeGenerated, structure3d.KindUnsupportedPiece, structure3d.KindEmpty:
		return "seedmap.structure_preview.reason." + kind
	}
	return "seedmap.structure_preview.reason.unknown"
}

// structurePreviewMaxPitch keeps the orbit just short of straight up or
// down (about ±89°), where yaw would flip.
const structurePreviewMaxPitch = 89 * math.Pi / 180

// orbitPitch applies a vertical drag: dragging down (dy > 0) raises the
// camera to look down; dragging up lowers it, past the horizon, to look
// from below. The clamp is symmetric.
func orbitPitch(pitch float64, dy float32) float64 {
	return math.Max(-structurePreviewMaxPitch, math.Min(structurePreviewMaxPitch, pitch+float64(dy)*0.01))
}

// layoutSeededDetailKey names the tooltip that says what a layout-seeded
// preview checked; the mansion has its own wording.
func layoutSeededDetailKey(id string) string {
	switch id {
	case "minecraft:mansion":
		return "seedmap.structure_preview.layout_seeded_detail_mansion"
	case "minecraft:stronghold":
		return "seedmap.structure_preview.layout_seeded_detail_stronghold"
	case "minecraft:fortress":
		return "seedmap.structure_preview.layout_seeded_detail_fortress"
	case "minecraft:desert_pyramid", "minecraft:jungle_pyramid", "minecraft:swamp_hut":
		return "seedmap.structure_preview.layout_seeded_detail_temple"
	}
	return "seedmap.structure_preview.layout_seeded_detail"
}

// structurePreviewZoom is the opening zoom; the mansion is about 80 blocks
// across, so it opens further out to fit the panel.
func structurePreviewZoom(id string) float64 {
	if id == "minecraft:mansion" {
		return 8
	}
	return 14
}

// structureUsesGround reports whether the code-built pieces of id react to
// the terrain (skip air, stop at liquid, fit height); only those pay for
// building the noise ground.
func structureUsesGround(id string) bool {
	switch id {
	case "minecraft:mineshaft", "minecraft:mineshaft_mesa", "minecraft:stronghold", "minecraft:fortress",
		"minecraft:desert_pyramid", "minecraft:jungle_pyramid", "minecraft:swamp_hut",
		"iceandfire:fire_dragon_roost", "iceandfire:ice_dragon_roost", "iceandfire:lightning_dragon_roost",
		"iceandfire:siren_island":
		return true
	}
	return strings.HasPrefix(id, "minecraft:ruined_portal")
}

// structurePreviewBuild is what the preview needs from the background build.
type structurePreviewBuild struct {
	res       *structure3d.PreviewResult
	err       error
	surfY     int
	noiseOK   bool
	eraApprox bool
}

// buildStructurePreview builds the mesh, structure and terrain slab of hit:
// the work loadStructurePreview runs off the UI thread (and the headless
// PNG harness runs directly).
func buildStructurePreview(svc *launcher.Service, rootID, versionID, world string, seed int64, dim string, sampler *launcher.LocalSampler, caves bool, version string, hits []launcher.FeatureHit, hit launcher.FeatureHit) structurePreviewBuild {
	return buildStructurePreviewProgressive(context.Background(), svc, rootID, versionID, world, seed, dim, sampler, caves, version, hits, hit, nil)
}

func buildStructurePreviewProgressive(ctx context.Context, svc *launcher.Service, rootID, versionID, world string, seed int64, dim string, sampler *launcher.LocalSampler, caves bool, version string, hits []launcher.FeatureHit, hit launcher.FeatureHit, ready func(structurePreviewBuild)) structurePreviewBuild {
	out := func() structurePreviewBuild {
		// A save can mix generation eras (chunks made before Tectonic and
		// Lithostitched were installed): build the hit with the era of its
		// start chunk, and drop the exact label where the era is unclear.
		var without []string
		eraApprox := false
		if world != "" && dim == "minecraft:overworld" {
			if m, eerr := svc.DetectEras(ctx, rootID, versionID, world); eerr == nil {
				ev := m.EraAt(hit.X>>4, hit.Z>>4)
				eraApprox = ev.Approx && len(m.Eras) > 1
				if ev.Era > 0 {
					if smp, serr := svc.EraSampler(ctx, rootID, versionID, world, ev.Era); serr == nil {
						sampler, without = smp, m.Eras[ev.Era].Without
					} else {
						eraApprox = true
					}
				}
			} else {
				log.Printf("structure preview %s: era detection: %v", hit.ID, eerr)
			}
		}
		var store *structure3d.Store
		var err error
		var layoutSampler *launcher.LocalSampler // start placed by another era's terrain
		if world != "" && dim == "minecraft:overworld" {
			// Pool-only mods do not show in the terrain: a saved start picks
			// the mod set that reproduces its own pieces.
			var exact bool
			var startEra int
			store, exact, startEra, err = svc.StartStoreEra(ctx, rootID, versionID, world, hit.ID, hit.X>>4, hit.Z>>4)
			if err == nil {
				eraApprox = !exact
				if m, eerr := svc.DetectEras(ctx, rootID, versionID, world); eerr == nil && m.EraAt(hit.X>>4, hit.Z>>4).Era != startEra {
					if smp, serr := svc.EraSampler(ctx, rootID, versionID, world, startEra); serr == nil {
						layoutSampler = smp
					} else {
						eraApprox = true
					}
				}
			}
		} else {
			store, err = svc.StructureAssetStoreEra(rootID, versionID, world, without)
		}
		var res *structure3d.PreviewResult
		surfY := 64
		noiseOK := false
		if err == nil {
			defer store.Close()
			target, rerr := resolveHitTarget(store, hit, seed, dim, sampler)
			if rerr != nil {
				err = rerr
			} else {
				var endG *previewGround
				if dim == "minecraft:the_end" && sampler != nil {
					endG = newPreviewGround(seed, dim, sampler, nil, 1, false, "", 0, 0, 0, 0, nil)
				}
				_, noiseOK = sampler.TerrainInfo(dim)
				var surfMu sync.Mutex
				surfCache := map[[2]int]int{}
				surface := structure3d.HeightFunc(func(x, z int) int {
					if endG != nil {
						return endG.EndSurface(x, z)
					}
					if sampler == nil {
						return 64
					}
					k := [2]int{x, z}
					surfMu.Lock()
					h, ok := surfCache[k]
					surfMu.Unlock()
					if ok {
						return h
					}
					h = 64
					if noiseOK && dim != "minecraft:the_nether" {
						// final_density solid top: noise-stage terrain before
						// surface rules and features.
						if top, ok := sampler.TerrainSurfaceAt(dim, x, z); ok {
							h = top
						}
					} else if v, ok := sampler.SurfaceHeightAt(dim, x, z); ok {
						h = v
					}
					surfMu.Lock()
					surfCache[k] = h
					surfMu.Unlock()
					return h
				})
				var ground structure3d.Ground
				if noiseOK && sampler != nil && dim != "minecraft:the_end" && structureUsesGround(target) {
					ground = newPreviewGroundContext(ctx, seed, dim, sampler, surface, 1, caves, "", hit.X-64, hit.Z-64, 128, 128, nil).StructureGround()
				}
				if ctx.Err() != nil {
					return structurePreviewBuild{err: ctx.Err()}
				}
				seaLevel := 0
				if info, ok := sampler.TerrainInfo(dim); ok && endG == nil && dim != "minecraft:the_nether" {
					seaLevel = info.SeaLevel // surface is the solid noise top (no water)
				}
				// The layout follows the start's era; the ground stays the
				// chunk's finished terrain.
				layoutSurface := surface
				if layoutSampler != nil {
					layoutSurface = func(x, z int) int {
						if top, ok := layoutSampler.TerrainSurfaceAt(dim, x, z); ok {
							return top
						}
						return surface(x, z)
					}
				}
				res, err = launcher.PreviewStructureSea(store, target, dim, seed, hit.X, hit.Z,
					layoutSurface, ground, seaLevel, structurePreviewFullW, structurePreviewFullH,
					structure3d.Camera{Yaw: 0.6, Pitch: 0.55, Zoom: structurePreviewZoom(hit.ID)})
				surfY = surface(hit.X, hit.Z)
				if dim == "minecraft:the_nether" {
					surfY = netherLava
				}
				if err == nil && res != nil && res.Mesh != nil && res.Unsupported == "" && sampler != nil {
					if ctx.Err() != nil {
						return structurePreviewBuild{err: ctx.Err()}
					}
					if ready != nil {
						first := *res
						ready(structurePreviewBuild{&first, nil, surfY, noiseOK, eraApprox})
						res.Mesh = res.Mesh.Fork()
					}
					t0 := time.Now()
					n := addPreviewNeighbours(store, res.Mesh, hits, hit, dim, seed, surface, sampler)
					log.Printf("structure preview %s: %d neighbouring structures on the slab, %v", hit.ID, n, time.Since(t0).Round(time.Millisecond))
					t0 = time.Now()
					addPreviewGroundContext(ctx, res.Mesh, surface, seed, dim, sampler, caves, version, store)
					log.Printf("structure preview %s: terrain slab %v, %d faces", hit.ID, time.Since(t0).Round(time.Millisecond), res.Mesh.Faces())
				}
			}
		}
		if ctx.Err() != nil {
			return structurePreviewBuild{err: ctx.Err()}
		}
		return structurePreviewBuild{res, err, surfY, noiseOK, eraApprox}
	}
	return out()
}
