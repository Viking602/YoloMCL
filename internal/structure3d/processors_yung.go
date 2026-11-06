package structure3d

import (
	"encoding/json"
	"strings"

	"github.com/Viking602/YoloMCL/internal/worldgen"
)

// YUNG's structure processors (YungsApi 1.21.1 mods), ported from javap of
// the jars. Seed-exact where the processor only reads the template block and
// StructurePlaceSettings.getRandom(pos); terrain reads (air/fluid checks,
// leg columns) use the estimated surface and mark the scene approximate.

// stateRandomizer is yungsapi BlockStateRandomizer: first entry whose
// [cumul, cumul+p) holds nextFloat, else the default.
type stateRandomizer struct {
	def     BlockState
	entries []struct {
		state BlockState
		p     float32
	}
}

func parseRandomizer(raw json.RawMessage) (*stateRandomizer, bool) {
	var o struct {
		Default json.RawMessage `json:"defaultBlockState"`
		Entries []struct {
			State json.RawMessage `json:"blockState"`
			P     float32         `json:"probability"`
		} `json:"entries"`
	}
	if len(raw) == 0 || json.Unmarshal(raw, &o) != nil || len(o.Default) == 0 {
		return nil, false
	}
	r := &stateRandomizer{def: parseStateJSON(o.Default)}
	for _, e := range o.Entries {
		r.entries = append(r.entries, struct {
			state BlockState
			p     float32
		}{parseStateJSON(e.State), e.P})
	}
	return r, true
}

func (r *stateRandomizer) get(rng *worldgen.JavaRandom) BlockState {
	f := rng.NextFloat()
	var cumul float32
	for _, e := range r.entries {
		if cumul <= f && f < cumul+e.p {
			return e.state
		}
		cumul += e.p
	}
	return r.def
}

func withProps(s BlockState, kv map[string]string) BlockState {
	p := map[string]string{}
	for k, v := range s.Properties {
		p[k] = v
	}
	for k, v := range kv {
		p[k] = v
	}
	return BlockState{Name: s.Name, Properties: p}
}

// blockProps lists the properties a vanilla block has, by name shape; used
// for hasProperty on randomizer outputs that only name the block.
func blockProps(name string) map[string]bool {
	switch {
	case strings.HasSuffix(name, "_stairs"):
		return map[string]bool{"facing": true, "half": true, "shape": true, "waterlogged": true}
	case strings.HasSuffix(name, "_slab"):
		return map[string]bool{"type": true, "waterlogged": true}
	case strings.HasSuffix(name, "_wall"):
		return map[string]bool{"north": true, "east": true, "south": true, "west": true, "up": true, "waterlogged": true}
	case strings.HasSuffix(name, "_fence"):
		return map[string]bool{"north": true, "east": true, "south": true, "west": true, "waterlogged": true}
	case strings.HasSuffix(name, "_wall_torch"):
		return map[string]bool{"facing": true}
	}
	return nil
}

func hasProp(s BlockState, k string) bool {
	if _, ok := s.Properties[k]; ok {
		return true
	}
	return blockProps(s.Name)[k]
}

// worldState is what LevelReader.getBlockState sees at w during placement:
// structure blocks placed so far, else the estimated terrain. ok=false when
// there is no terrain estimate.
func (c *procCtx) worldState(w [3]int) (BlockState, bool) {
	if s, ok := c.existing(w); ok {
		return s, true
	}
	if c.height == nil {
		return BlockState{}, false
	}
	c.approx = true
	if w[1] <= c.height(w[0], w[2]) {
		return BlockState{Name: "minecraft:stone"}, true
	}
	if c.sea != 0 && w[1] < c.sea {
		return BlockState{Name: "minecraft:water"}, true
	}
	return BlockState{Name: "minecraft:air"}, true
}

func isAirName(n string) bool {
	return n == "minecraft:air" || n == "minecraft:cave_air" || n == "minecraft:void_air"
}

func isFluidState(s BlockState) bool {
	return s.Name == "minecraft:water" || s.Name == "minecraft:lava" || s.Properties["waterlogged"] == "true" ||
		s.Name == "minecraft:kelp" || s.Name == "minecraft:kelp_plant" || s.Name == "minecraft:seagrass" || s.Name == "minecraft:tall_seagrass" || s.Name == "minecraft:bubble_column"
}

// leg fills downward from below w while the world is air or fluid, writing
// straight into the chunk like the YUNG leg processors.
func (c *procCtx) leg(w [3]int, next func() BlockState) {
	if c.place == nil {
		return
	}
	c.approx = true
	for y := w[1] - 1; y > -64 && y > w[1]-256; y-- {
		p := [3]int{w[0], y, w[2]}
		s, ok := c.worldState(p)
		if !ok || !(isAirName(s.Name) || isFluidState(s)) {
			return
		}
		c.place(p, next())
	}
}

