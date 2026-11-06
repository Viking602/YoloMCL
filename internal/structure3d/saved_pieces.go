package structure3d

import (
	"fmt"
	"strings"

	"github.com/Viking602/YoloMCL/internal/worldgen"
)

// SavedPiece is a structure piece as saved in a structure start (id, BB,
// O = Direction.get2DDataValue, and the piece's extra tags).
type SavedPiece struct {
	ID  string
	BB  [6]int
	O   int
	Tag map[string]any
}

// dirFrom2D maps get2DDataValue (S, W, N, E) to worldgen.Dir*.
func dirFrom2D(o int) int {
	switch o {
	case 0:
		return worldgen.DirSouth
	case 1:
		return worldgen.DirWest
	case 2:
		return worldgen.DirNorth
	case 3:
		return worldgen.DirEast
	}
	return -1
}

func tagInt(t map[string]any, k string) int64 {
	switch v := t[k].(type) {
	case int32:
		return int64(v)
	case int64:
		return v
	case int8:
		return int64(v)
	case int16:
		return int64(v)
	}
	return 0
}

func tagBool(t map[string]any, k string) bool { return tagInt(t, k) != 0 }

// PostProcessSaved runs the vanilla postProcess of saved pieces of a
// code-built structure ("minecraft:fortress", ...) and returns every
// position the pieces wrote, air included. It is how the block output is
// checked against saves independently of the piece layout.
func PostProcessSaved(structure string, pieces []SavedPiece, ground Ground, minY int) (map[[3]int]BlockState, error) {
	return PostProcessSavedIn(structure, pieces, SavedWorld{Ground: ground, MinY: minY, MaxY: 4096})
}

// SavedWorld is what PostProcessSavedIn reads besides the pieces: the
// pre-structure ground, the build height range, the OCEAN_FLOOR_WG height
// (nil: estimated from Ground), and the world seed with the structure's
// feature index for the per-chunk decoration random (mineshafts).
type SavedWorld struct {
	Ground       Ground
	MinY, MaxY   int
	Floor        func(x, z int) int
	Seed         int64
	FeatureIndex int
}

// PostProcessSavedIn is PostProcessSaved with the full world inputs.
func PostProcessSavedIn(structure string, pieces []SavedPiece, sw SavedWorld) (map[[3]int]BlockState, error) {
	ground, minY := sw.Ground, sw.MinY
	w := newPieceWorld(ground, minY, minY)
	w.maxY, w.floor = sw.MaxY, sw.Floor
	bbs := make([]worldgen.Box, len(pieces))
	states := make([]*pc, len(pieces))
	for i, p := range pieces {
		bbs[i] = worldgen.Box(p.BB)
		states[i] = &pc{w: w, bb: bbs[i], dir: dirFrom2D(p.O), needChest: tagBool(p.Tag, "Chest"), selfSeed: int32(tagInt(p.Tag, "Seed")), spawnerDone: tagBool(p.Tag, "Mob")}
	}
	switch structure {
	case "minecraft:fortress":
		ids := make([]string, len(pieces))
		for i, p := range pieces {
			ids[i] = strings.TrimPrefix(p.ID, "minecraft:")
			ids[i] = savedFortressID[ids[i]]
			if ids[i] == "" {
				return nil, fmt.Errorf("unknown fortress piece %s", p.ID)
			}
			states[i].spawnerDone = false // Mob is saved after generation
		}
		runPieces(w, bbs, states, nil, func(i int, g *pc) { fortressPost(ids[i], g) })
	case "minecraft:mineshaft", "minecraft:mineshaft_mesa":
		ms := make([]*msPiece, len(pieces))
		for i, p := range pieces {
			m := &msPiece{mesa: tagInt(p.Tag, "MST") == 1}
			switch strings.TrimPrefix(p.ID, "minecraft:") {
			case "msroom":
				m.kind = worldgen.MineshaftRoom
				m.entrances = tagBoxes(p.Tag["Entrances"])
				states[i].dir = -1
			case "mscorridor":
				m.kind = worldgen.MineshaftCorridor
				m.rails, m.spider, m.sections = tagBool(p.Tag, "hr"), tagBool(p.Tag, "sc"), int(tagInt(p.Tag, "Num"))
			case "mscrossing":
				m.kind = worldgen.MineshaftCrossing
				m.twoFloor = tagBool(p.Tag, "tf")
				states[i].dir = -1
			case "msstairs":
				m.kind = worldgen.MineshaftStairs
			default:
				return nil, fmt.Errorf("unknown mineshaft piece %s", p.ID)
			}
			ms[i] = m
		}
		runPieces(w, bbs, states, msRand(sw.Seed, sw.FeatureIndex), func(i int, g *pc) { msPost(ms[i], g) })
	case "minecraft:stronghold":
		sh := make([]*worldgen.StrongholdPiece, len(pieces))
		for i, p := range pieces {
			door, _ := p.Tag["EntryDoor"].(string)
			if door == "" {
				door = "OPENING"
			}
			q := &worldgen.StrongholdPiece{ID: p.ID, Box: bbs[i], Dir: states[i].dir, Door: door,
				Tall: tagBool(p.Tag, "Tall"), Steps: int(tagInt(p.Tag, "Steps")), Kind: int(tagInt(p.Tag, "Type"))}
			if strings.HasSuffix(p.ID, ":sh5c") {
				q.Flags = [4]bool{tagBool(p.Tag, "leftLow"), tagBool(p.Tag, "leftHigh"), tagBool(p.Tag, "rightLow"), tagBool(p.Tag, "rightHigh")}
			} else {
				q.Flags = [4]bool{tagBool(p.Tag, "Left"), tagBool(p.Tag, "Right")}
			}
			sh[i] = q
			states[i].needChest, states[i].spawnerDone = true, false // saved after generation
		}
		runPieces(w, bbs, states, decoRand(sw.Seed, sw.FeatureIndex, worldgen.StepSurfaceStructures), func(i int, g *pc) { strongholdPost(sh[i], g) })
	case "minecraft:desert_pyramid", "minecraft:jungle_pyramid", "minecraft:swamp_hut":
		// HPos is saved, so the box is already fitted; the chest and trap
		// flags are saved after generation and start false.
		for i := range pieces {
			t := &templeState{hpos: 0}
			runPieces(w, bbs[i:i+1], states[i:i+1], decoRand(sw.Seed, sw.FeatureIndex, worldgen.StepSurfaceStructures),
				func(_ int, g *pc) { templePost(structure, t, g) })
		}
	default:
		return nil, fmt.Errorf("no piece code for %s", structure)
	}
	return w.m, nil
}

