package nativeui

import (
	"context"
	"fmt"
	"math"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/egoist/mygo/ui"

	"github.com/Viking602/YoloMCL/internal/launcher"
)

type seedMapView struct {
	key, rootID, versionID, world, seedText string
	yText                                   string
	cancel                                  context.CancelFunc
	generation, request                     uint64
	seed                                    int64
	snapshot                                *launcher.WorldgenSnapshot
	sampler                                 *launcher.LocalSampler
	tiles                                   *launcher.SeedMapTiles
	viewport, requested                     launcher.SeedMapViewport
	frame                                   launcher.SeedMapFrame
	raster                                  seedMapRaster
	camera                                  seedMapCamera
	hits                                    []launcher.FeatureHit
	hitsVersion                             uint64
	loading, pending, details               bool
	progress                                string // localized open stage while loading
	// completing: an early (provisional) sampler is shown for earlyDims while
	// the final snapshot is still being built.
	completing                                               bool
	earlyDims                                                []string
	earlyCancel                                              context.CancelFunc
	err                                                      string
	selected                                                 *launcher.FeatureHit
	selectedSpawn                                            *launcher.SeedMapSpawn
	labels                                                   seedMapLabels
	saves                                                    []launcher.World
	level                                                    launcher.LevelInfo
	estimate                                                 *launcher.SeedMapSpawn
	query, radius, xText, zText                              string
	searching, searchQueued, searchLimited, legend, searched bool
	searchGeneration                                         uint64
	results                                                  []launcher.FeatureHit
	searchError                                              string
	sourceError                                              string
	hideMarkers                                              bool
	markerGroup                                              []launcher.FeatureHit
	markerLayout                                             seedMapMarkerLayout
	pixelsPerBlock                                           int
	hover, inspected                                         *launcher.SeedMapPoint
	sliceMode                                                bool
	pointPopup                                               bool
	pointOpen                                                bool
	pointTarget                                              string
	canvasFocus, previewFocus                                ui.Handle
	previewPanelFocus, previewExpandFocus                    ui.Handle
	structurePreview                                         *seedMapStructurePreview
	jumpNearest                                              bool            // the next search result centres and selects the nearest hit
	keys                                                     map[ui.Key]bool // held pan keys on the focused canvas
	keyTick                                                  time.Time
	keyFast                                                  bool
	wheel                                                    float32 // touchpad scroll not yet worth a zoom step
	pinch                                                    float32 // pinch scale not yet worth a zoom step
	biomeSearch                                              string  // biome being located, or "" when idle
	biomeGeneration                                          uint64
	biomeStatus                                              string
	biomeCancel                                              context.CancelFunc
	biomeStatusWarn                                          bool
	showCaves                                                bool         // carver overlay at the slice Y
	caves                                                    seedMapCaves // carver shape and mask cache
	showOres                                                 bool         // ore X-ray at the slice Y
	ores                                                     seedMapOres  // ore cell cache
	oreFilter                                                string       // single mineral shown, empty for all
}

func (a *App) closeSeedMap() {
	if v := a.seedMap; v != nil {
		v.discardStructurePreview()
		if v.cancel != nil {
			v.cancel()
		}
		if v.biomeCancel != nil {
			v.biomeCancel()
		}
		a.seedMap = nil
	}
}

func (a *App) reconcileSeedMap() {
	v := a.seedMap
	if v == nil {
		return
	}
	r := a.root()
	if a.page != "home" || a.instTab != "seedmap" || r == nil || v.key != r.ID+"|"+r.SelectedVersion {
		a.closeSeedMap()
	}
}

func (a *App) openSeedMap(world string) {
	r := a.root()
	if r == nil || a.service == nil || a.launcherUpdateRequired() {
		return
	}
	a.setInstTab("seedmap")
	a.closeSeedMap()
	v := &seedMapView{key: r.ID + "|" + r.SelectedVersion, rootID: r.ID, versionID: r.SelectedVersion, world: world, seedText: "0", yText: "64", radius: "4096", xText: "0", zText: "0", viewport: launcher.SeedMapViewport{Width: 768, Height: 480, BlocksPerPixel: 1, Y: 64, Dimension: "minecraft:overworld"}}
	a.seedMap = v
	a.loadSeedMap(v, true)
}