type yungProc struct {
	fn func(c *procCtx, b procBlock) *procBlock
}

func (p yungProc) process(c *procCtx, b procBlock) *procBlock { return p.fn(c, b) }

func swapProc(from, to string) processor {
	return yungProc{func(c *procCtx, b procBlock) *procBlock {
		if b.state.Name == from {
			b.state = BlockState{Name: to}
		}
		return &b
	}}
}

func legProc(marker string, out func(BlockState) BlockState, column func(rng *worldgen.JavaRandom) BlockState) processor {
	return yungProc{func(c *procCtx, b procBlock) *procBlock {
		if b.state.Name != marker {
			return &b
		}
		b.state = out(b.state)
		st := b.state
		next := func() BlockState { return st }
		if column != nil {
			rng := procRandom(b.world)
			next = func() BlockState { return column(rng) }
		}
		c.leg(b.world, next)
		return &b
	}}
}

func constState(name string) func(BlockState) BlockState {
	return func(BlockState) BlockState { return BlockState{Name: name} }
}

var stoneBrickSelector = &stateRandomizer{def: BlockState{Name: "minecraft:stone_bricks"}, entries: []struct {
	state BlockState
	p     float32
}{{BlockState{Name: "minecraft:mossy_stone_bricks"}, 0.5}, {BlockState{Name: "minecraft:cracked_stone_bricks"}, 0.2}}}

// portYung lets tests measure the unported baseline.
var portYung = true