// savedFortressID maps StructurePieceType registry names to piece ids.
var savedFortressID = map[string]string{
	"nebs": worldgen.NeBridge, "nebef": worldgen.NeBridgeEnd, "nebcr": worldgen.NeBridgeCross,
	"nerc": worldgen.NeRoomCross, "nesr": worldgen.NeStairsRoom, "nemt": worldgen.NeThrone,
	"nece": worldgen.NeEntrance, "nesc": worldgen.NeCorridor, "nescsc": worldgen.NeCorridorX,
	"nescrt": worldgen.NeCorridorR, "nesclt": worldgen.NeCorridorL, "neccs": worldgen.NeCorridorStep,
	"nectb": worldgen.NeBalcony, "necsr": worldgen.NeStalkRoom, "nestart": worldgen.NeStart,
}

// tagBoxes reads a list of BoundingBox int arrays.
func tagBoxes(v any) []worldgen.Box {
	var out []worldgen.Box
	add := func(a []int32) {
		if len(a) == 6 {
			out = append(out, worldgen.Box{int(a[0]), int(a[1]), int(a[2]), int(a[3]), int(a[4]), int(a[5])})
		}
	}
	switch l := v.(type) {
	case [][]int32:
		for _, a := range l {
			add(a)
		}
	case []any:
		for _, e := range l {
			if a, ok := e.([]int32); ok {
				add(a)
			}
		}
	}
	return out
}

// msRand is the per-chunk decoration random of the mineshaft step.
func msRand(seed int64, index int) func(cx, cz int) pieceRand {
	return func(cx, cz int) pieceRand {
		return &dr{worldgen.NewDecorationRandom(seed, cx, cz, index, worldgen.StepUndergroundStructures)}
	}
}

type dr struct{ r *worldgen.DecorationRandom }

func (d *dr) nextInt(n int) int  { return int(d.r.NextInt(int32(n))) }
func (d *dr) nextFloat() float32 { return d.r.NextFloat() }
func (d *dr) nextBool() bool     { return d.r.NextBoolean() }
func (d *dr) nextLong() int64    { return d.r.NextLong() }
