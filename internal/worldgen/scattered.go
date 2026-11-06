package worldgen

import "strconv"

// HorizontalFaces is Direction.Plane.HORIZONTAL order.
var HorizontalFaces = [4]string{"north", "east", "south", "west"}

// ScatteredOrientation mirrors ScatteredFeaturePiece's
// getRandomHorizontalDirection for desert pyramids, jungle temples and swamp huts.
func ScatteredOrientation(seed int64, cx, cz int32) string {
	return HorizontalFaces[LargeFeatureRandom(seed, cx, cz).NextInt(4)]
}

var (
	shipwreckBeached = []string{"with_mast", "sideways_full", "sideways_fronthalf", "sideways_backhalf", "rightsideup_full", "rightsideup_fronthalf", "rightsideup_backhalf", "with_mast_degraded", "rightsideup_full_degraded", "rightsideup_fronthalf_degraded", "rightsideup_backhalf_degraded"}
	shipwreckOcean   = []string{"with_mast", "upsidedown_full", "upsidedown_fronthalf", "upsidedown_backhalf", "sideways_full", "sideways_fronthalf", "sideways_backhalf", "rightsideup_full", "rightsideup_fronthalf", "rightsideup_backhalf", "with_mast_degraded", "upsidedown_full_degraded", "upsidedown_fronthalf_degraded", "upsidedown_backhalf_degraded", "sideways_full_degraded", "sideways_fronthalf_degraded", "sideways_backhalf_degraded", "rightsideup_full_degraded", "rightsideup_fronthalf_degraded", "rightsideup_backhalf_degraded"}
)

// Shipwreck mirrors ShipwreckStructure.generatePieces: template and rotation;
// the template sits at (chunk min X, 90, chunk min Z) before height fitting.
func Shipwreck(seed int64, cx, cz int32, beached bool) (template, rotation string) {
	r := LargeFeatureRandom(seed, cx, cz)
	rotation = Rotations[r.NextInt(4)]
	list := shipwreckOcean
	if beached {
		list = shipwreckBeached
	}
	return "minecraft:shipwreck/" + list[r.NextInt(int32(len(list)))], rotation
}

// RuinedPortalSetup is one entry of a ruined_portal structure's "setups".
type RuinedPortalSetup struct {
	Placement            string  `json:"placement"`
	AirPocketProbability float32 `json:"air_pocket_probability"`
	Weight               float32 `json:"weight"`
}

// RuinedPortal mirrors RuinedPortalStructure.findGenerationPoint up to the
// template, rotation and mirror choice.
func RuinedPortal(seed int64, cx, cz int32, setups []RuinedPortalSetup) (template, rotation, mirror, placement string) {
	p := RuinedPortalFull(seed, cx, cz, setups, nil, nil)
	return p.Template, p.Rotation, p.Mirror, p.Placement
}

// RuinedPortalLayout is the seeded part of a ruined portal start. R is left
// positioned for findSuitableY.
type RuinedPortalLayout struct {
	Template, Rotation, Mirror, Placement string
	AirPocket                             bool
	R                                     *JavaRandom
}

// RuinedPortalFull is RuinedPortal with Lithostitched's
// RuinedPortalStructureMixin (breaks_seed_parity): when standard/giant are
// non-nil the vanilla template draw still happens and a second
// nextInt(len(list)) picks from the template list.
func RuinedPortalFull(seed int64, cx, cz int32, setups []RuinedPortalSetup, standard, giant []string) (p RuinedPortalLayout) {
	if len(setups) == 0 {
		return
	}
	template, rotation, mirror, placement := "", "", "", ""
	r := LargeFeatureRandom(seed, cx, cz)
	setup := setups[0]
	if len(setups) > 1 {
		var total float32
		for _, s := range setups {
			total += s.Weight
		}
		g := r.NextFloat()
		for _, s := range setups {
			g -= s.Weight / total
			if g < 0 {
				setup = s
				break
			}
		}
	}
	switch ap := setup.AirPocketProbability; ap {
	case 0:
	case 1:
		p.AirPocket = true
	default:
		p.AirPocket = r.NextFloat() < ap
	}
	if r.NextFloat() < 0.05 {
		template = "minecraft:ruined_portal/giant_portal_" + strconv.Itoa(1+int(r.NextInt(3)))
		if len(giant) > 0 {
			template = giant[r.NextInt(int32(len(giant)))]
		}
	} else {
		template = "minecraft:ruined_portal/portal_" + strconv.Itoa(1+int(r.NextInt(10)))
		if len(standard) > 0 {
			template = standard[r.NextInt(int32(len(standard)))]
		}
	}
	rotation = Rotations[r.NextInt(4)]
	mirror = "FRONT_BACK"
	if r.NextFloat() < 0.5 {
		mirror = "NONE"
	}
	placement = setup.Placement
	p.Template, p.Rotation, p.Mirror, p.Placement, p.R = template, rotation, mirror, placement, r
	return p
}

// RuinedPortalY is RuinedPortalStructure.findSuitableY. height is the
// placement heightmap at the template's centre minus one, ySpan the box
// height, solid(k, i) whether corner i (minX/minZ, maxX/minZ, minX/maxZ,
// maxX/maxZ) is opaque for the placement's heightmap at y=k.
func RuinedPortalY(p RuinedPortalLayout, minY, height, ySpan int, solid func(y, corner int) bool) int {
	lo := minY + 15
	between := func(a, b int) int { return a + int(p.R.NextInt(int32(b-a+1))) }
	within := func(a, b int) int {
		if a < b {
			return between(a, b)
		}
		return b
	}
	var j int
	switch p.Placement {
	case "in_nether":
		if p.AirPocket {
			j = between(32, 100)
		} else if p.R.NextFloat() < 0.5 {
			j = between(27, 29)
		} else {
			j = between(29, 100)
		}
	case "in_mountain":
		j = within(70, height-ySpan)
	case "underground":
		j = within(lo, height-ySpan)
	case "partly_buried":
		j = height - ySpan + between(2, 8)
	default:
		j = height
	}
	k := j
	for ; k > lo; k-- {
		n := 0
		for c := 0; c < 4; c++ {
			if solid(k, c) {
				n++
			}
		}
		if n >= 3 {
			break
		}
	}
	return k
}

// TransformMirror is StructureTemplate.transform: mirror first, then rotate
// around the pivot.
func TransformMirror(local [3]int, mirror, rotation string, pivot [3]int) [3]int {
	switch mirror {
	case "LEFT_RIGHT":
		local[2] = -local[2]
	case "FRONT_BACK":
		local[0] = -local[0]
	}
	return Transform(local, rotation, pivot)
}