// parseYungProcessor returns ok=false for processors not ported (pass-through, ≈).
func parseYungProcessor(typ string, raw json.RawMessage) (processor, bool) {
	if !portYung {
		return nil, false
	}
	ns, name := Namespaced(typ)
	switch name {
	case "block_replace_processor":
		var o struct {
			Target   json.RawMessage `json:"target_block"`
			Output   json.RawMessage `json:"output"`
			Facing   bool            `json:"randomize_facing"`
			Half     bool            `json:"randomize_half"`
			Copy     bool            `json:"copy_input_properties"`
			Waterlog bool            `json:"preserve_waterlog"`
		}
		if json.Unmarshal(raw, &o) != nil {
			return nil, false
		}
		out, ok := parseRandomizer(o.Output)
		if !ok || len(o.Target) == 0 {
			return nil, false
		}
		target := parseStateJSON(o.Target).Name
		return yungProc{func(c *procCtx, b procBlock) *procBlock {
			if b.state.Name != target {
				return &b
			}
			rng := procRandom(b.world)
			s := out.get(rng)
			in := b.state
			if o.Copy {
				for _, k := range []string{"facing", "half", "shape", "type", "north", "east", "south", "west", "up"} {
					if v, ok := in.Properties[k]; ok && hasProp(s, k) {
						s = withProps(s, map[string]string{k: v})
					}
				}
			}
			if o.Facing && hasProp(s, "facing") {
				s = withProps(s, map[string]string{"facing": [4]string{"north", "east", "south", "west"}[rng.NextInt(4)]})
			}
			if o.Half {
				if hasProp(s, "half") {
					s = withProps(s, map[string]string{"half": map[bool]string{true: "top", false: "bottom"}[rng.NextBoolean()]})
				}
				if strings.HasSuffix(s.Name, "_door") || s.Name == "minecraft:tall_grass" || s.Name == "minecraft:large_fern" {
					s = withProps(s, map[string]string{"half": map[bool]string{true: "upper", false: "lower"}[rng.NextBoolean()]})
				}
				if hasProp(s, "type") && strings.HasSuffix(s.Name, "_slab") {
					s = withProps(s, map[string]string{"type": map[bool]string{true: "top", false: "bottom"}[rng.NextBoolean()]})
				}
			}
			if o.Waterlog && hasProp(s, "waterlogged") && in.Properties["waterlogged"] == "true" {
				s = withProps(s, map[string]string{"waterlogged": "true"})
			}
			b.state = s
			return &b
		}}, true
	case "pillar_processor":
		var o struct {
			Target json.RawMessage `json:"target_block"`
			Out    json.RawMessage `json:"target_block_output"`
			Pillar json.RawMessage `json:"pillar_states"`
		}
		if json.Unmarshal(raw, &o) != nil || len(o.Target) == 0 {
			return nil, false
		}
		out, ok1 := parseRandomizer(o.Out)
		pil, ok2 := parseRandomizer(o.Pillar)
		if !ok1 || !ok2 {
			return nil, false
		}
		target := parseStateJSON(o.Target).Name
		return yungProc{func(c *procCtx, b procBlock) *procBlock {
			if b.state.Name != target {
				return &b
			}
			rng := procRandom(b.world)
			b.state = out.get(rng)
			c.leg(b.world, func() BlockState { return pil.get(rng) })
			return &b
		}}, true
	}
	switch ns + ":" + name {
	case "betterdeserttemples:red_wool_processor":
		return swapProc("minecraft:red_wool", "minecraft:tnt"), true
	case "betterdeserttemples:blue_concrete_processor", "betterjungletemples:blue_concrete_processor":
		return swapProc("minecraft:blue_concrete", "minecraft:blue_terracotta"), true
	case "betterdeserttemples:yellow_concrete_processor":
		return swapProc("minecraft:yellow_concrete", "minecraft:yellow_terracotta"), true
	case "betteroceanmonuments:sand_gravel_processor":
		return swapProc("minecraft:yellow_stained_glass", "minecraft:gravel"), true
	case "betterdungeons:small_nether_dungeon_lava_block_processor":
		return swapProc("minecraft:orange_wool", "minecraft:lava"), true
	case "betteroceanmonuments:air_processor":
		return yungProc{func(c *procCtx, b procBlock) *procBlock {
			if b.state.Name == "minecraft:air" && c.sea != 0 && b.world[1] < c.sea {
				b.state = BlockState{Name: "minecraft:water"}
			}
			return &b
		}}, true
	case "betteroceanmonuments:structure_void_processor", "betteroceanmonuments:waterlogged_processor_void":
		return yungProc{func(c *procCtx, b procBlock) *procBlock {
			if b.state.Name == "minecraft:purpur_slab" || b.state.Name == "minecraft:brown_wool" {
				s, ok := c.worldState(b.world)
				if !ok {
					return nil
				}
				if _, placed := c.existing(b.world); placed {
					return nil // already in the scene
				}
				b.state = s
				if s.Name == "minecraft:stone" {
					return nil // terrain stays; the preview draws terrain itself
				}
			}
			return &b
		}}, true
	case "betterdungeons:small_dungeon_cobblestone_processor", "betterdungeons:zombie_rot_processor":
		names := map[string]bool{"minecraft:cobblestone": true}
		if name == "zombie_rot_processor" {
			names["minecraft:cyan_terracotta"] = true
			names["minecraft:cobblestone_stairs"] = true
		}
		return yungProc{func(c *procCtx, b procBlock) *procBlock {
			if names[b.state.Name] {
				if s, ok := c.worldState(b.world); ok && isAirName(s.Name) {
					b.state = BlockState{Name: "minecraft:cave_air"}
				}
			}
			return &b
		}}, true
	case "betterdungeons:small_dungeon_ceiling_processor":
		return yungProc{func(c *procCtx, b procBlock) *procBlock {
			if b.state.Name != "minecraft:orange_stained_glass" {
				return &b
			}
			s, ok := c.worldState(b.world)
			if !ok {
				b.state = BlockState{Name: "minecraft:cobblestone"}
				return &b
			}
			if isFluidState(s) {
				b.state = BlockState{Name: "minecraft:cobblestone"}
				return &b
			}
			if _, placed := c.existing(b.world); placed || isAirName(s.Name) || s.Name == "minecraft:stone" {
				return nil
			}
			b.state = s
			return &b
		}}, true
	case "betterjungletemples:torch_processor", "betterdeserttemples:torch_processor":
		desert := ns == "betterdeserttemples"
		return yungProc{func(c *procCtx, b procBlock) *procBlock {
			if b.state.Name == "minecraft:torch" || b.state.Name == "minecraft:wall_torch" {
				f := procRandom(b.world).NextFloat()
				if (!desert && f < 0.9) || (desert && f > 0.05) {
					b.state = BlockState{Name: "minecraft:air"}
				}
			}
			return &b
		}}, true
	case "betterdungeons:small_dungeon_head_processor", "betterdungeons:small_dungeon_ore_processor":
		return nil, true // config-gated; default config places nothing
	case "betteroceanmonuments:leg_processor":
		return legProc("minecraft:blue_stained_glass", constState("minecraft:prismarine_bricks"), nil), true
	case "betterwitchhuts:leg_processor":
		return legProc("minecraft:brown_stained_glass", func(BlockState) BlockState {
			return BlockState{Name: "minecraft:oak_log", Properties: map[string]string{"axis": "y"}}
		}, nil), true
	case "betterwitchhuts:fence_leg_processor":
		return legProc("minecraft:crimson_fence", func(in BlockState) BlockState {
			return BlockState{Name: "minecraft:oak_fence", Properties: in.Properties}
		}, nil), true
	case "betterdungeons:small_nether_dungeon_leg_processor":
		return legProc("minecraft:orange_terracotta", constState("minecraft:nether_bricks"), nil), true
	case "betterdungeons:small_dungeon_leg_processor":
		return legProc("minecraft:yellow_stained_glass", constState("minecraft:mossy_cobblestone"), stoneBrickSelector.get), true
	}
	return nil, false
}