// Each session has one ingest/compile task followed by one tile worker. All
// mutable view and texture state remains on the UI thread through App.post.
func (a *App) loadSeedMap(v *seedMapView, readSaveSeed bool) {
	seed, err := strconv.ParseInt(strings.TrimSpace(v.seedText), 10, 64)
	if err != nil {
		v.err = a.L("seedmap.invalid_seed")
		return
	}
	v.discardStructurePreview()
	if v.cancel != nil {
		v.cancel()
	}
	ctx, cancel := context.WithCancel(a.ctx)
	v.cancel = cancel
	if v.earlyCancel != nil {
		v.earlyCancel()
	}
	v.completing, v.earlyDims, v.earlyCancel = false, nil, nil
	v.generation++
	gen := v.generation
	v.tiles, v.sampler = nil, nil
	v.raster, v.camera = seedMapRaster{}, seedMapCamera{}
	v.hits = nil
	v.hitsVersion++
	v.frame = launcher.SeedMapFrame{}
	v.selected = nil
	v.selectedSpawn = nil
	v.hover, v.inspected = nil, nil
	v.loading, v.pending, v.err = true, false, ""
	v.progress = ""
	v.markerGroup, v.markerLayout = nil, seedMapMarkerLayout{}
	v.searching, v.searchQueued, v.jumpNearest = false, false, false
	v.results, v.searchError = nil, ""
	v.searched = false
	v.searchGeneration++
	snapshot := v.snapshot
	id, version, world := v.rootID, v.versionID, v.world
	service := a.service
	a.bgWG.Add(1)
	progress := func(stage, detail string) {
		a.post(func() {
			if a.seedMap == v && v.generation == gen && (v.loading || v.completing) {
				v.progress = a.seedMapProgressLabel(stage, detail)
			}
		})
	}
	ctx = launcher.WithSeedMapProgress(ctx, progress)
	var saves []launcher.World
	var savesErr error
	if snapshot == nil {
		// Show dimensions that provably cannot depend on mod registration code
		// as soon as every pack is read; the final snapshot replaces it.
		ctx = launcher.WithSeedMapEarly(ctx, func(p *launcher.WorldgenSnapshot, level launcher.LevelInfo) {
			if readSaveSeed && world != "" && !level.HasRandomSeed {
				return
			}
			earlySeed := seed
			if readSaveSeed && level.HasRandomSeed {
				earlySeed = level.RandomSeed
			}
			earlySaves, earlySavesErr := saves, savesErr
			a.bgWG.Add(1)
			go func() {
				defer a.bgWG.Done()
				sampler, err := launcher.NewLocalSampler(ctx, p, earlySeed)
				if err != nil {
					return
				}
				if store, serr := service.StructureAssetStoreWorld(id, version, world); serr == nil {
					bindJigsawStarts(store, sampler)
					store.Close()
				}
				dims := launcher.SeedMapEarlyDimensions(ctx, p, sampler)
				if len(dims) == 0 || ctx.Err() != nil {
					return
				}
				var estimate *launcher.SeedMapSpawn
				if dims[0] == "minecraft:overworld" {
					estimate, _ = sampler.EstimateSpawn(ctx)
				}
				ectx, ecancel := context.WithCancel(ctx)
				tiles := launcher.NewSeedMapTiles(ectx, sampler)
				labels := readSeedMapLabels(p)
				a.post(func() {
					if a.seedMap != v || v.generation != gen || !v.loading || ctx.Err() != nil {
						ecancel()
						tiles.Close()
						return
					}
					v.sampler, v.tiles, v.labels, v.estimate = sampler, tiles, labels, estimate
					v.saves, v.level = earlySaves, level
					v.sourceError = ""
					if earlySavesErr != nil {
						v.sourceError = a.L("seedmap.source_failed") + " " + a.friendlyErr(earlySavesErr)
					}
					v.completing, v.earlyDims, v.earlyCancel = true, dims, ecancel
					if !slices.Contains(dims, v.viewport.Dimension) {
						v.viewport.Dimension = dims[0]
					}
					if v.viewport.Structure != "" && !slices.ContainsFunc(sampler.Structures(v.viewport.Dimension), func(structure launcher.SeedMapStructure) bool {
						return structure.ID == v.viewport.Structure
					}) {
						v.viewport.Structure = ""
					}
					v.loading = false
					v.viewport.Surface = !v.sliceMode && sampler.SupportsSurface(v.viewport.Dimension)
					v.seedText = strconv.FormatInt(earlySeed, 10)
					v.seed = earlySeed
					if readSaveSeed {
						if spawns := v.spawns(); len(spawns) > 0 && spawns[0].Dimension == v.viewport.Dimension {
							v.center(spawns[0].X, spawns[0].Z)
						}
					}
					a.requestSeedMap(v)
				})
				<-tiles.Done()
			}()
		})
	}
	go func() {
		defer a.bgWG.Done()
		var level launcher.LevelInfo
		var e error
		fresh := snapshot == nil
		if snapshot == nil {
			saves, savesErr = service.SeedMapSaves(ctx, id, version)
			snapshot, level, e = service.SeedMapWorldgen(ctx, id, version, world)
		}
		missing := readSaveSeed && world != "" && !level.HasRandomSeed
		if readSaveSeed && level.HasRandomSeed {
			seed = level.RandomSeed
		}
		var sampler *launcher.LocalSampler
		if e == nil {
			progress("sampler", "")
			sampler, e = launcher.NewLocalSampler(ctx, snapshot, seed)
		}
		if e == nil {
			if store, serr := service.StructureAssetStoreWorld(id, version, world); serr == nil {
				bindJigsawStarts(store, sampler)
				store.Close()
			}
		}
		var estimate *launcher.SeedMapSpawn
		if e == nil {
			progress("spawn", "")
			estimate, e = sampler.EstimateSpawn(ctx)
		}
		if ctx.Err() != nil {
			return
		}
		if e != nil {
			a.post(func() {
				if a.seedMap == v && v.generation == gen {
					a.dropSeedMapEarly(v)
					v.loading = false
					v.err = a.friendlyErr(e)
				}
			})
			return
		}
		tiles := launcher.NewSeedMapTiles(ctx, sampler)
		labels := readSeedMapLabels(snapshot)
		a.post(func() {
			if a.seedMap != v || v.generation != gen || ctx.Err() != nil {
				tiles.Close()
				return
			}
			wasEarly := v.completing
			if wasEarly {
				if v.earlyCancel != nil {
					v.earlyCancel()
				}
				if v.tiles != nil {
					v.tiles.Close()
				}
				v.completing, v.earlyDims, v.earlyCancel, v.progress = false, nil, nil, ""
			}
			v.snapshot, v.sampler, v.tiles = snapshot, sampler, tiles
			v.labels, v.estimate = labels, estimate
			if fresh {
				v.saves, v.level = saves, level
				v.sourceError = ""
				if savesErr != nil {
					v.sourceError = a.L("seedmap.source_failed") + " " + a.friendlyErr(savesErr)
				}
			}
			dimensions := seedMapDimensions(sampler)
			if !slices.Contains(dimensions, v.viewport.Dimension) && len(dimensions) > 0 {
				v.viewport.Dimension = dimensions[0]
			}
			if v.viewport.Structure != "" && !slices.ContainsFunc(sampler.Structures(v.viewport.Dimension), func(structure launcher.SeedMapStructure) bool {
				return structure.ID == v.viewport.Structure
			}) {
				v.viewport.Structure = ""
			}
			v.loading = false
			v.viewport.Surface = !v.sliceMode && sampler.SupportsSurface(v.viewport.Dimension)
			v.seedText = strconv.FormatInt(seed, 10)
			v.seed = seed
			if missing {
				v.seedText = ""
				v.err = a.L("seedmap.missing_seed")
				return
			}
			if readSaveSeed && !wasEarly {
				if spawns := v.spawns(); len(spawns) > 0 && spawns[0].Dimension == v.viewport.Dimension {
					v.center(spawns[0].X, spawns[0].Z)
				}
			}
			a.requestSeedMap(v)
		})
		<-tiles.Done()
	}()
}

func (a *App) requestSeedMap(v *seedMapView) {
	if v.tiles == nil || v.sampler == nil {
		return
	}
	v.request++
	v.markerGroup = nil
	v.hover, v.inspected = nil, nil
	// Resizing or zooming changes raster work, not the nearby query. Resume
	// that query after sampling instead of silently cancelling the click.
	if v.searching && v.viewport.X == v.requested.X && v.viewport.Y == v.requested.Y && v.viewport.Z == v.requested.Z && v.viewport.Dimension == v.requested.Dimension && v.viewport.Structure == v.requested.Structure {
		v.searchQueued = true
	}
	v.searchGeneration++
	v.searching = v.searchQueued
	if !v.searchQueued {
		v.jumpNearest = false
	}
	request, gen := v.request, v.generation
	if v.requested.Dimension != v.viewport.Dimension || v.requested.Structure != v.viewport.Structure {
		v.hits = nil
		v.hitsVersion++
	}
	v.requested = v.viewport
	v.pending = true
	delivery := &seedMapDelivery{}
	_, err := v.tiles.RequestTiles(v.viewport, func(frame launcher.SeedMapFrame) {
		if !delivery.put(frame) {
			return
		}
		a.post(func() {
			frame := delivery.take()
			if a.seedMap != v || v.generation != gen || v.request != request {
				return
			}
			v.frame, v.pending = frame, frame.Partial
			if frame.Err != nil {
				v.searching, v.searchQueued, v.jumpNearest = false, false, false
				v.err = a.friendlyErr(frame.Err)
				return
			}
			v.raster.accept(frame)
			if !frame.Partial {
				v.hits = frame.Hits
				v.hitsVersion++
			}
		})
	})
	if err != nil {
		v.pending = false
		v.searching, v.searchQueued, v.jumpNearest = false, false, false
		v.err = a.friendlyErr(err)
	}
}

func (v *seedMapView) zoom(delta int) {
	v.camera.coast = time.Time{}
	v.pixelsPerBlock = max(1, v.pixelsPerBlock)
	if delta > 0 {
		if v.viewport.BlocksPerPixel > 1 {
			v.viewport.BlocksPerPixel = max(1, v.viewport.BlocksPerPixel/2)
		} else {
			v.pixelsPerBlock = min(8, v.pixelsPerBlock*2)
		}
	} else if v.pixelsPerBlock > 1 {
		v.pixelsPerBlock /= 2
	} else {
		v.viewport.BlocksPerPixel = min(32, v.viewport.BlocksPerPixel*2)
	}
}

func (v *seedMapView) center(x, z int) {
	v.camera = seedMapCamera{}
	v.placeCamera(float64(x), float64(z))
}

func (v *seedMapView) spawns() []launcher.SeedMapSpawn {
	var result []launcher.SeedMapSpawn
	if v.world != "" && v.level.HasRandomSeed && v.seed == v.level.RandomSeed {
		for _, spawn := range []*launcher.SeedMapSpawn{v.level.WorldSpawn, v.level.PlayerSpawn} {
			if spawn != nil {
				result = append(result, *spawn)
			}
		}
	}
	if len(result) == 0 && v.estimate != nil {
		result = append(result, *v.estimate)
	}
	return result
}

func (a *App) seedMapDimension(v *seedMapView, id string) {
	if id == v.viewport.Dimension {
		return
	}
	v.discardStructurePreview()
	v.viewport.Dimension, v.viewport.Structure = id, ""
	v.viewport.Surface = !v.sliceMode && v.sampler != nil && v.sampler.SupportsSurface(id)
	v.hover, v.inspected = nil, nil
	v.frame, v.selected = launcher.SeedMapFrame{}, nil
	v.raster, v.camera = seedMapRaster{}, seedMapCamera{}
	v.hits = nil
	v.hitsVersion++
	v.selectedSpawn = nil
	v.results, v.searchError = nil, ""
	v.searching, v.searchQueued, v.jumpNearest = false, false, false
	v.searched = false
	v.searchGeneration++
	v.biomeGeneration++
	v.biomeSearch, v.biomeStatus = "", ""
}

// Search and tile sampling share one worker. Keep the user's search queued
// until the changed structure filter has finished refreshing its viewport.
func (a *App) searchSeedMap(v *seedMapView) {
	if v.tiles == nil || v.viewport.Structure == "" {
		return
	}
	if _, err := strconv.Atoi(v.radius); err != nil {
		return
	}
	v.searchGeneration++
	v.searchQueued = true
	v.searching, v.results, v.searchError, v.searchLimited = true, nil, "", false
	v.searched = true
}

func (a *App) startSeedMapSearch(v *seedMapView) {
	v.searchQueued = false
	radius, err := strconv.Atoi(v.radius)
	if err != nil || v.tiles == nil || v.viewport.Structure == "" {
		v.searching, v.jumpNearest = false, false
		return
	}
	gen, session := v.searchGeneration, v.generation
	q := launcher.SeedMapSearch{Dimension: v.viewport.Dimension, Structure: v.viewport.Structure, X: v.viewport.X, Y: v.viewport.Y, Z: v.viewport.Z, Radius: radius, Limit: 32}
	_, err = v.tiles.Search(q, func(result launcher.SeedMapSearchResult, err error) {
		a.post(func() {
			if a.seedMap != v || v.generation != session || v.searchGeneration != gen {
				return
			}
			v.searching, v.results, v.searchLimited = false, result.Hits, result.Limited
			if err != nil {
				v.searchError = a.friendlyErr(err)
			}
			if v.jumpNearest {
				v.jumpNearest = false
				if hit, ok := nearestHit(result.Hits, q.X, q.Z); ok && err == nil {
					v.center(hit.X, hit.Z)
					v.selected, v.selectedSpawn, v.markerGroup = &hit, nil, nil
				}
			}
		})
	})
	if err != nil {
		v.searching = false
		v.jumpNearest = false
		v.searchError = a.friendlyErr(err)
	}
}

// nearestHit returns the hit closest to (x, z); the search origin is the map
// centre at the time the search started.
func nearestHit(hits []launcher.FeatureHit, x, z int) (launcher.FeatureHit, bool) {
	best, bestD := -1, int64(0)
	for i, h := range hits {
		dx, dz := int64(h.X-x), int64(h.Z-z)
		if d := dx*dx + dz*dz; best < 0 || d < bestD {
			best, bestD = i, d
		}
	}
	if best < 0 {
		return launcher.FeatureHit{}, false
	}
	return hits[best], true
}

// cancelSeedMapLoad stops the in-flight open; the refresh button retries.
func (a *App) cancelSeedMapLoad(v *seedMapView) {
	if v == nil || !v.loading && !v.completing {
		return
	}
	a.dropSeedMapEarly(v)
	if v.cancel != nil {
		v.cancel()
		v.cancel = nil
	}
	v.generation++
	v.loading, v.progress = false, ""
	v.err = a.L("seedmap.cancelled")
}

func (a *App) seedMapProgressLabel(stage, detail string) string {
	switch stage {
	case "register":
		return fmt.Sprintf(a.L("seedmap.progress.register"), detail)
	case "prepare", "sources", "regions", "biomes", "sampler", "spawn":
		return a.L("seedmap.progress." + stage)
	}
	return a.L("seedmap.loading")
}

// dropSeedMapEarly discards a provisional sampler (final result failed or the
// user cancelled); it never outlives its open request.
func (a *App) dropSeedMapEarly(v *seedMapView) {
	if !v.completing {
		return
	}
	if v.earlyCancel != nil {
		v.earlyCancel()
	}
	if v.tiles != nil {
		v.tiles.Close()
	}
	v.tiles, v.sampler = nil, nil
	v.raster, v.camera = seedMapRaster{}, seedMapCamera{}
	v.frame = launcher.SeedMapFrame{}
	v.pending = false
	v.completing, v.earlyDims, v.earlyCancel = false, nil, nil
}

// dimensionOptions limits the selector to verified early dimensions while
// the final snapshot is still being built.
func (v *seedMapView) dimensionOptions() []string {
	options := seedMapDimensions(v.sampler)
	if !v.completing {
		return options
	}
	return slices.DeleteFunc(options, func(id string) bool { return !slices.Contains(v.earlyDims, id) })
}

// findSeedMapBiome locates the nearest cell of biome from the view centre at
// the current Y within the chosen radius, then centres the map on it.
func (a *App) findSeedMapBiome(v *seedMapView, biome string) {
	if v.sampler == nil {
		return
	}
	radius, err := strconv.Atoi(v.radius)
	if err != nil {
		return
	}
	if v.biomeCancel != nil {
		v.biomeCancel()
	}
	ctx, cancel := context.WithCancel(a.ctx)
	v.biomeCancel = cancel
	v.biomeGeneration++
	gen, session := v.biomeGeneration, v.generation
	sampler, dim, x, y, z := v.sampler, v.viewport.Dimension, v.viewport.X, v.viewport.Y, v.viewport.Z
	name := a.seedMapName("biome", biome, a.isEN())
	v.biomeSearch, v.biomeStatus, v.biomeStatusWarn = biome, fmt.Sprintf(a.L("seedmap.biome_searching"), name), false
	a.bgWG.Add(1)
	go func() {
		defer a.bgWG.Done()
		hit, err := sampler.FindBiome(ctx, dim, biome, x, y, z, radius)
		a.post(func() {
			if a.seedMap != v || v.generation != session || v.biomeGeneration != gen {
				return
			}
			v.biomeSearch = ""
			switch {
			case err != nil:
				if ctx.Err() == nil {
					v.biomeStatus, v.biomeStatusWarn = a.friendlyErr(err), true
				} else {
					v.biomeStatus = ""
				}
			case !hit.Found:
				v.biomeStatus, v.biomeStatusWarn = fmt.Sprintf(a.L("seedmap.biome_none"), name, radius), true
			default:
				dx, dz := float64(hit.X-x), float64(hit.Z-z)
				v.biomeStatus = fmt.Sprintf(a.L("seedmap.biome_found"), name, hit.X, hit.Z, int(math.Hypot(dx, dz)), hit.Step)
				v.center(hit.X, hit.Z)
				v.selected, v.selectedSpawn, v.markerGroup = nil, nil, nil
			}
		})
	}()
}
